package wasabee

import (
	// "fmt"
	"database/sql"
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
	Zone   Zone        `json:"id"`
	Name   string      `json:"name"`
	Points []zonepoint `json:"points"` // just a string for the client to parse
	Color  string      `json:"color"`
}

type zonepoint struct {
	Position uint8   `json:"position"`
	Lat      float64 `json:"lat"`
	Lon      float64 `json:"lng"`
}

func defaultZones() []ZoneListElement {
	zones := []ZoneListElement{
		{zonePrimary, "Primary", nil, "purple"},
	}
	return zones
}

func (o *Operation) insertZone(z ZoneListElement, tx *sql.Tx) error {
	if tx == nil {
		tx, _ = db.Begin()

		defer func() {
			err := tx.Rollback()
			if err != nil && err != sql.ErrTxDone {
				Log.Error(err)
			}
		}()
	}

	_, err := tx.Exec("INSERT INTO zone (ID, opID, name, color) VALUES (?, ?, ?, ?) ON DUPLICATE KEY UPDATE name = ?, color = ?", z.Zone, o.ID, z.Name, z.Color, z.Name, z.Color)
	if err != nil {
		Log.Error(err)
		return err
	}

	// don't be too smart, just delete and re-add the points
	_, err = tx.Exec("DELETE FROM zonepoints WHERE opID = ? AND zoneID = ?", o.ID, z.Zone)
	if err != nil {
		Log.Error(err)
		return err
	}

	for _, p := range z.Points {
		// Log.Debug("inserting point", "pos", p.Position, "zone", z.Zone, "op", o.ID)
		_, err := tx.Exec("INSERT INTO zonepoints (zoneID, opID, position, point) VALUES (?, ?, ?, POINT(?, ?))", z.Zone, o.ID, p.Position, p.Lat, p.Lon)
		if err != nil {
			Log.Error(err)
			return err
		}
	}

	return nil
}

func (o *Operation) populateZones(zones []Zone) error {
	rows, err := db.Query("SELECT ID, name, color FROM zone WHERE opID = ? ORDER BY ID", o.ID)
	if err != nil {
		Log.Error(err)
		return err
	}

	defer rows.Close()
	var tmpZone ZoneListElement
	for rows.Next() {
		if err := rows.Scan(&tmpZone.Zone, &tmpZone.Name, &tmpZone.Color); err != nil {
			Log.Error(err)
			continue
		}
		if !tmpZone.Zone.inZones(zones) {
			continue
		}

		pointrows, err := db.Query("SELECT position, X(point), Y(point) FROM zonepoints WHERE opID = ? AND zoneID = ? ORDER BY position", o.ID, tmpZone.Zone)
		if err != nil {
			Log.Error(err)
			continue
		}
		defer pointrows.Close()
		for pointrows.Next() {
			var tmpPoint zonepoint
			if err := pointrows.Scan(&tmpPoint.Position, &tmpPoint.Lat, &tmpPoint.Lon); err != nil {
				Log.Error(err)
				continue
			}
			tmpZone.Points = append(tmpZone.Points, tmpPoint)
		}

		o.Zones = append(o.Zones, tmpZone)
		tmpZone.Points = nil
	}

	// use default for old ops w/o set zones
	if len(o.Zones) == 0 {
		o.Zones = defaultZones()
	}

	return nil
}
