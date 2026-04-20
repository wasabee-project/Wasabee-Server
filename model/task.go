package model

import (
	"context"
	"database/sql"
	"errors"

	"github.com/wasabee-project/Wasabee-Server/log"
	"github.com/wasabee-project/Wasabee-Server/messaging"
	"github.com/wasabee-project/Wasabee-Server/util"
)

// UnspecifiedTask is the type for tasks which could be either markers or links
type UnspecifiedTask interface {
	Claim(context.Context, GoogleID) error
	Reject(context.Context, GoogleID) error
	SetOrder(context.Context, int16) error
	GetOrder() int16
	IsAssignedTo(context.Context, GoogleID) bool
	Acknowledge(context.Context) error
}

// TaskID is the basic type for a task identifier
type TaskID string

// Task is the imported things for markers and links
type Task struct {
	ID           TaskID `json:"task"`
	State        string `json:"state"`
	Comment      string `json:"comment"`
	opID         OperationID
	Assignments  []GoogleID `json:"assignments"`
	DependsOn    []TaskID   `json:"dependsOn"`
	Zone         ZoneID     `json:"zone"`
	DeltaMinutes int32      `json:"deltaminutes"`
	Order        int16      `json:"order"`
}

// AddDepend add a single task dependency
func (t *Task) AddDepend(ctx context.Context, task TaskID) error {
	_, err := db.ExecContext(ctx, "INSERT INTO depends (opID, taskID, dependsOn) VALUES (?, ?, ?)", t.opID, t.ID, task)
	if err != nil {
		log.Error(err)
	}
	return err
}

// SetDepends overwrites a task's dependencies
func (t *Task) SetDepends(ctx context.Context, d []TaskID, tx *sql.Tx) error {
	// If no transaction provided, we assume the caller wanted a one-off or we shouldn't be here
	// But staying consistent with your pattern of optional TX:
	executor := txExecutor(tx)

	if _, err := executor.ExecContext(ctx, "DELETE FROM depends WHERE opID = ? AND taskID = ?", t.opID, t.ID); err != nil {
		return err
	}

	for _, depend := range d {
		if _, err := executor.ExecContext(ctx, "INSERT INTO depends (opID, taskID, dependsOn) VALUES (?, ?, ?)", t.opID, t.ID, depend); err != nil {
			return err
		}
	}
	return nil
}

// DelDepend deletes all dependencies for a task
func (t *Task) DelDepend(ctx context.Context, task TaskID) error {
	_, err := db.ExecContext(ctx, "DELETE FROM depends WHERE opID = ? AND taskID = ? AND dependsOn = ?", t.opID, t.ID, task)
	return err
}

// dependsPrecache -- used to save queries in op.Populate
func (o OperationID) dependsPrecache(ctx context.Context) (map[TaskID][]TaskID, error) {
	buf := make(map[TaskID][]TaskID)

	rows, err := db.QueryContext(ctx, "SELECT taskID, dependsOn FROM depends WHERE opID = ?", o)
	if err != nil {
		return buf, err
	}
	defer rows.Close()

	for rows.Next() {
		var t, d TaskID
		if err := rows.Scan(&t, &d); err != nil {
			continue
		}
		buf[t] = append(buf[t], d)
	}
	return buf, nil
}

// GetAssignments gets all assignments for a task
func (t *Task) GetAssignments(ctx context.Context, tx *sql.Tx) ([]GoogleID, error) {
	tmp := make([]GoogleID, 0)
	if t.ID == "" {
		return tmp, nil
	}

	executor := txExecutor(tx)
	rows, err := executor.QueryContext(ctx, "SELECT DISTINCT gid FROM assignments WHERE opID = ? AND taskID = ?", t.opID, t.ID)
	if err != nil {
		return tmp, err
	}
	defer rows.Close()

	for rows.Next() {
		var g GoogleID
		if err := rows.Scan(&g); err != nil {
			continue
		}
		tmp = append(tmp, g)
	}
	return tmp, nil
}

// assignmentPrecache is used by op.Populate to reduce the number of queries
func (o OperationID) assignmentPrecache(ctx context.Context) (map[TaskID][]GoogleID, error) {
	buf := make(map[TaskID][]GoogleID)

	rows, err := db.QueryContext(ctx, "SELECT DISTINCT taskID, gid FROM assignments WHERE opID = ?", o)
	if err != nil {
		return buf, err
	}
	defer rows.Close()

	for rows.Next() {
		var g GoogleID
		var t TaskID
		if err := rows.Scan(&t, &g); err != nil {
			continue
		}
		buf[t] = append(buf[t], g)
	}
	return buf, nil
}

// SetAssignments assigns a task to agents
func (t *Task) SetAssignments(ctx context.Context, gs []GoogleID, tx *sql.Tx) error {
	var currentTx *sql.Tx
	var err error

	if tx == nil {
		currentTx, err = db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		defer currentTx.Rollback()
	} else {
		currentTx = tx
	}

	b, err := t.GetAssignments(ctx, currentTx)
	if err != nil {
		return err
	}

	before := make(map[GoogleID]bool)
	for _, gid := range b {
		before[gid] = true
	}

	if len(gs) > 0 {
		deduped := make(map[GoogleID]bool)
		for _, gid := range gs {
			if gid != "" {
				deduped[gid] = true
			}
		}

		for gid := range deduped {
			if before[gid] {
				delete(before, gid)
			} else {
				if _, err := currentTx.ExecContext(ctx, "REPLACE INTO assignments (opID, taskID, gid) VALUES (?, ?, ?)", t.opID, t.ID, gid); err != nil {
					return err
				}
				messaging.SendAssignment(ctx, messaging.GoogleID(gid), messaging.TaskID(t.ID), messaging.OperationID(t.opID), "assigned")
			}
		}

		for gid := range before {
			if _, err := currentTx.ExecContext(ctx, "DELETE FROM assignments WHERE opID = ? AND taskID = ? AND gid = ?", t.opID, t.ID, gid); err != nil {
				return err
			}
		}
	} else if len(before) > 0 {
		if err := t.ClearAssignments(ctx, currentTx); err != nil {
			return err
		}
	}

	if tx == nil {
		return currentTx.Commit()
	}
	return nil
}

// ClearAssignments removes any assignments for this task
func (t *Task) ClearAssignments(ctx context.Context, tx *sql.Tx) error {
	executor := txExecutor(tx)
	if _, err := executor.ExecContext(ctx, "DELETE FROM assignments WHERE taskID = ? AND opID = ?", t.ID, t.opID); err != nil {
		return err
	}

	_, err := executor.ExecContext(ctx, "UPDATE task SET state = 'pending' WHERE ID = ? AND opID = ? AND state != 'completed'", t.ID, t.opID)
	return err
}

// IsAssignedTo checks to see if a task is assigned to a particular agent
func (t *Task) IsAssignedTo(ctx context.Context, gid GoogleID) bool {
	var x int
	err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM assignments WHERE opID = ? AND taskID = ? AND gid = ?", t.opID, t.ID, gid).Scan(&x)
	return err == nil && x == 1
}

// Claim assigns a task to the calling agent
func (t *Task) Claim(ctx context.Context, gid GoogleID) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, "INSERT IGNORE INTO assignments (opID, taskID, gid) VALUES (?,?,?)", t.opID, t.ID, gid); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "UPDATE task SET state = 'acknowledged' WHERE ID = ? AND opID = ?", t.ID, t.opID); err != nil {
		return err
	}
	return tx.Commit()
}

// Complete marks as task as completed
func (t *Task) Complete(ctx context.Context) error {
	_, err := db.ExecContext(ctx, "UPDATE task SET state = 'completed' WHERE ID = ? AND opID = ?", t.ID, t.opID)
	return err
}

// Incomplete marks a task as not completed
func (t *Task) Incomplete(ctx context.Context) error {
	_, err := db.ExecContext(ctx, "UPDATE task SET state = 'assigned' WHERE ID = ? AND opID = ?", t.ID, t.opID)
	return err
}

// Acknowledge marks a task as acknowledged
func (t *Task) Acknowledge(ctx context.Context) error {
	_, err := db.ExecContext(ctx, "UPDATE task SET state = 'acknowledged' WHERE ID = ? AND opID = ?", t.ID, t.opID)
	return err
}

// Reject unassigns an agent from a task
func (t *Task) Reject(ctx context.Context, gid GoogleID) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, "UPDATE task SET state = 'pending' WHERE ID = ? AND opID = ?", t.ID, t.opID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM assignments WHERE opID = ? AND taskID = ? AND gid = ?", t.opID, t.ID, gid); err != nil {
		return err
	}
	return tx.Commit()
}

// SetDelta sets the DeltaMinutes of a link
func (t *Task) SetDelta(ctx context.Context, delta int) error {
	_, err := db.ExecContext(ctx, "UPDATE task SET delta = ? WHERE ID = ? and opID = ?", delta, t.ID, t.opID)
	return err
}

// SetComment sets the comment on a task
func (t *Task) SetComment(ctx context.Context, comment string) error {
	desc := makeNullString(util.Sanitize(comment))
	_, err := db.ExecContext(ctx, "UPDATE task SET comment = ? WHERE ID = ? AND opID = ?", desc, t.ID, t.opID)
	return err
}

// SetZone updates the task's zone
func (t *Task) SetZone(ctx context.Context, z ZoneID) error {
	_, err := db.ExecContext(ctx, "UPDATE task SET zone = ? WHERE ID = ? AND opID = ?", z, t.ID, t.opID)
	return err
}

// SetOrder updates the task's order
func (t *Task) SetOrder(ctx context.Context, order int16) error {
	_, err := db.ExecContext(ctx, "UPDATE task SET taskorder = ? WHERE ID = ? AND opID = ?", order, t.ID, t.opID)
	return err
}

// GetOrder returns a tasks order
func (t *Task) GetOrder() int16 {
	return t.Order
}

// GetTask looks up and returns a populated Task from an id
func (o *Operation) GetTask(taskID TaskID) (*Task, error) {
	for _, m := range o.Markers {
		if m.Task.ID == taskID {
			return &m.Task, nil
		}
	}

	for _, l := range o.Links {
		if l.Task.ID == taskID {
			return &l.Task, nil
		}
	}

	return &Task{}, errors.New(ErrTaskNotFound)
}

// GetTaskByStepNumber returns a task based on it's operation position
// if multiple tasks share one step number, the results are non-deterministic
func (o *Operation) GetTaskByStepNumber(step int16) (UnspecifiedTask, error) {
	for _, m := range o.Markers {
		if m.Order == step {
			return &m, nil
		}
	}

	for _, l := range o.Links {
		if l.Order == step {
			return &l, nil
		}
	}
	return &Task{}, errors.New(ErrTaskNotFound)
}
