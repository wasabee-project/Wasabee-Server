package PhDevBin

import (
	"database/sql"
	//	"errors"
)

// user stuff
type UserData struct {
	GoogleID      string
	IngressName   string
	LocationKey   string
	OwnTracksPW   string
	OwnTracksJSON string
	Teams         []struct {
		Id    string
		Name  string
		State string
	}
	OwnedTeams []struct {
		Name string
		Team string
	}
	OwnedDraws []struct {
		Hash       string
		AuthTeam   string
		UploadTime string
		Expiration string
		Views      string
	}
	Telegram struct {
		UserName  string
		ID        int
		Verified  bool
		Authtoken string
	}
}

func InsertOrUpdateUser(gid string, name string) error {
	var tmpName = "Agent_" + gid[:5]
	lockey, err := GenerateSafeName()
	_, err = db.Exec("INSERT INTO user VALUES (?,?,?,NULL) ON DUPLICATE KEY UPDATE gid = ?", gid, tmpName, lockey, gid)
	if err != nil {
		Log.Notice(err)
		return err
	}
	_, err = db.Exec("INSERT INTO locations VALUES (?,NOW(),POINT(0,0)) ON DUPLICATE KEY UPDATE upTime = NOW()", gid)
	if err != nil {
		Log.Notice(err)
	}

	_, err = db.Exec("INSERT INTO otdata VALUES (?,'{ }') ON DUPLICATE KEY UPDATE gid = ?", gid, gid)
	if err != nil {
		Log.Notice(err)
	}
	return err
}

func SetIngressName(gid string, name string) error {
	_, err := db.Exec("UPDATE user SET iname = ? WHERE gid = ?", name, gid)
	if err != nil {
		Log.Notice(err)
	}
	return err
}

func SetOwnTracksPW(gid string, otpw string) error {
	_, err := db.Exec("UPDATE user SET OTpassword = PASSWORD(?) WHERE gid = ?", otpw, gid)
	if err != nil {
		Log.Notice(err)
	}
	return err
}

func VerifyOwnTracksPW(lockey string, otpw string) (string, error) {
	var gid sql.NullString

	r := db.QueryRow("SELECT gid FROM user WHERE OTpassword = PASSWORD(?) AND lockey = ?", otpw, lockey)
	err := r.Scan(&gid)
	if err != nil && err != sql.ErrNoRows {
		Log.Notice(err)
		return "", err
	}
	if (err != nil && err == sql.ErrNoRows) || gid.Valid == false {
		return "", nil
	}

	return gid.String, nil
}

func RemoveUserFromTeam(gid string, team string) error {
	_, err := db.Exec("DELETE FROM userteams WHERE gid = ? AND teamID = ?", team, gid)
	if err != nil {
		Log.Notice(err)
	}
	return err
}

func SetUserTeamState(gid string, team string, state string) error {
	if state != "On" {
		state = "Off"
	}
	_, err := db.Exec("UPDATE userteams SET state = ? WHERE gid = ? AND teamID = ?", state, gid, team)
	if err != nil {
		Log.Notice(err)
	}
	return err
}

func LockeyToGid(lockey string) (string, error) {
	var gid sql.NullString

	// does not need to be a prep'd query anymore
	r := lockeyToGid.QueryRow(lockey)
	err := r.Scan(&gid)
	if err != nil {
		Log.Notice(err)
		return "", err
	}
	if gid.Valid == false {
		return "", nil
	}

	return gid.String, nil
}

func GetUserData(gid string, ud *UserData) error {
	var in, lc, ot sql.NullString

	ud.GoogleID = gid

	row := db.QueryRow("SELECT iname, lockey, otpassword FROM user WHERE gid = ?", gid)
	err := row.Scan(&in, &lc, &ot)
	if err != nil {
		Log.Notice(err)
		return err
	}

	// convert from sql.NullString to string in the struct
	if in.Valid {
		ud.IngressName = in.String
	}
	if lc.Valid {
		ud.LocationKey = lc.String
	}
	if ot.Valid {
		ud.OwnTracksPW = ot.String
	}

	var teamID, name, state sql.NullString
	var tmp struct {
		Id    string
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
		err := rows.Scan(&teamID, &name, &state)
		if err != nil {
			Log.Error(err)
			return err
		}
		if teamID.Valid {
			tmp.Id = teamID.String
		} else {
			tmp.Id = ""
		}
		if name.Valid {
			tmp.Name = name.String
		} else {
			tmp.Name = ""
		}
		if state.Valid {
			tmp.State = state.String
		} else {
			tmp.State = ""
		}
		ud.Teams = append(ud.Teams, tmp)
	}

	var tmpO struct {
		Name string
		Team string
	}
	rowsO, err := db.Query("SELECT teamID, name FROM teams WHERE owner = ?", gid)
	if err != nil {
		Log.Error(err)
		return err
	}
	defer rowsO.Close()
	for rowsO.Next() {
		err := rowsO.Scan(&teamID, &name)
		if err != nil {
			Log.Error(err)
			return err
		}
		if teamID.Valid {
			tmpO.Team = teamID.String
		} else {
			tmpO.Team = ""
		}
		if name.Valid {
			tmpO.Name = name.String
		} else {
			tmpO.Name = ""
		}
		ud.OwnedTeams = append(ud.OwnedTeams, tmpO)
	}

	var tmpDoc struct {
		Hash       string
		AuthTeam   string
		UploadTime string
		Expiration string
		Views      string
	}
	rows1, err := db.Query("SELECT id, authteam, upload, expiration, views FROM documents WHERE uploader = ?", gid)
	if err != nil {
		Log.Error(err)
		return err
	}
	var docID, authteam, upload, expiration, views sql.NullString
	defer rows1.Close()
	for rows1.Next() {
		err := rows1.Scan(&docID, &authteam, &upload, &expiration, &views)
		if err != nil {
			Log.Error(err)
			return err
		}
		if docID.Valid {
			tmpDoc.Hash = docID.String
		} else {
			tmpDoc.Hash = ""
		}
		if authteam.Valid {
			tmpDoc.AuthTeam = authteam.String
		} else {
			tmpDoc.AuthTeam = ""
		}
		if upload.Valid {
			tmpDoc.UploadTime = upload.String
		} else {
			tmpDoc.UploadTime = ""
		}
		if expiration.Valid {
			tmpDoc.Expiration = expiration.String
		} else {
			tmpDoc.Expiration = ""
		}
		if views.Valid {
			tmpDoc.Views = views.String
		} else {
			tmpDoc.Views = ""
		}
		ud.OwnedDraws = append(ud.OwnedDraws, tmpDoc)
	}

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
