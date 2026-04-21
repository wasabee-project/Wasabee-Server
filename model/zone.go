package model

import (
	"context"
	"database/sql"
	"strconv"

	"github.com/wasabee-project/Wasabee-Server/log"
)

// Zone is the sub-operation zone identifier
type ZoneID int

// Zone constants
const (
	ZoneAssignOnly ZoneID = -1
	ZoneAll        ZoneID = 0
	zonePrimary    ZoneID = 1
	zoneMax               = 32
)

// Valid returns a boolean if the zone is in the valid range
func (z ZoneID) Valid() bool {
	return z >= ZoneAll && z <= zoneMax
}

// ZoneFromString takes a string and returns a valid zone or zonePrimary if invalid input
func ZoneFromString(in string) ZoneID {
	if in == "" || in == "undefined" {
		return zonePrimary
	}

	i, err := strconv.ParseInt(in, 10, 32)
	if err != nil {
		return zonePrimary
	}

	z := ZoneID(i)
	if !z.Valid() {
		return zonePrimary
	}
	return z
}

func (z ZoneID) inZones(zones []ZoneID) bool {
	for _, t := range zones {
		// ZoneAll (0) is a wildcard
		if t == ZoneAll || t == z {
			return true
		}
	}
	return false
}

// ZoneListElement is used to map display names to zones
type ZoneListElement struct {
	Name   string      `json:"name"`
	Color  string      `json:"color"`
	Points []zonepoint `json:"points"`
	Zone   ZoneID      `json:"id"`
}

type zonepoint struct {
	Position uint8   `json:"position"`
	Lat      float64 `json:"lat"`
	Lon      float64 `json:"lng"`
}

func defaultZones() []ZoneListElement {
	return []ZoneListElement{
		{Name: "Primary", Color: "purple", Zone: zonePrimary},
	}
}

func (o *Operation) insertZone(ctx context.Context, z ZoneListElement, tx *sql.Tx) error {
	executor := txExecutor(tx)

	// REPLACE is appropriate here as (ID, opID) is the unique key
	_, err := executor.ExecContext(ctx, "REPLACE INTO zone (ID, opID, name, color) VALUES (?, ?, ?, ?)",
		z.Zone, o.ID, z.Name, z.Color)
	if err != nil {
		return err
	}

	// Refresh points: delete and re-add
	_, err = executor.ExecContext(ctx, "DELETE FROM zonepoints WHERE opID = ? AND zoneID = ?", o.ID, z.Zone)
	if err != nil {
		return err
	}

	for _, p := range z.Points {
		_, err := executor.ExecContext(ctx, "INSERT INTO zonepoints (zoneID, opID, position, point) VALUES (?, ?, ?, Point(?, ?))",
			z.Zone, o.ID, p.Position, p.Lat, p.Lon)
		if err != nil {
			return err
		}
	}
	return nil
}

func (o *Operation) populateZones(ctx context.Context) error {
	rows, err := db.QueryContext(ctx, "SELECT ID, name, color FROM zone WHERE opID = ?", o.ID)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var tmpZone ZoneListElement
		if err := rows.Scan(&tmpZone.Zone, &tmpZone.Name, &tmpZone.Color); err != nil {
			continue
		}

		// Using ST_X and ST_Y for spatial point decomposition
		pointrows, err := db.QueryContext(ctx, "SELECT position, ST_X(point), ST_Y(point) FROM zonepoints WHERE opID = ? AND zoneID = ?", o.ID, tmpZone.Zone)
		if err != nil {
			log.Error(err)
			continue
		}

		for pointrows.Next() {
			var tmpPoint zonepoint
			if err := pointrows.Scan(&tmpPoint.Position, &tmpPoint.Lat, &tmpPoint.Lon); err != nil {
				continue
			}
			tmpZone.Points = append(tmpZone.Points, tmpPoint)
		}
		pointrows.Close()

		o.Zones = append(o.Zones, tmpZone)
	}

	if len(o.Zones) == 0 {
		o.Zones = defaultZones()
	}

	return nil
}

func (o OperationID) deleteZone(ctx context.Context, z ZoneID, tx *sql.Tx) error {
	executor := txExecutor(tx)

	// Delete points first (though FK cascade should handle it)
	_, _ = executor.ExecContext(ctx, "DELETE FROM zonepoints WHERE opID = ? AND zoneID = ?", o, z)

	_, err := executor.ExecContext(ctx, "DELETE FROM zone WHERE opID = ? AND ID = ?", o, z)
	return err
}
