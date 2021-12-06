package model

import (
	"github.com/wasabee-project/Wasabee-Server/log"
)

// TaskID
type TaskID string

// Task is the imported things for markers and links
type Task struct {
	ID   TaskID `json:"ID"`
	opID OperationID
}

/* taskI is the basic task type interface
type taskI interface {
	Reject(*Operation, GoogleID) (string, error)
	Claim(*Operation, GoogleID) (string, error)
	Assign(*Operation, GoogleID) (string, error)
	Acknowledge(*Operation, GoogleID) (string, error)
	Complete(*Operation, GoogleID) (string, error)
	Incomplete(*Operation, GoogleID) (string, error)
	Comment(*Operation, string) (string, error)
	Zone(*Operation, Zone) (string, error)
	Delta(*Operation, int) (string, error)
} */

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
		log.Infow("task depends found", "taskID", tt, "dependsOn", tt)

		tmp = append(tmp, tt)
	}

	return tmp, nil
}
