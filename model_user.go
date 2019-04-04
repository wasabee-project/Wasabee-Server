package PhDevBin

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

// TelegramID is a Telegram user ID
// type TelegramID int

// UserData is the complete user struct, used for the /me page
type UserData struct {
	GoogleID      GoogleID
	IngressName   string
	Level         float64
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
	Home struct {
		Lat    float64
		Lon    float64
		Radius float64
	} // unused currently Tony wants it, requires schema change
}

// InitUser is called from Oauth callback to set up a user for the first time.
// It also checks and updates V and enl.rocks data. It returns true if the user is authorized to continue, false if the user is blacklisted or otherwise locked at V or enl.rocks.
func (gid GoogleID) InitUser() (bool, error) {
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

	// if the user doesn't exist, prepopulate everything
	_, err = gid.IngressName()
	if err != nil && err.Error() == "sql: no rows in result set" {
		if tmpName == "" {
			tmpName = "Agent_" + gid.String()[:8]
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
		_, err = db.Exec("INSERT IGNORE INTO locations VALUES (?,NOW(),POINT(0,0))", gid)
		if err != nil {
			Log.Error(err)
			return false, err
		}
		_, err = db.Exec("INSERT IGNORE INTO otdata VALUES (?,'{ }')", gid)
		if err != nil {
			Log.Error(err)
			return false, err
		}
	}
	if err != nil && err.Error() != "sql: no rows in result set" {
		Log.Error(err)
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

// SetOwnTracksPW updates the database with a new OwnTracks password for a given user
// TODO: move to model_owntracks.go
func (gid GoogleID) SetOwnTracksPW(otpw string) error {
	_, err := db.Exec("UPDATE user SET OTpassword = PASSWORD(?) WHERE gid = ?", otpw, gid)
	if err != nil {
		Log.Notice(err)
	}
	return err
}

// VerifyOwnTracksPW is used to check that the supplied password matches the stored password hash for the given user
// upon success it returns the gid for the lockey (which is also the owntracks username), on failure it returns ""
// TODO: move to model_owntracks.go
func (lockey LocKey) VerifyOwnTracksPW(otpw string) (GoogleID, error) {
	var gid GoogleID

	r := db.QueryRow("SELECT gid FROM user WHERE OTpassword = PASSWORD(?) AND lockey = ?", otpw, lockey)
	err := r.Scan(&gid)
	if err != nil && err != sql.ErrNoRows {
		Log.Notice(err)
		return "", err
	}
	if err != nil && err == sql.ErrNoRows {
		return "", nil
	}

	return gid, nil
}

// SetTeamState updates the users state on the team (Off|On|Primary)
// XXX move to model_team.go
func (gid GoogleID) SetTeamState(teamID TeamID, state string) error {
	if state == "Primary" {
		_ = gid.ClearPrimaryTeam()
	}

	if _, err := db.Exec("UPDATE userteams SET state = ? WHERE gid = ? AND teamID = ?", state, gid, teamID); err != nil {
		Log.Notice(err)
	}
	return nil
}

// SetTeamStateName -- same as SetTeamState, but takes a team's human name rather than ID
// XXX BUG: if multiple teams use the same name this will not work
func (gid GoogleID) SetTeamStateName(teamname string, state string) error {
	var id TeamID
	row := db.QueryRow("SELECT teamID FROM teams WHERE name = ?", teamname)
	err := row.Scan(&id)
	if err != nil {
		Log.Notice(err)
	}

	return gid.SetTeamState(id, state)
}

// Gid converts a location share key to a user's gid
// TODO: quit using a prebuilt query from database.go
func (lockey LocKey) Gid() (GoogleID, error) {
	var gid GoogleID

	r := lockeyToGid.QueryRow(lockey)
	err := r.Scan(&gid)
	if err != nil {
		Log.Notice(err)
		return "", err
	}

	return gid, nil
}

// GetUserData populates a UserData struct based on the gid
func (gid GoogleID) GetUserData(ud *UserData) error {
	var ot sql.NullString
	var otJSON sql.NullString

	ud.GoogleID = gid

	row := db.QueryRow("SELECT u.iname, u.level, u.lockey, u.OTpassword, u.VVerified, u.VBlacklisted, u.Vid, u.RocksVerified, u.RAID, ot.otdata FROM user=u, otdata=ot WHERE u.gid = ? AND ot.gid = u.gid", gid)
	err := row.Scan(&ud.IngressName, &ud.Level, &ud.LocationKey, &ot, &ud.VVerified, &ud.VBlacklisted, &ud.Vid, &ud.RocksVerified, &ud.RAID, &otJSON)
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

// UserLocation updates the database to reflect a user's current location
// OwnTracks data is updated to reflect the change
// TODO: react based on the location
func (gid GoogleID) UserLocation(lat, lon, source string) error {
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

// ResetLocKey updates the database with a new OwnTracks password for a given user
func (gid GoogleID) ResetLocKey() error {
	newlockey := GenerateName()
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
