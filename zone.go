package wasabee

import (
	"fmt"
	"strconv"
)

// Zone is the sub-operation zone identifer
type Zone int

// ZoneAll is a reserved name for the wildcard zone
const (
	ZoneAssignOnly Zone = -1
	ZoneAll        Zone = 0
	zonePrimary    Zone = 1
	zoneMax             = 32
)

// Valid returns a boolean if the zone is in the valid range
func (z Zone) Valid() bool {
	if z >= ZoneAll && z <= zoneMax {
		return true
	}
	return false
}

// ZoneFromString takes a string and returns a valid zone or zonePrimary if invalid input
func ZoneFromString(in string) Zone {
	if in == "" || in == "undefined" {
		return zonePrimary
	}

	i, err := strconv.ParseInt(in, 10, 32)
	if err != nil {
		Log.Error(err)
		return zonePrimary
	}

	z := Zone(i)

	if !z.Valid() {
		z = zonePrimary
	}
	return z
}

func (z Zone) inZones(zones []Zone) bool {
	for _, t := range zones {
		// ZoneAll is set, anything goes
		if t == ZoneAll {
			return true
		}
		// this zone is set, permit
		if t == z {
			return true
		}
	}
	// no match found, fail
	return false
}

// ZoneListElement is used to map display names to zones
type ZoneListElement struct {
	Zone   Zone    `json:"id"`
	Name   string  `json:"name"`
	Points []zonepoint `json:"points"` // just a string for the client to parse
	Color  string  `json:"color"`
}

type zonepoint struct {
  Position uint8 `json:"pos"`
  Lat float64 `json:"lat"`
  Lon float64 `json:"lng"`
}

func defaultZones() []ZoneListElement {
	zones := []ZoneListElement{
		{zonePrimary, "Primary", nil, "purple"},
		{2, "Alpha", nil, "red"},
		{3, "Beta", nil, "yellow"},
		{4, "Gamma", nil, "green"},
	}
	return zones
}

func (o *Operation) insertZone(z ZoneListElement) error {
	_, err := db.Exec("INSERT INTO zone (ID, opID, name, color) VALUES (?, ?, ?, ?) ON DUPLICATE KEY UPDATE name = ?", z.Zone, o.ID, z.Name, z.Name, z.Color)
	if err != nil {
		Log.Error(err)
		return err
	}
	
	for _, p := range z.Points {
	    sqpoint := fmt.Sprintf("(%f,%f)", p.Lat, p.Lon)
		_, err := db.Exec("INSERT INTO zonepoints (zoneID, opID, position, point) VALUES (?, ?, ?, ?, ?) ON DUPLICATE KEY UPDATE point = ?", z.Zone, o.ID, p.Position, sqpoint, sqpoint)
		if err != nil {
			Log.Error(err)
			return err
		}
	}
    // XXX do points

	return nil
}

func (o *Operation) populateZones() error {
	rows, err := db.Query("SELECT ID, name, color FROM zone WHERE opID = ? ORDER BY ID", o.ID)
	if err != nil {
		Log.Error(err)
		return err
	}

	defer rows.Close()
	var tmpZone ZoneListElement
	for rows.Next() {
		err := rows.Scan(&tmpZone.Zone, &tmpZone.Name, &tmpZone.Color)
		if err != nil {
			Log.Error(err)
			continue
		}
		o.Zones = append(o.Zones, tmpZone)

		// XXX do points
	}

	// use default for old ops w/o set zones
	if len(o.Zones) == 0 {
		o.Zones = defaultZones()
	}

	return nil
}
