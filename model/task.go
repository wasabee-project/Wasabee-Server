package model

import (
	"database/sql"
	"fmt"

	"github.com/wasabee-project/Wasabee-Server/log"
)

// UnspecifiedTask is the type for tasks which could be either markers or links
type UnspecifiedTask interface {
	Claim(GoogleID) error
	Reject(GoogleID) error
	SetOrder(int16) error
	GetOrder() int16
	IsAssignedTo(GoogleID) bool
	Acknowledge() error
}

// TaskID is the basic type for a task identifier
type TaskID string

// Task is the imported things for markers and links
type Task struct {
	ID           TaskID     `json:"task"`
	Assignments  []GoogleID `json:"assignments"`
	DependsOn    []TaskID   `json:"dependsOn"`
	Zone         Zone       `json:"zone"`
	DeltaMinutes int32      `json:"deltaminutes"`
	State        string     `json:"state"`
	Comment      string     `json:"comment"`
	Order        int16      `json:"order"`
	opID         OperationID
}

// AddDepend add a single task dependency
func (t *Task) AddDepend(task TaskID) error {
	_, err := db.Exec("INSERT INTO depends (opID, taskID, dependsOn) VALUES (?, ?, ?)", t.opID, t.ID, task)
	if err != nil {
		log.Error(err)
		return err
	}
	return nil
}

// SetDepends overwrites a task's dependencies, if tx is null, one is created
func (t *Task) SetDepends(d []TaskID, tx *sql.Tx) error {
	if len(d) < 1 {
		return nil
	}

	needtx := false
	if tx == nil {
		needtx = true
		tx, _ = db.Begin()

		defer func() {
			if err := tx.Rollback(); err != nil && err != sql.ErrTxDone {
				log.Error(err)
			}
		}()
	}

	if _, err := tx.Exec("DELETE FROM depends WHERE opID = ? AND taskID = ?", t.opID, t.ID); err != nil {
		log.Error(err)
		return err
	}

	// we could just blit them all at once
	for _, depend := range d {
		if _, err := tx.Exec("INSERT INTO depends (opID, taskID, dependsOn) VALUES (?, ?, ?)", t.opID, t.ID, depend); err != nil {
			log.Error(err)
			return err
		}
	}

	if needtx {
		if err := tx.Commit(); err != nil {
			log.Error(err)
			return err
		}
	}
	return nil
}

// DelDepend deletes all dependencies for a task
func (t *Task) DelDepend(task TaskID) error {
	_, err := db.Exec("DELETE FROM depends WHERE opID = ? AND taskID = ? AND dependsOn = ?", t.opID, t.ID, task)
	if err != nil {
		log.Error(err)
		return err
	}
	return nil
}

// dependsPrecache -- used to save queries in op.Populate
func (o OperationID) dependsPrecache() (map[TaskID][]TaskID, error) {
	buf := make(map[TaskID][]TaskID)

	rows, err := db.Query("SELECT taskID, dependsOn FROM depends WHERE opID = ?", o)
	if err != nil {
		log.Error(err)
		return buf, err
	}
	defer rows.Close()

	var t, d TaskID
	for rows.Next() {
		if err := rows.Scan(&t, &d); err != nil {
			log.Error(err)
			continue
		}
		_, ok := buf[t]
		if !ok {
			tmp := make([]TaskID, 0)
			tmp = append(tmp, d)
			buf[t] = tmp
		} else {
			buf[t] = append(buf[t], d)
		}
	}
	return buf, nil
}

// get all dependencies for a task
/* func (t *Task) getDepends() ([]TaskID, error) {
	tmp := make([]TaskID, 0)

	rows, err := db.Query("SELECT dependsOn FROM depends WHERE opID = ? AND taskID = ?", t.opID, t.ID)
	if err != nil {
		log.Error(err)
		return tmp, err
	}
	defer rows.Close()

	for rows.Next() {
		var depend TaskID
		if err := rows.Scan(&depend); err != nil {
			log.Error(err)
			continue
		}
		tmp = append(tmp, depend)
	}

	return tmp, nil
} */

// GetAssignments gets all assignments for a task
func (t *Task) GetAssignments() ([]GoogleID, error) {
	tmp := make([]GoogleID, 0)

	if t.ID == "" {
		return tmp, nil
	}

	rows, err := db.Query("SELECT DISTINCT gid FROM assignments WHERE opID = ? AND taskID = ?", t.opID, t.ID)
	if err != nil {
		log.Error(err)
		return tmp, err
	}
	defer rows.Close()

	var g GoogleID
	for rows.Next() {
		if err := rows.Scan(&g); err != nil {
			log.Error(err)
			continue
		}
		tmp = append(tmp, g)
	}

	return tmp, nil
}

// assignmentsPrecache is used by op.Populate to reduce the number of queries
func (o OperationID) assignmentPrecache() (map[TaskID][]GoogleID, error) {
	buf := make(map[TaskID][]GoogleID)

	rows, err := db.Query("SELECT DISTINCT taskID, gid FROM assignments WHERE opID = ?", o)
	if err != nil {
		log.Error(err)
		return buf, err
	}
	defer rows.Close()

	var g GoogleID
	var t TaskID
	for rows.Next() {
		if err := rows.Scan(&t, &g); err != nil {
			log.Error(err)
			continue
		}
		_, ok := buf[t]
		if !ok {
			tmp := make([]GoogleID, 0)
			tmp = append(tmp, g)
			buf[t] = tmp
		} else {
			buf[t] = append(buf[t], g)
		}
	}
	return buf, nil
}

// SetAssignments assigns a task to an agent using a given transaction, if the transaction is nil, one is created for this block
func (t *Task) SetAssignments(gs []GoogleID, tx *sql.Tx) error {
	needtx := false
	if tx == nil {
		needtx = true
		tx, _ = db.Begin()

		defer func() {
			if err := tx.Rollback(); err != nil && err != sql.ErrTxDone {
				log.Error(err)
			}
		}()
	}

	// we could be smarter and load the existing, then only add new, but this is fast and easy
	if err := t.ClearAssignments(tx); err != nil {
		log.Error(err)
		return err
	}

	if len(gs) > 0 {
		// remove any duplicates
		deduped := make(map[GoogleID]bool)
		for _, gid := range gs {
			deduped[gid] = true
		}

		for gid := range deduped {
			if gid == "" {
				continue
			}
			_, err := tx.Exec("INSERT INTO assignments (opID, taskID, gid) VALUES (?, ?, ?)", t.opID, t.ID, gid)
			if err != nil {
				log.Error(err)
				return err
			}
		}

		if _, err := tx.Exec("UPDATE task SET state = 'assigned' WHERE ID = ? AND opID = ?", t.ID, t.opID); err != nil {
			log.Error(err)
			return err
		}
	}

	if needtx {
		if err := tx.Commit(); err != nil {
			log.Error(err)
			return err
		}
	}

	return nil
}

// ClearAssignments removes any assignments for this task from the database
func (t Task) ClearAssignments(tx *sql.Tx) error {
	if _, err := tx.Exec("DELETE FROM assignments WHERE taskID = ? AND opID = ?", t.ID, t.opID); err != nil {
		log.Error(err)
		return err
	}
	if _, err := tx.Exec("UPDATE task SET state = 'pending' WHERE ID = ? AND opID = ?", t.ID, t.opID); err != nil {
		log.Error(err)
		return err
	}
	return nil
}

// IsAssignedTo checks to see if a task is assigned to a particular agent
func (t Task) IsAssignedTo(gid GoogleID) bool {
	var x int

	err := db.QueryRow("SELECT COUNT(*) FROM assignments WHERE opID = ? AND taskID = ? AND gid = ?", t.opID, t.ID, gid).Scan(&x)
	if err != nil {
		log.Error(err)
		return false
	}
	return x == 1
}

// Claim assignes a task to the calling agent
func (t Task) Claim(gid GoogleID) error {
	if _, err := db.Exec("INSERT INTO assignments (opID, taskID, gid) VALUES (?,?,?)", t.opID, t.ID, gid); err != nil {
		log.Error(err)
		return err
	}
	if _, err := db.Exec("UPDATE task SET state = 'acknowledged' WHERE ID = ? AND opID = ?", t.ID, t.opID); err != nil {
		log.Error(err)
		return err
	}
	return nil
}

// Complete marks as task as completed
func (t Task) Complete() error {
	if _, err := db.Exec("UPDATE task SET state = 'completed' WHERE ID = ? AND opID = ?", t.ID, t.opID); err != nil {
		log.Error(err)
		return err
	}
	return nil
}

// Incomplete marks a task as not completed
func (t Task) Incomplete() error {
	if _, err := db.Exec("UPDATE task SET state = 'assigned' WHERE ID = ? AND opID = ?", t.ID, t.opID); err != nil {
		log.Error(err)
		return err
	}
	return nil
}

// Acknowledge marks a task as acknowledged
func (t Task) Acknowledge() error {
	if _, err := db.Exec("UPDATE task SET state = 'acknowledged' WHERE ID = ? AND opID = ?", t.ID, t.opID); err != nil {
		log.Error(err)
		return err
	}
	return nil
}

// Reject unassignes an agent from a task
func (t Task) Reject(gid GoogleID) error {
	if _, err := db.Exec("UPDATE task SET state = 'pending' WHERE ID = ? AND opID = ?", t.ID, t.opID); err != nil {
		log.Error(err)
		return err
	}
	if _, err := db.Exec("DELETE FROM assignments WHERE opID = ? AND taskID = ? AND gid = ?", t.opID, t.ID, gid); err != nil {
		log.Error(err)
		return err
	}
	return nil
}

// SetDelta sets the DeltaMinutes of a link in an operation
func (t Task) SetDelta(delta int) error {
	_, err := db.Exec("UPDATE link SET delta = ? WHERE ID = ? and opID = ?", delta, t.ID, t.opID)
	if err != nil {
		log.Error(err)
	}
	return err
}

// SetComment sets the comment on a task
func (t Task) SetComment(desc string) error {
	_, err := db.Exec("UPDATE task SET comment = ? WHERE ID = ? AND opID = ?", MakeNullString(desc), t.ID, t.opID)
	if err != nil {
		log.Error(err)
		return err
	}
	return nil
}

// SetZone updates the task's zone
func (t Task) SetZone(z Zone) error {
	if _, err := db.Exec("UPDATE task SET zone = ? WHERE ID = ? AND opID = ?", z, t.ID, t.opID); err != nil {
		log.Error(err)
		return err
	}
	return nil
}

// SetOrder updates the task'sorder
func (t Task) SetOrder(order int16) error {
	if _, err := db.Exec("UPDATE task SET order = ? WHERE ID = ? AND opID = ?", order, t.ID, t.opID); err != nil {
		log.Error(err)
		return err
	}
	return nil
}

// GetOrder returns a tasks order
func (t Task) GetOrder() int16 {
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

	return &Task{}, fmt.Errorf(ErrTaskNotFound)
}

// GetTaskByStepNumber returns a task based on it's operation position
// if multiple tasks share one step number, the results are non-deterministic
func (o *Operation) GetTaskByStepNumber(step int16) (UnspecifiedTask, error) {
	for _, m := range o.Markers {
		if m.Order == step {
			return m, nil
		}
	}

	for _, l := range o.Links {
		if l.Order == step {
			return l, nil
		}
	}
	return Task{}, fmt.Errorf(ErrTaskNotFound)
}

// checkAssignments validates that assignments are made to agents on teams -- uses the precache
func (t *Task) checkAssignments(agentMap map[GoogleID]bool) {
	var new []GoogleID

	for _, gid := range t.Assignments {
		if _, ok := agentMap[gid]; ok {
			new = append(new, gid)
		} else {
			log.Warnw("removing assignment to agent no on any teams", "task", t.ID, "op", t.opID, "gid", gid)
		}
	}

	t.Assignments = new
}
