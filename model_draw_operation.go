package wasabee

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// OperationID wrapper to ensure type safety
type OperationID string

// Operation is defined by the Wasabee IITC plugin.
// It is the top level item in the JSON file.
type Operation struct {
	ID        OperationID       `json:"ID"`
	Name      string            `json:"name"`
	Gid       GoogleID          `json:"creator"` // IITC plugin sending agent name, need to convert to GID
	Color     string            `json:"color"`   // could be an enum, but freeform is fine for now
	OpPortals []Portal          `json:"opportals"`
	Anchors   []PortalID        `json:"anchors"` // We should let the clients build this themselves
	Links     []Link            `json:"links"`
	Blockers  []Link            `json:"blockers"` // we ignore this for now
	Markers   []Marker          `json:"markers"`
	Teams     []OpPermission    `json:"teamlist"`
	Modified  string            `json:"modified"`
	Comment   string            `json:"comment"`
	Keys      []KeyOnHand       `json:"keysonhand"`
	Fetched   string            `json:"fetched"`
	Zones     []ZoneListElement `json:"zones"`
}

// OpStat is a minimal struct to determine if the op has been updated
type OpStat struct {
	ID       OperationID `json:"ID"`
	Name     string      `json:"name"`
	Gid      GoogleID    `json:"creator"`
	Modified string      `json:"modified"`
}

// DrawInsert parses a raw op sent from the IITC plugin and stores it in the database
// use ONLY for initial op creation
// All assignment data and key count data is assumed to be correct
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
		if _, err = o.insertKey(k); err != nil {
			Log.Error(err)
			continue
		}
	}

	// pre 0.18 clients do not send zone data
	if len(o.Zones) == 0 {
		o.Zones = defaultZones()
	}
	for _, z := range o.Zones {
		if err = o.insertZone(z); err != nil {
			Log.Error(err)
			continue
		}
	}

	return nil
}

// DrawUpdate is called to UPDATE an existing draw
// Links are added/removed as necessary -- assignments _are_ overwritten
// Markers are added/removed as necessary -- assignments _are_ overwritten
// Key count data is left untouched (unless the portal is no longer listed in the portals list).
func DrawUpdate(opID OperationID, op json.RawMessage, gid GoogleID) (string, error) {
	var o Operation
	if err := json.Unmarshal(op, &o); err != nil {
		Log.Error(err)
		return "", err
	}

	if opID != o.ID {
		err := fmt.Errorf("incoming op.ID does not match the URL specified ID: refusing update")
		Log.Errorw(err.Error(), "resource", opID, "mismatch", opID)
		return "", err
	}

	// ignore incoming team data -- only trust what is stored in DB
	o.Teams = nil

	// this repopulates the team data with what is in the DB
	if !o.WriteAccess(gid) {
		err := fmt.Errorf("write access denied to op: %s", o.ID)
		Log.Error(err)
		return "", err
	}

	if err := drawOpUpdateWorker(o); err != nil {
		Log.Error(err)
		return "", err
	}
	return o.Touch()
}

func drawOpUpdateWorker(o Operation) error {
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
	var pid PortalID
	defer portalRows.Close()
	for portalRows.Next() {
		err := portalRows.Scan(&pid)
		if err != nil {
			Log.Error(err)
			continue
		}
		curPortals[pid] = pid
	}
	// update/add portals that were sent in the update
	portalMap := make(map[PortalID]Portal)
	for _, p := range o.OpPortals {
		portalMap[p.ID] = p
		if err = o.ID.updatePortal(p); err != nil {
			Log.Error(err)
			continue
		}
		delete(curPortals, p.ID)
	}
	// clear portals that were not sent in this update
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
	var mid MarkerID
	defer markerRows.Close()
	for markerRows.Next() {
		err := markerRows.Scan(&mid)
		if err != nil {
			Log.Error(err)
			continue
		}
		curMarkers[mid] = mid
	}
	// add/update markers sent in this update
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
	// remove all markers not sent in this update
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
	var lid LinkID
	defer linkRows.Close()
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

	// pre 0.18 clients do not send zone info
	if len(o.Zones) == 0 {
		o.Zones = defaultZones()
	}
	// update and insert are the saem
	for _, z := range o.Zones {
		if err = o.insertZone(z); err != nil {
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

	// deletedate is automatic
	_, err := db.Exec("INSERT INTO deletedops (opID, deletedate, gid) VALUES (?, NOW(), ?)", o.ID, gid)
	if err != nil {
		Log.Error(err)
		// carry on
	}

	firebaseBroadcastDelete(o.ID)

	_, err = db.Exec("DELETE FROM operation WHERE ID = ?", o.ID)
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

// IsDeletedOp reports back if a particular op has been deleted
func (opID OperationID) IsDeletedOp() bool {
	var i int
	r := db.QueryRow("SELECT COUNT(*) FROM deletedops WHERE opID = ?", opID)
	err := r.Scan(&i)
	if err != nil {
		Log.Error(err)
		return false
	}
	if i > 0 {
		return true
	}
	return false
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

	t := time.Now().UTC()
	o.Fetched = fmt.Sprint(t.Format(time.RFC1123))

	if err := o.PopulateTeams(); err != nil {
		Log.Error(err)
		return err
	}

	read, zones := o.ReadAccess(gid)
	if !read {
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

	// start with everything -- filter after the rest is set up
	if err = o.populatePortals(); err != nil {
		Log.Error(err)
		return err
	}

	if err = o.populateMarkers(zones, gid); err != nil {
		Log.Error(err)
		return err
	}

	if err = o.populateLinks(zones, gid); err != nil {
		Log.Error(err)
		return err
	}

	// built based on available links, zone filtering has taken places
	if err = o.populateAnchors(); err != nil {
		Log.Error(err)
		return err
	}

	if err = o.populateKeys(); err != nil {
		Log.Error(err)
		return err
	}

	// it wouldn't hurt to filter even for ZoneAll
	if !ZoneAll.inZones(zones) {
		// populate portals, links and anchors first
		if err = o.filterPortals(); err != nil {
			Log.Error(err)
			return err
		}
	}

	if err = o.populateZones(); err != nil {
		Log.Error(err)
		return err
	}

	return nil
}

/*
type objectID interface {
	fmt.Stringer
} */

// SetInfo changes the description of an operation
func (o *Operation) SetInfo(info string, gid GoogleID) (string, error) {
	// check isowner (already done in http/pdraw.go, but there may be other callers in the future
	_, err := db.Exec("UPDATE operation SET comment = ? WHERE ID = ?", info, o.ID)
	if err != nil {
		Log.Error(err)
		return "", err
	}
	return o.Touch()
}

// Touch updates the modified timestamp on an operation
func (o *Operation) Touch() (string, error) {
	_, err := db.Exec("UPDATE operation SET modified = UTC_TIMESTAMP() WHERE ID = ?", o.ID)
	if err != nil {
		Log.Error(err)
		return "", err
	}
	updateID := GenerateID(40)

	o.firebaseMapChange(updateID)
	return updateID, nil
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
