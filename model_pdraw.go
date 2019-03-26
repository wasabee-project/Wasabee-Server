package PhDevBin

import (
	"database/sql"
	"encoding/json"
	"errors"
)

// OperationID wrapper to ensure type safety
type OperationID string

// MarkerType will be an enum once we figure out the full list
type MarkerType string

// Operation is defined by the PhtivDraw IITC plugin.
// It is the top level item in the JSON file.
type Operation struct {
	ID      OperationID `json:"ID"`
	Name    string      `json:"name"`
	Gid     GoogleID    `json:"creator"`
	Color   string      `json:"color"`
	Portals []Portal    `json:"portals"`
	Links   []Link      `json:"links"`
	Markers []Marker    `json:"markers"`
}

// Portal is defined by the PhtivDraw IITC plugin.
type Portal struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Lat     string `json:"lat"` // I'd rather float64 but this is what I'm given
	Lon     string `json:"lon"` // I'd rather float64 but this is what I'm given
	Comment string `json:"comment"`
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
		return errors.New("Unauthorized")
	}

	_, err = db.Exec("DELETE FROM operation WHERE ID = ?", o.ID)
	if err != nil {
		Log.Notice(err)
		return err
	}
	// the foreign key constraints should take care of these, but just in case...
	_, _ = db.Exec("DELETE FROM marker WHERE opID = ?", o.ID)
	_, _ = db.Exec("DELETE FROM link WHERE opID = ?", o.ID)
	_, _ = db.Exec("DELETE FROM portal WHERE opID = ?", o.ID)

	// start the insert process
	_, err = db.Exec("INSERT INTO operation (ID, name, gid, color) VALUES (?, ?, ?, ?)", o.ID, o.Name, gid, o.Color)
	if err != nil {
		Log.Notice(err)
		return err
	}

	for _, m := range o.Markers {
		if err = insertMarker(&m, o.ID); err != nil {
			Log.Error(err)
			continue
		}
	}
	for _, l := range o.Links {
		if err = insertLink(&l, o.ID); err != nil {
			Log.Error(err)
			continue
		}
	}

	// I bet this isn't needed since they should be covered in links and markers... but just in case
	for _, p := range o.Portals {
		if err = insertPortal(&p, o.ID); err != nil {
			Log.Error(err)
			continue
		}
	}
	return nil
}

func insertMarker(m *Marker, o OperationID) error {
	_, err := db.Exec("INSERT INTO marker (ID, opID, portalID, type, comment) VALUES (?, ?, ?, ?, ?)",
		m.ID, o, m.Portal.ID, m.Type, m.Comment)
	if err != nil {
		Log.Error(err)
		return err
	}
	if err = insertPortal(&m.Portal, o); err != nil {
		Log.Error(err)
		return err
	}
	return nil
}

func insertPortal(p *Portal, o OperationID) error {
	_, err := db.Exec("INSERT IGNORE INTO portal (ID, opID, name, loc) VALUES (?, ?, ?, POINT(?,?))",
		p.ID, o, p.Name, p.Lon, p.Lat)
	if err != nil {
		Log.Error(err)
		return err
	}
	return nil
}

func insertLink(l *Link, o OperationID) error {
	_, err := db.Exec("INSERT INTO link (ID, fromPortalID, toPortalID, opID, description) VALUES (?, ?, ?, ?, ?)",
		l.ID, l.From.ID, l.To.ID, o, l.Desc)
	if err != nil {
		Log.Error(err)
	}
	if err = insertPortal(&l.To, o); err != nil {
		Log.Error(err)
		return err
	}
	if err = insertPortal(&l.From, o); err != nil {
		Log.Error(err)
		return err
	}
	return nil
}
