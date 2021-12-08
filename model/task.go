package model

import (
	"database/sql"

	"github.com/wasabee-project/Wasabee-Server/log"
)

// TaskID
type TaskID string

type taskState uint8

// Task is the imported things for markers and links
type Task struct {
	ID           TaskID     `json:"ID"`
	Assignments  []GoogleID `json:"assignments"`
	DependsOn    []TaskID   `json:"dependsOn"`
	Zone         Zone       `json:"zone"`
	DeltaMinutes int32      `json:"deltaminutes"`
	State        string     `json:"state"`
	Comment      string     `json:"comment"`
	Order        uint16     `json:"order"`
	opID         OperationID
}

// enum for state values
const (
	taskStateUnassigned taskState = iota
	taskStateAssigned
	taskStateAcknowledged
	taskStateCompleted
)

func (t taskState) String() string {
	return [...]string{"Unassigned", "Assigned", "Acknowledged", "Completed"}[t]
}

func toTaskState(s string) taskState {
	switch s {
	case "pending":
		return taskStateUnassigned
	case "assigned":
		return taskStateAssigned
	case "acknowledged":
		return taskStateAcknowledged
	case "completed":
		return taskStateCompleted
	}
	return taskStateUnassigned
}

// add/remove depends
func (t *Task) AddDepend(task string) error {
	// sanity checks

	_, err := db.Exec("INSERT INTO depends (opID, taskID, dependsOn) VALUES (?, ?, ?)", t.opID, t.ID, task)
	if err != nil {
		log.Error(err)
		return err
	}
	return nil
}

func (t *Task) DelDepend(task string) error {
	// sanity checks

	_, err := db.Exec("DELETE FROM depends WHERE opID = ? AND taskID = ? AND dependsOn = ?", t.opID, t.ID, task)
	if err != nil {
		log.Error(err)
		return err
	}
	return nil
}

func (t *Task) Depends() ([]TaskID, error) {
	tmp := make([]TaskID, 0)

	rows, err := db.Query("SELECT dependsOn FROM depends WHERE opID = ? AND taskID = ? ORDER BY dependsOn", t.opID, t.ID)
	if err != nil {
		log.Error(err)
		return tmp, err
	}
	defer rows.Close()

	for rows.Next() {
		var tt TaskID
		err := rows.Scan(&t)
		if err != nil {
			log.Error(err)
			continue
		}
		log.Infow("task depends found", "taskID", t.ID, "dependsOn", tt)

		tmp = append(tmp, tt)
	}

	return tmp, nil
}

func (t *Task) GetAssignments() ([]GoogleID, error) {
	tmp := make([]GoogleID, 0)

	rows, err := db.Query("SELECT gid FROM assignments WHERE opID = ? AND taskID = ? ORDER BY gid", t.opID, t.ID)
	if err != nil {
		log.Error(err)
		return tmp, err
	}
	defer rows.Close()

	for rows.Next() {
		var g GoogleID
		err := rows.Scan(&g)
		if err != nil {
			log.Error(err)
			continue
		}
		log.Infow("task assignment found", "taskID", t.ID, "assigned to", g)

		tmp = append(tmp, g)
	}

	return tmp, nil
}

// Assign assigns a task to an agent
func (t *Task) Assign(gs []GoogleID) error {
	_, err := db.Exec("DELETE FROM assignments WHERE taskID = ? AND opID = ?", t.ID, t.opID)
	if err != nil {
		log.Error(err)
		return err
	}

	for _, gid := range gs {
		_, err := db.Exec("INSERT INTO assignments (opID, taskID, gid) VALUES  (?, ?, ?)", t.opID, t.ID, gid)
		if err != nil {
			log.Error(err)
			return err
		}
	}
	return nil
}

// AssignTX assigns a task to an agent using a given transaction
func (t *Task) AssignTX(gs []GoogleID, tx *sql.Tx) error {
	_, err := tx.Exec("DELETE FROM assignments WHERE taskID = ? AND opID = ?", t.ID, t.opID)
	if err != nil {
		log.Error(err)
		return err
	}

	for _, gid := range gs {
		_, err := tx.Exec("INSERT INTO assignments (opID, taskID, gid) VALUES  (?, ?, ?)", t.opID, t.ID, gid)
		if err != nil {
			log.Error(err)
			return err
		}
	}
	return nil
}

// IsAssignedTo checks to see if a task is assigned to a particular agent
func (t *Task) IsAssignedTo(gid GoogleID) bool {
	var x int

	err := db.QueryRow("SELECT COUNT(*) FROM assignments WHERE opID = ? AND taskID = ? AND gid = ?", t.opID, t.ID, gid).Scan(&x)
	if err != nil {
		log.Error(err)
		return false
	}
	return x == 1
}

func (t *Task) Claim(gid GoogleID) error {
	return nil
}

func (t *Task) Complete(gid GoogleID) error {
	return nil
}

func (t *Task) Incomplete(gid GoogleID) error {
	return nil
}

func (t *Task) Acknowledge(gid GoogleID) error {
	return nil
}

func (t *Task) Reject(gid GoogleID) error {
	return nil
}

// Delta sets the DeltaMinutes of a link in an operation
func (t *Task) SetDelta(delta int) error {
	_, err := db.Exec("UPDATE link SET delta = ? WHERE ID = ? and opID = ?", delta, t.ID, t.opID)
	if err != nil {
		log.Error(err)
	}
	return err
}

// Comment sets the comment on a task
func (t *Task) SetComment(desc string) error {
	_, err := db.Exec("UPDATE task SET comment = ? WHERE ID = ? AND opID = ?", MakeNullString(desc), t.ID, t.opID)
	if err != nil {
		log.Error(err)
		return err
	}
	return nil
}

// Zone updates the marker's zone
func (t *Task) SetZone(z Zone) error {
	if _, err := db.Exec("UPDATE marker SET zone = ? WHERE ID = ? AND opID = ?", z, t.ID, t.opID); err != nil {
		log.Error(err)
		return err
	}
	return nil
}
