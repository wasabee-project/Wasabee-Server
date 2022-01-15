package model

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/wasabee-project/Wasabee-Server/log"
	"github.com/wasabee-project/Wasabee-Server/messaging"
	"github.com/wasabee-project/Wasabee-Server/util"
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
	ThrowOrder int16    `json:"throwOrderPos"` // deprecated, use Order from Task
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
		l.Order = int16(l.ThrowOrder)
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

	comment := makeNullString(util.Sanitize(l.Comment))

	_, err := db.Exec("REPLACE INTO task (ID, opID, comment, taskorder, state, zone, delta) VALUES (?, ?, ?, ?, ?, ?, ?)",
		l.ID, opID, comment, l.Order, l.State, l.Zone, l.DeltaMinutes)
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

	// clears if none set
	if err := l.SetAssignments(l.Assignments, nil); err != nil {
		log.Error(err)
		return err
	}

	// do not clear if old client (yet)
	if len(l.DependsOn) > 0 {
		err = l.SetDepends(l.DependsOn, nil)
		if err != nil {
			log.Error(err)
			return err
		}
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

	// copy these values down
	l.Task.ID = TaskID(l.ID)
	l.opID = opID

	if !l.Zone.Valid() || l.Zone == ZoneAll {
		l.Zone = zonePrimary
	}

	// use the old if it is set
	if l.Desc != "" {
		l.Comment = l.Desc
	}
	if l.ThrowOrder != 0 {
		l.Order = int16(l.ThrowOrder)
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

	comment := makeNullString(util.Sanitize(l.Comment))

	_, err := tx.Exec("REPLACE INTO task (ID, opID, comment, taskorder, state, zone, delta) VALUES (?, ?, ?, ?, ?, ?, ?)",
		l.ID, opID, comment, l.Order, l.State, l.Zone, l.DeltaMinutes)
	if err != nil {
		log.Error(err)
		return err
	}

	_, err = tx.Exec("REPLACE INTO link (ID, opID, fromPortalID, toPortalID, color, mu) VALUES (?, ?, ?, ?, ?, ?)", l.ID, opID, l.From, l.To, l.Color, l.MuCaptured)
	if err != nil {
		log.Error(err)
		return err
	}

	// only update assignments if Changed bit is set -- don't flood messages if nothing changed
	if l.Changed {
		// empty assignments clears them
		if err := l.SetAssignments(l.Assignments, tx); err != nil {
			log.Error(err)
			return err
		}
		for _, g := range l.Assignments {
			messaging.SendAssignment(messaging.GoogleID(g), messaging.TaskID(l.ID), messaging.OperationID(opID), "assigned")
		}
	}

	// do not clear if they used an old client
	if len(l.DependsOn) > 0 {
		if err := l.SetDepends(l.DependsOn, tx); err != nil {
			log.Error(err)
			return err
		}
	}

	return nil
}

// PopulateLinks fills in the Links list for the Operation.
func (o *Operation) populateLinks(zones []Zone, inGid GoogleID, assignments map[TaskID][]GoogleID, depends map[TaskID][]TaskID) error {
	var description sql.NullString

	rows, err := db.Query("SELECT link.ID, link.fromPortalID, link.toPortalID, task.comment, task.taskorder, task.state, link.color, task.zone, task.delta FROM link JOIN task ON link.ID = task.ID WHERE task.opID = ? AND link.opID = task.opID", o.ID)
	if err != nil {
		log.Error(err)
		return err
	}
	defer rows.Close()

	for rows.Next() {
		tmpLink := Link{}
		tmpLink.opID = o.ID

		err := rows.Scan(&tmpLink.ID, &tmpLink.From, &tmpLink.To, &description, &tmpLink.Order, &tmpLink.State, &tmpLink.Color, &tmpLink.Zone, &tmpLink.DeltaMinutes)
		if err != nil {
			log.Error(err)
			continue
		}
		tmpLink.Task.ID = TaskID(tmpLink.ID)

		if description.Valid {
			tmpLink.Desc = description.String
			tmpLink.Comment = description.String
		}

		tmpLink.ThrowOrder = tmpLink.Order

		if a, ok := assignments[tmpLink.Task.ID]; ok {
			tmpLink.Assignments = a
			tmpLink.AssignedTo = a[0]
		}

		if d, ok := depends[tmpLink.Task.ID]; ok {
			tmpLink.DependsOn = d
		}

		if tmpLink.State == "completed" {
			tmpLink.Completed = true
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

// SetColor changes the color of a link in an operation
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
		err := fmt.Errorf(ErrGetLinkUnpopulated)
		log.Error(err)
		return &Link{}, err
	}

	for _, l := range o.Links {
		if l.ID == linkID {
			return &l, nil
		}
	}

	return &Link{}, fmt.Errorf(ErrLinkNotFound)
}
