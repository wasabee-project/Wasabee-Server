package wasabee

import (
	"database/sql"
	"strings"
)

// LinkID wrapper to ensure type safety
type LinkID string

// Link is defined by the Wasabee IITC plugin.
type Link struct {
	ID         LinkID   `json:"ID"`
	From       PortalID `json:"fromPortalId"`
	To         PortalID `json:"toPortalId"`
	Desc       string   `json:"description"`
	AssignedTo GoogleID `json:"assignedTo"`
	ThrowOrder float64  `json:"throwOrderPos"`
}

// insertLink adds a link to the database
func (o *Operation) insertLink(l Link) error {
	if l.To == l.From {
		Log.Debug("source and destination the same, ignoring link")
		return nil
	}

	_, err := db.Exec("INSERT INTO link (ID, fromPortalID, toPortalID, opID, description, gid, throworder) VALUES (?, ?, ?, ?, ?, ?, ?)",
		l.ID, l.From, l.To, o.ID, l.Desc, l.AssignedTo, l.ThrowOrder)
	if err != nil {
		Log.Error(err)
		return err
	}
	return nil
}

// PopulateLinks fills in the Links list for the Operation. No authorization takes place.
func (o *Operation) PopulateLinks() error {
	var tmpLink Link
	var description, gid sql.NullString

	rows, err := db.Query("SELECT ID, fromPortalID, toPortalID, description, gid, throworder FROM link WHERE opID = ? ORDER BY throworder", o.ID)
	if err != nil {
		Log.Error(err)
		return err
	}
	defer rows.Close()
	for rows.Next() {
		err := rows.Scan(&tmpLink.ID, &tmpLink.From, &tmpLink.To, &description, &gid, &tmpLink.ThrowOrder)
		if err != nil {
			Log.Error(err)
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
		o.Links = append(o.Links, tmpLink)
	}
	return nil
}

// String returns the string version of a LinkID
func (l LinkID) String() string {
	return string(l)
}

// AssignLink assigns a link to an agent, sending them a message that they have an assignment
func (opID OperationID) AssignLink(linkID LinkID, gid GoogleID) error {
	_, err := db.Exec("UPDATE link SET gid = ? WHERE ID = ? AND opID = ?", gid, linkID, opID)
	if err != nil {
		Log.Error(err)
		return err
	}

	/*
		link := struct {
			OpID   OperationID
			LinkID LinkID
		}{
			OpID:   opID,
			LinkID: linkID,
		}

		msg, err := gid.ExecuteTemplate("assignLink", link)
		if err != nil {
			Log.Error(err)
			msg = fmt.Sprintf("assigned a marker for op %s", opID)
			// do not report send errors up the chain, just log
		}
		if string(gid) != "" {
			_, err = gid.SendMessage(msg)
			if err != nil {
				Log.Error(err)
				// do not report send errors up the chain, just log
			}
		}
	*/

	if err = opID.Touch(); err != nil {
		Log.Error(err)
	}

	return nil
}

// LinkDescription updates the description for a link
func (opID OperationID) LinkDescription(linkID LinkID, desc string) error {
	_, err := db.Exec("UPDATE link SET description = ? WHERE ID = ? AND opID = ?", desc, linkID, opID)
	if err != nil {
		Log.Error(err)
		return err
	}
	if err = opID.Touch(); err != nil {
		Log.Error(err)
	}
	return nil
}

// LinkOrder changes the order of the throws for an operation
func (opID OperationID) LinkOrder(order string, gid GoogleID) error {
	// check isowner (already done in http/pdraw.go, but there may be other callers in the future

	stmt, err := db.Prepare("UPDATE link SET throworder = ? WHERE opID = ? AND ID = ?")
	if err != nil {
		Log.Error(err)
		return err
	}

	pos := 1
	links := strings.Split(order, ",")
	for i := range links {
		if links[i] == "000" { // the header, could be anyplace in the order if the user was being silly
			continue
		}
		if _, err := stmt.Exec(pos, opID, links[i]); err != nil {
			Log.Error(err)
			continue
		}
		pos++
	}
	if err = opID.Touch(); err != nil {
		Log.Error(err)
	}
	return nil
}
