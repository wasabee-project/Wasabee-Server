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
	Name        string
	Verified    bool
	Blacklisted bool
	Color       string
	State       bool
	LocKey      string
	Lat         float64
	Lon         float64
	Date        string
	OwnTracks   json.RawMessage `json:"OwnTracks,omitmissing"`
	Distance    float64         `json:"Distance,omitmissing"`
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
	Distance        float64
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
	if err != nil || i < 1 {
		return false, err
	}
	return true, nil
}

func FetchTeam(team string, teamList *TeamData, fetchAll bool) error {
	var state, lat, lon, otdata sql.NullString // otdata can no longer be null, once the test users all get updated this can be removed
	var tmpU User
	var tmpT Target

	var err error
	var rows *sql.Rows
	if fetchAll != true {
		rows, err = db.Query("SELECT u.iname, u.lockey, x.color, x.state, Y(l.loc), X(l.loc), l.upTime, o.otdata, u.VVerified, u.VBlacklisted "+
			"FROM teams=t, userteams=x, user=u, locations=l, otdata=o "+
			"WHERE t.teamID = ? AND t.teamID = x.teamID AND x.gid = u.gid AND x.gid = l.gid AND u.gid = o.gid "+
			"AND x.state != 'Off'", team)
	} else {
		rows, err = db.Query("SELECT u.iname, u.lockey, x.color, x.state, Y(l.loc), X(l.loc), l.upTime, o.otdata, u.VVerified, u.VBlacklisted "+
			"FROM teams=t, userteams=x, user=u, locations=l, otdata=o "+
			"WHERE t.teamID = ? AND t.teamID = x.teamID AND x.gid = u.gid AND x.gid = l.gid AND u.gid = o.gid ", team)
	}
	if err != nil {
		Log.Error(err)
		return err
	}

	defer rows.Close()
	for rows.Next() {
		err := rows.Scan(&tmpU.Name, &tmpU.LocKey, &tmpU.Color, &state, &lat, &lon, &tmpU.Date, &otdata, &tmpU.Verified, &tmpU.Blacklisted)
		if err != nil {
			Log.Error(err)
			return err
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
		}
		if lon.Valid {
			tmpU.Lon, _ = strconv.ParseFloat(lon.String, 64)
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
	targetrows, err = db.Query("SELECT Id, Y(loc), X(loc), radius, type, name, expiration, linkdst FROM target WHERE teamID = ?", team)
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
		}
		if lon.Valid {
			tmpT.Lon, _ = strconv.ParseFloat(lon.String, 64)
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

func RenameTeam(name string, id string) error {
	_, err := db.Exec("UPDATE teams SET name = ? WHERE teamID = ?", name, id)
	if err != nil {
		Log.Notice(err)
	}
	return err
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

func TeammatesNearGid(gid string, maxdistance, maxresults int, teamList *TeamData) error {
	var state, lat, lon, otdata sql.NullString
	var tmpU User
	var rows *sql.Rows

	err := db.QueryRow("SELECT Y(loc), X(loc) FROM locations WHERE gid = ?", gid).Scan(&lat, &lon)
	if err != nil {
		Log.Error(err)
		return err
	}

	// no ST_Distance_Sphere in MariaDB yet...
	rows, err = db.Query("SELECT DISTINCT u.iname, u.lockey, x.color, x.state, Y(l.loc), X(l.loc), l.upTime, o.otdata, u.VVerified, u.VBlacklisted, "+
		"ROUND(6371 * acos (cos(radians(?)) * cos(radians(Y(l.loc))) * cos(radians(X(l.loc)) - radians(?)) + sin(radians(?)) * sin(radians(Y(l.loc))))) AS distance "+
		"FROM userteams=x, user=u, locations=l, otdata=o "+
		"WHERE x.teamID IN (SELECT teamID FROM userteams WHERE gid = ? AND state != 'Off') "+
		"AND x.state != 'Off' AND x.gid = u.gid AND x.gid = l.gid AND x.gid = o.gid AND l.upTime > SUBTIME(NOW(), '12:00:00') "+
		"HAVING distance < ? AND distance > 0 ORDER BY distance LIMIT 0,?", lat, lon, lat, gid, maxdistance, maxresults)
	if err != nil {
		Log.Error(err)
		return err
	}

	defer rows.Close()
	for rows.Next() {
		err := rows.Scan(&tmpU.Name, &tmpU.LocKey, &tmpU.Color, &state, &lat, &lon, &tmpU.Date, &otdata, &tmpU.Verified, &tmpU.Blacklisted, &tmpU.Distance)
		if err != nil {
			Log.Error(err)
			return err
		}
		if state.Valid && state.String != "Off" {
			tmpU.State = true
		}
		if lat.Valid {
			tmpU.Lat, _ = strconv.ParseFloat(lat.String, 64)
		}
		if lon.Valid {
			tmpU.Lon, _ = strconv.ParseFloat(lon.String, 64)
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
	return nil
}

func TargetsNearGid(gid string, maxdistance, maxresults int, targetList *TeamData) error {
	var lat, lon, linkdst sql.NullString
	var tmpT Target
	var rows *sql.Rows

	err := db.QueryRow("SELECT Y(loc), X(loc) FROM locations WHERE gid = ?", gid).Scan(&lat, &lon)
	if err != nil {
		Log.Error(err)
		return err
	}

	// no ST_Distance_Sphere in MariaDB yet...
	rows, err = db.Query("SELECT DISTINCT Id, name, radius, type, expiration, linkdst, Y(loc), X(loc), "+
		"ROUND(6371 * acos (cos(radians(?)) * cos(radians(Y(loc))) * cos(radians(X(loc)) - radians(?)) + sin(radians(?)) * sin(radians(Y(loc))))) AS distance "+
		"FROM target "+
		"WHERE teamID IN (SELECT teamID FROM userteams WHERE gid = ? AND state != 'Off') "+
		"HAVING distance < ? ORDER BY distance LIMIT 0,?", lat, lon, lat, gid, maxdistance, maxresults)
	if err != nil {
		Log.Error(err)
		return err
	}

	defer rows.Close()
	for rows.Next() {
		err := rows.Scan(&tmpT.Id, &tmpT.Name, &tmpT.Radius, &tmpT.Type, &tmpT.Expiration, &linkdst, &lat, &lon, &tmpT.Distance)
		if err != nil {
			Log.Error(err)
			return err
		}
		if linkdst.Valid {
			tmpT.LinkDestination = linkdst.String
		}
		if lat.Valid {
			tmpT.Lat, _ = strconv.ParseFloat(lat.String, 64)
		}
		if lon.Valid {
			tmpT.Lon, _ = strconv.ParseFloat(lon.String, 64)
		}
		targetList.Target = append(targetList.Target, tmpT)
	}
	err = rows.Err()
	if err != nil {
		Log.Error(err)
		return err
	}
	return nil
}
