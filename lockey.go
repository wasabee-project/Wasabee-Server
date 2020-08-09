package wasabee

import (
	"database/sql"
	"fmt"
)

/*
 * The original plan for Wasabee was to be an XFAC tool, but that was quickly scrapped.
 * The LocKey was to be a unique identifier that could be given to your operators.
 * Much has changed since then, now the LocKey is used for setting up Telegram and one-time-tokens
 * The LocKey is disposable, and designed to change frequently
 *
 * The GoogleID is the primary identifier
 */

// LocKey is the location share key, a transitory ID for an agent
type LocKey string

// Gid converts a location share key to a agent's gid
func (lockey LocKey) Gid() (GoogleID, error) {
	var gid GoogleID

	err := db.QueryRow("SELECT gid FROM agent WHERE lockey = ?", lockey).Scan(&gid)
	if err != nil && err == sql.ErrNoRows {
		err := fmt.Errorf("invalid LocKey")
		Log.Info(err)
		return "", err
	}
	if err != nil {
		Log.Info(err)
		return "", err
	}

	return gid, nil
}
