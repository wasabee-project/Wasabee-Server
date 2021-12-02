package model

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/wasabee-project/Wasabee-Server/Firebase"
	"github.com/wasabee-project/Wasabee-Server/log"
)

// LinkID wrapper to ensure type safety
type LinkID string

// Link is defined by the Wasabee IITC plugin.
type Link struct {
	ID           LinkID   `json:"ID"`
	From         PortalID `json:"fromPortalId"`
	To           PortalID `json:"toPortalId"`
	Desc         string   `json:"description"`
	AssignedTo   GoogleID `json:"assignedTo"`
	ThrowOrder   int32    `json:"throwOrderPos"`
	Completed    bool     `json:"completed"` // to be deprecated
	State        string   `json:"_"`         // to be implemented
	Color        string   `json:"color"`
	Zone         Zone     `json:"zone"`
	DeltaMinutes int      `json:"deltaminutes"`
	MuCaptured   int      `json:"mu"`
	Changed      bool     `json:"changed,omitempty"`
	opID         OperationID
	Task
}

// insertLink adds a link to the database
func (opID OperationID) insertLink(l Link) error {
	if l.To == l.From {
		log.Infow("source and destination the same, ignoring link", "resource", opID)
		return nil
	}

	if !l.Zone.Valid() || l.Zone == ZoneAll {
		l.Zone = zonePrimary
	}

	_, err := db.Exec("INSERT INTO link (ID, fromPortalID, toPortalID, opID, description, gid, throworder, completed, color, zone, delta) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		l.ID, l.From, l.To, opID, MakeNullString(l.Desc), MakeNullString(l.AssignedTo), l.ThrowOrder, l.Completed, l.Color, l.Zone, l.DeltaMinutes)
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

	_, err := tx.Exec("INSERT INTO link (ID, fromPortalID, toPortalID, opID, description, gid, throworder, completed, color, zone, delta) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) ON DUPLICATE KEY UPDATE fromPortalID = ?, toPortalID = ?, description = ?, color=?, zone = ?, gid = ?, completed = ?, throworder = ?, delta = ?",
		l.ID, l.From, l.To, opID, MakeNullString(l.Desc), MakeNullString(l.AssignedTo), l.ThrowOrder, l.Completed, l.Color, l.Zone, l.DeltaMinutes,
		l.From, l.To, MakeNullString(l.Desc), l.Color, l.Zone, MakeNullString(l.AssignedTo), l.Completed, l.ThrowOrder, l.DeltaMinutes)
	if err != nil {
		log.Error(err)
		return err
	}

	if l.Changed && l.AssignedTo != "" {
		wfb.AssignLink(wfb.GoogleID(l.AssignedTo), wfb.TaskID(l.ID), wfb.OperationID(opID), "assigned")
	}

	return nil
}

// PopulateLinks fills in the Links list for the Operation. No authorization takes place.
func (o *Operation) populateLinks(zones []Zone, inGid GoogleID) error {
	var tmpLink Link
	tmpLink.opID = o.ID
	var description, gid sql.NullString

	var err error
	var rows *sql.Rows
	rows, err = db.Query("SELECT ID, fromPortalID, toPortalID, description, gid, throworder, completed, color, zone, delta FROM link WHERE opID = ? ORDER BY throworder", o.ID)
	if err != nil {
		log.Error(err)
		return err
	}
	defer rows.Close()
	for rows.Next() {
		err := rows.Scan(&tmpLink.ID, &tmpLink.From, &tmpLink.To, &description, &gid, &tmpLink.ThrowOrder, &tmpLink.Completed, &tmpLink.Color, &tmpLink.Zone, &tmpLink.DeltaMinutes)
		if err != nil {
			log.Error(err)
			continue
		}
		if description.Valid {
			tmpLink.Desc = description.String
		} else {
			tmpLink.Desc = ""
		}
		if gid.Valid {
			tmpLink.AssignedTo = GoogleID(gid.String)
		} else {
			tmpLink.AssignedTo = ""
		}
		// this isn't in a zone with which we are concerned AND not assigned to me, skip
		if !tmpLink.Zone.inZones(zones) && tmpLink.AssignedTo != inGid {
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

// Assign assigns a link to an agent
func (l Link) Assign(gid GoogleID) error {
	// gid of 0 unsets the assignment
	if gid == "0" {
		gid = ""
	}

	_, err := db.Exec("UPDATE link SET gid = ? WHERE ID = ? AND opID = ?", MakeNullString(gid), l.ID, l.opID)
	if err != nil {
		log.Error(err)
		return err
	}

	if gid != "" {
		wfb.AssignLink(wfb.GoogleID(l.AssignedTo), wfb.TaskID(l.ID), wfb.OperationID(l.opID), "assigned")
	}
	return err
}

// LinkDescription updates the description for a link
func (l Link) Comment(desc string) error {
	_, err := db.Exec("UPDATE link SET description = ? WHERE ID = ? AND opID = ?", MakeNullString(desc), l.ID, l.opID)
	if err != nil {
		log.Error(err)
		return err
	}
	return nil
}

// LinkCompleted updates the completed flag for a link
func (l Link) Complete(completed bool) error {
	_, err := db.Exec("UPDATE link SET completed = ? WHERE ID = ? AND opID = ?", completed, l.ID, l.opID)
	if err != nil {
		log.Error(err)
		return err
	}
	return nil
}

// IsAssignedTo checks to see if a link is assigned to a particular agent
func (l Link) IsAssignedTo(gid GoogleID) bool {
	if l.AssignedTo == gid {
		return true
	}

	var x int

	err := db.QueryRow("SELECT COUNT(*) FROM link WHERE opID = ? AND ID = ? AND gid = ?", l.opID, l.ID, gid).Scan(&x)
	if err != nil {
		log.Error(err)
		return false
	}
	if x != 1 {
		return false
	}
	return true
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
func (l Link) SetColor(color string) error {
	_, err := db.Exec("UPDATE link SET color = ? WHERE ID = ? and opID = ?", color, l.ID, l.opID)
	if err != nil {
		log.Error(err)
	}
	return err
}

// Delta sets the DeltaMinutes of a link in an operation
func (l Link) SetDelta(delta int) error {
	_, err := db.Exec("UPDATE link SET delta = ? WHERE ID = ? and opID = ?", delta, l.ID, l.opID)
	if err != nil {
		log.Error(err)
	}
	return err
}

// Swap changes the direction of a link in an operation
func (l Link) Swap() error {
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

// Zone sets a link's zone
func (l Link) SetZone(z Zone) error {
	if _, err := db.Exec("UPDATE link SET zone = ? WHERE ID = ? AND opID = ?", z, l.ID, l.opID); err != nil {
		log.Error(err)
		return err
	}
	return nil
}

// GetLink looks up and returns a populated Link from an id
func (o *Operation) GetLink(linkID LinkID) (Link, error) {
	if len(o.Links) == 0 { // XXX not a good test, not all ops have links
		err := fmt.Errorf("Attempt to use GetLink on unpopulated *Operation")
		log.Error(err)
		return Link{}, err
	}

	for _, l := range o.Links {
		if l.ID == linkID {
			return l, nil
		}
	}

	var l Link
	err := fmt.Errorf("link not found")
	return l, err
}

// Reject allows an agent to refuse to take a target
// gid must be the assigned agent.
func (l Link) Reject(gid GoogleID) error {
	if !l.IsAssignedTo(gid) {
		err := fmt.Errorf("link not assigned to you")
		log.Errorw(err.Error(), "GID", gid, "resource", l.opID, "link", l.ID)
		return err
	}
	return l.Assign("")
}

func (l Link) Claim(gid GoogleID) error {
	var assignedTo sql.NullString
	err := db.QueryRow("SELECT gid FROM link WHERE opID = ? AND ID = ?", l.opID, l.ID).Scan(&assignedTo)
	if err != nil {
		log.Error(err)
		return err
	}

	// link already assigned to someone (even claiming agent)
	if assignedTo.Valid {
		err := fmt.Errorf("link already assigned")
		log.Errorw(err.Error(), "GID", gid, "resource", l.opID, "link", l.ID)
		return err
	}

	return l.Assign(gid)
}

// Acknowledge -- to be implemented
func (l Link) Acknowledge(gid GoogleID) (string, error) {
	return "", nil
}

// Incomplete -- to be implemented
func (l Link) Incomplete(gid GoogleID) (string, error) {
	return "", nil
}
