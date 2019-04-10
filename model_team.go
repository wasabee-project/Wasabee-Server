package WASABI

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// TeamData is the wrapper type containing all the team info
type TeamData struct {
	Name      string
	ID        TeamID
	Agent     []Agent
	Markers   []Marker
	Waypoints []Waypoint
	RocksComm string
	RocksKey  string
}

// Agent is the light version of AgentData, containing visible information exported to teams
type Agent struct {
	Gid           GoogleID
	Name          string
	Level         int64
	EnlID         EnlID
	Verified      bool            `json:"Vverified,omitmissing"`
	Blacklisted   bool            `json:"blacklisted"`
	RocksVerified bool            `json:"rocks,omitmissing"`
	Color         string          `json:"color,omitmissing"`
	State         bool            `json:"state,omitmissing"`
	LocKey        string          `json:"lockey,omitmissing"`
	Lat           float64         `json:"lat,omitmissing"`
	Lon           float64         `json:"lng,omitmissing"`
	Date          string          `json:"date,omitmissing"`
	OwnTracks     json.RawMessage `json:"OwnTracks,omitmissing"`
	Distance      float64         `json:"distance,omitmissing"`
}

// AgentInTeam checks to see if a agent is in a team and (On|Primary).
// allowOff == true will report if a agent is in a team even if they are Off. That should ONLY be used to display lists of teams to the calling agent.
func (gid GoogleID) AgentInTeam(team TeamID, allowOff bool) (bool, error) {
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
// fetchAll includes agent for whom their state == off, should only be used to display lists to the calling agent
func (teamID TeamID) FetchTeam(teamList *TeamData, fetchAll bool) error {
	var state, lat, lon, otdata sql.NullString // otdata can no longer be null, once the test agent all get updated this can be removed
	var tmpU Agent

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
		teamList.Agent = append(teamList.Agent, tmpU)
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

// OwnsTeam returns true if the GoogleID owns the team identified by teamID
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
	_, err = db.Exec("INSERT INTO userteams (teamID, gid, state, color) VALUES (?,?,'On','00FF00')", team, gid)
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
	// XXX once we push to rocks, don't do it this way; remove each agent manually, so they get cleared from rocks community
	_, err := db.Exec("DELETE FROM userteams WHERE teamID = ?", teamID)
	if err != nil {
		Log.Notice(err)
	}
	_, err = db.Exec("DELETE FROM teams WHERE teamID = ?", teamID)
	if err != nil {
		Log.Notice(err)
	}
	return err
}

// toGid takes anything (for small values of anything) and returns a Gid for it.
// depends on EnlID.Gid(), LocKey.Gid(), TelegramID.Gid(), and SearchAgentName.
// If you pass in a string, it will see if it looks like a gid, eid, lockey, or agent name and try that.
func toGid(in interface{}) (GoogleID, error) {
	var gid GoogleID
	var err error
	switch v := in.(type) {
	case LocKey:
		lockey := v
		gid, err = lockey.Gid()
		if err != nil && err.Error() == "sql: no rows in result set" {
			err = fmt.Errorf("Unknown lockey: %s", lockey)
			Log.Info(err)
			return "", err
		}
	case GoogleID:
		gid = v
	case EnlID:
		eid := v
		gid, err = eid.Gid()
		if err != nil && err.Error() == "sql: no rows in result set" {
			err = fmt.Errorf("Unknown EnlID: %s", eid)
			Log.Info(err)
			return "", err
		}
	case TelegramID:
		tid := v
		gid, _, err = tid.GidV()
		if err != nil && err.Error() == "sql: no rows in result set" {
			err = fmt.Errorf("Unknown TelegramID: %d", tid)
			Log.Info(err)
			return "", err
		}
	default:
		tmp := v.(string)
		switch len(tmp) { // length gives us a guess, presence of a - makes us certain
		case 40:
			if strings.IndexByte(tmp, '-') != -1 {
				lockey := LocKey(tmp)
				gid, _ = toGid(lockey) // recurse and try again
			} else {
				eid := EnlID(tmp)   // Looks like an EnlID
				gid, _ = toGid(eid) // recurse and try again
			}
		case 21:
			if strings.IndexByte(tmp, '-') != -1 {
				lockey := LocKey(tmp)
				gid, _ = toGid(lockey) // recurse and try again
			} else {
				gid = GoogleID(tmp) // Looks like it already is a GoogleID
			}
		default:
			if strings.IndexByte(tmp, '-') != -1 {
				lockey := LocKey(tmp)
				gid, _ = toGid(lockey)
			} else {
				gid, _ = SearchAgentName(tmp)
				if err != nil && err.Error() == "sql: no rows in result set" { // XXX this can't happen any longer
					err = fmt.Errorf("Unknown agent (XXX impossible route): %s", tmp)
					Log.Info(err)
					return "", err
				} else if err != nil {
					Log.Notice(err)
					return "", err
				} else if gid == "" {
					err = fmt.Errorf("Unknown agent: %s", tmp)
					Log.Info(err)
					return "", err
				}
			}
		}
	}
	return gid, nil
}

// AddAgent adds a agent (identified by LocKey, EnlID or GoogleID) to a team
func (teamID TeamID) AddAgent(in interface{}) error {
	gid, err := toGid(in)
	if err != nil {
		Log.Error(err)
		return err
	}
	if gid == "" {
		err = fmt.Errorf("unable to identify agent to add")
		Log.Error(err)
		return err
	}

	_, err = db.Exec("INSERT IGNORE INTO userteams (teamID, gid, state, color) VALUES (?, ?, 'Off', '00FF00')", teamID, gid)
	if err != nil {
		Log.Notice(err)
		return err
	}
	// XXX send message? Should take place in the caller since we can't know the langauge here
	return nil
}

// RemoveAgent removes a agent (identified by location share key, GoogleID, agent name, or EnlID) from a team.
func (teamID TeamID) RemoveAgent(in interface{}) error {
	gid, err := toGid(in)
	if err != nil {
		Log.Error(err)
		return err
	}

	_, err = db.Exec("DELETE FROM userteams WHERE teamID = ? AND gid = ?", teamID, gid)
	if err != nil {
		Log.Notice(err)
		return (err)
	}
	// XXX push to rocks community
	return nil
}

// ClearPrimaryTeam sets any team marked as primary to "On" for a agent
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
	var tmpU Agent
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
		teamList.Agent = append(teamList.Agent, tmpU)
	}
	err = rows.Err()
	if err != nil {
		Log.Error(err)
		return err
	}
	return nil
}

// WaypointsNear returns any Waypoints near the specified gid, up to distance maxdistance, with a maximum of maxresults returned
// the Agents portion of the TeamData is uninitialized
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

	/* This would use the ST_ Index... instead of offloading it until the HAVING -- saving a lot of db calculations if we get a lot of Waypoints
	   AND MBRContains( LineString(
	Point( 42.353443 + 1 / ( 111.1 / COS(RADIANS(-71.076584))), -71.076584 + 1 / 111.1),
	Point( 42.353443 - 1 / ( 111.1 / COS(RADIANS(-71.076584))), -71.076584 - 1 / 111.1)
	   ), loc)
	*/

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
		tmpW.Type = "waypoint"
		tmpW.Share = true
		td.Waypoints = append(td.Waypoints, tmpW)
	}
	err = rows.Err()
	if err != nil {
		Log.Error(err)
		return err
	}

	err = gid.pdMarkersNear(maxdistance, maxresults, td)
	if err != nil {
		Log.Error(err)
		return err
	}
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

// SetTeamState updates the agent's state on the team (Off|On|Primary)
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
	row := db.QueryRow("SELECT teamID FROM teams WHERE LOWER(name) LIKE LOWER(?) ORDER BY teamID LIMIT 0,1", teamname)
	err := row.Scan(&id)
	if err != nil {
		Log.Notice(err)
	}

	return gid.SetTeamState(id, state)
}

// PrimaryTeam is called to determine an agent's primary team -- which is where Waypoint data is saved
func (gid GoogleID) PrimaryTeam() (string, error) {
	var primary string
	err := db.QueryRow("SELECT teamID FROM userteams WHERE gid = ? AND state = 'Primary'", gid).Scan(&primary)
	if err != nil && err.Error() == "sql: no rows in result set" {
		Log.Debug("Primary Team Not Set")
		return "", nil
	}
	if err != nil {
		Log.Error(err)
		return "", err
	}
	return primary, nil
}

// FetchAgent populates the minimal Agent struct with data anyone can see
func FetchAgent(id string, agent *Agent) error {
	gid, err := toGid(id)
	if err != nil {
		Log.Error(err)
		return err
	}

	// XXX redundant with previous check?
	if gid == "" {
		return fmt.Errorf("Unknown agent (redundant check?): %s", id)
	}

	err = db.QueryRow("SELECT u.gid, u.iname, u.level, u.VVerified, u.VBlacklisted, u.Vid, u.RocksVerified FROM user=u WHERE u.gid = ?", gid).Scan(
		&agent.Gid, &agent.Name, &agent.Level, &agent.Verified, &agent.Blacklisted, &agent.EnlID, &agent.RocksVerified)
	if err != nil {
		Log.Error(err)
		return err
	}
	return nil
}
