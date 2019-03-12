package PhDevBin

import (
	"database/sql"
	"encoding/json"
	"strconv"
)

// team stuff
type TeamData struct {
	Name   string
	Id     string
	User   []User
	Target []Target
}

type User struct {
	Name      string
	Color     string
	State     string // enum On Off
	LocKey    string
	Lat       string
	Lon       string
	Date      string
	OwnTracks json.RawMessage
}

type Target struct {
	Id              int
	Name            string
	Lat             string
	Lon             string
	Radius          int    // in meters
	Type            string // enum ?
	Expiration      string
	LinkDestination string
	// PortalID   string
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
	var tmpU User
	var tmpT Target

	var err error
	var rows *sql.Rows
	if fetchAll != true {
		rows, err = db.Query("SELECT u.iname, u.lockey, x.color, x.state, X(l.loc), Y(l.loc), l.upTime, o.otdata "+
			"FROM teams=t, userteams=x, user=u, locations=l, otdata=o "+
			"WHERE t.teamID = ? AND t.teamID = x.teamID AND x.gid = u.gid AND x.gid = l.gid AND u.gid = o.gid "+
			"AND x.state = 'On'", team)
	} else {
		rows, err = db.Query("SELECT u.iname, u.lockey, x.color, x.state, X(l.loc), Y(l.loc), l.upTime, o.otdata "+
			"FROM teams=t, userteams=x, user=u, locations=l, otdata=o "+
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
			tmpU.Name = iname.String
		} else {
			tmpU.Name = ""
		}
		if lockey.Valid {
			tmpU.LocKey = lockey.String
		} else {
			tmpU.LocKey = ""
		}
		if color.Valid {
			tmpU.Color = color.String
		} else {
			tmpU.Color = ""
		}
		if state.Valid {
			tmpU.State = state.String
		} else {
			tmpU.State = "Off"
		}
		if lat.Valid {
			tmpU.Lat = lat.String
		} else {
			tmpU.Lat = "0"
		}
		if lon.Valid {
			tmpU.Lon = lon.String
		} else {
			tmpU.Lon = "0"
		}
		if uptime.Valid {
			tmpU.Date = uptime.String
		} else {
			tmpU.Date = ""
		}
		if otdata.Valid {
			tmpU.OwnTracks = json.RawMessage(otdata.String)
		} else {
			tmpU.OwnTracks = json.RawMessage("{ }")
		}
		teamList.User = append(teamList.User, tmpU)
	}
	err = rows.Err()
	if err != nil {
		Log.Error(err)
		return err
	}

	if err := db.QueryRow("SELECT name FROM teams WHERE teamID = ?", team).Scan(&teamList.Name); err != nil {
		Log.Error(err)
		return err
	}
	teamList.Id = team

	var targetid, radius, targettype, targetname, expiration, linkdst sql.NullString
	var targetrows *sql.Rows
	targetrows, err = db.Query("SELECT Id, X(loc), Y(loc), radius, type, name, expiration, linkdst FROM target WHERE teamID = ?", team)
	if err != nil {
		Log.Error(err)
		return err
	}

	defer targetrows.Close()
	for targetrows.Next() {
		err := targetrows.Scan(&targetid, &lat, &lon, &radius, &targettype, &targetname, &expiration, &linkdst)
		if err != nil {
			Log.Error(err)
			return err
		}

		if targetid.Valid {
			i, _ := strconv.Atoi(targetid.String)
			tmpT.Id = i
		} else {
			tmpT.Id = 0
		}
		if lat.Valid {
			tmpT.Lat = lat.String
		} else {
			tmpT.Lat = "0"
		}
		if lon.Valid {
			tmpT.Lon = lon.String
		} else {
			tmpT.Lon = "0"
		}
		if radius.Valid {
			i, _ := strconv.Atoi(radius.String)
			tmpT.Radius = i
		} else {
			tmpT.Radius = 30
		}
		if targettype.Valid {
			tmpT.Type = targettype.String
		} else {
			tmpT.Type = "target"
		}
		if targetname.Valid {
			tmpT.Name = targetname.String
		} else {
			tmpT.Name = "Unnamed Target"
		}
		if expiration.Valid {
			tmpT.Expiration = expiration.String
		} else {
			tmpT.Expiration = ""
		}
		if linkdst.Valid {
			tmpT.LinkDestination = linkdst.String
		} else {
			tmpT.LinkDestination = ""
		}

		teamList.Target = append(teamList.Target, tmpT)
	}
	err = targetrows.Err()
	if err != nil {
		Log.Error(err)
		return err
	}

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
