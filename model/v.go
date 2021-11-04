package model

import (
	"github.com/wasabee-project/Wasabee-Server/log"
	"github.com/wasabee-project/Wasabee-Server/v"
	"database/sql"
)

// VUpdate updates the database to reflect an agent's current status at V.
// It should be called whenever a agent logs in via a new service (if appropriate); currently only https does.
func (gid GoogleID) VUpdate(vres *Vresult) error {
	if !vc.configured {
		return nil
	}

	if vres.Status == "ok" && vres.Data.Agent != "" {
		_, err := db.Exec("UPDATE agent SET Vname = ?, level = ?, VVerified = ?, VBlacklisted = ?, Vid = ? WHERE gid = ?",
			vres.Data.Agent, vres.Data.Level, vres.Data.Verified, vres.Data.Blacklisted, MakeNullString(vres.Data.EnlID), gid)

		// doppelkeks error
		if err != nil && strings.Contains(err.Error(), "Error 1062") {
			vname := fmt.Sprintf("%s-%s", vres.Data.Agent, gid)
			log.Warnw("dupliate ingress agent name detected at v", "GID", vres.Data.Agent, "new name", vname)
			if _, err := db.Exec("UPDATE agent SET Vname = ?, level = ?, VVerified = ?, VBlacklisted = ?, Vid = ? WHERE gid = ?",
				vname, vres.Data.Level, vres.Data.Verified, vres.Data.Blacklisted, MakeNullString(vres.Data.EnlID), gid); err != nil {
				log.Error(err)
				return err
			}
		} else if err != nil {
			log.Error(err)
			return err
		}
	}
	return nil
}

// EnlID returns the V EnlID for a agent if it is known.
/* func (gid GoogleID) EnlID() (v.EnlID, error) {
	var vid sql.NullString
	err := db.QueryRow("SELECT Vid FROM agent WHERE gid = ?", gid).Scan(&vid)
	if err != nil {
		log.Error(err)
		return "", err
	}
	if !vid.Valid {
		return "", nil
	}
	e := EnlID(vid.String)

	return e, err
} */

// Gid looks up a GoogleID from an EnlID
/* func (eid EnlID) Gid() (GoogleID, error) {
	var gid GoogleID
	err := db.QueryRow("SELECT gid FROM agent WHERE Vid = ?", eid).Scan(&gid)
	if err != nil {
		log.Error(err)
		return "", err
	}
	return gid, nil
} */

// VTeam gets the configured V team information for a teamID
func (teamID TeamID) VTeam() (int64, uint8, error) {
	var team int64
	var role uint8
	err := db.QueryRow("SELECT vteam, vrole FROM team WHERE teamID = ?", teamID).Scan(&team, &role)
	if err != nil {
		log.Error(err)
		return 0, 0, err
	}
	return team, role, nil
}
