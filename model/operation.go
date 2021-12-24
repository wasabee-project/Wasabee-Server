package model

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/wasabee-project/Wasabee-Server/generatename"
	"github.com/wasabee-project/Wasabee-Server/log"
)

// OperationID wrapper to ensure type safety
type OperationID string

// Operation is defined by the Wasabee IITC plugin.
// It is the top level item in the JSON file.
type Operation struct {
	ID        OperationID `json:"ID"`      // 40-char string
	Name      string      `json:"name"`    // freeform
	Gid       GoogleID    `json:"creator"` // IITC plugin sends agent name on first upload, we convert to GID
	Color     string      `json:"color"`   // now free-form
	OpPortals []Portal    `json:"opportals"`
	Anchors   []PortalID  `json:"anchors"` // We should let the clients build this themselves
	Links     []Link      `json:"links"`
	// Blockers   []Link            `json:"blockers"` // ignored by Wasabee-Server -- do not store this
	Markers       []Marker          `json:"markers"`
	Teams         []OpPermission    `json:"teamlist"`
	Modified      string            `json:"modified"`      // time.RFC1123 format
	LastEditID    string            `json:"lasteditid"`    // 40-char string, generated by Touch()
	ReferenceTime string            `json:"referencetime"` // time.RFC1123 format
	Comment       string            `json:"comment"`
	Keys          []KeyOnHand       `json:"keysonhand"`
	Fetched       string            `json:"fetched"` // time.RFC1123 format
	Zones         []ZoneListElement `json:"zones"`
}

// OpStat is a minimal struct to determine if the op has been updated
type OpStat struct {
	ID         OperationID `json:"ID"`
	Name       string      `json:"name"`
	Gid        GoogleID    `json:"creator"`
	Modified   string      `json:"modified"` // time.RFC1123 format
	LastEditID string      `json:"lasteditid"`
}

// DrawInsert parses a raw op sent from the IITC plugin and stores it in the database
// use ONLY for initial op creation
// All assignment data and key count data is assumed to be correct
func DrawInsert(op json.RawMessage, gid GoogleID) error {
	var o Operation
	if err := json.Unmarshal(op, &o); err != nil {
		log.Error(err)
		return err
	}

	// check to see if this opID is already in use
	var count int
	err := db.QueryRow("SELECT COUNT(ID) FROM operation WHERE ID = ?", o.ID).Scan(&count)
	if err != nil {
		log.Error(err)
		return err
	}
	if count != 0 {
		err := fmt.Errorf("attempt to POST to an existing opID; use PUT to update an existing op")
		log.Infow(err.Error(), "GID", gid)
		return err
	}
	if o.ID.IsDeletedOp() {
		err := fmt.Errorf("attempt to reuse a deleted opID; duplicate and upload the copy instead")
		log.Infow(err.Error(), "GID", gid, "opID", o.ID)
		return err
	}

	if err = drawOpInsertWorker(&o, gid); err != nil {
		log.Error(err)
		return err
	}
	return nil
}

func drawOpInsertWorker(o *Operation, gid GoogleID) error {
	// convert from RFC1123 to SQL format
	reftime, err := time.Parse(time.RFC1123, o.ReferenceTime)
	if err != nil {
		// log.Debugw(err.Error(), "message", "bad reference time, defaulting to now()")
		reftime = time.Now()
	}

	// start the insert process
	_, err = db.Exec("INSERT INTO operation (ID, name, gid, color, modified, comment, referencetime) VALUES (?, ?, ?, ?, UTC_TIMESTAMP(), ?, ?)", o.ID, o.Name, gid, o.Color, MakeNullString(o.Comment), reftime.Format("2006-01-02 15:04:05"))
	if err != nil {
		log.Error(err)
		return err
	}

	portalMap := make(map[PortalID]bool)
	for _, p := range o.OpPortals {
		portalMap[p.ID] = true
		if err = o.ID.insertPortal(p); err != nil {
			log.Error(err)
			continue
		}
	}

	for _, m := range o.Markers {
		m.opID = o.ID

		_, ok := portalMap[m.PortalID]
		if !ok {
			log.Warnw("portalID missing from portal list", "portal", m.PortalID, "resource", o.ID)
			continue
		}

		// assignments here would be invalid since teams are not known
		m.AssignedTo = ""
		if err = o.ID.insertMarker(m); err != nil {
			log.Error(err)
			continue
		}
	}

	for _, l := range o.Links {
		l.opID = o.ID

		_, ok := portalMap[l.From]
		if !ok {
			log.Warnw("source portal missing from portal list", "portal", l.From, "resource", o.ID)
			continue
		}
		_, ok = portalMap[l.To]
		if !ok {
			log.Warnw("destination portal missing from portal list", "portal", l.To, "resource", o.ID)
			continue
		}
		// assignments here would be invalid since teams are not known
		l.AssignedTo = ""
		if err = o.ID.insertLink(l); err != nil {
			log.Error(err)
			continue
		}
	}

	for _, k := range o.Keys {
		if err := o.insertKey(k); err != nil {
			log.Error(err)
			continue
		}
	}

	// pre 0.18 clients do not send zone data
	if len(o.Zones) == 0 {
		o.Zones = defaultZones()
	}
	for _, z := range o.Zones {
		if err = o.insertZone(z, nil); err != nil {
			log.Error(err)
			continue
		}
	}

	return nil
}

// DrawUpdate is called to UPDATE an existing draw
// Links are added/removed as necessary -- assignments _are_ overwritten
// Markers are added/removed as necessary -- assignments _are_ overwritten
// Key count data is left untouched (unless the portal is no longer listed in the portals list).
func DrawUpdate(opID OperationID, op json.RawMessage, gid GoogleID) error {
	var o Operation
	if err := json.Unmarshal(op, &o); err != nil {
		log.Error(err)
		return err
	}

	if opID != o.ID {
		err := fmt.Errorf("incoming op.ID does not match the URL specified ID: refusing update")
		log.Errorw(err.Error(), "resource", opID, "mismatch", opID)
		return err
	}

	if o.ID.IsDeletedOp() {
		err := fmt.Errorf("attempt to update a deleted opID; duplicate and upload the copy instead")
		log.Infow(err.Error(), "GID", gid, "opID", o.ID)
		return err
	}

	// ignore incoming team data -- only trust what is stored in DB
	o.Teams = nil

	// this repopulates the team data with what is in the DB
	if !o.WriteAccess(gid) {
		err := fmt.Errorf("write access denied to op: %s", o.ID)
		log.Error(err)
		return err
	}

	if err := drawOpUpdateWorker(&o); err != nil {
		log.Error(err)
		return err
	}
	return nil
}

func drawOpUpdateWorker(o *Operation) error {
	// log.Debug("op update locking")
	if _, err := db.Exec("SELECT GET_LOCK(?,1)", o.ID); err != nil {
		log.Error(err)
		return err
	}
	defer func() {
		if _, err := db.Exec("SELECT RELEASE_LOCK(?)", o.ID); err != nil {
			log.Error(err)
		}
		// log.Debug("op update unlocking")
	}()

	tx, err := db.Begin()
	if err != nil {
		log.Error(err)
		return err
	}

	defer func() {
		err := tx.Rollback()
		if err != nil && err != sql.ErrTxDone {
			log.Error(err)
		}
	}()

	reftime, err := time.Parse(time.RFC1123, o.ReferenceTime)
	if err != nil {
		// log.Debugw(err.Error(), "message", "bad reference time, defaulting to now()")
		reftime = time.Now()
	}

	_, err = tx.Exec("UPDATE operation SET name = ?, color = ?, comment = ?, referencetime = ? WHERE ID = ?",
		o.Name, o.Color, MakeNullString(o.Comment), reftime.Format("2006-01-02 15:04:05"), o.ID)
	if err != nil {
		log.Error(err)
		return err
	}

	portalMap, err := drawOpUpdatePortals(o, tx)
	if err != nil {
		log.Error(err)
		return err
	}

	agentMap, err := allOpAgents(o.Teams, tx)
	if err != nil {
		log.Error(err)
		return err
	}

	// clear here, add back in individual tasks
	_, err = tx.Exec("DELETE FROM depends WHERE opID = ?", o.ID)
	if err != nil {
		log.Error(err)
		return err
	}

	if err := drawOpUpdateMarkers(o, portalMap, agentMap, tx); err != nil {
		log.Error(err)
		return err
	}

	if err := drawOpUpdateLinks(o, portalMap, agentMap, tx); err != nil {
		log.Error(err)
		return err
	}

	if err := drawOpUpdateZones(o, tx); err != nil {
		log.Error(err)
		return err
	}

	if err := tx.Commit(); err != nil {
		log.Error(err)
		return err
	}

	// XXX TBD remove unused opkey portals?
	return nil
}

func drawOpUpdatePortals(o *Operation, tx *sql.Tx) (map[PortalID]Portal, error) {
	// get the current portal list and stash in map
	curPortals := make(map[PortalID]bool)
	portalRows, err := tx.Query("SELECT ID FROM portal WHERE OpID = ?", o.ID)
	if err != nil {
		log.Error(err)
		return nil, err
	}
	defer portalRows.Close()

	for portalRows.Next() {
		var pid PortalID
		err := portalRows.Scan(&pid)
		if err != nil {
			log.Error(err)
			continue
		}
		curPortals[pid] = true
	}

	// update/add portals that were sent in the update
	portalMap := make(map[PortalID]Portal)
	for _, p := range o.OpPortals {
		portalMap[p.ID] = p
		if err = o.ID.updatePortal(p, tx); err != nil {
			log.Error(err)
			continue
		}
		delete(curPortals, p.ID)
	}
	// clear portals that were not sent in this update
	for k := range curPortals {
		if err := o.ID.deletePortal(k, tx); err != nil {
			log.Error(err)
			continue
		}
	}
	return portalMap, nil
}

func drawOpUpdateMarkers(o *Operation, portalMap map[PortalID]Portal, agentMap map[GoogleID]bool, tx *sql.Tx) error {
	curMarkers := make(map[MarkerID]bool)
	markerRows, err := tx.Query("SELECT ID FROM marker WHERE OpID = ?", o.ID)
	if err != nil {
		log.Error(err)
		return err
	}
	defer markerRows.Close()

	for markerRows.Next() {
		var mid MarkerID
		err := markerRows.Scan(&mid)
		if err != nil {
			log.Error(err)
			continue
		}
		curMarkers[mid] = true
	}
	// add/update markers sent in this update
	for _, m := range o.Markers {
		m.opID = o.ID

		_, ok := portalMap[m.PortalID]
		if !ok {
			log.Warnw("portal missing from portal list", "marker", m.PortalID, "resource", o.ID)
			continue
		}
		_, ok = agentMap[GoogleID(m.AssignedTo)]
		if !ok {
			// log.Debugw("marker assigned to agent not on any current team", "marker", m.PortalID, "resource", o.ID)
			m.AssignedTo = ""
		}
		if err = o.ID.updateMarker(m, tx); err != nil {
			log.Error(err)
			continue
		}
		delete(curMarkers, m.ID)
	}
	// remove all markers not sent in this update
	for k := range curMarkers {
		err = o.ID.deleteMarker(k, tx)
		if err != nil {
			log.Error(err)
			continue
		}
	}
	return nil
}

func drawOpUpdateLinks(o *Operation, portalMap map[PortalID]Portal, agentMap map[GoogleID]bool, tx *sql.Tx) error {
	curLinks := make(map[LinkID]bool)
	linkRows, err := tx.Query("SELECT ID FROM link WHERE OpID = ?", o.ID)
	if err != nil {
		log.Error(err)
		return err
	}
	defer linkRows.Close()

	for linkRows.Next() {
		var lid LinkID
		err := linkRows.Scan(&lid)
		if err != nil {
			log.Error(err)
			continue
		}
		curLinks[lid] = true
	}
	for _, l := range o.Links {
		l.opID = o.ID

		_, ok := portalMap[l.From]
		if !ok {
			log.Warnw("source portal missing from portal list", "portal", l.From, "resource", o.ID)
			continue
		}
		_, ok = portalMap[l.To]
		if !ok {
			log.Warnw("destination portal missing from portal list", "portal", l.To, "resource", o.ID)
			continue
		}
		_, ok = agentMap[GoogleID(l.AssignedTo)]
		if !ok {
			// log.Debugw("link assigned to agent not on any current team", "link", l.ID, "resource", o.ID)
			l.AssignedTo = ""
		}
		if err = o.ID.updateLink(l, tx); err != nil {
			log.Error(err)
			continue
		}
		delete(curLinks, l.ID)
	}
	for k := range curLinks {
		if err = o.ID.deleteLink(k, tx); err != nil {
			log.Error(err)
			continue
		}
	}
	return nil
}

func drawOpUpdateZones(o *Operation, tx *sql.Tx) error {
	// pre 0.18 clients do not send zone info
	if len(o.Zones) == 0 {
		o.Zones = defaultZones()
	}

	curZones := make(map[Zone]bool)
	zoneRows, err := tx.Query("SELECT ID from zone WHERE OpID = ?", o.ID)
	if err != nil {
		log.Error(err)
		return err
	}
	defer zoneRows.Close()

	for zoneRows.Next() {
		var zoneID Zone
		err := zoneRows.Scan(&zoneID)
		if err != nil {
			log.Error(err)
			return err
		}
		curZones[zoneID] = true
	}

	// update and insert are the saem
	for _, z := range o.Zones {
		if err := o.insertZone(z, tx); err != nil {
			log.Error(err)
			continue
		}
		delete(curZones, z.Zone)
	}
	for k := range curZones {
		if err = o.ID.deleteZone(k, tx); err != nil {
			log.Error(err)
			continue
		}
	}

	return nil
}

// Delete removes an operation and all associated data
func (o *Operation) Delete(gid GoogleID) error {
	if !o.IsOwner(gid) {
		err := fmt.Errorf("permission denied")
		log.Error(err)
		return err
	}

	_, err := db.Exec("INSERT INTO deletedops (opID, deletedate, gid) VALUES (?, UTC_TIMESTAMP(), ?)", o.ID, gid)
	if err != nil {
		log.Error(err)
		// carry on
	}

	_, err = db.Exec("DELETE FROM operation WHERE ID = ?", o.ID)
	if err != nil {
		log.Error(err)
		return err
	}

	// the foreign key constraints should take care of these, but just in case...
	tables := []string{"marker", "link", "portal", "anchor", "opkeys", "permissions"}
	for _, v := range tables {
		q := fmt.Sprintf("DELETE FROM %s WHERE opID = ?", v)
		if _, err = db.Exec(q, o.ID); err != nil {
			log.Info(err)
			// carry on
		}
	}

	return nil
}

// IsDeletedOp reports back if a particular op has been deleted
func (opID OperationID) IsDeletedOp() bool {
	var i int
	r := db.QueryRow("SELECT COUNT(*) FROM deletedops WHERE opID = ?", opID)
	err := r.Scan(&i)
	if err != nil {
		log.Error(err)
		return false
	}
	return (i > 0)
}

// Populate takes a pointer to an Operation and fills it in; o.ID must be set
// checks to see that either the gid created the operation or the gid is on the team assigned to the operation
func (o *Operation) Populate(gid GoogleID) error {
	var comment sql.NullString
	err := db.QueryRow("SELECT name, gid, color, modified, comment, lasteditid, referencetime FROM operation WHERE ID = ?", o.ID).Scan(&o.Name, &o.Gid, &o.Color, &o.Modified, &comment, &o.LastEditID, &o.ReferenceTime)
	if err != nil && err == sql.ErrNoRows {
		err = fmt.Errorf("operation not found")
		log.Errorw(err.Error(), "resource", o.ID, "GID", gid, "opID", o.ID)
		return err
	}
	if err != nil {
		log.Error(err)
		return err
	}

	o.Fetched = fmt.Sprint(time.Now().UTC().Format(time.RFC1123))
	// convert from SQL to RFC1123 for start time
	st, err := time.ParseInLocation("2006-01-02 15:04:05", o.ReferenceTime, time.UTC)
	if err != nil {
		log.Error(err)
		return err
	}
	o.ReferenceTime = st.Format(time.RFC1123)

	if comment.Valid {
		o.Comment = comment.String
	} else {
		o.Comment = ""
	}

	// ReadAccess will do this if we don't, but this is a harmless redundancy since it won't double-query (unless no permissions are set)
	if err := o.PopulateTeams(); err != nil {
		log.Error(err)
		return err
	}

	read, zones := o.ReadAccess(gid)
	assignedOnly := o.AssignedOnlyAccess(gid)
	if !read {
		if assignedOnly {
			zones = []Zone{ZoneAssignOnly}
		} else {
			return fmt.Errorf("unauthorized: you are not on a team authorized to see this full operation (%s: %s)", gid, o.ID)
		}
	}

	// get all the assignments in a single query, so we don't lock up the database when one agent requests 50 ops, each with hundreds of links
	assignments, err := o.ID.assignmentPrecache()
	if err != nil {
		log.Error(err)
		return err
	}

	// same for depends
	depends, err := o.ID.dependsPrecache()
	if err != nil {
		log.Error(err)
		return err
	}

	// start with everything -- filter after the rest is set up
	if err = o.populatePortals(); err != nil {
		log.Error(err)
		return err
	}

	if err = o.populateMarkers(zones, gid, assignments, depends); err != nil {
		log.Error(err)
		return err
	}

	if err = o.populateLinks(zones, gid, assignments, depends); err != nil {
		log.Error(err)
		return err
	}

	if err = o.populateAnchors(); err != nil {
		log.Error(err)
		return err
	}

	if assignedOnly {
		if err = o.populateMyKeys(gid); err != nil {
			log.Error(err)
			return err
		}
	} else {
		if err = o.populateKeys(); err != nil {
			log.Error(err)
			return err
		}
	}

	if !ZoneAll.inZones(zones) {
		if err = o.filterPortals(); err != nil {
			log.Error(err)
			return err
		}
	}

	if err = o.populateZones(); err != nil {
		log.Error(err)
		return err
	}
	return nil
}

// SetInfo changes the description of an operation
func (o *Operation) SetInfo(info string, gid GoogleID) error {
	_, err := db.Exec("UPDATE operation SET comment = ? WHERE ID = ?", info, o.ID)
	if err != nil {
		log.Error(err)
		return err
	}
	return nil
}

// Touch updates the modified timestamp on an operation
func (o *Operation) Touch() (string, error) {
	updateID := generatename.GenerateID(40)

	_, err := db.Exec("UPDATE operation SET modified = UTC_TIMESTAMP(), lasteditid = ? WHERE ID = ?", updateID, o.ID)
	if err != nil {
		log.Error(err)
		return "", err
	}

	log.Debugw("touch", "updateID", updateID)
	return updateID, nil
}

// Stat returns useful info on an operation
func (opID OperationID) Stat() (*OpStat, error) {
	s := OpStat{}
	s.ID = opID
	err := db.QueryRow("SELECT name, gid, modified, lasteditid FROM operation WHERE ID = ?", opID).Scan(&s.Name, &s.Gid, &s.Modified, &s.LastEditID)
	if err != nil && err != sql.ErrNoRows {
		log.Error(err)
		return &s, err
	}
	if err != nil && err == sql.ErrNoRows {
		err = fmt.Errorf("no such operation")
		log.Warnw(err.Error(), "resource", opID)
		return &s, err
	}
	return &s, nil
}

// Rename changes an op's name
func (opID OperationID) Rename(gid GoogleID, name string) error {
	if !opID.IsOwner(gid) {
		err := fmt.Errorf("permission denied")
		log.Warnw(err.Error(), "GID", gid, "resource", opID)
		return err
	}

	if name == "" {
		err := fmt.Errorf("invalid name")
		log.Warnw(err.Error(), "GID", gid, "resource", opID)
		return err
	}

	_, err := db.Exec("UPDATE operation SET name = ? WHERE ID = ?", name, opID)
	if err != nil {
		log.Error(err)
		return err
	}
	return nil
}

func allOpAgents(perms []OpPermission, tx *sql.Tx) (map[GoogleID]bool, error) {
	am := make(map[GoogleID]bool)

	for _, p := range perms {
		rows, err := tx.Query("SELECT gid FROM agentteams WHERE teamID = ?", p.TeamID)
		if err != nil {
			log.Error(err)
			continue
		}
		defer rows.Close()

		for rows.Next() {
			var gid GoogleID
			if err := rows.Scan(&gid); err != nil {
				log.Error(err)
				continue
			}
			am[gid] = true
		}
	}
	return am, nil
}
