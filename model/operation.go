package model

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/wasabee-project/Wasabee-Server/log"
	"github.com/wasabee-project/Wasabee-Server/util"
)

// OperationID wrapper to ensure type safety
type OperationID string

// Operation is defined by the Wasabee IITC plugin.
type Operation struct {
	ID            OperationID       `json:"ID"`
	Name          string            `json:"name"`
	Gid           GoogleID          `json:"creator"`
	Color         string            `json:"color"`
	OpPortals     []Portal          `json:"opportals"`
	Anchors       []PortalID        `json:"anchors"`
	Links         []Link            `json:"links"`
	Markers       []Marker          `json:"markers"`
	Teams         []OpPermission    `json:"teamlist"`
	Modified      string            `json:"modified"`
	LastEditID    string            `json:"lasteditid"`
	ReferenceTime string            `json:"referencetime"`
	Comment       string            `json:"comment"`
	Keys          []KeyOnHand       `json:"keysonhand"`
	Fetched       string            `json:"fetched"`
	Zones         []ZoneListElement `json:"zones"`
}

// OpStat is a minimal struct to determine if the op has been updated
type OpStat struct {
	ID         OperationID `json:"ID"`
	Name       string      `json:"name"`
	Gid        GoogleID    `json:"creator"`
	Modified   string      `json:"modified"`
	LastEditID string      `json:"lasteditid"`
}

// DrawInsert parses a raw op and stores it
func DrawInsert(ctx context.Context, o *Operation, gid GoogleID) error {
	if o.ID.Valid(ctx) {
		err := errors.New("attempt to create an opID that is already in use")
		log.Infow(err.Error(), "GID", gid, "opID", o.ID)
		return err
	}

	if o.ID.IsDeletedOp(ctx) {
		err := errors.New("attempt to reuse a deleted opID; duplicate and upload the copy instead")
		log.Infow(err.Error(), "GID", gid, "opID", o.ID)
		return err
	}

	reftime, err := time.Parse(time.RFC1123, o.ReferenceTime)
	if err != nil {
		reftime = time.Now()
	}

	comment := makeNullString(util.Sanitize(o.Comment))

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		log.Error(err)
		return err
	}

	defer func() {
		err := tx.Rollback()
		if err != nil && !errors.Is(err, sql.ErrTxDone) {
			log.Error(err)
		}
	}()

	_, err = tx.ExecContext(ctx, "INSERT INTO operation (ID, name, gid, color, modified, comment, referencetime) VALUES (?, ?, ?, ?, UTC_TIMESTAMP(), ?, ?)", o.ID, o.Name, gid, o.Color, comment, reftime.Format("2006-01-02 15:04:05"))
	if err != nil {
		log.Error(err)
		return err
	}

	portalMap := make(map[PortalID]bool)
	for _, p := range o.OpPortals {
		portalMap[p.ID] = true
		if err = o.ID.insertPortal(ctx, p, tx); err != nil {
			return err
		}
	}

	for _, m := range o.Markers {
		m.opID = o.ID
		if !portalMap[m.PortalID] {
			err := errors.New("attempt to add marker to unknown portal")
			log.Warnw(err.Error(), "portal", m.PortalID, "resource", o.ID)
			return err
		}
		m.AssignedTo = ""
		if err = o.ID.insertMarker(ctx, m, tx); err != nil {
			return err
		}
	}

	for _, l := range o.Links {
		l.opID = o.ID
		if !portalMap[l.From] || !portalMap[l.To] {
			err := errors.New("attempt to link unknown portals")
			log.Warnw(err.Error(), "resource", o.ID)
			return err
		}
		l.AssignedTo = ""
		if err = o.ID.insertLink(ctx, l, tx); err != nil {
			return err
		}
	}

	for _, k := range o.Keys {
		if err := o.insertKey(ctx, k, tx); err != nil {
			return err
		}
	}

	if len(o.Zones) == 0 {
		o.Zones = defaultZones()
	}
	for _, z := range o.Zones {
		if err = o.insertZone(ctx, z, tx); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// DrawUpdate updates an existing draw
func DrawUpdate(ctx context.Context, o *Operation, gid GoogleID) error {
	if o.ID.IsDeletedOp(ctx) {
		err := errors.New("attempt to update a deleted opID")
		return err
	}

	if !o.ID.Valid(ctx) {
		return errors.New("update op.ID does not exist")
	}

	o.Teams = nil
	// Note: WriteAccess will need to be updated to take ctx
	if !o.WriteAccess(ctx, gid) {
		return fmt.Errorf("write access denied to op: %s", o.ID)
	}

	if _, err := db.ExecContext(ctx, "SELECT GET_LOCK(?,1)", o.ID); err != nil {
		log.Error(err)
		return err
	}
	defer func() {
		if _, err := db.ExecContext(context.Background(), "SELECT RELEASE_LOCK(?)", o.ID); err != nil {
			log.Error(err)
		}
	}()

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		log.Error(err)
		return err
	}

	defer func() {
		err := tx.Rollback()
		if err != nil && !errors.Is(err, sql.ErrTxDone) {
			log.Error(err)
		}
	}()

	reftime, err := time.Parse(time.RFC1123, o.ReferenceTime)
	if err != nil {
		reftime = time.Now()
	}

	comment := makeNullString(util.Sanitize(o.Comment))
	_, err = tx.ExecContext(ctx, "UPDATE operation SET name = ?, color = ?, comment = ?, referencetime = ? WHERE ID = ?",
		o.Name, o.Color, comment, reftime.Format("2006-01-02 15:04:05"), o.ID)
	if err != nil {
		log.Error(err)
		return err
	}

	portalMap, err := drawOpUpdatePortals(ctx, o, tx)
	if err != nil {
		return err
	}

	agentMap, err := allOpAgents(ctx, o.Teams, tx)
	if err != nil {
		return err
	}

	if err := drawOpUpdateMarkers(ctx, o, portalMap, agentMap, tx); err != nil {
		return err
	}

	if err := drawOpUpdateLinks(ctx, o, portalMap, agentMap, tx); err != nil {
		return err
	}

	if err := drawOpUpdateZones(ctx, o, tx); err != nil {
		return err
	}

	return tx.Commit()
}

func drawOpUpdatePortals(ctx context.Context, o *Operation, tx *sql.Tx) (map[PortalID]Portal, error) {
	curPortals := make(map[PortalID]bool)
	portalRows, err := tx.QueryContext(ctx, "SELECT ID FROM portal WHERE OpID = ?", o.ID)
	if err != nil {
		return nil, err
	}
	defer portalRows.Close()

	for portalRows.Next() {
		var pid PortalID
		if err := portalRows.Scan(&pid); err != nil {
			continue
		}
		curPortals[pid] = true
	}

	portalMap := make(map[PortalID]Portal)
	for _, p := range o.OpPortals {
		portalMap[p.ID] = p
		if err = o.ID.updatePortal(ctx, p, tx); err != nil {
			return portalMap, err
		}
		delete(curPortals, p.ID)
	}
	for k := range curPortals {
		if err := o.ID.deletePortal(ctx, k, tx); err != nil {
			return portalMap, err
		}
	}
	return portalMap, nil
}

func drawOpUpdateMarkers(ctx context.Context, o *Operation, portalMap map[PortalID]Portal, agentMap map[GoogleID]bool, tx *sql.Tx) error {
	curMarkers := make(map[MarkerID]bool)
	markerRows, err := tx.QueryContext(ctx, "SELECT ID FROM marker WHERE OpID = ?", o.ID)
	if err != nil {
		return err
	}
	defer markerRows.Close()

	for markerRows.Next() {
		var mid MarkerID
		if err := markerRows.Scan(&mid); err != nil {
			continue
		}
		curMarkers[mid] = true
	}
	for _, m := range o.Markers {
		m.opID = o.ID
		if _, ok := portalMap[m.PortalID]; !ok {
			log.Warnw("attempt to add marker to unknown portal", "marker", m.PortalID, "resource", o.ID)
			return errors.New("unknown portal")
		}

		if err := o.ID.updateMarker(ctx, m, tx); err != nil {
			return err
		}
		delete(curMarkers, m.ID)
	}

	for k := range curMarkers {
		if err := o.ID.deleteMarker(ctx, k, tx); err != nil {
			return err
		}
	}
	return nil
}

func drawOpUpdateLinks(ctx context.Context, o *Operation, portalMap map[PortalID]Portal, agentMap map[GoogleID]bool, tx *sql.Tx) error {
	curLinks := make(map[LinkID]bool)
	linkRows, err := tx.QueryContext(ctx, "SELECT ID FROM link WHERE OpID = ?", o.ID)
	if err != nil {
		return err
	}
	defer linkRows.Close()

	for linkRows.Next() {
		var lid LinkID
		if err := linkRows.Scan(&lid); err != nil {
			continue
		}
		curLinks[lid] = true
	}
	for _, l := range o.Links {
		l.opID = o.ID
		if _, ok := portalMap[l.From]; !ok {
			return errors.New("missing source portal")
		}
		if _, ok := portalMap[l.To]; !ok {
			return errors.New("missing target portal")
		}

		if err = o.ID.updateLink(ctx, l, tx); err != nil {
			return err
		}
		delete(curLinks, l.ID)
	}
	for k := range curLinks {
		if err = o.ID.deleteLink(ctx, k, tx); err != nil {
			return err
		}
	}
	return nil
}

func drawOpUpdateZones(ctx context.Context, o *Operation, tx *sql.Tx) error {
	if len(o.Zones) == 0 {
		o.Zones = defaultZones()
	}

	curZones := make(map[ZoneID]bool)
	zoneRows, err := tx.QueryContext(ctx, "SELECT ID from zone WHERE OpID = ?", o.ID)
	if err != nil {
		return err
	}
	defer zoneRows.Close()

	for zoneRows.Next() {
		var zoneID ZoneID
		if err := zoneRows.Scan(&zoneID); err != nil {
			return err
		}
		curZones[zoneID] = true
	}

	for _, z := range o.Zones {
		if err := o.insertZone(ctx, z, tx); err != nil {
			continue
		}
		delete(curZones, z.Zone)
	}
	for k := range curZones {
		if err = o.ID.deleteZone(ctx, k, tx); err != nil {
			continue
		}
	}
	return nil
}

// Delete removes an operation
func (o *Operation) Delete(ctx context.Context, gid GoogleID) error {
	if !o.ID.IsOwner(ctx, gid) {
		return errors.New("permission denied")
	}

	_, err := db.ExecContext(ctx, "INSERT INTO deletedops (opID, deletedate, gid) VALUES (?, UTC_TIMESTAMP(), ?)", o.ID, gid)
	if err != nil {
		log.Error(err)
	}

	_, err = db.ExecContext(ctx, "DELETE FROM operation WHERE ID = ?", o.ID)
	if err != nil {
		log.Error(err)
		return err
	}

	tables := []string{"marker", "link", "portal", "opkeys", "permissions"}
	for _, v := range tables {
		q := fmt.Sprintf("DELETE FROM %s WHERE opID = ?", v)
		if _, err = db.ExecContext(ctx, q, o.ID); err != nil {
			log.Info(err)
		}
	}

	return nil
}

// IsDeletedOp reports if an op is deleted
func (opID OperationID) IsDeletedOp(ctx context.Context) bool {
	var i int
	err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM deletedops WHERE opID = ?", opID).Scan(&i)
	if err != nil {
		log.Error(err)
		return false
	}
	return (i > 0)
}

// Populate fills in the Operation struct
func (o *Operation) Populate(ctx context.Context, gid GoogleID) error {
	var comment sql.NullString
	err := db.QueryRowContext(ctx, "SELECT name, gid, color, modified, comment, lasteditid, referencetime FROM operation WHERE ID = ?", o.ID).
		Scan(&o.Name, &o.Gid, &o.Color, &o.Modified, &comment, &o.LastEditID, &o.ReferenceTime)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errors.New(ErrOpNotFound)
		}
		return err
	}

	o.Fetched = time.Now().UTC().Format(time.RFC1123)
	st, err := time.ParseInLocation(time.RFC3339, o.ReferenceTime, time.UTC)
	if err != nil {
		log.Error(err)
		return err
	}
	o.ReferenceTime = st.Format(time.RFC1123)

	if comment.Valid {
		o.Comment = comment.String
	}

	if err := o.PopulateTeams(ctx); err != nil {
		return err
	}

	read, zones := o.ReadAccess(ctx, gid)
	assignedOnly := o.AssignedOnlyAccess(ctx, gid)
	if !read {
		if assignedOnly {
			zones = []ZoneID{ZoneAssignOnly}
		} else {
			return fmt.Errorf("unauthorized access to op %s by %s", o.ID, gid)
		}
	}

	assignments, err := o.ID.assignmentPrecache(ctx)
	if err != nil {
		return err
	}

	depends, err := o.ID.dependsPrecache(ctx)
	if err != nil {
		return err
	}

	if err = o.populatePortals(ctx); err != nil {
		return err
	}

	if err = o.populateMarkers(ctx, zones, gid, assignments, depends); err != nil {
		return err
	}

	if err = o.populateLinks(ctx, zones, gid, assignments, depends); err != nil {
		return err
	}

	if err = o.populateAnchors(); err != nil {
		return err
	}

	if assignedOnly {
		err = o.populateMyKeys(ctx, gid)
	} else {
		err = o.populateKeys(ctx)
	}
	if err != nil {
		return err
	}

	if !ZoneAll.inZones(zones) {
		if err = o.filterPortals(); err != nil { // filterPortals is usually in-memory
			return err
		}
	}

	return o.populateZones(ctx)
}

// SetInfo changes the description
func (o *Operation) SetInfo(ctx context.Context, info string) error {
	_, err := db.ExecContext(ctx, "UPDATE operation SET comment = ? WHERE ID = ?", info, o.ID)
	return err
}

// Touch updates modified timestamp
func (o *Operation) Touch(ctx context.Context) (string, error) {
	updateID := util.GenerateID(40)
	_, err := db.ExecContext(ctx, "UPDATE operation SET modified = UTC_TIMESTAMP(), lasteditid = ? WHERE ID = ?", updateID, o.ID)
	return updateID, err
}

// Stat returns op info
func (opID OperationID) Stat(ctx context.Context) (*OpStat, error) {
	s := &OpStat{ID: opID}
	err := db.QueryRowContext(ctx, "SELECT name, gid, modified, lasteditid FROM operation WHERE ID = ?", opID).
		Scan(&s.Name, &s.Gid, &s.Modified, &s.LastEditID)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return s, errors.New("no such operation")
		}
		return s, err
	}
	return s, nil
}

// Rename changes op name
func (opID OperationID) Rename(ctx context.Context, gid GoogleID, name string) error {
	if !opID.IsOwner(ctx, gid) {
		return errors.New("permission denied")
	}
	if name == "" {
		return errors.New("invalid name")
	}
	_, err := db.ExecContext(ctx, "UPDATE operation SET name = ? WHERE ID = ?", name, opID)
	return err
}

func allOpAgents(ctx context.Context, perms []OpPermission, tx *sql.Tx) (map[GoogleID]bool, error) {
	am := make(map[GoogleID]bool)
	for _, p := range perms {
		rows, err := tx.QueryContext(ctx, "SELECT gid FROM agentteams WHERE teamID = ?", p.TeamID)
		if err != nil {
			continue
		}
		defer rows.Close()

		for rows.Next() {
			var gid GoogleID
			if err := rows.Scan(&gid); err == nil {
				am[gid] = true
			}
		}
	}
	return am, nil
}

func (opID OperationID) Valid(ctx context.Context) bool {
	var count int
	err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM operation WHERE ID = ?", opID).Scan(&count)
	return err == nil && count == 1
}

func (opID OperationID) Get(ctx context.Context, gid GoogleID) (*Operation, error) {
	var o Operation
	o.ID = opID
	err := o.Populate(ctx, gid)
	return &o, err
}
