package wasabee

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	// "html/template"
	// "strings"
	"time"
)

// OperationID wrapper to ensure type safety
type OperationID string

// Operation is defined by the Wasabee IITC plugin.
// It is the top level item in the JSON file.
type Operation struct {
	ID         OperationID    `json:"ID"`
	Name       string         `json:"name"`
	Gid        GoogleID       `json:"creator"` // IITC plugin sending agent name, need to convert to GID
	Color      string         `json:"color"`   // could be an enum, but freeform is fine for now
	OpPortals  []Portal       `json:"opportals"`
	Anchors    []PortalID     `json:"anchors"` // We should let the clients build this themselves
	Links      []Link         `json:"links"`
	Blockers   []Link         `json:"blockers"` // we ignore this for now
	Markers    []Marker       `json:"markers"`
	Teams      []ExtendedTeam `json:"teamlist"`
	Modified   string         `json:"modified"`
	Comment    string         `json:"comment"`
	Keys       []KeyOnHand    `json:"keysonhand"`
	Fetched    string         `json:"fetched"`
	UpdateMode string         `json:"mode,omitempty"`
}

// OpStat is a minimal struct to determine if the op has been updated
type OpStat struct {
	ID       OperationID `json:"ID"`
	Name     string      `json:"name"`
	Gid      GoogleID    `json:"creator"`
	Modified string      `json:"modified"`
}

// ExtendedTeam is the form of permission
type ExtendedTeam struct {
	TeamID TeamID `json:"teamid"`
	Role   etRole `json:"role"`
}

type etRole string

const (
	etRoleRead         etRole = "read"
	etRoleWrite        etRole = "write"
	etRoleAssignedOnly etRole = "assignedonly"
)

func (et etRole) isValid() error {
	switch et {
	case etRoleRead, etRoleWrite, etRoleAssignedOnly:
		return nil
	}
	return errors.New("invalid etRole")
}

// DrawInsert parses a raw op sent from the IITC plugin and stores it in the database
// use ONLY for initial op creation
func DrawInsert(op json.RawMessage, gid GoogleID) error {
	var o Operation
	if err := json.Unmarshal(op, &o); err != nil {
		Log.Error(err)
		return err
	}

	// check to see if this opID is already in use
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM operation WHERE ID = ?", o.ID).Scan(&count)
	if err != nil {
		Log.Error(err)
		return err
	}
	if count != 0 {
		err := fmt.Errorf("attempt to POST to an existing opID; use PUT to update an existing op")
		Log.Errorw(err.Error(), "GID", gid)
		return err
	}

	if err = drawOpInsertWorker(o, gid); err != nil {
		Log.Error(err)
		return err
	}
	return nil
}

func drawOpInsertWorker(o Operation, gid GoogleID) error {
	// start the insert process
	_, err := db.Exec("INSERT INTO operation (ID, name, gid, color, modified, comment) VALUES (?, ?, ?, ?, UTC_TIMESTAMP(), ?)", o.ID, o.Name, gid, o.Color, MakeNullString(o.Comment))
	if err != nil {
		Log.Error(err)
		return err
	}

	portalMap := make(map[PortalID]Portal)
	for _, p := range o.OpPortals {
		portalMap[p.ID] = p
		if err = o.ID.insertPortal(p); err != nil {
			Log.Error(err)
			continue
		}
	}

	for _, m := range o.Markers {
		_, ok := portalMap[m.PortalID]
		if !ok {
			Log.Warnw("portalID missing from portal list", "portal", m.PortalID, "resource", o.ID)
			continue
		}
		if err = o.ID.insertMarker(m); err != nil {
			Log.Error(err)
			continue
		}
	}

	for _, l := range o.Links {
		_, ok := portalMap[l.From]
		if !ok {
			Log.Warnw("source portal missing from portal list", "portal", l.From, "resource", o.ID)
			continue
		}
		_, ok = portalMap[l.To]
		if !ok {
			Log.Warnw("destination portal missing from portal list", "portal", l.To, "resource", o.ID)
			continue
		}
		if err = o.ID.insertLink(l); err != nil {
			Log.Error(err)
			continue
		}
	}

	for _, k := range o.Keys {
		if err = o.insertKey(k); err != nil {
			Log.Error(err)
			continue
		}
	}

	return nil
}

// DrawUpdate is called to UPDATE an existing draw
// in order to minimize races between the various writers, the following conditions are enforced
//
// Active mode
// Links are added/removed as necessary -- assignments and status are not overwritten (deleting a link removes the assignment/status)
// Markers are added/removed as necessary -- assignments and status are not overwritten (deleting the marker removes the assignment/status)
//
// Design mode: default
// Links are added/removed as necessary -- assignments _are_ overwritten
// Markers are added/removed as necessary -- assignments _are_ overwritten
//
// Key count data is left untouched (unless the portal is no longer listed in the portals list).
func DrawUpdate(opID OperationID, op json.RawMessage, gid GoogleID) error {
	var o Operation
	if err := json.Unmarshal(op, &o); err != nil {
		Log.Error(err)
		return err
	}

	if opID != o.ID {
		err := fmt.Errorf("incoming op.ID does not match the URL specified ID: refusing update")
		Log.Errorw(err.Error(), "resource", opID, "mismatch", opID)
		return err
	}

	// ignore incoming team data
	o.Teams = nil
	if !o.WriteAccess(gid) {
		err := fmt.Errorf("write access denied to op: %s", o.ID)
		Log.Error(err)
		return err
	}

	if err := drawOpUpdateWorker(o); err != nil {
		Log.Error(err)
		return err
	}

	if err := o.Touch(); err != nil {
		Log.Error(err)
		return err
	}
	return nil
}

func drawOpUpdateWorker(o Operation) error {
	// designMode := true
	if o.UpdateMode == "active" {
		// designMode = false
		Log.Debugf("activeMode update")
	}

	_, err := db.Exec("UPDATE operation SET name = ?, color = ?, comment = ? WHERE ID = ?",
		o.Name, o.Color, MakeNullString(o.Comment), o.ID)
	if err != nil {
		Log.Error(err)
		return err
	}

	// get the current portal list and stash in map
	curPortals := make(map[PortalID]PortalID)
	portalRows, err := db.Query("SELECT ID FROM portal WHERE OpID = ?", o.ID)
	if err != nil {
		Log.Error(err)
		return err
	}
	defer portalRows.Close()
	var pid PortalID
	for portalRows.Next() {
		err := portalRows.Scan(&pid)
		if err != nil {
			Log.Error(err)
			continue
		}
		curPortals[pid] = pid
	}
	// update/add portals
	portalMap := make(map[PortalID]Portal)
	for _, p := range o.OpPortals {
		portalMap[p.ID] = p
		if err = o.ID.updatePortal(p); err != nil {
			Log.Error(err)
			continue
		}
		delete(curPortals, p.ID)
	}
	// clear portals that are no longer used
	for k := range curPortals {
		err := o.ID.deletePortal(k)
		if err != nil {
			Log.Error(err)
			continue
		}
	}

	curMarkers := make(map[MarkerID]MarkerID)
	markerRows, err := db.Query("SELECT ID FROM marker WHERE OpID = ?", o.ID)
	if err != nil {
		Log.Error(err)
		return err
	}
	defer markerRows.Close()
	var mid MarkerID
	for markerRows.Next() {
		err := markerRows.Scan(&mid)
		if err != nil {
			Log.Error(err)
			continue
		}
		curMarkers[mid] = mid
	}
	for _, m := range o.Markers {
		_, ok := portalMap[m.PortalID]
		if !ok {
			Log.Warnw("portal missing from portal list", "marker", m.PortalID, "resource", o.ID)
			continue
		}
		if err = o.ID.updateMarker(m); err != nil {
			Log.Error(err)
			continue
		}
		delete(curMarkers, m.ID)
	}
	for k := range curMarkers {
		err = o.ID.deleteMarker(k)
		if err != nil {
			Log.Error(err)
			continue
		}
	}

	curLinks := make(map[LinkID]LinkID)
	linkRows, err := db.Query("SELECT ID FROM link WHERE OpID = ?", o.ID)
	if err != nil {
		Log.Error(err)
		return err
	}
	defer linkRows.Close()
	var lid LinkID
	for linkRows.Next() {
		err := linkRows.Scan(&lid)
		if err != nil {
			Log.Error(err)
			continue
		}
		curLinks[lid] = lid
	}
	for _, l := range o.Links {
		_, ok := portalMap[l.From]
		if !ok {
			Log.Warnw("source portal missing from portal list", "portal", l.From, "resource", o.ID)
			continue
		}
		_, ok = portalMap[l.To]
		if !ok {
			Log.Warnw("destination portal missing from portal list", "portal", l.To, "resource", o.ID)
			continue
		}
		if err = o.ID.updateLink(l); err != nil {
			Log.Error(err)
			continue
		}
		delete(curLinks, l.ID)
	}
	for k := range curLinks {
		err = o.ID.deleteLink(k)
		if err != nil {
			Log.Error(err)
			continue
		}
	}

	// XXX TBD remove unused opkey portals?

	return nil
}

// Delete removes an operation and all associated data
func (o *Operation) Delete(gid GoogleID) error {
	if !o.ID.IsOwner(gid) {
		err := fmt.Errorf("permission denied")
		Log.Error(err)
		return err
	}

	_, err := db.Exec("DELETE FROM operation WHERE ID = ?", o.ID)
	if err != nil {
		Log.Error(err)
		return err
	}
	// the foreign key constraints should take care of these, but just in case...
	_, _ = db.Exec("DELETE FROM marker WHERE opID = ?", o.ID)
	_, _ = db.Exec("DELETE FROM link WHERE opID = ?", o.ID)
	_, _ = db.Exec("DELETE FROM portal WHERE opID = ?", o.ID)
	// XXX not needed going forward, but leaving for now
	_, _ = db.Exec("DELETE FROM anchor WHERE opID = ?", o.ID)
	_, _ = db.Exec("DELETE FROM opkeys WHERE opID = ?", o.ID)
	_, _ = db.Exec("DELETE FROM opteams WHERE opID = ?", o.ID)

	return nil
}

// Populate takes a pointer to an Operation and fills it in; o.ID must be set
// checks to see that either the gid created the operation or the gid is on the team assigned to the operation
func (o *Operation) Populate(gid GoogleID) error {
	var comment sql.NullString
	// permission check and populate Operation top level
	r := db.QueryRow("SELECT name, gid, color, modified, comment FROM operation WHERE ID = ?", o.ID)
	err := r.Scan(&o.Name, &o.Gid, &o.Color, &o.Modified, &comment)

	if err != nil && err == sql.ErrNoRows {
		err = fmt.Errorf("operation not found")
		Log.Error(err.Error(), "resource", o.ID, "GID", gid)
		return err
	}
	if err != nil {
		Log.Error(err)
		return err
	}

	if err := o.PopulateTeams(); err != nil {
		Log.Error(err)
		return err
	}

	if !o.ReadAccess(gid) {
		if o.AssignedOnlyAccess(gid) {
			var a Assignments
			err = gid.Assignments(o.ID, &a)
			if err != nil {
				Log.Error(err)
				return err
			}
			for _, p := range a.Portals {
				o.OpPortals = append(o.OpPortals, p)
			}
			o.Markers = append(o.Markers, a.Markers...)
			o.Links = append(o.Links, a.Links...)

			return nil
		}
		return fmt.Errorf("unauthorized: you are not on a team authorized to see this operation (%s: %s)", gid, o.ID)
	}

	if comment.Valid {
		o.Comment = comment.String
	}

	if err = o.populatePortals(); err != nil {
		Log.Error(err)
		return err
	}

	if err = o.populateMarkers(); err != nil {
		Log.Error(err)
		return err
	}

	if err = o.populateLinks(); err != nil {
		Log.Error(err)
		return err
	}

	if err = o.populateAnchors(); err != nil {
		Log.Error(err)
		return err
	}

	if err = o.populateKeys(); err != nil {
		Log.Error(err)
		return err
	}
	t := time.Now()
	o.Fetched = fmt.Sprint(t.Format(time.RFC1123))

	return nil
}

/*
type objectID interface {
	fmt.Stringer
} */

// SetInfo changes the description of an operation
func (o *Operation) SetInfo(info string, gid GoogleID) error {
	// check isowner (already done in http/pdraw.go, but there may be other callers in the future
	_, err := db.Exec("UPDATE operation SET comment = ? WHERE ID = ?", info, o.ID)
	if err != nil {
		Log.Error(err)
		return err
	}
	if err = o.Touch(); err != nil {
		Log.Error(err)
	}
	return nil
}

// Touch updates the modified timestamp on an operation
func (o *Operation) Touch() error {
	_, err := db.Exec("UPDATE operation SET modified = UTC_TIMESTAMP() WHERE ID = ?", o.ID)
	if err != nil {
		Log.Error(err)
		return err
	}

	o.firebaseMapChange()
	return nil
}

// Stat returns useful info on an operation
func (opID OperationID) Stat() (OpStat, error) {
	var s OpStat
	s.ID = opID
	err := db.QueryRow("SELECT name, gid, modified FROM operation WHERE ID = ?", opID).Scan(&s.Name, &s.Gid, &s.Modified)
	if err != nil && err != sql.ErrNoRows {
		Log.Error(err)
		return s, err
	}
	if err != nil && err == sql.ErrNoRows {
		err = fmt.Errorf("no such operation")
		Log.Warnw(err.Error(), "resource", opID)
		return s, err
	}
	return s, nil
}

// Rename changes an op's name
func (opID OperationID) Rename(gid GoogleID, name string) error {
	if !opID.IsOwner(gid) {
		err := fmt.Errorf("permission denied")
		Log.Warnw(err.Error(), "GID", gid, "resource", opID)
		return err
	}

	if name == "" {
		err := fmt.Errorf("invalid name")
		Log.Warnw(err.Error(), "GID", gid, "resource", opID)
		return err
	}

	_, err := db.Exec("UPDATE operation SET name = ? WHERE ID = ?", name, opID)
	if err != nil {
		Log.Error(err)
		return err
	}
	return nil
}

// Copy duplicates an operation and returns the new op's ID
func (o *Operation) Copy(gid GoogleID, complete bool) (OperationID, error) {
	var new Operation

	if o.Name == "" { // not populated
		err := o.Populate(gid)
		if err != nil {
			Log.Error(err)
			return "", err
		}
	}

	tmpid, err := GenerateSafeName()
	if err != nil {
		Log.Error(err)
		return "", err
	}
	new.ID = OperationID(tmpid)
	new.Name = fmt.Sprintf("%s %s", o.Name, "COPY")
	new.Color = o.Color
	copy(new.OpPortals, o.OpPortals)
	copy(new.Anchors, o.Anchors)
	copy(new.Links, o.Links)
	copy(new.Markers, o.Markers)
	// should this be in the complete section?
	copy(new.Teams, o.Teams)

	if !complete {
		for _, l := range new.Links {
			l.AssignedTo = ""
		}
		for _, m := range new.Markers {
			m.AssignedTo = ""
		}

		if err = drawOpInsertWorker(new, gid); err != nil {
			Log.Error(err)
			return "", err
		}
	}

	return new.ID, nil
}

// OpValidColor just returns a prebuilt color list for drawing color menus in the UI
// XXX this is now completely overkill
func opValidColor(color string) string {
	type opColor struct {
		Name string
		Hex  string
	}

	ocm := map[string]opColor{
		"main":   {"Op Color", "ff0000"},
		"groupa": {"Orange", "ff6600"},
		"groupb": {"Light Orange", "ff9900"},
		"groupc": {"Tan", "bb9900"},
		"groupd": {"Purple", "bb22cc"},
		"groupe": {"Teal", "33cccc"},
		"groupf": {"Pink", "ff55ff"},
	}

	if _, ok := ocm[color]; ok {
		return color
	}
	return "main"
}
