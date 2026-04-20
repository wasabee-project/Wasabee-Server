package model

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/wasabee-project/Wasabee-Server/log"
	"github.com/wasabee-project/Wasabee-Server/util"
)

// PortalID wrapper to ensure type safety
type PortalID string

// Portal is defined by the Wasabee IITC plugin.
type Portal struct {
	ID       PortalID `json:"id"`
	Name     string   `json:"name"`
	Lat      string   `json:"lat"` // passing these as strings matches typical JSON input from IITC
	Lon      string   `json:"lng"`
	Comment  string   `json:"comment"`
	Hardness string   `json:"hardness"`
	opID     OperationID
}

// insertPortal adds a portal to the database
func (opID OperationID) insertPortal(ctx context.Context, p Portal, tx *sql.Tx) error {
	comment := makeNullString(util.Sanitize(p.Comment))
	hardness := makeNullString(util.Sanitize(p.Hardness))

	executor := txExecutor(tx)
	_, err := executor.ExecContext(ctx, "INSERT IGNORE INTO portal (ID, opID, name, loc, comment, hardness) VALUES (?, ?, ?, POINT(?, ?), ?, ?)",
		p.ID, opID, p.Name, p.Lon, p.Lat, comment, hardness)
	if err != nil {
		log.Error(err)
	}
	return err
}

func (opID OperationID) updatePortal(ctx context.Context, p Portal, tx *sql.Tx) error {
	comment := makeNullString(util.Sanitize(p.Comment))
	hardness := makeNullString(util.Sanitize(p.Hardness))

	executor := txExecutor(tx)
	// REPLACE works here because we expect the IITC plugin to be the source of truth for portal metadata
	_, err := executor.ExecContext(ctx, "REPLACE INTO portal (ID, opID, name, loc, comment, hardness) VALUES (?, ?, ?, POINT(?, ?), ?, ?)",
		p.ID, opID, p.Name, p.Lon, p.Lat, comment, hardness)
	if err != nil {
		log.Error(err)
	}
	return err
}

func (opID OperationID) deletePortal(ctx context.Context, p PortalID, tx *sql.Tx) error {
	executor := txExecutor(tx)
	_, err := executor.ExecContext(ctx, "DELETE FROM portal WHERE ID = ? AND opID = ?", p, opID)
	return err
}

// populatePortals fills in the OpPortals list for the Operation.
func (o *Operation) populatePortals(ctx context.Context) error {
	rows, err := db.QueryContext(ctx, "SELECT ID, name, ST_Y(loc) AS lat, ST_X(loc) AS lon, comment, hardness FROM portal WHERE opID = ?", o.ID)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var p Portal
		p.opID = o.ID
		var comment, hardness sql.NullString

		if err := rows.Scan(&p.ID, &p.Name, &p.Lat, &p.Lon, &comment, &hardness); err != nil {
			log.Error(err)
			continue
		}
		if comment.Valid {
			p.Comment = comment.String
		}
		if hardness.Valid {
			p.Hardness = hardness.String
		}

		o.OpPortals = append(o.OpPortals, p)
	}
	return nil
}

// filterPortals reduces the portal list to only those used by anchors, markers, or keys
func (o *Operation) filterPortals() error {
	set := make(map[PortalID]Portal)

	// Anchors
	for _, a := range o.Anchors {
		if p, err := o.getPortal(a); err == nil {
			set[a] = p
		}
	}
	// Markers
	for _, m := range o.Markers {
		if p, err := o.getPortal(m.PortalID); err == nil {
			set[m.PortalID] = p
		}
	}
	// Keys
	for _, k := range o.Keys {
		if p, err := o.getPortal(k.ID); err == nil {
			set[k.ID] = p
		}
	}

	filteredList := make([]Portal, 0, len(set))
	for _, p := range set {
		filteredList = append(filteredList, p)
	}

	o.OpPortals = filteredList
	return nil
}

func (o *Operation) populateAnchors() error {
	set := make(map[PortalID]bool)
	for _, l := range o.Links {
		set[l.From] = true
		set[l.To] = true
	}

	o.Anchors = make([]PortalID, 0, len(set))
	for key := range set {
		o.Anchors = append(o.Anchors, key)
	}
	return nil
}

func (p PortalID) String() string {
	return string(p)
}

// PortalHardness updates the hardness on a portal
func (opID OperationID) PortalHardness(ctx context.Context, portalID PortalID, hardness string) error {
	h := makeNullString(util.Sanitize(hardness))
	_, err := db.ExecContext(ctx, "UPDATE portal SET hardness = ? WHERE ID = ? AND opID = ?", h, portalID, opID)
	return err
}

// PortalComment updates the comment on a portal
func (opID OperationID) PortalComment(ctx context.Context, portalID PortalID, comment string) error {
	c := makeNullString(util.Sanitize(comment))
	_, err := db.ExecContext(ctx, "UPDATE portal SET comment = ? WHERE ID = ? AND opID = ?", c, portalID, opID)
	return err
}

// PortalDetails returns information about the portal with access checking
func (o *Operation) PortalDetails(ctx context.Context, portalID PortalID, gid GoogleID) (*Portal, error) {
	read, _ := o.ReadAccess(ctx, gid)
	if !read {
		log.Errorw("unauthorized portal details access", "GID", gid, "resource", o.ID, "portal", portalID)
		return nil, errors.New("unauthorized")
	}

	return o.portalDetails(ctx, portalID, nil)
}

// internal transaction-safe version
func (o *Operation) portalDetails(ctx context.Context, portalID PortalID, tx *sql.Tx) (*Portal, error) {
	var p Portal
	p.ID = portalID
	p.opID = o.ID

	var comment, hardness sql.NullString
	executor := txExecutor(tx)
	err := executor.QueryRowContext(ctx, "SELECT name, ST_Y(loc) AS lat, ST_X(loc) AS lon, comment, hardness FROM portal WHERE opID = ? AND ID = ?", o.ID, portalID).
		Scan(&p.Name, &p.Lat, &p.Lon, &comment, &hardness)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("portal %s not in op", portalID)
		}
		return nil, err
	}

	if comment.Valid {
		p.Comment = comment.String
	}
	if hardness.Valid {
		p.Hardness = hardness.String
	}
	return &p, nil
}

// getPortal looks up a portal in the already-populated operation struct
func (o *Operation) getPortal(portalID PortalID) (Portal, error) {
	for _, p := range o.OpPortals {
		if p.ID == portalID {
			return p, nil
		}
	}
	return Portal{}, fmt.Errorf("portal %s not found in operation cache", portalID)
}
