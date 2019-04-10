package WASABI

import (
	"database/sql"
	"fmt"
)

// GoogleID is the primary location for interfacing with the user type
type GoogleID string

// TeamID is the primary means for interfacing with teams
type TeamID string

// LocKey is the location share key
type LocKey string

// EnlID is a V EnlID
type EnlID string

// AgentData is the complete user struct, used for the /me page
type AgentData struct {
	GoogleID      GoogleID
	IngressName   string
	Level         int64
	LocationKey   string
	OwnTracksPW   string
	VVerified     bool
	VBlacklisted  bool
	Vid           EnlID
	OwnTracksJSON string
	RocksVerified bool
	RAID          bool
	Teams         []struct {
		ID        string
		Name      string
		State     string
		RocksComm string
	}
	OwnedTeams []struct {
		ID        string
		Name      string
		RocksComm string
		RocksKey  string
	}
	Telegram struct {
		UserName  string
		ID        int64
		Verified  bool
		Authtoken string
	}
}

// InitAgent is called from Oauth callback to set up a agent for the first time.
// It also checks and updates V and enl.rocks data. It returns true if the agent is authorized to continue, false if the agent is blacklisted or otherwise locked at V or enl.rocks.
func (gid GoogleID) InitAgent() (bool, error) {
	var authError error // delay reporting authorization problems until after INSERT/Vupdate/RocksUpdate
	var tmpName string

	var vdata Vresult
	err := gid.VSearch(&vdata)
	if err != nil {
		Log.Notice(err)
	}
	if vdata.Data.Agent != "" {
		err = gid.VUpdate(&vdata)
		if err != nil {
			Log.Notice(err)
			return false, err
		}
		if tmpName == "" {
			tmpName = vdata.Data.Agent
		}
		if vdata.Data.Quarantine == true {
			authError = fmt.Errorf("%s quarantined at V", vdata.Data.Agent)
		}
		if vdata.Data.Flagged == true {
			authError = fmt.Errorf("%s flagged at V", vdata.Data.Agent)
		}
		if vdata.Data.Blacklisted == true {
			authError = fmt.Errorf("%s blacklisted at V", vdata.Data.Agent)
		}
		if vdata.Data.Banned == true {
			authError = fmt.Errorf("%s banned at V", vdata.Data.Agent)
		}
	}

	var rocks RocksAgent
	err = gid.RocksSearch(&rocks)
	if err != nil {
		Log.Error(err)
	}
	if rocks.Agent != "" {
		err = gid.RocksUpdate(&rocks)
		if err != nil {
			Log.Notice(err)
			return false, err
		}
		if tmpName == "" {
			tmpName = rocks.Agent
		}
		if rocks.Smurf == true {
			authError = fmt.Errorf("%s listed as a smurf at enl.rocks", rocks.Agent)
		}
	}

	// if the agent doesn't exist, prepopulate everything
	_, err = gid.IngressName()
	if err != nil && err.Error() == "sql: no rows in result set" {
		if tmpName == "" {
			tmpName = "UnverifiedAgent_" + gid.String()[:15]
		}
		lockey, err := GenerateSafeName()
		if err != nil {
			Log.Error(err)
			return false, err
		}
		_, err = db.Exec("INSERT IGNORE INTO user (gid, iname, level, lockey, OTpassword, VVerified, VBlacklisted, Vid, RocksVerified, RAID) VALUES (?,?,?,?,NULL,?,?,?,?,?)",
			gid, tmpName, vdata.Data.Level, lockey, vdata.Data.Verified, vdata.Data.Blacklisted, vdata.Data.EnlID, rocks.Verified, 0)
		if err != nil {
			Log.Error(err)
			return false, err
		}
		_, err = db.Exec("INSERT IGNORE INTO locations (gid, upTime, loc) VALUES (?,NOW(),POINT(0,0))", gid)
		if err != nil {
			Log.Error(err)
			return false, err
		}
		_, err = db.Exec("INSERT IGNORE INTO otdata (gid, otdata) VALUES (?,'{ }')", gid)
		if err != nil {
			Log.Error(err)
			return false, err
		}
		_ = gid.ownTracksExternalUpdate("0", "0", "reaper")
	} else if err != nil {
		return false, err
	}

	if authError != nil {
		Log.Notice(authError)
		return false, authError
	}
	return true, nil
}

// SetIngressName is called to update the agent's ingress name in the database.
// The ingress name cannot be updated if V or Rocks verification has taken place.
// XXX Do we even want to allow this any longer since V and rocks are giving us data? Unverified agents can just live with the Agent_XXXXXX name.
func (gid GoogleID) SetIngressName(name string) error {
	// if VVerified or RocksVerified: ignore name changes -- let the V/Rocks functions take care of that
	_, err := db.Exec("UPDATE user SET iname = ? WHERE gid = ? AND VVerified = 0 AND RocksVerified = 0", name, gid)
	if err != nil {
		Log.Notice(err)
	}
	return err
}

// Gid converts a location share key to a agent's gid
func (lockey LocKey) Gid() (GoogleID, error) {
	var gid GoogleID

	r := db.QueryRow("SELECT gid FROM user WHERE lockey = ?", lockey)
	err := r.Scan(&gid)
	if err != nil {
		Log.Notice(err)
		return "", err
	}

	return gid, nil
}

// GetAgentData populates a AgentData struct based on the gid
func (gid GoogleID) GetAgentData(ud *AgentData) error {
	var ot sql.NullString
	var otJSON sql.NullString

	ud.GoogleID = gid

	row := db.QueryRow("SELECT u.iname, u.level, u.lockey, u.OTpassword, u.VVerified, u.VBlacklisted, u.Vid, u.RocksVerified, u.RAID, ot.otdata FROM user=u, otdata=ot WHERE u.gid = ? AND ot.gid = u.gid", gid)
	err := row.Scan(&ud.IngressName, &ud.Level, &ud.LocationKey, &ot, &ud.VVerified, &ud.VBlacklisted, &ud.Vid, &ud.RocksVerified, &ud.RAID, &otJSON)
	if err != nil && err.Error() == "sql: no rows in result set" {
		// if you delete yourself and don't wait for your session cookie to expire to rejoin...
		err = fmt.Errorf("Unknown GoogleID: %s. Try restarting your browser.", gid)
		return err
	}
	if err != nil {
		Log.Notice(err)
		return err
	}

	if ot.Valid {
		ud.OwnTracksPW = ot.String
	}
	if otJSON.Valid {
		ud.OwnTracksJSON = otJSON.String
	} else {
		ud.OwnTracksJSON = "{ }"
	}

	var teamname sql.NullString
	var tmp struct {
		ID        string
		Name      string
		State     string
		RocksComm string
	}

	rows, err := db.Query("SELECT t.teamID, t.name, x.state, rockscomm "+
		"FROM teams=t, userteams=x "+
		"WHERE x.gid = ? AND x.teamID = t.teamID", gid)
	if err != nil {
		Log.Error(err)
		return err
	}

	defer rows.Close()
	var rc sql.NullString
	for rows.Next() {
		err := rows.Scan(&tmp.ID, &teamname, &tmp.State, &rc)
		if err != nil {
			Log.Error(err)
			return err
		}
		// teamname can be null
		if teamname.Valid {
			tmp.Name = teamname.String
		} else {
			tmp.Name = ""
		}
		if rc.Valid {
			tmp.RocksComm = rc.String
		} else {
			tmp.RocksComm = ""
		}
		ud.Teams = append(ud.Teams, tmp)
	}

	var ownedTeam struct {
		ID        string
		Name      string
		RocksComm string
		RocksKey  string
	}
	ownedTeamRow, err := db.Query("SELECT teamID, name, rockscomm, rockskey FROM teams WHERE owner = ?", gid)
	if err != nil {
		Log.Error(err)
		return err
	}
	defer ownedTeamRow.Close()
	var rockscomm, rockskey sql.NullString
	for ownedTeamRow.Next() {
		err := ownedTeamRow.Scan(&ownedTeam.ID, &teamname, &rockscomm, &rockskey)
		if err != nil {
			Log.Error(err)
			return err
		}
		// can be null -- but should change schema to disallow that
		if teamname.Valid {
			ownedTeam.Name = teamname.String
		} else {
			ownedTeam.Name = ""
		}
		if rockscomm.Valid {
			ownedTeam.RocksComm = rockscomm.String
		} else {
			ownedTeam.RocksComm = ""
		}
		if rockskey.Valid {
			ownedTeam.RocksKey = rockskey.String
		} else {
			ownedTeam.RocksKey = ""
		}
		ud.OwnedTeams = append(ud.OwnedTeams, ownedTeam)
	}

	var authtoken sql.NullString
	rows3 := db.QueryRow("SELECT telegramName, telegramID, verified, authtoken FROM telegram WHERE gid = ?", gid)
	err = rows3.Scan(&ud.Telegram.UserName, &ud.Telegram.ID, &ud.Telegram.Verified, &authtoken)
	if err != nil && err.Error() == "sql: no rows in result set" {
		ud.Telegram.ID = 0
		ud.Telegram.Verified = false
		ud.Telegram.Authtoken = ""
	} else if err != nil {
		Log.Error(err)
		return err
	}
	ud.Telegram.Authtoken = authtoken.String

	return nil
}

// AgentLocation updates the database to reflect a agent's current location
// OwnTracks data is updated to reflect the change
// TODO: react based on the location
func (gid GoogleID) AgentLocation(lat, lon, source string) error {
	var point string

	// sanity checing on bounds?
	// YES, store lon,lat -- the ST_ functions expect it this way
	point = fmt.Sprintf("POINT(%s %s)", lon, lat)
	if _, err := db.Exec("UPDATE locations SET loc = PointFromText(?), upTime = NOW() WHERE gid = ?", point, gid); err != nil {
		Log.Notice(err)
		return err
	}

	if source != "OwnTracks" {
		err := gid.ownTracksExternalUpdate(lat, lon, source)
		if err != nil {
			Log.Notice(err)
			return err
		}
		// put it out onto MQTT
	}

	// XXX check for waypoints in range
	// XXX check for markers in range

	// XXX send notifications

	return nil
}

// ResetLocKey updates the database with a new OwnTracks password for a given agent
func (gid GoogleID) ResetLocKey() error {
	newlockey, _ := GenerateSafeName()
	_, err := db.Exec("UPDATE user SET lockey = ? WHERE gid = ?", newlockey, gid)
	if err != nil {
		Log.Notice(err)
	}
	return err
}

// IngressName returns an agent's name for a GoogleID
func (gid GoogleID) IngressName() (string, error) {
	var iname string
	r := db.QueryRow("SELECT iname FROM user WHERE gid = ?", gid)
	err := r.Scan(&iname)

	return iname, err
}

func (gid GoogleID) String() string {
	return string(gid)
}

func (eid EnlID) String() string {
	return string(eid)
}

// revalidateEveryone -- if the schema changes or another reason causes us to need to pull data from V and rocks, this is a function which does that
// V had bulk API functions we should use instead. This is good enough, and I hope we don't need it again.
func revalidateEveryone() error {
	rows, err := db.Query("SELECT gid FROM user")
	if err != nil {
		Log.Error(err)
		return err
	}

	var gid GoogleID
	defer rows.Close()
	for rows.Next() {
		var v Vresult
		var r RocksAgent

		err := rows.Scan(&gid)
		if err != nil {
			Log.Error(err)
			continue
		}
		err = gid.VSearch(&v)
		if err != nil {
			Log.Error(err)
			continue
		}
		err = gid.VUpdate(&v)
		if err != nil {
			Log.Error(err)
			continue
		}
		err = gid.RocksSearch(&r)
		if err != nil {
			Log.Error(err)
			continue
		}
		err = gid.RocksUpdate(&r)
		if err != nil {
			Log.Error(err)
			continue
		}
	}
	return nil
}

// SearchAgentName gets a GoogleID from an Agent's name
func SearchAgentName(agent string) (GoogleID, error) {
	var gid GoogleID
	err := db.QueryRow("SELECT gid FROM user WHERE LOWER(iname) LIKE LOWER(?)", agent).Scan(&gid)
	if err != nil {
		Log.Notice(err)
		return "", err
	}
	return gid, nil
}

// Delete removes an agent and all associated data
func (gid GoogleID) Delete() error {
	// teams require special attention since they might be linked to .rocks communities
	var teamID TeamID
	rows, err := db.Query("SELECT teamID FROM teams WHERE owner = ?", gid)
	if err != nil {
		Log.Notice(err)
		return err
	}
	for rows.Next() {
		rows.Scan(&teamID)
		teamID.Delete()
	}

	// brute force delete everyhing else
	_, err = db.Exec("DELETE FROM user WHERE gid = ?", gid)
	if err != nil {
		Log.Notice(err)
		return err
	}

	// the foreign key constraints should take care of these, but just in case...
	_, _ = db.Exec("DELETE FROM otdata WHERE gid = ?", gid)
	_, _ = db.Exec("DELETE FROM locations WHERE gid = ?", gid)
	_, _ = db.Exec("DELETE FROM telegram WHERE gid = ?", gid)
	_, _ = db.Exec("DELETE FROM userteams WHERE gid = ?", gid)

	return nil
}
