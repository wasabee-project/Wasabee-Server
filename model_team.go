package PhDevBin

import (
	"database/sql"
	"strconv"
	"encoding/json"
)

// team stuff
type TeamData struct {
	Name  string
	Id    string
	User  []struct {
		Name   string
		Color  string
		State  string // enum On Off
		LocKey string
		Lat    string
		Lon    string
		Date   string
		OwnTracks json.RawMessage 
	}
	Target []struct {
		Name       string
		PortalID   string
		Lat        string
		Lon        string
		Range      int    // in meters
		Kind       string // enum ?
		AssignedTo string
	}
}

func UserInTeam(id string, team string, allowOff bool) (bool, error) {
	var count string

	var err error
	if allowOff {
		err = db.QueryRow("SELECT COUNT(*) FROM userteams WHERE teamID = ? AND gid = ?", team, id).Scan(&count)
	} else {
		err = db.QueryRow("SELECT COUNT(*) FROM userteams WHERE teamID = ? AND gid = ? AND state = 'On'", team, id).Scan(&count)
	}
	if err != nil {
		return false, err
	}
	i, err := strconv.Atoi(count)
	if i < 1 {
		return false, nil
	}
	return true, nil
}

func FetchTeam(team string, teamList *TeamData, fetchAll bool) error {
	var iname, color, state, lockey, lat, lon, uptime, otdata sql.NullString
	var tmp struct {
		Name   string
		Color  string
		State  string
		LocKey string
		Lat    string
		Lon    string
		Date   string
		OwnTracks json.RawMessage
	}

	var err error
	var rows *sql.Rows
	if fetchAll != true {
		rows, err = db.Query("SELECT u.iname, u.lockey, x.color, x.state, X(l.loc), Y(l.loc), l.upTime, o.otdata "+
			"FROM teams=t, userteams=x, user=u, locations=l, otdata=o "+
			"WHERE t.teamID = ? AND t.teamID = x.teamID AND x.gid = u.gid AND x.gid = l.gid AND u.gid = o.gid "+
			"AND x.state = 'On'", team)
	} else {
		rows, err = db.Query("SELECT u.iname, u.lockey, x.color, x.state, X(l.loc), Y(l.loc), l.upTime, o.otdata "+
			"FROM teams=t, userteams=x, user=u, locations=l "+
			"WHERE t.teamID = ? AND t.teamID = x.teamID AND x.gid = u.gid AND x.gid = l.gid AND u.gid = o.gid ", team)
	}
	if err != nil {
		Log.Error(err)
		return err
	}

	defer rows.Close()
	for rows.Next() {
		err := rows.Scan(&iname, &lockey, &color, &state, &lat, &lon, &uptime, &otdata)
		if err != nil {
			Log.Error(err)
			return err
		}
		if iname.Valid {
			tmp.Name = iname.String
		} else {
			tmp.Name = ""
		}
		if lockey.Valid {
			tmp.LocKey = lockey.String
		} else {
			tmp.LocKey = ""
		}
		if color.Valid {
			tmp.Color = color.String
		} else {
			tmp.Color = ""
		}
		if state.Valid {
			tmp.State = state.String
		} else {
			tmp.State = "Off"
		}
		if lat.Valid { // this will need love
			tmp.Lat = lat.String
		} else {
			tmp.Lat = "0"
		}
		if lon.Valid { // this will need love
			tmp.Lon = lon.String
		} else {
			tmp.Lon = "0"
		}
		if uptime.Valid { // this will need love
			tmp.Date = uptime.String
		} else {
			tmp.Date = ""
		}
		if otdata.Valid { // this will need love
			tmp.OwnTracks = json.RawMessage(otdata.String)
		} else {
			tmp.OwnTracks = json.RawMessage("{ }")
		}
		teamList.User = append(teamList.User, tmp)
	}
	err = rows.Err()
	if err != nil {
		Log.Error(err)
		return err
	}

	// owner should probably not be exposed like this, show the display name instead
	err = db.QueryRow("SELECT name FROM teams WHERE teamID = ?", team).Scan(&teamList.Name)
	if err != nil {
		Log.Error(err)
		return err
	}
	teamList.Id = team

	return nil
}

func UserOwnsTeam(id string, team string) (bool, error) {
	var owner string

	err := db.QueryRow("SELECT owner FROM teams WHERE teamID = ?", team).Scan(&owner)
	// returning err w/o checking is lazy, but same result
	if id == owner {
		return true, err
	}
	return false, err
}

func NewTeam(name string, id string) (string, error) {
	team, err := GenerateSafeName()
	if err != nil {
		Log.Notice(err)
		return "", err
	}
	_, err = db.Exec("INSERT INTO teams VALUES (?,?,?)", team, id, name)
	if err != nil {
		Log.Notice(err)
	}
	_, err = db.Exec("INSERT INTO userteams VALUES (?,?,'On','FF0000')", team, id)
	if err != nil {
		Log.Notice(err)
	}
	return name, err
}

func DeleteTeam(teamID string) error {
	_, err := db.Exec("DELETE FROM teams WHERE teamID = ?", teamID)
	if err != nil {
		Log.Notice(err)
	}
	_, err = db.Exec("DELETE FROM userteams WHERE teamID = ?", teamID)
	if err != nil {
		Log.Notice(err)
	}
	return err
}

func AddUserToTeam(teamID string, id string) error {
	var gid sql.NullString
	err := db.QueryRow("SELECT gid FROM user WHERE lockey = ?", id).Scan(&gid)
	if err != nil {
		Log.Notice(id)
		Log.Notice(err)
		return err
	}

	_, err = db.Exec("INSERT INTO userteams values (?, ?, 'Off', '')", teamID, gid)
	if err != nil {
		Log.Notice(err)
	}
	return err
}

func DelUserFromTeam(teamID string, id string) error {
	var gid sql.NullString
	err := db.QueryRow("SELECT gid FROM user WHERE lockey = ?", id).Scan(&gid)
	if err != nil {
		Log.Notice(id)
		Log.Notice(err)
		return err
	}

	_, err = db.Exec("DELETE FROM userteams WHERE teamID = ? AND gid = ?", teamID, gid)
	if err != nil {
		Log.Notice(err)
	}
	return err
}

func UserLocation(id string, lat string, lon string) error {
	var point string
	// sanity checing on bounds?
	point = "POINT(" + lat + " " + lon + ")"
	_, err := locQuery.Exec(point, id)
	if err != nil {
		Log.Notice(err)
	}
	return err
}
