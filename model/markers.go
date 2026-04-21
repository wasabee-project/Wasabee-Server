package model

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/wasabee-project/Wasabee-Server/log"
	"github.com/wasabee-project/Wasabee-Server/util"
)

// MarkerID wrapper to ensure type safety
type MarkerID string

// MarkerType for specific marker actions
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
	ID    AttributeID `json:"ID"`
	Name  string      `json:"name"`
	Value string      `json:"value"`
}

// insertMarker adds a marker and its underlying task to the database
func (opID OperationID) insertMarker(ctx context.Context, m Marker, tx *sql.Tx) error {
	if m.State == "" {
		m.State = "pending"
	}

	if !m.Zone.Valid() || m.Zone == ZoneAll {
		m.Zone = zonePrimary
	}

	comment := makeNullString(util.Sanitize(m.Comment))
	executor := txExecutor(tx)

	// 1. Insert into Task table first
	_, err := executor.ExecContext(ctx, "INSERT INTO task (ID, opID, comment, taskorder, state, zone, delta) VALUES (?, ?, ?, ?, ?, ?, ?)",
		m.ID, opID, comment, m.Order, m.State, m.Zone, m.DeltaMinutes)
	if err != nil {
		return err
	}

	// 2. Insert into Marker table
	_, err = executor.ExecContext(ctx, "INSERT INTO marker (ID, opID, PortalID, type) VALUES (?, ?, ?, ?)", m.ID, opID, m.PortalID, m.Type)
	if err != nil {
		return err
	}

	// Legacy support for AssignedTo
	if m.AssignedTo != "" {
		m.Assignments = append(m.Assignments, m.AssignedTo)
	}

	// Set assignments and dependencies using the Task methods (passed through the executor/tx)
	m.Task.ID = TaskID(m.ID)
	m.Task.opID = opID
	if err := m.SetAssignments(ctx, m.Assignments, tx); err != nil {
		return err
	}

	if len(m.DependsOn) > 0 {
		if err := m.SetDepends(ctx, m.DependsOn, tx); err != nil {
			return err
		}
	}

	if len(m.Attributes) > 0 {
		if err := m.setAttributes(ctx, m.Attributes, tx); err != nil {
			return err
		}
	}

	return nil
}

func (opID OperationID) updateMarker(ctx context.Context, m Marker, tx *sql.Tx) error {
	if m.State == "" {
		m.State = "pending"
	}

	m.Task.ID = TaskID(m.ID)
	m.Task.opID = opID

	if !m.Zone.Valid() || m.Zone == ZoneAll {
		m.Zone = zonePrimary
	}

	if m.AssignedTo != "" {
		m.Assignments = append(m.Assignments, m.AssignedTo)
	}

	comment := makeNullString(util.Sanitize(m.Comment))
	executor := txExecutor(tx)

	// Update Task
	_, err := executor.ExecContext(ctx, "INSERT INTO task (ID, opID, comment, taskorder, state, zone, delta) VALUES (?, ?, ?, ?, ?, ?, ?) ON DUPLICATE KEY UPDATE comment = ?, taskorder = ?, state = ?, zone = ?, delta = ?",
		m.ID, opID, comment, m.Order, m.State, m.Zone, m.DeltaMinutes,
		comment, m.Order, m.State, m.Zone, m.DeltaMinutes)
	if err != nil {
		return err
	}

	// Update Marker
	_, err = executor.ExecContext(ctx, "REPLACE INTO marker (ID, opID, PortalID, type) VALUES (?, ?, ?, ?)", m.ID, opID, m.PortalID, m.Type)
	if err != nil {
		return err
	}

	if err := m.SetAssignments(ctx, m.Assignments, tx); err != nil {
		return err
	}

	if len(m.DependsOn) > 0 {
		if err := m.SetDepends(ctx, m.DependsOn, tx); err != nil {
			return err
		}
	}

	if len(m.Attributes) > 0 {
		return m.setAttributes(ctx, m.Attributes, tx)
	}

	return nil
}

func (opID OperationID) deleteMarker(ctx context.Context, mid MarkerID, tx *sql.Tx) error {
	executor := txExecutor(tx)
	// Cascading deletes in the DB should handle the marker table if task is deleted,
	// but we'll be explicit for safety.
	_, err := executor.ExecContext(ctx, "DELETE FROM task WHERE opID = ? and ID = ?", opID, mid)
	return err
}

// populateMarkers fills in the Markers list for the Operation.
func (o *Operation) populateMarkers(ctx context.Context, zones []ZoneID, gid GoogleID, assignments map[TaskID][]GoogleID, depends map[TaskID][]TaskID) error {
	rows, err := db.QueryContext(ctx, "SELECT marker.ID, marker.PortalID, marker.type, task.comment, task.state, task.taskorder, task.zone, task.delta FROM marker JOIN task ON marker.ID = task.ID WHERE marker.opID = ? AND marker.opID = task.opID", o.ID)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var tmpMarker Marker
		tmpMarker.opID = o.ID
		var comment sql.NullString

		if err := rows.Scan(&tmpMarker.ID, &tmpMarker.PortalID, &tmpMarker.Type, &comment, &tmpMarker.State, &tmpMarker.Order, &tmpMarker.Zone, &tmpMarker.DeltaMinutes); err != nil {
			continue
		}

		tmpMarker.Task.ID = TaskID(tmpMarker.ID)
		if tmpMarker.State == "" {
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
		}

		// Authorization/Zone filter
		if !tmpMarker.Zone.inZones(zones) && !tmpMarker.IsAssignedTo(ctx, gid) {
			continue
		}

		_ = tmpMarker.loadAttributes(ctx)
		o.Markers = append(o.Markers, tmpMarker)
	}
	return nil
}

func (m MarkerType) String() string { return string(m) }
func (m MarkerID) String() string   { return string(m) }

// GetMarker returns a populated Marker from the operation cache
func (o *Operation) GetMarker(markerID MarkerID) (*Marker, error) {
	for _, m := range o.Markers {
		if m.ID == markerID {
			return &m, nil
		}
	}
	return nil, errors.New(ErrMarkerNotFound)
}

func (m *Marker) setAttributes(ctx context.Context, a []Attribute, tx *sql.Tx) error {
	executor := txExecutor(tx)
	if _, err := executor.ExecContext(ctx, "DELETE FROM markerattributes WHERE opID = ? AND markerID = ?", m.opID, m.ID); err != nil {
		return err
	}

	for _, v := range a {
		if _, err := executor.ExecContext(ctx, "INSERT INTO markerattributes (ID, opID, markerID, name, value) VALUES (?, ?, ?, ?, ?)", v.ID, m.opID, m.ID, v.Name, v.Value); err != nil {
			log.Error(err)
			continue
		}
	}
	return nil
}

func (m *Marker) loadAttributes(ctx context.Context) error {
	rows, err := db.QueryContext(ctx, "SELECT ID, name, value FROM markerattributes WHERE opID = ? AND markerID = ?", m.opID, m.ID)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var tmp Attribute
		if err := rows.Scan(&tmp.ID, &tmp.Name, &tmp.Value); err == nil {
			m.Attributes = append(m.Attributes, tmp)
		}
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

// MarkerOrder changes the order of the tasks for an operation
func (o *Operation) MarkerOrder(ctx context.Context, order string) error {
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
		if _, err := stmt.ExecContext(ctx, pos, o.ID, markers[i]); err != nil {
			log.Error(err)
			continue
		}
		pos++
	}
	return nil
}
