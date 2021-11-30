package model

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/wasabee-project/Wasabee-Server/Firebase"
	"github.com/wasabee-project/Wasabee-Server/log"
)

// MarkerID wrapper to ensure type safety
type MarkerID string

// MarkerType will be an enum once we figure out the full list
type MarkerType string

// Marker is defined by the Wasabee IITC plugin.
type Marker struct {
	ID           MarkerID    `json:"ID"`
	PortalID     PortalID    `json:"portalId"`
	Type         MarkerType  `json:"type"`
	Comment      string      `json:"comment"`
	AssignedTo   GoogleID    `json:"assignedTo"`
	AssignedTeam TeamID      `json:"assignedTeam"`
	CompletedID  GoogleID    `json:"completedID"`
	State        string      `json:"state"`
	Order        int         `json:"order"`
	Zone         Zone        `json:"zone"`
	DeltaMinutes int         `json:"deltaminutes"`
	opID         OperationID `json:"_"`
	Task
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
		log.Error(err)
		return err
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
			log.Error(err)
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
		log.Error(err)
		return err
	}

	if assignmentChanged {
		wfb.AssignMarker(wfb.GoogleID(m.AssignedTo), wfb.TaskID(m.ID), wfb.OperationID(m.opID), m.State)
	}

	return nil
}

func (opID OperationID) deleteMarker(mid MarkerID, tx *sql.Tx) error {
	_, err := tx.Exec("DELETE FROM marker WHERE opID = ? and ID = ?", opID, mid)
	if err != nil {
		log.Error(err)
		return err
	}
	return nil
}

// PopulateMarkers fills in the Markers list for the Operation.
func (o *Operation) populateMarkers(zones []Zone, gid GoogleID) error {
	var tmpMarker Marker
	tmpMarker.opID = o.ID

	var assignedGid, comment, completedID sql.NullString

	var err error
	var rows *sql.Rows
	rows, err = db.Query("SELECT ID, PortalID, type, gid, comment, state, oporder, completedby AS completedID, zone, delta FROM marker WHERE opID = ? ORDER BY oporder, type", o.ID)
	if err != nil {
		log.Error(err)
		return err
	}
	defer rows.Close()
	for rows.Next() {
		err := rows.Scan(&tmpMarker.ID, &tmpMarker.PortalID, &tmpMarker.Type, &assignedGid, &comment, &tmpMarker.State, &tmpMarker.Order, &completedID, &tmpMarker.Zone, &tmpMarker.DeltaMinutes)
		if err != nil {
			log.Error(err)
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

// Assign assigns a marker to an agent, sending them a message
func (m Marker) Assign(gid GoogleID) error {
	// unassign
	if gid == "0" {
		gid = ""
	}

	_, err := db.Exec("UPDATE marker SET gid = ?, state = ? WHERE ID = ? AND opID = ?", MakeNullString(gid), "assigned", m.ID, m.opID)
	if err != nil {
		log.Error(err)
		return err
	}

	if gid.String() != "" {
		wfb.AssignMarker(wfb.GoogleID(gid), wfb.TaskID(m.ID), wfb.OperationID(m.opID), "assigned")
	}
	return err
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
func (m Marker) Claim(gid GoogleID) error {
	if m.AssignedTo != "" {
		err := fmt.Errorf("can only claim unassigned markers")
		log.Errorw(err.Error(), "GID", gid, "resource", m.opID, "marker", m)
		return err
	}

	return m.Assign(gid)
}

// MarkerComment updates the comment on a marker
func (m Marker) SetComment(comment string) error {
	if _, err := db.Exec("UPDATE marker SET comment = ? WHERE ID = ? AND opID = ?", MakeNullString(comment), m.ID, m.opID); err != nil {
		log.Error(err)
		return err
	}
	return nil
}

// Zone updates the marker's zone
func (m Marker) SetZone(z Zone) error {
	if _, err := db.Exec("UPDATE marker SET zone = ? WHERE ID = ? AND opID = ?", z, m.ID, m.opID); err != nil {
		log.Error(err)
		return err
	}
	return nil
}

// Delta updates the marker's DeltaMinutes
func (m Marker) Delta(delta int) error {
	if _, err := db.Exec("UPDATE marker SET delta = ? WHERE ID = ? AND opID = ?", delta, m.ID, m.opID); err != nil {
		log.Error(err)
		return err
	}
	return nil
}

func (m Marker) isAssignee(gid GoogleID) (bool, error) {
	// also searching for GID and doing count(*) might be faster
	var ns sql.NullString
	err := db.QueryRow("SELECT gid FROM marker WHERE ID = ? and opID = ?", m.ID, m.opID).Scan(&ns)
	if err != nil && err != sql.ErrNoRows {
		log.Error(err)
		return false, err
	}
	if err != nil && err == sql.ErrNoRows {
		err = fmt.Errorf("no such marker")
		log.Warnw(err.Error(), "resource", m.opID, "marker", m)
		return false, err
	}
	if !ns.Valid {
		err = fmt.Errorf("marker not assigned")
		log.Warnw(err.Error(), "resource", m.opID, "marker", m)
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
func (m Marker) Acknowledge(gid GoogleID) error {
	assignee, err := m.isAssignee(gid)
	if err != nil {
		log.Warnw(err.Error(), "resource", m.opID, "marker", m)
		return err
	}
	if !assignee {
		err := fmt.Errorf("marker not assigned to you")
		log.Warnw(err.Error(), "resource", m.opID, "marker", m)
		return err
	}
	if _, err = db.Exec("UPDATE marker SET state = ? WHERE ID = ? AND opID = ?", "acknowledged", m.ID, m.opID); err != nil {
		log.Error(err)
		return err
	}

	teams, err := m.opID.Teams()
	if err != nil {
		log.Error(err)
		return err
	}
	for t := range teams {
		wfb.MarkerStatus(string(m.ID), string(m.opID), string(t), "acknowledged")
	}
	return nil
}

// Complete marks a marker as completed
func (m Marker) Complete(gid GoogleID) error {
	write := m.opID.WriteAccess(gid)
	assignee, err := m.isAssignee(gid)
	if err != nil {
		log.Errorw(err.Error(), "GID", gid, "resource", m.opID, "marker", m)
		return err
	}
	if !assignee && !write {
		err := fmt.Errorf("permission denied")
		log.Errorw(err.Error(), "GID", gid, "resource", m.opID, "marker", m)
		return err
	}
	if _, err := db.Exec("UPDATE marker SET state = ?, completedby = ? WHERE ID = ? AND opID = ?", "completed", gid, m.ID, m.opID); err != nil {
		log.Error(err)
		return err
	}
	teams, err := m.opID.Teams()
	if err != nil {
		log.Error(err)
		return err
	}
	for t := range teams {
		wfb.MarkerStatus(string(m.ID), string(m.opID), string(t), "completed")
	}
	return nil
}

// Incomplete marks a marker as not-completed
func (m Marker) Incomplete(gid GoogleID) error {
	write := m.opID.WriteAccess(gid)
	assignee, err := m.isAssignee(gid)
	if err != nil {
		log.Errorw(err.Error(), "GID", gid, "resource", m.opID, "marker", m)
		return err
	}
	if !assignee && !write {
		err := fmt.Errorf("permission denied")
		log.Errorw(err.Error(), "GID", gid, "resource", m.opID, "marker", m)
		return err
	}
	if _, err := db.Exec("UPDATE marker SET state = ?, completedby = NULL WHERE ID = ? AND opID = ?", "assigned", m.ID, m.opID); err != nil {
		log.Error(err)
		return err
	}
	teams, err := m.opID.Teams()
	if err != nil {
		log.Error(err)
		return err
	}
	for t := range teams {
		wfb.MarkerStatus(string(m.ID), string(m.opID), string(t), "incomplete")
	}
	return nil
}

// Reject allows an agent to refuse to take a target
// gid must be the assigned agent.
func (m Marker) Reject(gid GoogleID) error {
	assignee, err := m.isAssignee(gid)
	if err != nil {
		log.Errorw(err.Error(), "GID", gid, "resource", m.opID, "marker", m)
		return err
	}
	if !assignee {
		err := fmt.Errorf("marker not assigned to you")
		log.Errorw(err.Error(), "GID", gid, "resource", m.opID, "marker", m)
		return err
	}
	if _, err = db.Exec("UPDATE marker SET state = 'pending', gid = NULL WHERE ID = ? AND opID = ?", m.ID, m.opID); err != nil {
		log.Error(err)
		return err
	}
	teams, err := m.opID.Teams()
	if err != nil {
		log.Error(err)
		return err
	}
	for t := range teams {
		wfb.MarkerStatus(string(m.ID), string(m.opID), string(t), "reject")
	}
	return nil
}

// MarkerOrder changes the order of the tasks for an operation
func (o *Operation) MarkerOrder(order string) error {
	stmt, err := db.Prepare("UPDATE marker SET oporder = ? WHERE opID = ? AND ID = ?")
	if err != nil {
		log.Error(err)
		return err
	}

	pos := 1
	markers := strings.Split(order, ",")
	for i := range markers {
		if markers[i] == "000" { // the header, could be any place in the order if the user was being silly
			continue
		}
		if _, err := stmt.Exec(pos, o.ID, markers[i]); err != nil {
			log.Error(err)
			continue
		}
		pos++
	}
	return nil
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
