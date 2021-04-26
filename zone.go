package wasabee

import (
	"strconv"
)

// Zone is the sub-operation zone identifer
type Zone int

// ZoneAll is a reserved name for the wildcard zone
const (
	ZoneAssignOnly Zone = -1
	ZoneAll     Zone = 0
	zonePrimary Zone = 1
	zoneMax          = 32
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
	Zone Zone   `json:"id"`
	Name string `json:"name"`
}

func defaultZones() []ZoneListElement {
	zones := []ZoneListElement{
		{zonePrimary, "Primary"},
		{2, "Alpha"},
		{3, "Beta"},
		{4, "Gamma"},
		{5, "Delta"},
		{6, "Epsilon"},
		{7, "Zeta"},
		{8, "Eta"},
		{9, "Theta"},
	}
	return zones
}

func (o *Operation) insertZone(z ZoneListElement) error {
	_, err := db.Exec("INSERT INTO zone (ID, opID, name) VALUES (?, ?, ?) ON DUPLICATE KEY UPDATE name = ?", z.Zone, o.ID, z.Name, z.Name)
	if err != nil {
		Log.Error(err)
		return err
	}
	return nil
}

func (o *Operation) populateZones() error {
	rows, err := db.Query("SELECT ID, name FROM zone WHERE opID = ? ORDER BY ID", o.ID)
	if err != nil {
		Log.Error(err)
		return err
	}

	defer rows.Close()
	var tmpZone ZoneListElement
	for rows.Next() {
		err := rows.Scan(&tmpZone.Zone, &tmpZone.Name)
		if err != nil {
			Log.Error(err)
			continue
		}
		o.Zones = append(o.Zones, tmpZone)
	}

	// use default for old ops w/o set zones
	if len(o.Zones) == 0 {
		o.Zones = defaultZones()
	}

	return nil
}
