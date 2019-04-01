package PhDevBin

import (
	"database/sql"
	"encoding/json"
	"strconv"
)

// TeamData is the wrapper type containing all the team info
type TeamData struct {
	Name      string
	ID        TeamID
	User      []User
	Markers   []Marker
	Waypoints []Waypoint
	RocksComm string
	RocksKey  string
}

// User is the light version of UserData, containing publicly visible information exported to teams
type User struct {
	Gid         GoogleID
	Name        string
	EnlID       EnlID
	Verified    bool
	Blacklisted bool
	Color       string
	State       bool
	LocKey      string
	Lat         float64 `json:"lat"`
	Lon         float64 `json:"lng"`
	Date        string
	OwnTracks   json.RawMessage `json:"OwnTracks,omitmissing"`
	Distance    float64         `json:"Distance,omitmissing"`
}

// UserInTeam checks to see if a user is in a team and (On|Primary).
// allowOff == true will report if a user is in a team even if they are Off. That should ONLY be used to display lists of teams to the calling user.
func (gid GoogleID) UserInTeam(team TeamID, allowOff bool) (bool, error) {
	var count string

	var err error
	if allowOff {
		err = db.QueryRow("SELECT COUNT(*) FROM userteams WHERE teamID = ? AND gid = ?", team, gid).Scan(&count)
	} else {
		err = db.QueryRow("SELECT COUNT(*) FROM userteams WHERE teamID = ? AND gid = ? AND state != 'Off'", team, gid).Scan(&count)
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

// FetchTeam populates an entire TeamData struct
// fetchAll includes users for whom their state == off, should only be used to display lists to the calling user
func (teamID TeamID) FetchTeam(teamList *TeamData, fetchAll bool) error {
	var state, lat, lon, otdata sql.NullString // otdata can no longer be null, once the test users all get updated this can be removed
	var tmpU User

	var err error
	var rows *sql.Rows
	if fetchAll != true {
		rows, err = db.Query("SELECT u.gid, u.iname, u.lockey, x.color, x.state, Y(l.loc), X(l.loc), l.upTime, o.otdata, u.VVerified, u.VBlacklisted, u.Vid "+
			"FROM teams=t, userteams=x, user=u, locations=l, otdata=o "+
			"WHERE t.teamID = ? AND t.teamID = x.teamID AND x.gid = u.gid AND x.gid = l.gid AND u.gid = o.gid "+
			"AND x.state != 'Off'", teamID)
	} else {
		rows, err = db.Query("SELECT u.gid, u.iname, u.lockey, x.color, x.state, Y(l.loc), X(l.loc), l.upTime, o.otdata, u.VVerified, u.VBlacklisted, u.Vid "+
			"FROM teams=t, userteams=x, user=u, locations=l, otdata=o "+
			"WHERE t.teamID = ? AND t.teamID = x.teamID AND x.gid = u.gid AND x.gid = l.gid AND u.gid = o.gid ", teamID)
	}
	if err != nil {
		Log.Error(err)
		return err
	}

	defer rows.Close()
	for rows.Next() {
		err := rows.Scan(&tmpU.Gid, &tmpU.Name, &tmpU.LocKey, &tmpU.Color, &state, &lat, &lon, &tmpU.Date, &otdata, &tmpU.Verified, &tmpU.Blacklisted, &tmpU.EnlID)
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

	var rockscomm, rockskey sql.NullString
	if err := db.QueryRow("SELECT name, rockscomm, rockskey FROM teams WHERE teamID = ?", teamID).Scan(&teamList.Name, &rockscomm, &rockskey); err != nil {
		Log.Error(err)
		return err
	}
	teamList.ID = teamID
	if rockscomm.Valid {
		teamList.RocksComm = rockscomm.String
	}
	if rockskey.Valid {
		teamList.RocksKey = rockskey.String
	}

	// Markers
	err = teamID.pdMarkers(teamList)
	if err != nil {
		Log.Error(err)
	}
	// Waypoints
	err = teamID.otWaypoints(teamList)
	if err != nil {
		Log.Error(err)
	}

	return nil
}

// OwnsTeam returns true if the userID owns the team identified by teamID
func (gid GoogleID) OwnsTeam(teamID TeamID) (bool, error) {
	var owner GoogleID

	err := db.QueryRow("SELECT owner FROM teams WHERE teamID = ?", teamID).Scan(&owner)
	// check for err or trust that the calling function will do that?
	if gid == owner {
		return true, err
	}
	return false, err
}

// NewTeam initializes a new team and returns a teamID
// the creating gid is added and enabled on that team by default
func (gid GoogleID) NewTeam(name string) (TeamID, error) {
	team, err := GenerateSafeName()
	if err != nil {
		Log.Notice(err)
		return "", err
	}
	_, err = db.Exec("INSERT INTO teams (teamID, owner, name, rockskey, rockscomm) VALUES (?,?,?,NULL,NULL)", team, gid, name)
	if err != nil {
		Log.Notice(err)
	}
	_, err = db.Exec("INSERT INTO userteams VALUES (?,?,'On','FF0000')", team, gid)
	if err != nil {
		Log.Notice(err)
	}
	return TeamID(name), err
}

// Rename sets a new name for a teamID
// does not check team ownership -- caller should take care of authorization
func (teamID TeamID) Rename(name string) error {
	_, err := db.Exec("UPDATE teams SET name = ? WHERE teamID = ?", name, teamID)
	if err != nil {
		Log.Notice(err)
	}
	return err
}

// Delete removes the team identified by teamID
// does not check team ownership -- caller should take care of authorization
func (teamID TeamID) Delete() error {
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

// AddUser adds a user (identified by LocKey or GoogleID) to a team
func (teamID TeamID) AddUser(in interface{}) error {
	var gid GoogleID
	switch v := in.(type) {
	case LocKey:
		lockey := v
		gid, _ = lockey.Gid()
	case GoogleID:
		gid = v
	default:
		Log.Debug("fed unknown type, guessing string")
		x := v.(string)
		gid = GoogleID(x)
	}

	_, err := db.Exec("INSERT INTO userteams values (?, ?, 'Off', '')", teamID, gid)
	if err != nil {
		tmp := err.Error()
		if tmp[:10] != "Error 1062" {
			Log.Notice(err)
			return err
		}
	}
	// XXX this needs to be a template and translated
	_, err = gid.SendMessage("You have been invited on to a team called; you can enable it now if you want.")
	if err != nil {
		Log.Notice(err)
		return (err)
	}
	return nil
}

// RemoveUser removes a user (identified by location share key) from a team.
func (teamID TeamID) RemoveUser(in interface{}) error {
	var gid GoogleID
	switch v := in.(type) {
	case LocKey:
		lockey := v
		gid, _ = lockey.Gid()
	case GoogleID:
		gid = v
	default:
		Log.Debug("fed unknown type, guessing string")
		x := v.(string)
		gid = GoogleID(x)
	}

	_, err := db.Exec("DELETE FROM userteams WHERE teamID = ? AND gid = ?", teamID, gid)
	if err != nil {
		Log.Notice(err)
		return (err)
	}
	return nil
}

// ClearPrimaryTeam sets any team marked as primary to "On" for a user
func (gid GoogleID) ClearPrimaryTeam() error {
	_, err := db.Exec("UPDATE userteams SET state = 'On' WHERE state = 'Primary' AND gid = ?", gid)
	if err != nil {
		Log.Notice(err)
		return (err)
	}
	return nil
}

// TeammatesNear identifies other agents who are on ANY mutual team within maxdistance km, returning at most maxresults
func (gid GoogleID) TeammatesNear(maxdistance, maxresults int, teamList *TeamData) error {
	var state, lat, lon, otdata sql.NullString
	var tmpU User
	var rows *sql.Rows

	err := db.QueryRow("SELECT Y(loc), X(loc) FROM locations WHERE gid = ?", gid).Scan(&lat, &lon)
	if err != nil {
		Log.Error(err)
		return err
	}
	// Log.Debug("Teammates Near: " + gid.String() + " @ " + lat.String + "," + lon.String + " " + strconv.Itoa(maxdistance) + " " + strconv.Itoa(maxresults))

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

// WaypointsNear returns any Waypoints near the specified gid, up to distance maxdistance, with a maximum of maxresults returned
// the Users portion of the TeamData is uninitialized
func (gid GoogleID) WaypointsNear(maxdistance, maxresults int, td *TeamData) error {
	var lat, lon sql.NullString
	var rows *sql.Rows
	var tmpW Waypoint

	err := db.QueryRow("SELECT Y(loc), X(loc) FROM locations WHERE gid = ?", gid).Scan(&lat, &lon)
	if err != nil {
		Log.Error(err)
		return err
	}
	// Log.Debug("Waypoints Near: " + gid.String() + " @ " + lat.String + "," + lon.String + " " + strconv.Itoa(maxdistance) + " " + strconv.Itoa(maxresults))

	// no ST_Distance_Sphere in MariaDB yet...
	rows, err = db.Query("SELECT DISTINCT Id, name, radius, type, Y(loc), X(loc), teamID, "+
		"ROUND(6371 * acos (cos(radians(?)) * cos(radians(Y(loc))) * cos(radians(X(loc)) - radians(?)) + sin(radians(?)) * sin(radians(Y(loc))))) AS distance "+
		"FROM waypoints "+
		"WHERE teamID IN (SELECT teamID FROM userteams WHERE gid = ? AND state != 'Off') "+
		"HAVING distance < ? ORDER BY distance LIMIT 0,?", lat, lon, lat, gid, maxdistance, maxresults)
	if err != nil {
		Log.Error(err)
		return err
	}

	defer rows.Close()
	for rows.Next() {
		err := rows.Scan(&tmpW.ID, &tmpW.Desc, &tmpW.Radius, &tmpW.MarkerType, &lat, &lon, &tmpW.TeamID, &tmpW.Distance)
		if err != nil {
			Log.Error(err)
			return err
		}
		if lat.Valid {
			tmpW.Lat, _ = strconv.ParseFloat(lat.String, 64)
		}
		if lon.Valid {
			tmpW.Lon, _ = strconv.ParseFloat(lon.String, 64)
		}
		td.Waypoints = append(td.Waypoints, tmpW)
	}
	err = rows.Err()
	if err != nil {
		Log.Error(err)
		return err
	}

	err = gid.pdMarkersNear(maxdistance, maxresults, td)
	return nil
}

// MarkersNear returns markers near the gid's current location and populates both the Markers and Waypoints of a TeamData
func (gid GoogleID) MarkersNear(maxdistance, maxresults int, td *TeamData) error {
	return nil
}

// SetRocks links a team to a community at enl.rocks.
// Does not check team ownership -- caller should take care of authorization.
// Local adds/deletes will be pushed to the community (API management must be enabled on the community at enl.rocks).
// adds/deletes at enl.rocks will be pushed here (onJoin/onLeave web hooks must be configured in the community at enl.rocks)
func (teamID TeamID) SetRocks(key, community string) error {
	_, err := db.Exec("UPDATE teams SET rockskey = ?, rockscomm = ? WHERE teamID = ?", key, community, teamID)
	if err != nil {
		Log.Notice(err)
	}
	return err
}

func (teamID TeamID) String() string {
	return string(teamID)
}
