package model

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/wasabee-project/Wasabee-Server/log"
	"github.com/wasabee-project/Wasabee-Server/messaging"
)

// MarkerID wrapper to ensure type safety
type MarkerID string

// MarkerType will be an enum once we figure out the full list
type MarkerType string

// Marker is defined by the Wasabee IITC plugin.
type Marker struct {
	ID         MarkerID   `json:"ID"`
	PortalID   PortalID   `json:"portalId"`
	Type       MarkerType `json:"type"`
	AssignedTo GoogleID   `json:"assignedTo"` // deprecated, use Assignments from Task
	Task
}

// TODO use the logic from insertZone to unify insertMarker and updateMarker

// insertMarkers adds a marker to the database
func (opID OperationID) insertMarker(m Marker) error {
	if m.State == "" {
		m.State = "pending"
	}

	if !m.Zone.Valid() || m.Zone == ZoneAll {
		m.Zone = zonePrimary
	}

	_, err := db.Exec("REPLACE INTO task (ID, opID, comment, taskorder, state, zone, delta) VALUES (?, ?, ?, ?, ?, ?, ?)",
		m.ID, opID, MakeNullString(m.Comment), m.Order, m.State, m.Zone, m.DeltaMinutes)
	if err != nil {
		log.Error(err)
		return err
	}

	_, err = db.Exec("REPLACE INTO marker (ID, opID, PortalID, type) VALUES (?, ?, ?, ?)", m.ID, opID, m.PortalID, m.Type)
	if err != nil {
		log.Error(err)
		return err
	}

	// until clients get updated
	if m.AssignedTo != "" {
		m.Assignments = append(m.Assignments, m.AssignedTo)
	}

	err = m.Assign(m.Assignments, nil)
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
		err := tx.QueryRow("SELECT COUNT(*) FROM assignments WHERE ID = ? AND opID = ? AND gid = ?", m.ID, opID, m.AssignedTo).Scan(&count)
		if err != nil {
			log.Error(err)
			return err
		}
		if count != 1 {
			assignmentChanged = true
		}
	}

	_, err := tx.Exec("REPLACE INTO task (ID, opID, comment, taskorder, state, zone, delta) VALUES (?, ?, ?, ?, ?, ?, ?)",
		m.ID, opID, MakeNullString(m.Comment), m.Order, m.State, m.Zone, m.DeltaMinutes)
	if err != nil {
		log.Error(err)
		return err
	}

	_, err = tx.Exec("REPLACE INTO marker (ID, opID, PortalID, type) VALUES (?, ?, ?, ?)", m.ID, opID, m.PortalID, m.Type)
	if err != nil {
		log.Error(err)
		return err
	}

	if assignmentChanged {
		// until clients get updated
		if m.AssignedTo != "" {
			m.Assignments = append(m.Assignments, m.AssignedTo)
		}

		err = m.Assign(m.Assignments, tx)
		if err != nil {
			log.Error(err)
			return err
		}
		messaging.SendAssignment(messaging.GoogleID(m.AssignedTo), messaging.TaskID(m.ID), messaging.OperationID(m.opID), m.State)
	}

	return nil
}

func (opID OperationID) deleteMarker(mid MarkerID, tx *sql.Tx) error {
	// deleting the task would cascade and take this out... but this is safe
	_, err := tx.Exec("DELETE FROM marker WHERE opID = ? and ID = ?", opID, mid)
	if err != nil {
		log.Error(err)
		return err
	}

	_, err = tx.Exec("DELETE FROM link WHERE opID = ? and ID = ?", opID, mid)
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

	var comment sql.NullString

	var err error
	var rows *sql.Rows
	rows, err = db.Query("SELECT marker.ID, marker.PortalID, marker.type, task.comment, task.state, task.taskorder, task.zone, task.delta FROM marker JOIN task ON marker.ID = task.ID WHERE marker.opID = ? ORDER BY taskorder, type", o.ID)
	if err != nil {
		log.Error(err)
		return err
	}
	defer rows.Close()
	for rows.Next() {
		err := rows.Scan(&tmpMarker.ID, &tmpMarker.PortalID, &tmpMarker.Type, &comment, &tmpMarker.State, &tmpMarker.Order, &tmpMarker.Zone, &tmpMarker.DeltaMinutes)
		if err != nil {
			log.Error(err)
			continue
		}
		// fill in shadowed ID
		tmpMarker.Task.ID = TaskID(tmpMarker.ID)

		if tmpMarker.State == "" { // enums in sql default to "" if invalid, WTF?
			tmpMarker.State = "pending"
		}

		// log.Debugw("marker ID", "id", tmpMarker.ID)

		tmpMarker.Assignments, err = tmpMarker.GetAssignments()
		if err != nil {
			log.Error(err)
			return err
		}
		if len(tmpMarker.Assignments) > 0 {
			// log.Debugw("assignments", "marker", tmpMarker.ID, "assignments", tmpMarker.Assignments)
			tmpMarker.AssignedTo = tmpMarker.Assignments[0]
		} else {
			tmpMarker.AssignedTo = ""
		}

		if comment.Valid {
			tmpMarker.Comment = comment.String
		} else {
			tmpMarker.Comment = ""
		}

		// if the marker is not in the zones with which we are concerned AND not assigned to me, skip
		if !tmpMarker.Zone.inZones(zones) && !tmpMarker.IsAssignedTo(gid) {
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

// GetMarker lookup and return a populated Marker from an id
func (o *Operation) GetMarker(markerID MarkerID) (*Marker, error) {
	if len(o.Markers) == 0 { // XXX not a good test, not all ops have markers
		err := fmt.Errorf("Attempt to use GetMarker on unpopulated *Operation")
		log.Error(err)
		return &Marker{}, err
	}

	for _, m := range o.Markers {
		if m.ID == markerID {
			return &m, nil
		}
	}

	return &Marker{}, fmt.Errorf("marker not found")
}

// MarkerOrder changes the order of the tasks for an operation
func (o *Operation) MarkerOrder(order string) error {
	stmt, err := db.Prepare("UPDATE marker SET taskorder = ? WHERE opID = ? AND ID = ?")
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
