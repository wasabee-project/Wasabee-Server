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

// MarkerAttributeID wrapper to ensure type safety
type MarkerAttributeID string

// Marker is defined by the Wasabee IITC plugin.
type Marker struct {
	ID           MarkerID          `json:"ID"`
	PortalID     PortalID          `json:"portalId"`
	Type         MarkerType        `json:"type"`
	Comment      string            `json:"comment"`
	AssignedTo   GoogleID          `json:"assignedTo"`
	AssignedTeam TeamID            `json:"assignedTeam"`
	CompletedID  GoogleID          `json:"completedID"`
	State        string            `json:"state"`
	Order        int               `json:"order"`
	Zone         Zone              `json:"zone"`
	DeltaMinutes int               `json:"deltaminutes"`
	Attributes   []MarkerAttribute `json:"attributes"`
	DependsOn    []TaskID          `json:"dependsOn"`
}

// MarkerAttribute is used for the per-marker-type extended attributes
type MarkerAttribute struct {
	ID         MarkerAttributeID `json:"ID"`
	AssignedTo GoogleID          `json:"assignedTo"`
	// AssignedTeam TeamID `json:"assignedTeam"` -- perhaps...
	Key   string `json:"key"`
	Value string `json:"value"`
}

// insertMarkers adds a marker to the database
func (opID OperationID) insertMarker(m Marker) error {
	if m.State == "" {
		m.State = "pending"
	}

	if !m.Zone.Valid() || m.Zone == ZoneAll {
		m.Zone = zonePrimary
	}

	_, err := db.Exec("INSERT INTO marker (ID, opID, PortalID, type, gid, comment, state, oporder, zone, delta) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		m.ID, opID, m.PortalID, m.Type, MakeNullString(m.AssignedTo), MakeNullString(m.Comment), m.State, m.Order, m.Zone, m.DeltaMinutes)
	if err != nil {
		Log.Error(err)
		return err
	}

	for _, ma := range m.Attributes {
		Log.Debugw("marker attributes set", "attribute", ma)
		_, err := db.Exec("INSERT INTO markerattributes (ID, opID, markerID, assignedTo, name, value) VALUES (?, ?, ?, ?, ?, ?)",
			ma.ID, opID, m.ID, ma.AssignedTo, ma.Key, ma.Value)
		if err != nil {
			Log.Error(err)
			return err
		}
	}

	for _, d := range m.DependsOn {
		Log.Infow("marker depends set", "marker", m.ID, "depends on", d)
		_, err := db.Exec("INSERT INTO depends (opID, taskID, dependsOn) VALUES (?, ?, ?)",
			opID, m.ID, d)
		if err != nil {
			Log.Error(err)
			return err
		}
	}
	return nil
}

func (opID OperationID) updateMarker(m Marker, tx *sql.Tx) error {
	if m.State == "" {
		m.State = "pending"
	}

	if !m.Zone.Valid() || m.Zone == ZoneAll {
		m.Zone = zonePrimary
	}

	assignmentChanged := false
	if m.AssignedTo != "" {
		var count uint8
		err := tx.QueryRow("SELECT COUNT(*) FROM marker WHERE ID = ? AND opID = ? AND gid = ?", m.ID, opID, m.AssignedTo).Scan(&count)
		if err != nil {
			Log.Error(err)
			return err
		}
		if count != 1 {
			assignmentChanged = true
		}
	}

	_, err := tx.Exec("INSERT INTO marker (ID, opID, PortalID, type, gid, comment, state, oporder, zone, delta) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?) ON DUPLICATE KEY UPDATE type = ?, PortalID = ?, gid = ?, comment = ?, state = ?, zone = ?, oporder = ?, delta = ?",
		m.ID, opID, m.PortalID, m.Type, MakeNullString(m.AssignedTo), MakeNullString(m.Comment), m.State, m.Order, m.Zone, m.DeltaMinutes,
		m.Type, m.PortalID, MakeNullString(m.AssignedTo), MakeNullString(m.Comment), m.State, m.Zone, m.Order, m.DeltaMinutes)
	if err != nil {
		Log.Error(err)
		return err
	}

	if assignmentChanged {
		Log.Debugw("marker assignment changed, sending FB", "marker", m.ID, "resource", opID, "GID", m.AssignedTo)
		opID.firebaseAssignMarker(m.AssignedTo, m.ID, m.State, "")
	}

	_, err = tx.Exec("DELETE FROM markerattributes WHERE opID = ? AND markerID = ?", opID, m.ID)
	if err != nil {
		Log.Error(err)
		return err
	}
	for _, ma := range m.Attributes {
		Log.Debugw("marker attributes set", "attribute", ma)
		_, err := tx.Exec("INSERT INTO markerattributes (ID, opID, markerID, assignedTo, name, value) VALUES (?, ?, ?, ?, ?, ?)",
			ma.ID, opID, m.ID, ma.AssignedTo, ma.Key, ma.Value)
		if err != nil {
			Log.Error(err)
			return err
		}
	}

	for _, d := range m.DependsOn {
		Log.Infow("marker depends set", "marker", m.ID, "depends on", d)
		_, err := tx.Exec("INSERT INTO depends (opID, taskID, dependsOn) VALUES (?, ?, ?)",
			opID, m.ID, d)
		if err != nil {
			Log.Error(err)
			return err
		}
	}
	return nil
}

func (opID OperationID) deleteMarker(mid MarkerID, tx *sql.Tx) error {
	_, err := tx.Exec("DELETE FROM marker WHERE opID = ? and ID = ?", opID, mid)
	if err != nil {
		Log.Error(err)
		return err
	}
	return nil
}

// PopulateMarkers fills in the Markers list for the Operation.
func (o *Operation) populateMarkers(zones []Zone, gid GoogleID) error {
	var tmpMarker Marker

	var assignedGid, comment, completedID sql.NullString

	var err error
	var rows *sql.Rows
	rows, err = db.Query("SELECT ID, PortalID, type, gid, comment, state, oporder, completedby AS completedID, zone, delta FROM marker WHERE opID = ? ORDER BY oporder, type", o.ID)
	if err != nil {
		Log.Error(err)
		return err
	}
	defer rows.Close()
	for rows.Next() {
		err := rows.Scan(&tmpMarker.ID, &tmpMarker.PortalID, &tmpMarker.Type, &assignedGid, &comment, &tmpMarker.State, &tmpMarker.Order, &completedID, &tmpMarker.Zone, &tmpMarker.DeltaMinutes)
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

		if comment.Valid {
			tmpMarker.Comment = comment.String
		} else {
			tmpMarker.Comment = ""
		}

		if completedID.Valid {
			tmpMarker.CompletedID = GoogleID(completedID.String)
		} else {
			tmpMarker.CompletedID = ""
		}

		// if the marker is not in the zones with which we are concerned AND not assigned to me, skip
		if !tmpMarker.Zone.inZones(zones) && tmpMarker.AssignedTo != gid {
			continue
		}

		if tmpMarker.Attributes, err = tmpMarker.ID.populateAttributes(o.ID); err != nil {
			Log.Error(err)
		}

		if tmpMarker.DependsOn, err = tmpMarker.ID.populateDepends(o.ID); err != nil {
			Log.Error(err)
		}

		tmpMarker.DependsOn, _ = tmpMarker.ID.Depends(o)

		o.Markers = append(o.Markers, tmpMarker)
	}

	return nil
}

func (m MarkerID) populateAttributes(opID OperationID) ([]MarkerAttribute, error) {
	var attrs []MarkerAttribute
	var t MarkerAttribute
	var gid sql.NullString

	var err error
	var rows *sql.Rows
	rows, err = db.Query("SELECT ID, name, value, assignedTo FROM markerattributes WHERE markerID = ? AND  opID = ?", m, opID)
	if err != nil {
		Log.Error(err)
		return attrs, err
	}
	defer rows.Close()
	for rows.Next() {
		err := rows.Scan(&t.ID, &t.Key, &t.Value, &gid)
		if err != nil {
			Log.Error(err)
			return attrs, err
		}
		if gid.Valid {
			t.AssignedTo = GoogleID(gid.String)
		} else {
			t.AssignedTo = ""
		}
		attrs = append(attrs, t)
	}
	return attrs, nil
}

func (t MarkerID) populateDepends(opID OperationID) ([]TaskID, error) {
	var deps []TaskID
	var d TaskID

	var err error
	var rows *sql.Rows
	rows, err = db.Query("SELECT dependsOn FROM depends WHERE taskID = ? AND opID = ?", t, opID)
	if err != nil {
		Log.Error(err)
		return deps, err
	}
	defer rows.Close()
	for rows.Next() {
		err := rows.Scan(&d)
		if err != nil {
			Log.Error(err)
			return deps, err
		}
		deps = append(deps, d)
	}
	return deps, nil
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
func (o *Operation) AssignMarker(markerID MarkerID, gid GoogleID) (string, error) {
	// unassign
	if gid == "0" {
		gid = ""
	}

	_, err := db.Exec("UPDATE marker SET gid = ?, state = ? WHERE ID = ? AND opID = ?", MakeNullString(gid), "assigned", markerID, o.ID)
	if err != nil {
		Log.Error(err)
		return "", err
	}

	updateID, err := o.Touch()
	if gid.String() != "" {
		o.ID.firebaseAssignMarker(gid, markerID, "assigned", updateID)
	}
	return updateID, err
}

// GetMarker lookup and return a populated Marker from an id
func (o *Operation) GetMarker(markerID MarkerID) (Marker, error) {
	for _, m := range o.Markers {
		if m.ID == markerID {
			return m, nil
		}
	}

	var m Marker
	err := fmt.Errorf("marker not found")
	return m, err
}

// ClaimMarker assigns a marker to the claiming agent
func (o *Operation) ClaimMarker(markerID MarkerID, gid GoogleID) (string, error) {
	m, err := o.GetMarker(markerID)
	if err != nil {
		Log.Error(err)
		return "", err
	}

	if m.AssignedTo != "" {
		err := fmt.Errorf("can only claim unassigned markers")
		Log.Errorw(err.Error(), "GID", gid, "resource", o.ID, "marker", markerID)
		return "", err
	}

	return o.AssignMarker(markerID, gid)
}

// MarkerComment updates the comment on a marker
func (o *Operation) MarkerComment(markerID MarkerID, comment string) (string, error) {
	if _, err := db.Exec("UPDATE marker SET comment = ? WHERE ID = ? AND opID = ?", MakeNullString(comment), markerID, o.ID); err != nil {
		Log.Error(err)
		return "", err
	}
	return o.Touch()
}

// Zone updates the marker's zone
func (m MarkerID) Zone(o *Operation, z Zone) (string, error) {
	if _, err := db.Exec("UPDATE marker SET zone = ? WHERE ID = ? AND opID = ?", z, m, o.ID); err != nil {
		Log.Error(err)
		return "", err
	}
	return o.Touch()
}

// Delta updates the marker's DeltaMinutes
func (m MarkerID) Delta(o *Operation, delta int) (string, error) {
	if _, err := db.Exec("UPDATE marker SET delta = ? WHERE ID = ? AND opID = ?", delta, m, o.ID); err != nil {
		Log.Error(err)
		return "", err
	}
	return o.Touch()
}

func (m MarkerID) isAssignee(o *Operation, gid GoogleID) (bool, error) {
	var ns sql.NullString
	err := db.QueryRow("SELECT gid FROM marker WHERE ID = ? and opID = ?", m, o.ID).Scan(&ns)
	if err != nil && err != sql.ErrNoRows {
		Log.Error(err)
		return false, err
	}
	if err != nil && err == sql.ErrNoRows {
		err = fmt.Errorf("no such marker")
		Log.Warnw(err.Error(), "resource", o.ID, "marker", m)
		return false, err
	}
	if !ns.Valid {
		err = fmt.Errorf("marker not assigned")
		Log.Warnw(err.Error(), "resource", o.ID, "marker", m)
		return false, err
	}
	markerGid := GoogleID(ns.String)
	if gid == markerGid {
		return true, nil
	}
	return false, nil
}

// Acknowledge that a marker has been assigned
// gid must be the assigned agent.
func (m MarkerID) Acknowledge(o *Operation, gid GoogleID) (string, error) {
	assignee, err := m.isAssignee(o, gid)
	if err != nil {
		Log.Warnw(err.Error(), "resource", o.ID, "marker", m)
		return "", err
	}
	if !assignee {
		err := fmt.Errorf("marker not assigned to you")
		Log.Warnw(err.Error(), "resource", o.ID, "marker", m)
		return "", err
	}
	if _, err = db.Exec("UPDATE marker SET state = ? WHERE ID = ? AND opID = ?", "acknowledged", m, o.ID); err != nil {
		Log.Error(err)
		return "", err
	}
	updateID, err := o.Touch()
	o.firebaseMarkerStatus(m, "acknowledged", updateID)
	return updateID, err
}

// Complete marks a marker as completed
func (m MarkerID) Complete(o *Operation, gid GoogleID) (string, error) {
	write := o.WriteAccess(gid)
	assignee, err := m.isAssignee(o, gid)
	if err != nil {
		Log.Errorw(err.Error(), "GID", gid, "resource", o.ID, "marker", m)
		return "", err
	}
	if !assignee && !write {
		err := fmt.Errorf("permission denied")
		Log.Errorw(err.Error(), "GID", gid, "resource", o.ID, "marker", m)
		return "", err
	}
	if _, err := db.Exec("UPDATE marker SET state = ?, completedby = ? WHERE ID = ? AND opID = ?", "completed", gid, m, o.ID); err != nil {
		Log.Error(err)
		return "", err
	}
	updateID, err := o.Touch()
	o.firebaseMarkerStatus(m, "completed", updateID)
	return updateID, err
}

// Incomplete marks a marker as not-completed
func (m MarkerID) Incomplete(o *Operation, gid GoogleID) (string, error) {
	write := o.WriteAccess(gid)
	assignee, err := m.isAssignee(o, gid)
	if err != nil {
		Log.Errorw(err.Error(), "GID", gid, "resource", o.ID, "marker", m)
		return "", err
	}
	if !assignee && !write {
		err := fmt.Errorf("permission denied")
		Log.Errorw(err.Error(), "GID", gid, "resource", o.ID, "marker", m)
		return "", err
	}
	if _, err := db.Exec("UPDATE marker SET state = ?, completedby = NULL WHERE ID = ? AND opID = ?", "assigned", m, o.ID); err != nil {
		Log.Error(err)
		return "", err
	}
	updateID, err := o.Touch()
	o.firebaseMarkerStatus(m, "assigned", updateID)
	return updateID, err
}

// Reject allows an agent to refuse to take a target
// gid must be the assigned agent.
func (m MarkerID) Reject(o *Operation, gid GoogleID) (string, error) {
	assignee, err := m.isAssignee(o, gid)
	if err != nil {
		Log.Errorw(err.Error(), "GID", gid, "resource", o.ID, "marker", m)
		return "", err
	}
	if !assignee {
		err := fmt.Errorf("marker not assigned to you")
		Log.Errorw(err.Error(), "GID", gid, "resource", o.ID, "marker", m)
		return "", err
	}
	if _, err = db.Exec("UPDATE marker SET state = 'pending', gid = NULL WHERE ID = ? AND opID = ?", m, o.ID); err != nil {
		Log.Error(err)
		return "", err
	}
	updateID, err := o.Touch()
	o.firebaseMarkerStatus(m, "pending", updateID)
	return updateID, err
}

// MarkerOrder changes the order of the throws for an operation
func (o *Operation) MarkerOrder(order string, gid GoogleID) (string, error) {
	stmt, err := db.Prepare("UPDATE marker SET oporder = ? WHERE opID = ? AND ID = ?")
	if err != nil {
		Log.Error(err)
		return "", err
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
	return o.Touch()
}

// SetZone sets a marker's zone -- caller must authorize
func (m MarkerID) SetZone(o *Operation, z Zone) (string, error) {
	if _, err := db.Exec("UPDATE marker SET zone = ? WHERE ID = ? AND opID = ?", z, m, o.ID); err != nil {
		Log.Error(err)
		return "", err
	}
	return o.Touch()
}

func NewMarkerType(old MarkerType) string {
	switch old {
	case "CapturePortalMarker":
		return "capture"
	case "LetDecayPortalAlert":
		return "decay"
	case "ExcludeMarker":
		return "exclude"
	case "DestroyPortalAlert":
		return "destroy"
	case "FarmPortalMarker":
		return "farm"
	case "GotoPortalMarker":
		return "goto"
	case "GetKeyPortalMarker":
		return "key"
	case "CreateLinkAlert":
		return "link"
	case "MeetAgentPortalMarker":
		return "meetagent"
	case "OtherPortalAlert":
		return "other"
	case "RechargePortalAlert":
		return "recharge"
	case "UpgradePortalAlert":
		return "upgrade"
	case "UseVirusPortalAlert":
		return "virus"
	}
	return old.String()
}

func (m MarkerID) AddDepend(o *Operation, task string) (string, error) {
	// sanity checks

	_, err := db.Exec("INSERT INTO depends (opID, taskID, dependsOn) VALUES (?, ?, ?)", o.ID, m, task)
	if err != nil {
		Log.Error(err)
		return "", err
	}
	return o.Touch()
}

func (m MarkerID) DelDepend(o *Operation, task string) (string, error) {
	// sanity checks

	_, err := db.Exec("DELETE FROM depends WHERE opID = ? AND taskID = ? AND dependsOn = ?", o.ID, m, task)
	if err != nil {
		Log.Error(err)
		return "", err
	}
	return o.Touch()
}

func (m MarkerID) Depends(o *Operation) ([]TaskID, error) {
	tmp := make([]TaskID, 0)

	var err error
	var rows *sql.Rows

	rows, err = db.Query("SELECT dependsOn FROM depends WHERE opID = ? AND taskID = ? ORDER BY dependsOn", o.ID, m)
	if err != nil {
		Log.Error(err)
		return tmp, err
	}
	defer rows.Close()

	for rows.Next() {
		var t TaskID
		err := rows.Scan(&t)
		if err != nil {
			Log.Error(err)
			continue
		}
		// Log.Infow("marker depends found", "taskID", m, "dependsOn", t)
		tmp = append(tmp, t)
	}
	return tmp, nil
}
