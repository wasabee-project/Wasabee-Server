package model

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/wasabee-project/Wasabee-Server/log"
	"github.com/wasabee-project/Wasabee-Server/messaging"
	"github.com/wasabee-project/Wasabee-Server/util"
)

// MarkerID wrapper to ensure type safety
type MarkerID string

// MarkerType will be an enum once we figure out the full list
type MarkerType string

// Marker is defined by the Wasabee IITC plugin.
type Marker struct {
	ID         MarkerID    `json:"ID"`
	PortalID   PortalID    `json:"portalId"`
	Type       MarkerType  `json:"type"`
	AssignedTo GoogleID    `json:"assignedTo,omitempty"` // deprecated, use Assignments from Task
	Attributes []Attribute `json:"attributes,omitempty"`
	Task
}

// AttributeID is the attribute ID
type AttributeID string

// Attribute is per-marker-type data
type Attribute struct {
	ID         AttributeID `json:"ID"`
	MarkerID   MarkerID    `json:"markerID"`
	AssignedTo GoogleID    `json:"assignedTo,omitempty"`
	Name       string      `json:"name"`
	Value      string      `json:"value"`
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

	comment := makeNullString(util.Sanitize(m.Comment))

	_, err := db.Exec("REPLACE INTO task (ID, opID, comment, taskorder, state, zone, delta) VALUES (?, ?, ?, ?, ?, ?, ?)",
		m.ID, opID, comment, m.Order, m.State, m.Zone, m.DeltaMinutes)
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

	// empty m.Assignments clears any
	if err := m.SetAssignments(m.Assignments, nil); err != nil {
		log.Error(err)
		return err
	}

	// len() == 0 could be empty, or could be old client; do not clear them (yet)
	if len(m.DependsOn) > 0 {
		if err := m.SetDepends(m.DependsOn, nil); err != nil {
			log.Error(err)
			return err
		}
	}

	if len(m.Attributes) > 0 {
		if err := m.setAttributes(m.Attributes, nil); err != nil {
			log.Error(err)
			return err
		}
	}

	return nil
}

func (opID OperationID) updateMarker(m Marker, tx *sql.Tx) error {
	if m.State == "" {
		m.State = "pending"
	}

	// copy these values down
	m.Task.ID = TaskID(m.ID)
	m.opID = opID

	if !m.Zone.Valid() || m.Zone == ZoneAll {
		m.Zone = zonePrimary
	}

	// until clients update
	if m.AssignedTo != "" {
		m.Assignments = append(m.Assignments, m.AssignedTo)
	}

	comment := makeNullString(util.Sanitize(m.Comment))

	_, err := tx.Exec("REPLACE INTO task (ID, opID, comment, taskorder, state, zone, delta) VALUES (?, ?, ?, ?, ?, ?, ?)",
		m.ID, opID, comment, m.Order, m.State, m.Zone, m.DeltaMinutes)
	if err != nil {
		log.Error(err)
		return err
	}

	if _, err := tx.Exec("REPLACE INTO marker (ID, opID, PortalID, type) VALUES (?, ?, ?, ?)", m.ID, opID, m.PortalID, m.Type); err != nil {
		log.Error(err)
		return err
	}

	// empty m.Assignments clears any
	if err := m.SetAssignments(m.Assignments, tx); err != nil {
		log.Error(err)
		return err
	}

	// TBD: not spam assignments on updates if nothing has changed
	for _, g := range m.Assignments {
		go messaging.SendAssignment(messaging.GoogleID(g), messaging.TaskID(m.ID), messaging.OperationID(m.opID), m.State)
	}

	// do not clear them if someone is using an old client (yet)
	if len(m.DependsOn) > 0 {
		if err := m.SetDepends(m.DependsOn, tx); err != nil {
			log.Error(err)
			return err
		}
	}

	if len(m.Attributes) > 0 {
		if err := m.setAttributes(m.Attributes, tx); err != nil {
			log.Error(err)
			return err
		}
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
func (o *Operation) populateMarkers(zones []Zone, gid GoogleID, assignments map[TaskID][]GoogleID, depends map[TaskID][]TaskID) error {
	var comment sql.NullString

	rows, err := db.Query("SELECT marker.ID, marker.PortalID, marker.type, task.comment, task.state, task.taskorder, task.zone, task.delta FROM marker JOIN task ON marker.ID = task.ID WHERE marker.opID = ? AND marker.opID = task.opID", o.ID)
	if err != nil {
		log.Error(err)
		return err
	}
	defer rows.Close()

	for rows.Next() {
		tmpMarker := Marker{}
		tmpMarker.opID = o.ID

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

		if a, ok := assignments[tmpMarker.Task.ID]; ok {
			tmpMarker.Assignments = a
			tmpMarker.AssignedTo = a[0]
		}

		if d, ok := depends[tmpMarker.Task.ID]; ok {
			tmpMarker.DependsOn = d
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

		// load attributes
		tmpMarker.loadAttributes()

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
		err := fmt.Errorf(ErrGetMarkerUnpopulated)
		log.Error(err)
		return &Marker{}, err
	}

	for _, m := range o.Markers {
		if m.ID == markerID {
			return &m, nil
		}
	}

	return &Marker{}, fmt.Errorf(ErrMarkerNotFound)
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

// NewMarkerType is used to change from the old to the new marker type names
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

func (m *Marker) setAttributes(a []Attribute, tx *sql.Tx) error {
	needtx := false
	if tx == nil {
		needtx = true
		tx, _ = db.Begin()

		defer func() {
			if err := tx.Rollback(); err != nil && err != sql.ErrTxDone {
				log.Error(err)
			}
		}()
	}

	if _, err := tx.Exec("DELETE FROM markerattributes WHERE opID = ? AND markerID = ?", m.opID, m.ID); err != nil {
		log.Error(err)
		return err
	}

	for _, v := range a {
		if _, err := tx.Exec("INSERT INTO markerattributes (ID, opID, markerID, assignedTo, name, value) VALUES ()", v.ID, m.opID, m.ID, v.AssignedTo, v.Name, v.Value); err != nil {
			log.Error(err)
			continue
		}
	}

	if needtx {
		if err := tx.Commit(); err != nil {
			log.Error(err)
			return err
		}
	}

	return nil
}

func (m *Marker) loadAttributes() error {
	rows, err := db.Query("SELECT ID, assignedTo, name, value FROM markerattributes WHERE opID = ? AND markerID = ?", m.opID, m.ID)
	if err != nil {
		log.Error(err)
		return err
	}
	defer rows.Close()

	for rows.Next() {
		tmp := Attribute{}

		if err := rows.Scan(&tmp.ID, &tmp.AssignedTo, &tmp.Name, &tmp.Value); err != nil {
			log.Error(err)
			continue
		}

		m.Attributes = append(m.Attributes, tmp)
	}
	return nil
}
