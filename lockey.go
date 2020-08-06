package wasabee

/*
 * The original play for Wasabee was to be an XFAC tool, but that was quickly scrapped
 * the LocKey was to be a unique identifier that could be given to your operators
 * much has changed since then, now it is used for setting up Telegram and one-time-tokens
 * the GoogleID is the primary identifier
 */

// LocKey is the location share key
type LocKey string

// Gid converts a location share key to a agent's gid
func (lockey LocKey) Gid() (GoogleID, error) {
	var gid GoogleID

	err := db.QueryRow("SELECT gid FROM agent WHERE lockey = ?", lockey).Scan(&gid)
	if err != nil {
		Log.Info(err)
		return "", err
	}

	return gid, nil
}
