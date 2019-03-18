package PhDevBin

import (
	"database/sql"
	"encoding/json"
	"fmt"
)

// user stuff
type UserData struct {
	GoogleID      string
	IngressName   string
	LocationKey   string
	OwnTracksPW   string
	VVerified     bool
	Vblacklisted  bool
	OwnTracksJSON string
	Teams         []struct {
		ID    string
		Name  string
		State string
	}
	OwnedTeams []struct {
		ID   string
		Name string
	}
	Telegram struct {
		UserName  string
		ID        int
		Verified  bool
		Authtoken string
	}
}

// called from Oauth callback
func InitUser(gid string) error {
	var vdata Vresult

	err := VSearchUser(gid, &vdata)
	if err != nil {
		Log.Notice(err)
	}
	s, _ := json.MarshalIndent(&vdata, "", "  ")
	Log.Debug(string(s))

	var tmpName string
	if vdata.Data.Agent != "" {
		tmpName = vdata.Data.Agent
	} else {
		tmpName = "Agent_" + gid[:5]
	}

	lockey, err := GenerateSafeName()
	_, err = db.Exec("INSERT IGNORE INTO user (gid, iname, lockey, OTpassword, VVerified, Vblacklisted) VALUES (?,?,?,NULL,?,?)", gid, tmpName, lockey, vdata.Data.Verified, vdata.Data.Blacklisted)
	if err != nil {
		Log.Notice(err)
		return err
	}
	_, err = db.Exec("INSERT IGNORE INTO locations VALUES (?,NOW(),POINT(0,0))", gid)
	if err != nil {
		Log.Notice(err)
	}

	_, err = db.Exec("INSERT IGNORE INTO otdata VALUES (?,'{ }')", gid)
	if err != nil {
		Log.Notice(err)
	}
	return err
}

func SetIngressName(gid string, name string) error {
	// if VVerified, ignore name changes -- let the V functions take care of that
	_, err := db.Exec("UPDATE user SET iname = ? WHERE gid = ? AND VVerified = 0", name, gid)
	if err != nil {
		Log.Notice(err)
	}
	return err
}

// move to model_owntracks.go
func SetOwnTracksPW(gid string, otpw string) error {
	_, err := db.Exec("UPDATE user SET OTpassword = PASSWORD(?) WHERE gid = ?", otpw, gid)
	if err != nil {
		Log.Notice(err)
	}
	return err
}

// move to model_owntracks.go
func VerifyOwnTracksPW(lockey string, otpw string) (string, error) {
	var gid string

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

// XXX move to model_team.go
func RemoveUserFromTeam(gid string, team string) error {
	if _, err := db.Exec("DELETE FROM userteams WHERE gid = ? AND teamID = ?", team, gid); err != nil {
		Log.Notice(err)
	}
	return nil
}

// XXX move to model_team.go
func SetUserTeamState(gid string, team string, state string) error {
	if state == "Primary" {
		_ = ClearPrimaryTeam(gid)
	}

	if _, err := db.Exec("UPDATE userteams SET state = ? WHERE gid = ? AND teamID = ?", state, gid, team); err != nil {
		Log.Notice(err)
	}
	return nil
}

// XXX move to model_team.go
func SetUserTeamStateName(gid string, teamname string, state string) error {
	Log.Debug(teamname)
	var id string
	row := db.QueryRow("SELECT teamID FROM teams WHERE name = ?", teamname)
	err := row.Scan(&id)
	if err != nil {
		Log.Notice(err)
	}

	return SetUserTeamState(gid, id, state)
}

func LockeyToGid(lockey string) (string, error) {
	var gid string

	r := lockeyToGid.QueryRow(lockey)
	err := r.Scan(&gid)
	if err != nil {
		Log.Notice(err)
		return "", err
	}

	return gid, nil
}

func GetUserData(gid string, ud *UserData) error {
	var ot sql.NullString

	ud.GoogleID = gid

	row := db.QueryRow("SELECT iname, lockey, OTpassword, VVerified, Vblacklisted FROM user WHERE gid = ?", gid)
	err := row.Scan(&ud.IngressName, &ud.LocationKey, &ot, &ud.VVerified, &ud.Vblacklisted)
	if err != nil {
		Log.Notice(err)
		return err
	}

	if ot.Valid {
		ud.OwnTracksPW = ot.String
	}

	// err = VSearchUser(gid, &ud.VData)
	// s, _ := json.MarshalIndent(&ud.VData, "", "  ")
	// Log.Debug(string(s))

	var teamname sql.NullString
	var tmp struct {
		ID    string
		Name  string
		State string
	}

	rows, err := db.Query("SELECT t.teamID, t.name, x.state "+
		"FROM teams=t, userteams=x "+
		"WHERE x.gid = ? AND x.teamID = t.teamID", gid)
	if err != nil {
		Log.Error(err)
		return err
	}

	defer rows.Close()
	for rows.Next() {
		err := rows.Scan(&tmp.ID, &teamname, &tmp.State)
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
		ud.Teams = append(ud.Teams, tmp)
	}

	var ownedTeam struct {
		ID   string
		Name string
	}
	ownedTeamRow, err := db.Query("SELECT teamID, name FROM teams WHERE owner = ?", gid)
	if err != nil {
		Log.Error(err)
		return err
	}
	defer ownedTeamRow.Close()
	for ownedTeamRow.Next() {
		err := ownedTeamRow.Scan(&ownedTeam.ID, &teamname)
		if err != nil {
			Log.Error(err)
			return err
		}
		// can be null -- but this should be changed
		if teamname.Valid {
			ownedTeam.Name = teamname.String
		} else {
			ownedTeam.Name = ""
		}
		ud.OwnedTeams = append(ud.OwnedTeams, ownedTeam)
	}

	// XXX cannot be null -- just JOIN in main query
	var otJSON sql.NullString
	rows2 := db.QueryRow("SELECT otdata FROM otdata WHERE gid = ?", gid)
	err = rows2.Scan(&otJSON)
	if err != nil && err.Error() == "sql: no rows in result set" {
		ud.OwnTracksJSON = "{ }"
		return nil
	}
	if err != nil {
		Log.Error(err)
		return err
	}
	if otJSON.Valid {
		ud.OwnTracksJSON = otJSON.String
	} else {
		ud.OwnTracksJSON = "{ }"
	}
	// defer rows2.Close()

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
	// defer rows3.Close()

	return nil
}

func UserLocation(id, lat, lon, source string) error {
	var point string

	// sanity checing on bounds?
	point = fmt.Sprintf("POINT(%s %s)", lat, lon)
	if _, err := locQuery.Exec(point, id); err != nil {
		Log.Notice(err)
		return err
	}

	// XXX if source is not "OwnTracks" -- parse and rebuild the user's OT data?

	// XXX check for targets in range

	// XXX send notifications

	return nil
}
