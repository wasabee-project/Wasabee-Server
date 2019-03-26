package PhDevBin

import (
	"database/sql"
	"encoding/json"
	"errors"
)

// OperationID wrapper to ensure type safety
type OperationID string

// PortalID wrapper to ensure type safety
type PortalID string

// MarkerType will be an enum once we figure out the full list
type MarkerType string

// Operation is defined by the PhtivDraw IITC plugin.
// It is the top level item in the JSON file.
type Operation struct {
	ID        OperationID `json:"ID"`
	Name      string      `json:"name"`
	Gid       GoogleID    `json:"creator"` // IITC plugin sending agent name, need to convert to GID
	Color     string      `json:"color"`   // could be an enum, but freeform is fine for now
	OpPortals []Portal    `json:"opportals"`
	Anchors   []PortalID  `json:"anchors"`
	Links     []Link      `json:"links"`
	Markers   []Marker    `json:"markers"`
	TeamID    TeamID      `json:"teamid"` // not set in IITC Plugin yet
}

// Portal is defined by the PhtivDraw IITC plugin.
type Portal struct {
	ID      PortalID `json:"id"`
	Name    string   `json:"name"`
	Lat     string   `json:"lat"` // passing these as strings saves me parsing them
	Lon     string   `json:"lon"`
	Comment string   `json:"comment"` // currently not in database, need schema change
}

// Link is defined by the PhtivDraw IITC plugin.
type Link struct {
	ID         string   `json:"ID"`
	From       Portal   `json:"fromPortal"`
	To         Portal   `json:"toPortal"`
	Desc       string   `json:"description"`
	AssignedTo GoogleID `json:"assignedTo"`
}

// Marker is defined by the PhtivDraw IITC plugin.
type Marker struct {
	ID      string     `json:"ID"`
	Portal  Portal     `json:"portal"`
	Type    MarkerType `json:"type"`
	Comment string     `json:"comment"`
}

// PDrawInsert parses a raw op sent from the IITC plugin and stores it in the database
// it will completely overwrite an existing draw with the same ID
// if the current user is the same as the user who originally uploaded it
func PDrawInsert(op json.RawMessage, gid GoogleID) error {
	var o Operation
	if err := json.Unmarshal(op, &o); err != nil {
		Log.Error(err)
		return err
	}

	var opgid GoogleID
	var authorized bool
	r := db.QueryRow("SELECT gid FROM operation WHERE ID = ?", o.ID)
	err := r.Scan(&opgid)
	if err != nil && err != sql.ErrNoRows {
		Log.Notice(err)
		return err
	}
	if err != nil && err == sql.ErrNoRows {
		authorized = true
	}
	if opgid == gid {
		authorized = true
	}
	if authorized == false {
		return errors.New("Unauthorized: this operation owned by someone else")
	}

	// clear and start from a blank slate
	if err = o.Delete(); err != nil {
		Log.Notice(err)
		return err
	}

	// start the insert process
	_, err = db.Exec("INSERT INTO operation (ID, name, gid, color) VALUES (?, ?, ?, ?)", o.ID, o.Name, gid, o.Color)
	if err != nil {
		Log.Error(err)
		return err
	}

	for _, m := range o.Markers {
		if err = o.insertMarker(&m); err != nil {
			Log.Error(err)
			continue
		}
	}
	for _, l := range o.Links {
		if err = o.insertLink(&l); err != nil {
			Log.Error(err)
			continue
		}
	}
	for _, a := range o.Anchors {
		if err = o.insertAnchor(a); err != nil {
			Log.Error(err)
			continue
		}
	}

	// I bet this isn't needed since they should be covered in links and markers... but just in case
	for _, p := range o.OpPortals {
		if err = o.insertPortal(&p); err != nil {
			Log.Error(err)
			continue
		}
	}
	return nil
}

func (o *Operation) insertMarker(m *Marker) error {
	_, err := db.Exec("INSERT INTO marker (ID, opID, PortalID, type, comment) VALUES (?, ?, ?, ?, ?)",
		m.ID, o.ID, m.Portal.ID, m.Type, m.Comment)
	if err != nil {
		Log.Error(err)
		return err
	}
	if err = o.insertPortal(&m.Portal); err != nil {
		Log.Error(err)
		return err
	}
	return nil
}

func (o *Operation) insertPortal(p *Portal) error {
	_, err := db.Exec("INSERT IGNORE INTO portal (ID, opID, name, loc) VALUES (?, ?, ?, POINT(? ?))",
		p.ID, o.ID, p.Name, p.Lon, p.Lat)
	if err != nil {
		Log.Error(err)
		return err
	}
	return nil
}

func (o *Operation) insertAnchor(p PortalID) error {
	_, err := db.Exec("INSERT IGNORE INTO anchor (opID, portalID) VALUES (?, ?)", o.ID, p)
	if err != nil {
		Log.Error(err)
		return err
	}
	return nil
}

func (o *Operation) insertLink(l *Link) error {
	_, err := db.Exec("INSERT INTO link (ID, fromPortalID, toPortalID, opID, description) VALUES (?, ?, ?, ?, ?)",
		l.ID, l.From.ID, l.To.ID, o.ID, l.Desc)
	if err != nil {
		Log.Error(err)
	}
	if err = o.insertPortal(&l.To); err != nil {
		Log.Error(err)
		return err
	}
	if err = o.insertPortal(&l.From); err != nil {
		Log.Error(err)
		return err
	}
	return nil
}

func (o *Operation) Delete() error {
	_, err := db.Exec("DELETE FROM operation WHERE ID = ?", o.ID)
	if err != nil {
		Log.Notice(err)
		return err
	}
	// the foreign key constraints should take care of these, but just in case...
	_, _ = db.Exec("DELETE FROM marker WHERE opID = ?", o.ID)
	_, _ = db.Exec("DELETE FROM link WHERE opID = ?", o.ID)
	_, _ = db.Exec("DELETE FROM portal WHERE opID = ?", o.ID)
	_, _ = db.Exec("DELETE FROM anchor WHERE opID = ?", o.ID)
	return nil
}

// Populate takes a pointer to an Operation and fills it in; o.ID must be set
// checks to see that either the gid created the operation or the gid is on the team assigned to the operation
func (o *Operation) Populate(gid GoogleID) error {
	portalCache := make(map[PortalID]Portal)
	var authorized bool

	// permission check and populate Operation top level
	var teamID sql.NullString
	r := db.QueryRow("SELECT name, gid, color, teamID FROM operation WHERE ID = ?", o.ID)
	err := r.Scan(&o.Name, &o.Gid, &o.Color, &teamID)
	if err != nil {
		Log.Notice(err)
		return err
	}
	if teamID.Valid {
		o.TeamID = TeamID(teamID.String)
		if inteam, _ := gid.UserInTeam(o.TeamID, false); inteam == true {
			authorized = true
		}
	}
	if gid == o.Gid {
		authorized = true
	}
	if authorized == false {
		return errors.New("Unauthorized: this operation owned by someone else")
	}

	// get portal data into portalCache AND the Operation
	err = o.PopulatePortals(portalCache)
	if err != nil {
		Log.Notice(err)
		return err
	}

	// get marker data, filling in portals from portalCache
	err = o.PopulateMarkers(portalCache)
	if err != nil {
		Log.Notice(err)
		return err
	}

	// get link data, filling in portals from portalCache
	err = o.PopulateLinks(portalCache)
	if err != nil {
		Log.Notice(err)
		return err
	}

	err = o.PopulateAnchors()
	if err != nil {
		Log.Notice(err)
		return err
	}

	return nil
}

func (o *Operation) PopulatePortals(cache map[PortalID]Portal) error {
	var tmpPortal Portal
	// var comment sql.NullString

	rows, err := db.Query("SELECT ID, name, Y(loc) AS lat, X(loc) AS lon FROM portal WHERE opID = ? ORDER BY name", o.ID)
	if err != nil {
		Log.Error(err)
		return err
	}
	defer rows.Close()
	for rows.Next() {
		err := rows.Scan(&tmpPortal.ID, &tmpPortal.Name, &tmpPortal.Lat, &tmpPortal.Lon)
		if err != nil {
			Log.Error(err)
			continue
		}
		o.OpPortals = append(o.OpPortals, tmpPortal)
		cache[tmpPortal.ID] = tmpPortal
	}
	return nil
}

func (o *Operation) PopulateMarkers(cache map[PortalID]Portal) error {
	var tmpMarker Marker
	var comment sql.NullString
	var tmpPortalID PortalID

	rows, err := db.Query("SELECT ID, PortalID, type, comment FROM marker WHERE opID = ?", o.ID)
	if err != nil {
		Log.Error(err)
		return err
	}
	defer rows.Close()
	for rows.Next() {
		err := rows.Scan(&tmpMarker.ID, &tmpPortalID, &tmpMarker.Type, &comment)
		if err != nil {
			Log.Error(err)
			continue
		}
		if comment.Valid {
			tmpMarker.Comment = comment.String
		} else {
			tmpMarker.Comment = ""
		}
		cachedPortal, ok := cache[tmpPortalID]
		if ok == true {
			tmpMarker.Portal = cachedPortal
		} else {
			Log.Debug("Cache Miss")
			// query?
		}
		o.Markers = append(o.Markers, tmpMarker)
	}
	return nil
}

func (o *Operation) PopulateLinks(cache map[PortalID]Portal) error {
	var tmpLink Link
	var description sql.NullString
	var tmpFromPortalID PortalID
	var tmpToPortalID PortalID

	rows, err := db.Query("SELECT ID, fromPortalID, toPortalID, description FROM link WHERE opID = ?", o.ID)
	if err != nil {
		Log.Error(err)
		return err
	}
	defer rows.Close()
	for rows.Next() {
		err := rows.Scan(&tmpLink.ID, &tmpFromPortalID, &tmpToPortalID, &description)
		if err != nil {
			Log.Error(err)
			continue
		}
		if description.Valid {
			tmpLink.Desc = description.String
		} else {
			tmpLink.Desc = ""
		}
		cachedPortal, ok := cache[tmpFromPortalID]
		if ok == true {
			tmpLink.From = cachedPortal
		} else {
			Log.Debug("Cache Miss")
			// query?
		}

		cachedPortal, ok = cache[tmpToPortalID]
		if ok == true {
			tmpLink.To = cachedPortal
		} else {
			Log.Debug("Cache Miss")
			// query?
		}
		o.Links = append(o.Links, tmpLink)
	}
	return nil
}

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

func (p PortalID) String() string {
	return string(p)
}
