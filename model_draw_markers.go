package wasabee

import (
	"database/sql"
	"fmt"
	"strings"
)

// MarkerID wrapper to ensure type safety
type MarkerID string

// MarkerType will be an enum once we figure out the full list
type MarkerType string

// Marker is defined by the Wasabee IITC plugin.
type Marker struct {
	ID           MarkerID   `json:"ID"`
	PortalID     PortalID   `json:"portalId"`
	Type         MarkerType `json:"type"`
	Comment      string     `json:"comment"`
	AssignedTo   GoogleID   `json:"assignedTo"`
	AssignedTeam TeamID     `json:"assignedTeam"`
	IngressName  string     `json:"assignedNickname"`
	CompletedBy  string     `json:"completedBy"`
	CompletedID  GoogleID   `json:"completedID"`
	State        string     `json:"state"`
	Order        int        `json:"order"`
}

// insertMarkers adds a marker to the database
func (opID OperationID) insertMarker(m Marker) error {
	if m.State == "" {
		m.State = "pending"
	}

	_, err := db.Exec("INSERT INTO marker (ID, opID, PortalID, type, gid, comment, state, oporder) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
		m.ID, opID, m.PortalID, m.Type, MakeNullString(m.AssignedTo), MakeNullString(m.Comment), m.State, m.Order)
	if err != nil {
		Log.Error(err)
		return err
	}
	return nil
}

func (opID OperationID) updateMarker(m Marker) error {
	if m.State == "" {
		m.State = "pending"
	}

	_, err := db.Exec("INSERT INTO marker (ID, opID, PortalID, type, gid, comment, state, oporder) VALUES (?, ?, ?, ?, ?, ?, ?, ?) ON DUPLICATE KEY UPDATE type = ?, PortalID = ?, comment = ?",
		m.ID, opID, m.PortalID, m.Type, MakeNullString(m.AssignedTo), MakeNullString(m.Comment), m.State, m.Order, m.Type, m.PortalID, MakeNullString(m.Comment))
	if err != nil {
		Log.Error(err)
		return err
	}
	return nil
}

func (opID OperationID) deleteMarker(mid MarkerID) error {
	_, err := db.Exec("DELETE FROM marker WHERE opID = ? and ID = ?", opID, mid)
	if err != nil {
		Log.Error(err)
		return err
	}
	return nil
}

// PopulateMarkers fills in the Markers list for the Operation. No authorization takes place.
func (o *Operation) populateMarkers() error {
	var tmpMarker Marker

	var assignedGid, comment, assignedNick, completedBy, completedID sql.NullString

	rows, err := db.Query("SELECT m.ID, m.PortalID, m.type, m.gid, m.comment, m.state, a.iname AS assignedTo, b.iname AS completedBy, m.oporder, m.completedby AS completedID FROM marker=m LEFT JOIN agent=a ON m.gid = a.gid LEFT JOIN agent=b on m.completedby = b.gid WHERE m.opID = ? ORDER BY m.oporder, m.type", o.ID)
	if err != nil {
		Log.Error(err)
		return err
	}
	defer rows.Close()
	for rows.Next() {
		err := rows.Scan(&tmpMarker.ID, &tmpMarker.PortalID, &tmpMarker.Type, &assignedGid, &comment, &tmpMarker.State, &assignedNick, &completedBy, &tmpMarker.Order, &completedID)
		if err != nil {
			Log.Error(err)
			continue
		}
		if tmpMarker.State == "" { // enums in sql default to "" if invalid, WTF?
			tmpMarker.State = "pending"
		}
		if assignedGid.Valid {
			tmpMarker.AssignedTo = GoogleID(assignedGid.String)
		} else {
			tmpMarker.AssignedTo = ""
		}
		if assignedNick.Valid {
			tmpMarker.IngressName = assignedNick.String
		} else {
			tmpMarker.IngressName = ""
		}
		if comment.Valid {
			tmpMarker.Comment = comment.String
		} else {
			tmpMarker.Comment = ""
		}
		if completedBy.Valid {
			tmpMarker.CompletedBy = completedBy.String
		} else {
			tmpMarker.CompletedBy = ""
		}
		if completedID.Valid {
			tmpMarker.CompletedID = GoogleID(completedID.String)
		} else {
			tmpMarker.CompletedID = ""
		}
		o.Markers = append(o.Markers, tmpMarker)
	}

	return nil
}

// String returns the string version of a PortalID
func (m MarkerType) String() string {
	return string(m)
}

// String returns the string version of a MarkerID
func (m MarkerID) String() string {
	return string(m)
}

// AssignMarker assigns a marker to an agent, sending them a message
func (o *Operation) AssignMarker(markerID MarkerID, gid GoogleID, sendMessage bool) error {
	// unassign
	if gid == "0" {
		gid = ""
	}

	_, err := db.Exec("UPDATE marker SET gid = ?, state = ? WHERE ID = ? AND opID = ?", MakeNullString(gid), "assigned", markerID, o.ID)
	if err != nil {
		Log.Error(err)
		return err
	}

	// we are done if not sending messages or unassignming
	if !sendMessage || gid.String() == "" {
		return nil
	}

	o.ID.firebaseAssignMarker(gid, markerID)
	m, err := o.getMarker(markerID)
	if err != nil {
		Log.Error(err)
		return err
	}

	p, err := o.getPortal(m.PortalID)
	if err != nil {
		Log.Error(err)
		return err
	}

	templateData := struct {
		PortalID PortalID
		Name     string
		Lat      string
		Lon      string
		Type     string
		Sender   string
	}{
		PortalID: m.PortalID,
		Name:     p.Name,
		Lat:      p.Lat,
		Lon:      p.Lon,
		Type:     m.Type.String(),
		Sender:   "unavailable",
	}

	msg, err := gid.ExecuteTemplate("target", templateData)
	if err != nil {
		Log.Error(err)
		msg = fmt.Sprintf("assigned a marker for op %s", o.ID)
		// do not report send errors up the chain, just log
	}
	if _, err = gid.SendMessage(msg); err != nil {
		Log.Errorw("send message", "GID", gid, "error", err, "themsg", msg)
		// do not report send errors up the chain, just log
	}
	if err = o.Touch(); err != nil {
		Log.Error(err)
	}
	return nil
}

// lookup and return a populated Marker from an id
func (o *Operation) getMarker(markerID MarkerID) (Marker, error) {
	for _, m := range o.Markers {
		if m.ID == markerID {
			return m, nil
		}
	}

	var m Marker
	err := fmt.Errorf("marker not found")
	return m, err
}

// MarkerComment updates the comment on a marker
func (o *Operation) MarkerComment(markerID MarkerID, comment string) error {
	if _, err := db.Exec("UPDATE marker SET comment = ? WHERE ID = ? AND opID = ?", MakeNullString(comment), markerID, o.ID); err != nil {
		Log.Error(err)
		return err
	}
	if err := o.Touch(); err != nil {
		Log.Error(err)
	}
	return nil
}

// Acknowledge that a marker has been assigned
// gid must be the assigned agent.
func (m MarkerID) Acknowledge(o *Operation, gid GoogleID) error {
	var ns sql.NullString
	err := db.QueryRow("SELECT gid FROM marker WHERE ID = ? and opID = ?", m, o.ID).Scan(&ns)
	if err != nil && err != sql.ErrNoRows {
		Log.Info(err)
		return err
	}
	if err != nil && err == sql.ErrNoRows {
		err = fmt.Errorf("no such marker")
		Log.Warnw(err.Error(), "resource", o.ID, "marker", m)
		return err
	}
	if !ns.Valid {
		err = fmt.Errorf("marker not assigned")
		Log.Warnw(err.Error(), "resource", o.ID, "marker", m)
		return err
	}
	markerGid := GoogleID(ns.String)
	if gid != markerGid {
		err = fmt.Errorf("marker assigned to someone else")
		Log.Warnw(err.Error(), "resource", o.ID, "marker", m)
		return err
	}
	if _, err = db.Exec("UPDATE marker SET state = ? WHERE ID = ? AND opID = ?", "acknowledged", m, o.ID); err != nil {
		Log.Error(err)
		return err
	}
	if err = o.Touch(); err != nil {
		Log.Error(err)
	}

	o.firebaseMarkerStatus(m, "acknowledged")
	return nil
}

// Complete marks a marker as completed
func (m MarkerID) Complete(o Operation, gid GoogleID) error {
	if !o.ReadAccess(gid) {
		err := fmt.Errorf("permission denied")
		Log.Errorw(err.Error(), "GID", gid, "resource", o.ID, "marker", m)
		return err
	}
	if _, err := db.Exec("UPDATE marker SET state = ?, completedby = ? WHERE ID = ? AND opID = ?", "completed", gid, m, o.ID); err != nil {
		Log.Error(err)
		return err
	}
	if err := o.Touch(); err != nil {
		Log.Error(err)
	}

	o.firebaseMarkerStatus(m, "completed")
	return nil
}

// Incomplete marks a marker as not-completed
func (m MarkerID) Incomplete(o Operation, gid GoogleID) error {
	if !o.ReadAccess(gid) {
		err := fmt.Errorf("permission denied")
		Log.Errorw(err.Error(), "GID", gid, "resource", o.ID, "marker", m)
		return err
	}
	if _, err := db.Exec("UPDATE marker SET state = ?, completedby = NULL WHERE ID = ? AND opID = ?", "assigned", m, o.ID); err != nil {
		Log.Error(err)
		return err
	}
	if err := o.Touch(); err != nil {
		Log.Error(err)
	}

	o.firebaseMarkerStatus(m, "assigned")
	return nil
}

// Reject allows an agent to refuse to take a target
// gid must be the assigned agent.
func (m MarkerID) Reject(o *Operation, gid GoogleID) error {
	var ns sql.NullString
	err := db.QueryRow("SELECT gid FROM marker WHERE ID = ? and opID = ?", m, o.ID).Scan(&ns)
	if err != nil && err != sql.ErrNoRows {
		Log.Error(err)
		return err
	}
	if err != nil && err == sql.ErrNoRows {
		err = fmt.Errorf("no such marker")
		Log.Warnw(err.Error(), "GID", gid, "resource", o.ID, "marker", m)
		return err
	}
	if !ns.Valid {
		err = fmt.Errorf("marker not assigned")
		Log.Warnw(err.Error(), "GID", gid, "resource", o.ID, "marker", m)
		return err
	}
	markerGid := GoogleID(ns.String)
	if gid != markerGid {
		err = fmt.Errorf("marker assigned to someone else")
		Log.Warnw(err.Error(), "GID", gid, "resource", o.ID, "marker", m)
		return err
	}
	if _, err = db.Exec("UPDATE marker SET state = 'pending', gid = NULL WHERE ID = ? AND opID = ?", m, o.ID); err != nil {
		Log.Error(err)
		return err
	}
	if err = o.Touch(); err != nil {
		Log.Error(err)
	}

	o.firebaseMarkerStatus(m, "pending")
	return nil
}

// MarkerOrder changes the order of the throws for an operation
func (o *Operation) MarkerOrder(order string, gid GoogleID) error {
	stmt, err := db.Prepare("UPDATE marker SET oporder = ? WHERE opID = ? AND ID = ?")
	if err != nil {
		Log.Error(err)
		return err
	}

	pos := 1
	markers := strings.Split(order, ",")
	for i := range markers {
		if markers[i] == "000" { // the header, could be any place in the order if the user was being silly
			continue
		}
		if _, err := stmt.Exec(pos, o.ID, markers[i]); err != nil {
			Log.Error(err)
			continue
		}
		pos++
	}
	if err = o.Touch(); err != nil {
		Log.Error(err)
	}
	return nil
}
