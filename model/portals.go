package model

import (
	"database/sql"
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
	Lat      string   `json:"lat"` // passing these as strings saves me parsing them
	Lon      string   `json:"lng"`
	Comment  string   `json:"comment"`
	Hardness string   `json:"hardness"` // string for now, enum in the future
	opID     OperationID
}

// insertPortal adds a portal to the database
func (opID OperationID) insertPortal(p Portal, tx *sql.Tx) error {
	comment := makeNullString(util.Sanitize(p.Comment))
	hardness := makeNullString(util.Sanitize(p.Hardness))

	_, err := tx.Exec("INSERT IGNORE INTO portal (ID, opID, name, loc, comment, hardness) VALUES (?, ?, ?, POINT(?, ?), ?, ?)",
		p.ID, opID, p.Name, p.Lon, p.Lat, comment, hardness)
	if err != nil {
		log.Error(err)
		return err
	}
	return nil
}

func (opID OperationID) updatePortal(p Portal, tx *sql.Tx) error {
	comment := makeNullString(util.Sanitize(p.Comment))
	hardness := makeNullString(util.Sanitize(p.Hardness))

	_, err := tx.Exec("REPLACE INTO portal (ID, opID, name, loc, comment, hardness) VALUES (?, ?, ?, POINT(?, ?), ?, ?)", // REPLACE OK SCB (so long as any task is rebuilt after)
		p.ID, opID, p.Name, p.Lon, p.Lat, comment, hardness)
	if err != nil {
		log.Error(err)
		return err
	}
	return nil
}

func (opID OperationID) deletePortal(p PortalID, tx *sql.Tx) error {
	_, err := tx.Exec("DELETE FROM portal WHERE ID = ? AND opID = ?", p, opID)
	if err != nil {
		log.Error(err)
		return err
	}
	return nil
}

// PopulatePortals fills in the OpPortals list for the Operation. No authorization takes place.
func (o *Operation) populatePortals() error {
	var p Portal
	p.opID = o.ID

	rows, err := db.Query("SELECT ID, name, Y(loc) AS lat, X(loc) AS lon, comment, hardness FROM portal WHERE opID = ?", o.ID)
	if err != nil {
		log.Error(err)
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var comment, hardness sql.NullString

		err := rows.Scan(&p.ID, &p.Name, &p.Lat, &p.Lon, &comment, &hardness)
		if err != nil {
			log.Error(err)
			continue
		}
		if comment.Valid {
			p.Comment = comment.String
		} else {
			p.Comment = ""
		}
		if hardness.Valid {
			p.Hardness = hardness.String
		} else {
			p.Hardness = ""
		}

		o.OpPortals = append(o.OpPortals, p)
	}
	return nil
}

// reduce the portal list to keys, links and makers in this zone
func (o *Operation) filterPortals() error {
	var filteredList []Portal

	set := make(map[PortalID]Portal)
	for _, a := range o.Anchors {
		p, _ := o.getPortal(a)
		set[a] = p
	}

	for _, m := range o.Markers {
		p, _ := o.getPortal(m.PortalID)
		set[m.PortalID] = p
	}

	for _, k := range o.Keys {
		p, _ := o.getPortal(k.ID)
		set[k.ID] = p
	}

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

	for key := range set {
		o.Anchors = append(o.Anchors, key)
	}
	return nil
}

// String returns the string version of a PortalID
func (p PortalID) String() string {
	return string(p)
}

// PortalHardness updates the comment on a portal
func (opID OperationID) PortalHardness(portalID PortalID, hardness string) error {
	h := makeNullString(util.Sanitize(hardness))

	_, err := db.Exec("UPDATE portal SET hardness = ? WHERE ID = ? AND opID = ?", h, portalID, opID)
	if err != nil {
		log.Error(err)
		return err
	}
	return nil
}

// PortalComment updates the comment on a portal
func (opID OperationID) PortalComment(portalID PortalID, comment string) error {
	c := makeNullString(util.Sanitize(comment))

	_, err := db.Exec("UPDATE portal SET comment = ? WHERE ID = ? AND opID = ?", c, portalID, opID)
	if err != nil {
		log.Error(err)
		return err
	}
	return nil
}

// PortalDetails returns information about the portal
// does access checking (cached)
func (o *Operation) PortalDetails(portalID PortalID, gid GoogleID) (*Portal, error) {
	var p Portal
	p.ID = portalID
	p.opID = o.ID

	if read, _ := o.ReadAccess(gid); !read {
		err := fmt.Errorf("unauthorized: unable to get portal details")
		log.Errorw(err.Error(), "GID", gid, "resource", o.ID, "portal", portalID)
		return &p, err
	}

	var comment, hardness sql.NullString
	err := db.QueryRow("SELECT name, Y(loc) AS lat, X(loc) AS lon, comment, hardness FROM portal WHERE opID = ? AND ID = ?", o.ID, portalID).Scan(&p.Name, &p.Lat, &p.Lon, &comment, &hardness)
	if err != nil && err == sql.ErrNoRows {
		err := fmt.Errorf("portal %s not in op", portalID)
		return &p, err
	}
	if err != nil {
		log.Error(err)
		return &p, err
	}
	if comment.Valid {
		p.Comment = comment.String
	}
	if hardness.Valid {
		p.Hardness = hardness.String
	}
	return &p, nil
}

// a transaction-safe version, no access checking
func (opID OperationID) portalDetails(portalID PortalID, tx *sql.Tx) (*Portal, error) {
	var p Portal
	p.ID = portalID
	p.opID = opID

	var comment, hardness sql.NullString
	err := tx.QueryRow("SELECT name, Y(loc) AS lat, X(loc) AS lon, comment, hardness FROM portal WHERE opID = ? AND ID = ?", opID, portalID).Scan(&p.Name, &p.Lat, &p.Lon, &comment, &hardness)
	if err != nil && err == sql.ErrNoRows {
		err := fmt.Errorf("portal %s not in op", portalID)
		return &p, err
	}
	if err != nil {
		log.Error(err)
		return &p, err
	}
	if comment.Valid {
		p.Comment = comment.String
	}
	if hardness.Valid {
		p.Hardness = hardness.String
	}
	return &p, nil
}

// lookup and return a populated Portal from an ID
func (o *Operation) getPortal(portalID PortalID) (Portal, error) {
	// make sure op is populated

	for _, p := range o.OpPortals {
		if p.ID == portalID {
			return p, nil
		}
	}

	var p Portal
	err := fmt.Errorf("portal not found")
	return p, err
}
