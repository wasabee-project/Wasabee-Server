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
type OneTimeToken string

// String is a stringer for LocKey
func (ott OneTimeToken) String() string {
	return string(ott)
}

// Gid converts a location share key to a agent's gid
func (ott OneTimeToken) Gid() (GoogleID, error) {
	var gid GoogleID

	err := db.QueryRow("SELECT gid FROM agent WHERE OneTimeToken = ?", ott).Scan(&gid)
	if err != nil && err == sql.ErrNoRows {
		err := fmt.Errorf("invalid OneTimeToken")
		Log.Info(err)
		return "", err
	}
	if err != nil {
		Log.Info(err)
		return "", err
	}

	return gid, nil
}

// NewLocKey generates a new LocationKey for an agent -- exported for use in test scripts
func (gid GoogleID) NewOneTimeToken() (OneTimeToken, error) {
	// we could just use UUID() here...
	ott, err := GenerateSafeName()
	if err != nil {
		Log.Error(err)
		return "", err
	}
	if _, err = db.Exec("UPDATE agent SET OneTimeToken = ? WHERE gid = ?", ott, gid); err != nil {
		Log.Error(err)
		return "", err
	}
	o := OneTimeToken(ott)
	return o, nil
}

// Validate attempts to resolve a submitted OTT and updates it if valid
func (ott OneTimeToken) Increment() (GoogleID, error) {
	gid, err := ott.Gid()
	if err != nil {
		Log.Error(err)
		return "", err
	}

	_, err = gid.NewOneTimeToken()
	if err != nil {
		Log.Warn(err)
	}
	return gid, nil
}
