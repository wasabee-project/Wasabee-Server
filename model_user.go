package PhDevBin

import (
	"database/sql"
)

// user stuff
type UserData struct {
	IngressName string
	LocationKey string
	Teams       []struct {
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
}

func InsertOrUpdateUser(id string, name string) error {
	var tmpName = "Agent_" + id[:5]
	lockey, err := GenerateSafeName()
	_, err = db.Exec("INSERT INTO user VALUES (?,?,?) ON DUPLICATE KEY UPDATE gid = ?", id, tmpName, lockey, id)
	if err != nil {
		Log.Notice(err)
		return err
	}
	_, err = db.Exec("INSERT INTO locations VALUES (?,NOW(),POINT(0,0)) ON DUPLICATE KEY UPDATE upTime = NOW()", id)
	if err != nil {
		Log.Notice(err)
	}
	return err
}

func SetIngressName(id string, name string) error {
	_, err := db.Exec("UPDATE user SET iname = ? WHERE gid = ?", name, id)
	if err != nil {
		Log.Notice(err)
	}
	return err
}

func RemoveUserFromTeam(id string, team string) error {
	_, err := db.Exec("DELETE FROM userteams WHERE gid = ? AND teamID = ?", team, id)
	if err != nil {
		Log.Notice(err)
	}
	return err
}

func SetUserTeamState(id string, team string, state string) error {
	if state != "On" {
		state = "Off"
	}
	_, err := db.Exec("UPDATE userteams SET state = ? WHERE gid = ? AND teamID = ?", state, id, team)
	if err != nil {
		Log.Notice(err)
	}
	return err
}

func GetUserData(id string, ud *UserData) error {
	var in, lc sql.NullString

	row := db.QueryRow("SELECT iname, lockey FROM user WHERE gid = ?", id)
	err := row.Scan(&in, &lc)
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

	var teamID, name, state sql.NullString
	var tmp struct {
		Id    string
		Name  string
		State string
	}

	rows, err := db.Query("SELECT t.teamID, t.name, x.state "+
		"FROM teams=t, userteams=x "+
		"WHERE x.gid = ? AND x.teamID = t.teamID", id)
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
	rowsO, err := db.Query("SELECT teamID, name FROM teams WHERE owner = ?", id)
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
	rows1, err := db.Query("SELECT id, authteam, upload, expiration, views FROM documents WHERE uploader = ?", id)
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

	return nil
}
