package PhDevBin

import (
	"database/sql"
	"encoding/json"
	"fmt"
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
	State     bool
	LocKey    string
	Lat       float64
	Lon       float64
	Date      string
	OwnTracks json.RawMessage
}

type Target struct {
	Id              int
	Name            string
	Lat             float64
	Lon             float64
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
		err = db.QueryRow("SELECT COUNT(*) FROM userteams WHERE teamID = ? AND gid = ? AND state != 'Off'", team, id).Scan(&count)
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
			"AND x.state != 'Off'", team)
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
			if state.String != "Off" {
				tmpU.State = true
			} else {
				tmpU.State = false
			}
		} else {
			tmpU.State = false
		}
		if lat.Valid {
			tmpU.Lat, _ = strconv.ParseFloat(lat.String, 64)
		} else {
			var f float64
			f = 0
			tmpU.Lat = f
		}
		if lon.Valid {
			tmpU.Lon, _ = strconv.ParseFloat(lon.String, 64)
		} else {
			var f float64
			f = 0
			tmpU.Lon = f
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
			tmpT.Lat, _ = strconv.ParseFloat(lat.String, 64)
		} else {
			var f float64
			f = 0
			tmpT.Lat = f
		}
		if lon.Valid {
			tmpT.Lon, _ = strconv.ParseFloat(lon.String, 64)
		} else {
			var f float64
			f = 0
			tmpT.Lon = f
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

func AddUserToTeam(teamID string, lockey string) error {
	gid, err := LockeyToGid(lockey)
	if err != nil {
		Log.Notice(err)
		return err
	}

	_, err = db.Exec("INSERT INTO userteams values (?, ?, 'Off', '')", teamID, gid)
	if err != nil {
		tmp := err.Error()
		if tmp[:10] != "Error 1062" {
			Log.Notice(err)
			return err
		}
	}
	return nil
}

func DelUserFromTeam(teamID string, lockey string) error {
	gid, err := LockeyToGid(lockey)
	if err != nil {
		Log.Notice(err)
		return err
	}

	_, err = db.Exec("DELETE FROM userteams WHERE teamID = ? AND gid = ?", teamID, gid)
	if err != nil {
		Log.Notice(err)
		return (err)
	}
	return nil
}

func ClearPrimaryTeam(gid string) error {
	_, err := db.Exec("UPDATE userteams SET state = 'On' WHERE state = 'Primary' AND gid = ?", gid)
	if err != nil {
		Log.Notice(err)
		return (err)
	}
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
	return nil
}
