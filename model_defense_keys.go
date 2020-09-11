package wasabee

import (
	"database/sql"
	"time"
)

// DefensiveKeyList is the list of all defensive keys
type DefensiveKeyList struct {
	DefensiveKeys []DefensiveKey
	Fetched       string
}

// DefensiveKey is a sub-struct of DefensiveKeyList
type DefensiveKey struct {
	GID      GoogleID
	PortalID PortalID
	CapID    string
	Count    int32
}

// ListDefensiveKeys gets all keys an agent is authorized to know about.
func (gid GoogleID) ListDefensiveKeys() (DefensiveKeyList, error) {
	var dkl DefensiveKeyList

	rows, err := db.Query("SELECT gid, portalID, capID, count FROM defensivekeys WHERE gid IN (SELECT DISTINCT x.gid FROM agentteams=x, agentteams=y WHERE y.gid = ? AND y.state = 'On' AND x.teamID = y.teamID AND x.state = 'On')", gid)
	if err != nil && err != sql.ErrNoRows {
		Log.Error(err)
		return dkl, err
	}
	var dk DefensiveKey
	defer rows.Close()
	for rows.Next() {
		err := rows.Scan(&dk.GID, &dk.PortalID, &dk.CapID, &dk.Count)
		if err != nil {
			Log.Error(err)
			return dkl, err
		}
		dkl.DefensiveKeys = append(dkl.DefensiveKeys, dk)
	}

	dkl.Fetched = time.Now().Format(time.RFC3339)

	return dkl, nil
}

// InsertDefensiveKey adds a new key to the list
func (gid GoogleID) InsertDefensiveKey(portalID PortalID, capID string, count int32) error {
	if count < 1 {
		if _, err := db.Exec("DELETE FROM defensivekeys WHERE gid = ? AND portalID = ?", gid, portalID); err != nil {
			Log.Error(err)
			return err
		}
	} else {
		if _, err := db.Exec("INSERT INTO defensivekeys (gid, portalID, capID, count) VALUES (?, ?, ?, ?) ON DUPLICATE KEY UPDATE capID = ?, count = ?", gid, portalID, capID, count, capID, count); err != nil {
			Log.Error(err)
			return err
		}
	}
	return nil
}
