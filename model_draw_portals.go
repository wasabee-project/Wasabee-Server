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

// insertAnchor adds an anchor to the database
func (opID OperationID) insertAnchor(p PortalID) error {
	_, err := db.Exec("INSERT IGNORE INTO anchor (opID, portalID) VALUES (?, ?)", opID, p)
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
func (o *Operation) PopulatePortals() error {
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
func (o *Operation) PopulateAnchors() error {
	var anchor PortalID
	rows, err := db.Query("SELECT portalID FROM anchor WHERE opID = ?", o.ID)
	if err != nil {
		Log.Error(err)
		return err
	}
	defer rows.Close()
	for rows.Next() {
		err := rows.Scan(&anchor)
		if err != nil {
			Log.Error(err)
			continue
		}
		o.Anchors = append(o.Anchors, anchor)
	}
	return nil
}

// String returns the string version of a PortalID
func (p PortalID) String() string {
	return string(p)
}

// PortalHardness updates the comment on a portal
func (opID OperationID) PortalHardness(portalID PortalID, hardness string) error {
	_, err := db.Exec("UPDATE portal SET hardness = ? WHERE ID = ? AND opID = ?", MakeNullString(hardness), portalID, opID)
	if err != nil {
		Log.Error(err)
		return err
	}
	if err = opID.Touch(); err != nil {
		Log.Error(err)
	}
	return nil
}

// PortalComment updates the comment on a portal
func (opID OperationID) PortalComment(portalID PortalID, comment string) error {
	_, err := db.Exec("UPDATE portal SET comment = ? WHERE ID = ? AND opID = ?", MakeNullString(comment), portalID, opID)
	if err != nil {
		Log.Error(err)
		return err
	}
	if err = opID.Touch(); err != nil {
		Log.Error(err)
	}
	return nil
}

// PortalDetails returns information about the portal
func (opID OperationID) PortalDetails(portalID PortalID, gid GoogleID) (Portal, error) {
	var p Portal
	p.ID = portalID
	var teamID TeamID

	err := db.QueryRow("SELECT teamID FROM operation WHERE ID = ?", opID).Scan(&teamID)
	if err != nil {
		Log.Error(err)
		return p, err
	}
	var inteam bool
	inteam, err = gid.AgentInTeam(teamID, false)
	if err != nil {
		Log.Error(err)
		return p, err
	}
	if !inteam {
		err := fmt.Errorf("unauthorized: you are not on a team authorized to see this operation")
		Log.Error(err)
		return p, err
	}

	var comment, hardness sql.NullString
	err = db.QueryRow("SELECT name, Y(loc) AS lat, X(loc) AS lon, comment, hardness FROM portal WHERE opID = ? AND ID = ?", opID, portalID).Scan(&p.Name, &p.Lat, &p.Lon, &comment, &hardness)
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
	if err = opID.Touch(); err != nil {
		Log.Error(err)
	}
	return p, nil
}
