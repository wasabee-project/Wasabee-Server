package wasabee

import (
	"database/sql"
	"fmt"
)

// MarkerID wrapper to ensure type safety
type MarkerID string

// MarkerType will be an enum once we figure out the full list
type MarkerType string

// Marker is defined by the Wasabee IITC plugin.
type Marker struct {
	ID          MarkerID   `json:"ID"`
	PortalID    PortalID   `json:"portalId"`
	Type        MarkerType `json:"type"`
	Comment     string     `json:"comment"`
	AssignedTo  GoogleID   `json:"assignedTo"`
	IngressName string     `json:"assignedNickname"`
	CompletedBy string     `json:"completedBy"`
	State       string     `json:"state"`
}

// insertMarkers adds a marker to the database
func (o *Operation) insertMarker(m Marker) error {
	_, err := db.Exec("INSERT INTO marker (ID, opID, PortalID, type, gid, comment, state) VALUES (?, ?, ?, ?, ?, ?, ?)",
		m.ID, o.ID, m.PortalID, m.Type, m.AssignedTo, m.Comment, m.State)
	if err != nil {
		Log.Error(err)
		return err
	}
	return nil
}

// PopulateMarkers fills in the Markers list for the Operation. No authorization takes place.
func (o *Operation) PopulateMarkers() error {
	var tmpMarker Marker
	var gid, comment, iname, completedBy sql.NullString

	// XXX join with portals table, get name and order by name, don't expose it in this json -- will make the friendly in the https module easier
	rows, err := db.Query("SELECT m.ID, m.PortalID, m.type, m.gid, m.comment, m.state, a.iname, b.iname FROM marker=m LEFT JOIN agent=a ON m.gid = a.gid LEFT JOIN agent=b on m.completedby = b.gid WHERE m.opID = ?", o.ID)
	if err != nil {
		Log.Error(err)
		return err
	}
	defer rows.Close()
	for rows.Next() {
		err := rows.Scan(&tmpMarker.ID, &tmpMarker.PortalID, &tmpMarker.Type, &gid, &comment, &tmpMarker.State, &iname, &completedBy)
		if err != nil {
			Log.Error(err)
			continue
		}
		if gid.Valid {
			tmpMarker.AssignedTo = GoogleID(gid.String)
		} else {
			tmpMarker.Comment = ""
		}
		if comment.Valid {
			tmpMarker.Comment = comment.String
		} else {
			tmpMarker.Comment = ""
		}
		if iname.Valid {
			tmpMarker.IngressName = iname.String
		} else {
			tmpMarker.IngressName = ""
		}
		if completedBy.Valid {
			tmpMarker.CompletedBy = completedBy.String
		} else {
			tmpMarker.CompletedBy = ""
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
func (opID OperationID) AssignMarker(markerID MarkerID, gid GoogleID) error {
	_, err := db.Exec("UPDATE marker SET gid = ?, state = ? WHERE ID = ? AND opID = ?", gid, "assigned", markerID, opID)
	if err != nil {
		Log.Error(err)
		return err
	}

	marker := struct {
		OpID     OperationID
		MarkerID MarkerID
	}{
		OpID:     opID,
		MarkerID: markerID,
	}

	msg, err := gid.ExecuteTemplate("assignMarker", marker)
	if err != nil {
		Log.Error(err)
		msg = fmt.Sprintf("assigned a marker for op %s", opID)
		// do not report send errors up the chain, just log
	}
	_, err = gid.SendMessage(msg)
	if err != nil {
		Log.Errorf("%s %s %s", gid, err, msg)
		// do not report send errors up the chain, just log
	}
	if err = opID.Touch(); err != nil {
		Log.Error(err)
	}

	return nil
}

// MarkerComment updates the comment on a marker
func (opID OperationID) MarkerComment(markerID MarkerID, comment string) error {
	_, err := db.Exec("UPDATE marker SET comment = ? WHERE ID = ? AND opID = ?", comment, markerID, opID)
	if err != nil {
		Log.Error(err)
		return err
	}
	if err = opID.Touch(); err != nil {
		Log.Error(err)
	}
	return nil
}

// Acknowledge that a marker has been assigned
// gid must be the assigned agent.
func (m MarkerID) Acknowledge(opID OperationID, gid GoogleID) error {
	var ns sql.NullString
	err := db.QueryRow("SELECT gid FROM marker WHERE ID = ? and opID = ?", m, opID).Scan(&ns)
	if err != nil && err != sql.ErrNoRows {
		Log.Notice(err)
		return err
	}
	if err != nil && err == sql.ErrNoRows {
		err = fmt.Errorf("no such marker")
		Log.Error(err)
		return err
	}
	if !ns.Valid {
		err = fmt.Errorf("marker not assigned")
		Log.Error(err)
		return err
	}
	markerGid := GoogleID(ns.String)
	if gid != markerGid {
		err = fmt.Errorf("marker assigned to someone else")
		Log.Error(err)
		return err
	}
	_, err = db.Exec("UPDATE marker SET state = ? WHERE ID = ? AND opID = ?", "acknowledged", m, opID)
	if err != nil {
		Log.Error(err)
		return err
	}
	if err = opID.Touch(); err != nil {
		Log.Error(err)
	}

	return nil
}

// Finalize is when an operator verifies that a marker has been taken care of.
// gid must be the op owner.
func (m MarkerID) Finalize(opID OperationID, gid GoogleID) error {
	if !opID.IsOwner(gid) {
		err := fmt.Errorf("not operation owner")
		Log.Error(err)
		return err
	}
	_, err := db.Exec("UPDATE marker SET state = ? WHERE ID = ? AND opID = ?", "completed", m, opID)
	if err != nil {
		Log.Error(err)
		return err
	}
	if err = opID.Touch(); err != nil {
		Log.Error(err)
	}
	return nil
}

// Complete marks a marker as completed
func (m MarkerID) Complete(opID OperationID, gid GoogleID) error {
	if !opID.ReadAccess(gid) {
		err := fmt.Errorf("permission denied")
		Log.Error(err)
		return err
	}
	_, err := db.Exec("UPDATE marker SET state = ?, completedby = ? WHERE ID = ? AND opID = ?", "completed", gid, m, opID)
	if err != nil {
		Log.Error(err)
		return err
	}
	if err = opID.Touch(); err != nil {
		Log.Error(err)
	}
	return nil
}

// Incomplete marks a marker as not-completed
func (m MarkerID) Incomplete(opID OperationID, gid GoogleID) error {
	if !opID.ReadAccess(gid) {
		err := fmt.Errorf("permission denied")
		Log.Error(err)
		return err
	}
	_, err := db.Exec("UPDATE marker SET state = ?, completedby = NULL WHERE ID = ? AND opID = ?", "assigned", m, opID)
	if err != nil {
		Log.Error(err)
		return err
	}
	if err = opID.Touch(); err != nil {
		Log.Error(err)
	}
	return nil
}

// Reject allows an agent to refuse to take a target
// gid must be the assigned agent.
func (m MarkerID) Reject(opID OperationID, gid GoogleID) error {
	var ns sql.NullString
	err := db.QueryRow("SELECT gid FROM marker WHERE ID = ? and opID = ?", m, opID).Scan(&ns)
	if err != nil && err != sql.ErrNoRows {
		Log.Notice(err)
		return err
	}
	if err != nil && err == sql.ErrNoRows {
		err = fmt.Errorf("no such marker")
		Log.Error(err)
		return err
	}
	if !ns.Valid {
		err = fmt.Errorf("marker not assigned")
		Log.Error(err)
		return err
	}
	markerGid := GoogleID(ns.String)
	if gid != markerGid {
		err = fmt.Errorf("marker assigned to someone else")
		Log.Error(err)
		return err
	}
	_, err = db.Exec("UPDATE marker SET state = 'pending', gid = NULL WHERE ID = ? AND opID = ?", m, opID)
	if err != nil {
		Log.Error(err)
		return err
	}
	if err = opID.Touch(); err != nil {
		Log.Error(err)
	}
	return nil
}
