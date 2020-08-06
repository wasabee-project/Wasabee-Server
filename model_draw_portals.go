package wasabee

import (
	"database/sql"
	"fmt"
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
}

// insertPortal adds a portal to the database
func (opID OperationID) insertPortal(p Portal) error {
	_, err := db.Exec("INSERT IGNORE INTO portal (ID, opID, name, loc, comment, hardness) VALUES (?, ?, ?, POINT(?, ?), ?, ?)",
		p.ID, opID, p.Name, p.Lon, p.Lat, MakeNullString(p.Comment), MakeNullString(p.Hardness))
	if err != nil {
		Log.Error(err)
		return err
	}
	return nil
}

func (opID OperationID) updatePortal(p Portal) error {
	_, err := db.Exec("REPLACE INTO portal (ID, opID, name, loc, comment, hardness) VALUES (?, ?, ?, POINT(?, ?), ?, ?)",
		p.ID, opID, p.Name, p.Lon, p.Lat, MakeNullString(p.Comment), MakeNullString(p.Hardness))
	if err != nil {
		Log.Error(err)
		return err
	}
	return nil
}

func (opID OperationID) deletePortal(p PortalID) error {
	_, err := db.Exec("DELETE FROM portal WHERE ID = ? AND opID = ?", p, opID)
	if err != nil {
		Log.Error(err)
		return err
	}
	return nil
}

// PopulatePortals fills in the OpPortals list for the Operation. No authorization takes place.
func (o *Operation) populatePortals() error {
	var tmpPortal Portal

	var comment, hardness sql.NullString

	rows, err := db.Query("SELECT ID, name, Y(loc) AS lat, X(loc) AS lon, comment, hardness FROM portal WHERE opID = ? ORDER BY name", o.ID)
	if err != nil {
		Log.Error(err)
		return err
	}
	defer rows.Close()
	for rows.Next() {
		err := rows.Scan(&tmpPortal.ID, &tmpPortal.Name, &tmpPortal.Lat, &tmpPortal.Lon, &comment, &hardness)
		if err != nil {
			Log.Error(err)
			continue
		}
		if comment.Valid {
			tmpPortal.Comment = comment.String
		} else {
			tmpPortal.Comment = ""
		}
		if hardness.Valid {
			tmpPortal.Hardness = hardness.String
		} else {
			tmpPortal.Hardness = ""
		}

		o.OpPortals = append(o.OpPortals, tmpPortal)
	}
	return nil
}

// PopulateAnchors fills in the Anchors list for the Operation. No authorization takes place.
// XXX the clients _should_ build this themselves, but don't, yet.
func (o *Operation) populateAnchors() error {
	var fromPortalID, toPortalID PortalID
	rows, err := db.Query("SELECT fromPortalID, toPortalID FROM link WHERE opID = ?", o.ID)
	if err != nil {
		Log.Error(err)
		return err
	}
	defer rows.Close()

	set := make(map[PortalID]bool)
	for rows.Next() {
		err := rows.Scan(&fromPortalID, &toPortalID)
		if err != nil {
			Log.Error(err)
			continue
		}
		set[fromPortalID] = true
		set[toPortalID] = true
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
func (o *Operation) PortalHardness(portalID PortalID, hardness string) error {
	_, err := db.Exec("UPDATE portal SET hardness = ? WHERE ID = ? AND opID = ?", MakeNullString(hardness), portalID, o.ID)
	if err != nil {
		Log.Error(err)
		return err
	}
	if err = o.Touch(); err != nil {
		Log.Error(err)
	}
	return nil
}

// PortalComment updates the comment on a portal
func (o *Operation) PortalComment(portalID PortalID, comment string) error {
	_, err := db.Exec("UPDATE portal SET comment = ? WHERE ID = ? AND opID = ?", MakeNullString(comment), portalID, o.ID)
	if err != nil {
		Log.Error(err)
		return err
	}
	if err = o.Touch(); err != nil {
		Log.Error(err)
	}
	return nil
}

// PortalDetails returns information about the portal
func (o *Operation) PortalDetails(portalID PortalID, gid GoogleID) (Portal, error) {
	var p Portal
	p.ID = portalID

	if !o.ReadAccess(gid) {
		err := fmt.Errorf("unauthorized: unable to get portal details")
		Log.Error(err.Error(), "GID", gid, "resource", o.ID, "portal", portalID)
		return p, err
	}

	var comment, hardness sql.NullString
	err := db.QueryRow("SELECT name, Y(loc) AS lat, X(loc) AS lon, comment, hardness FROM portal WHERE opID = ? AND ID = ?", o.ID, portalID).Scan(&p.Name, &p.Lat, &p.Lon, &comment, &hardness)
	if err != nil {
		Log.Error(err)
		return p, err
	}
	if comment.Valid {
		p.Comment = comment.String
	}
	if hardness.Valid {
		p.Hardness = hardness.String
	}
	return p, nil
}
