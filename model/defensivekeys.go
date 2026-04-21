package model

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"time"

	"github.com/wasabee-project/Wasabee-Server/log"
)

// DefensiveKeyList is the list of all defensive keys an agent is authorized to see
type DefensiveKeyList struct {
	Fetched       string         `json:"fetched"`
	DefensiveKeys []DefensiveKey `json:"keys"`
}

// DefensiveKey describes a portal key held for defensive purposes
type DefensiveKey struct {
	GID      GoogleID `json:"GID"`
	PortalID PortalID `json:"PortalID"`
	CapID    string   `json:"CapID"`
	Name     string   `json:"Name"`
	Lat      string   `json:"Lat"`
	Lon      string   `json:"Lng"`
	Count    int32    `json:"Count"`
}

// ListDefensiveKeys gets all keys an agent is authorized to know about.
// Authorization: Agent must be on a team where they have loadWD=1 and the other agent has shareWD=1.
func (gid GoogleID) ListDefensiveKeys(ctx context.Context) (DefensiveKeyList, error) {
	var dkl DefensiveKeyList
	dkl.DefensiveKeys = make([]DefensiveKey, 0)

	// SQL optimization: Join agentteams me and other to find authorized peers
	rows, err := db.QueryContext(ctx, `
		SELECT k.gid, k.portalID, k.capID, k.count, k.name, ST_Y(k.loc) AS lat, ST_X(k.loc) AS lon 
		FROM defensivekeys k
		WHERE k.gid IN (
			SELECT DISTINCT other.gid 
			FROM agentteams AS other, agentteams AS me 
			WHERE me.gid = ? AND me.loadWD = 1 
			AND other.teamID = me.teamID 
			AND other.shareWD = 1
		)`, gid)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			dkl.Fetched = time.Now().Format(time.RFC3339)
			return dkl, nil
		}
		log.Error(err)
		return dkl, err
	}
	defer rows.Close()

	for rows.Next() {
		var dk DefensiveKey
		var name, lat, lon sql.NullString
		err := rows.Scan(&dk.GID, &dk.PortalID, &dk.CapID, &dk.Count, &name, &lat, &lon)
		if err != nil {
			continue
		}

		if name.Valid {
			dk.Name = name.String
		}
		if lat.Valid {
			dk.Lat = lat.String
		}
		if lon.Valid {
			dk.Lon = lon.String
		}
		dkl.DefensiveKeys = append(dkl.DefensiveKeys, dk)
	}

	dkl.Fetched = time.Now().Format(time.RFC3339)
	return dkl, nil
}

// InsertDefensiveKey adds or updates a key in the list
func (gid GoogleID) InsertDefensiveKey(ctx context.Context, dk DefensiveKey) error {
	if dk.Count < 1 {
		_, err := db.ExecContext(ctx, "DELETE FROM defensivekeys WHERE gid = ? AND portalID = ?", gid, dk.PortalID)
		return err
	}

	// Sanitize and convert coordinates
	flat, _ := strconv.ParseFloat(dk.Lat, 64)
	flon, _ := strconv.ParseFloat(dk.Lon, 64)

	_, err := db.ExecContext(ctx, `
		INSERT INTO defensivekeys (gid, portalID, capID, count, name, loc) 
		VALUES (?, ?, ?, ?, ?, Point(?, ?)) 
		ON DUPLICATE KEY UPDATE capID = ?, count = ?, name = ?, loc = Point(?, ?)`,
		gid, dk.PortalID, dk.CapID, dk.Count, dk.Name, flat, flon,
		dk.CapID, dk.Count, dk.Name, flat, flon)

	if err != nil {
		log.Error(err)
		return err
	}

	return nil
}
