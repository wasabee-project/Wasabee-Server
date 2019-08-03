package wasabee

// This is now only used in Telegram for user setup; when I get a chance, I'll convert it over to using the GID
// back when this was going to be an open/XFAC tool, the lockey was going to be the primary UID ; once we decided to have this an ENL only tool, the lockey became irrelevant

// LocKey is the location share key
type LocKey string

// Gid converts a location share key to a agent's gid
func (lockey LocKey) Gid() (GoogleID, error) {
       var gid GoogleID

       err := db.QueryRow("SELECT gid FROM agent WHERE lockey = ?", lockey).Scan(&gid)
       if err != nil {
               Log.Notice(err)
               return "", err
       }

       return gid, nil
}
