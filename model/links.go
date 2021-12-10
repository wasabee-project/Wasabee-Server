package model

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/wasabee-project/Wasabee-Server/log"
	"github.com/wasabee-project/Wasabee-Server/messaging"
)

// LinkID wrapper to ensure type safety
type LinkID string

// Link is the basic link data structure
type Link struct {
	ID         LinkID   `json:"ID"`
	From       PortalID `json:"fromPortalId"`
	To         PortalID `json:"toPortalId"`
	Desc       string   `json:"description"`   // deprecated, use Comment from Task
	AssignedTo GoogleID `json:"assignedTo"`    // deprecated, use Assignments from Task
	ThrowOrder int32    `json:"throwOrderPos"` // deprecated, use Order from Task
	Completed  bool     `json:"completed"`     // deprecated, use State from Task
	Color      string   `json:"color"`
	MuCaptured int      `json:"mu"`
	Changed    bool     `json:"changed,omitempty"`
	Task
}

// TODO use the logic from insertZone to unify insertLink and updateLink

// insertLink adds a link to the database
func (opID OperationID) insertLink(l Link) error {
	if l.To == l.From {
		log.Infow("source and destination the same, ignoring link", "resource", opID)
		return nil
	}

	if !l.Zone.Valid() || l.Zone == ZoneAll {
		l.Zone = zonePrimary
	}

	// use the old if it is set
	if l.Desc != "" {
		l.Comment = l.Desc
	}
	if l.ThrowOrder != 0 {
		l.Order = uint16(l.ThrowOrder)
	}
	if l.AssignedTo != "" {
		l.Assignments = append(l.Assignments, l.AssignedTo)
	}

	if l.State == "" {
		l.State = "pending"
	}

	if l.Completed {
		l.State = "completed"
	}

	_, err := db.Exec("REPLACE INTO task (ID, opID, comment, taskorder, state, zone, delta) VALUES (?, ?, ?, ?, ?, ?, ?)",
		l.ID, opID, MakeNullString(l.Comment), l.Order, l.State, l.Zone, l.DeltaMinutes)
	if err != nil {
		log.Error(err)
		return err
	}

	_, err = db.Exec("REPLACE INTO link (ID, opID, fromPortalID, toPortalID, color, mu) VALUES (?, ?, ?, ?, ?, ?)",
		l.ID, opID, l.From, l.To, l.Color, l.MuCaptured)
	if err != nil {
		log.Error(err)
		return err
	}

	err = l.Assign(l.Assignments, nil)
	if err != nil {
		log.Error(err)
		return err
	}

	return nil
}

func (opID OperationID) deleteLink(lid LinkID, tx *sql.Tx) error {
	_, err := tx.Exec("DELETE FROM link WHERE OpID = ? and ID = ?", opID, lid)
	if err != nil {
		log.Error(err)
		return err
	}
	return nil
}

func (opID OperationID) updateLink(l Link, tx *sql.Tx) error {
	if l.To == l.From {
		log.Infow("source and destination the same, ignoring link", "resource", opID)
		return nil
	}

	if !l.Zone.Valid() || l.Zone == ZoneAll {
		l.Zone = zonePrimary
	}

	// use the old if it is set
	if l.Desc != "" {
		l.Comment = l.Desc
	}
	if l.ThrowOrder != 0 {
		l.Order = uint16(l.ThrowOrder)
	}
	if l.AssignedTo != "" {
		l.Assignments = append(l.Assignments, l.AssignedTo)
	}
	if l.State == "" {
		l.State = "pending"
	}
	if l.Completed {
		l.State = "completed"
	}

	_, err := tx.Exec("REPLACE INTO task (ID, opID, comment, taskorder, state, zone, delta) VALUES (?, ?, ?, ?, ?, ?, ?)",
		l.ID, opID, MakeNullString(l.Comment), l.Order, l.State, l.Zone, l.DeltaMinutes)
	if err != nil {
		log.Error(err)
		return err
	}

	_, err = tx.Exec("REPLACE INTO link (ID, opID, fromPortalID, toPortalID, color, mu) VALUES (?, ?, ?, ?, ?, ?)", l.ID, opID, l.From, l.To, l.Color, l.MuCaptured)
	if err != nil {
		log.Error(err)
		return err
	}

	if l.Changed && len(l.Assignments) > 0 {
		messaging.SendAssignment(messaging.GoogleID(l.AssignedTo), messaging.TaskID(l.ID), messaging.OperationID(opID), "assigned")
		err := l.Assign(l.Assignments, tx)
		if err != nil {
			log.Error(err)
			return err
		}
	}

	return nil
}

// PopulateLinks fills in the Links list for the Operation.
func (o *Operation) populateLinks(zones []Zone, inGid GoogleID) error {
	var tmpLink Link
	tmpLink.opID = o.ID

	var description sql.NullString

	rows, err := db.Query("SELECT link.ID, link.fromPortalID, link.toPortalID, task.comment, task.taskorder, task.state, link.color, task.zone, task.delta FROM link JOIN task ON link.ID = task.ID WHERE task.opID = ? ORDER BY task.taskorder", o.ID)
	if err != nil {
		log.Error(err)
		return err
	}
	defer rows.Close()
	for rows.Next() {
		err := rows.Scan(&tmpLink.ID, &tmpLink.From, &tmpLink.To, &description, &tmpLink.Order, &tmpLink.State, &tmpLink.Color, &tmpLink.Zone, &tmpLink.DeltaMinutes)
		if err != nil {
			log.Error(err)
			continue
		}
		tmpLink.Task.ID = TaskID(tmpLink.ID)

		if description.Valid {
			tmpLink.Desc = description.String
			tmpLink.Comment = description.String
		} else {
			tmpLink.Desc = ""
		}

		tmpLink.Assignments, err = tmpLink.GetAssignments()
		if err != nil {
			log.Error(err)
			continue
		}
		if len(tmpLink.Assignments) > 0 {
			// log.Debugw("link assignment", "taskID", tmpLink.TaskID, "assignments", tmpLink.Assignments)
			tmpLink.AssignedTo = tmpLink.Assignments[0]
		} else {
			tmpLink.AssignedTo = ""
		}

		// this isn't in a zone with which we are concerned AND not assigned to me, skip
		if !tmpLink.Zone.inZones(zones) && !tmpLink.IsAssignedTo(inGid) {
			continue
		}
		o.Links = append(o.Links, tmpLink)
	}
	return nil
}

// String returns the string version of a LinkID
func (l LinkID) String() string {
	return string(l)
}

// LinkOrder changes the order of the throws for an operation
func (o *Operation) LinkOrder(order string) error {
	stmt, err := db.Prepare("UPDATE link SET throworder = ? WHERE opID = ? AND ID = ?")
	if err != nil {
		log.Error(err)
		return err
	}

	pos := 1
	links := strings.Split(order, ",")
	for i := range links {
		if links[i] == "000" { // the header, could be anyplace in the order if the user was being silly
			continue
		}
		if _, err := stmt.Exec(pos, o.ID, links[i]); err != nil {
			log.Error(err)
			continue
		}
		pos++
	}
	return err
}

// LinkColor changes the color of a link in an operation
func (l *Link) SetColor(color string) error {
	_, err := db.Exec("UPDATE link SET color = ? WHERE ID = ? and opID = ?", color, l.ID, l.opID)
	if err != nil {
		log.Error(err)
	}
	return err
}

// Swap changes the direction of a link in an operation
func (l *Link) Swap() error {
	var tmpLink Link

	err := db.QueryRow("SELECT fromPortalID, toPortalID FROM link WHERE opID = ? AND ID = ?", l.opID, l.ID).Scan(&tmpLink.From, &tmpLink.To)
	if err != nil {
		log.Error(err)
		return err
	}

	_, err = db.Exec("UPDATE link SET fromPortalID = ?, toPortalID = ? WHERE ID = ? and opID = ?", tmpLink.To, tmpLink.From, l.ID, l.opID)
	if err != nil {
		log.Error(err)
		return err
	}
	return nil
}

// GetLink looks up and returns a populated Link from an id
func (o *Operation) GetLink(linkID LinkID) (*Link, error) {
	if len(o.Links) == 0 { // XXX not a good test, not all ops have links
		err := fmt.Errorf("Attempt to use GetLink on unpopulated *Operation")
		log.Error(err)
		return &Link{}, err
	}

	for _, l := range o.Links {
		if l.ID == linkID {
			return &l, nil
		}
	}

	return &Link{}, fmt.Errorf("link not found")
}
