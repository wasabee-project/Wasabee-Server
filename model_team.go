package wasabi

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
	Waypoints []waypoint
	RocksComm string
	RocksKey  string
}

// Agent is the light version of AgentData, containing visible information exported to teams
type Agent struct {
	Gid           GoogleID
	Name          string
	Level         int64
	EnlID         EnlID
	Verified      bool            `json:"Vverified,omitempty"`
	Blacklisted   bool            `json:"blacklisted"`
	RocksVerified bool            `json:"rocks,omitempty"`
	Color         string          `json:"color,omitempty"`
	State         bool            `json:"state,omitempty"`
	LocKey        string          `json:"lockey,omitempty"`
	Lat           float64         `json:"lat,omitempty"`
	Lon           float64         `json:"lng,omitempty"`
	Date          string          `json:"date,omitempty"`
	OwnTracks     json.RawMessage `json:"OwnTracks,omitempty"`
	Distance      float64         `json:"distance,omitempty"`
}

// AgentInTeam checks to see if a agent is in a team and (On|Primary).
// allowOff == true will report if a agent is in a team even if they are Off. That should ONLY be used to display lists of teams to the calling agent.
func (gid GoogleID) AgentInTeam(team TeamID, allowOff bool) (bool, error) {
	var count string

	var err error
	if allowOff {
		err = db.QueryRow("SELECT COUNT(*) FROM agentteams WHERE teamID = ? AND gid = ?", team, gid).Scan(&count)
	} else {
		err = db.QueryRow("SELECT COUNT(*) FROM agentteams WHERE teamID = ? AND gid = ? AND state IN ('On', 'Primary')", team, gid).Scan(&count)
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
	var state, lat, lon, otdata string
	var tmpU Agent

	var err error
	var rows *sql.Rows
	if fetchAll {
		rows, err = db.Query("SELECT u.gid, u.iname, u.lockey, x.color, x.state, Y(l.loc), X(l.loc), l.upTime, o.otdata, u.VVerified, u.VBlacklisted, u.Vid "+
			"FROM team=t, agentteams=x, agent=u, locations=l, otdata=o "+
			"WHERE t.teamID = ? AND t.teamID = x.teamID AND x.gid = u.gid AND x.gid = l.gid AND u.gid = o.gid ", teamID)
	} else {
		rows, err = db.Query("SELECT u.gid, u.iname, u.lockey, x.color, x.state, Y(l.loc), X(l.loc), l.upTime, o.otdata, u.VVerified, u.VBlacklisted, u.Vid "+
			"FROM team=t, agentteams=x, agent=u, locations=l, otdata=o "+
			"WHERE t.teamID = ? AND t.teamID = x.teamID AND x.gid = u.gid AND x.gid = l.gid AND u.gid = o.gid "+
			"AND x.state IN ('On', 'Primary')", teamID)
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
		tmpU.State = isActive(state)
		tmpU.Lat, _ = strconv.ParseFloat(lat, 64)
		tmpU.Lon, _ = strconv.ParseFloat(lon, 64)
		tmpU.OwnTracks = json.RawMessage(otdata)
		teamList.Agent = append(teamList.Agent, tmpU)
	}

	var rockscomm, rockskey sql.NullString
	if err := db.QueryRow("SELECT name, rockscomm, rockskey FROM team WHERE teamID = ?", teamID).Scan(&teamList.Name, &rockscomm, &rockskey); err != nil {
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

	err := db.QueryRow("SELECT owner FROM team WHERE teamID = ?", teamID).Scan(&owner)
	if err != nil {
		return false, err
	}
	if gid == owner {
		return true, nil
	}
	return false, nil
}

// NewTeam initializes a new team and returns a teamID
// the creating gid is added and enabled on that team by default
func (gid GoogleID) NewTeam(name string) (TeamID, error) {
	var err error
	if name == "" {
		err = fmt.Errorf("attempting to create unnamed team")
		Log.Debug(err)
		return "", err
	}
	team, err := GenerateSafeName()
	if err != nil {
		Log.Notice(err)
		return "", err
	}
	_, err = db.Exec("INSERT INTO team (teamID, owner, name, rockskey, rockscomm) VALUES (?,?,?,NULL,NULL)", team, gid, name)
	if err != nil {
		Log.Notice(err)
		return "", err
	}
	_, err = db.Exec("INSERT INTO agentteams (teamID, gid, state, color) VALUES (?,?,'On','00FF00')", team, gid)
	if err != nil {
		Log.Notice(err)
		return TeamID(team), err
	}
	return TeamID(team), nil
}

// Rename sets a new name for a teamID
// does not check team ownership -- caller should take care of authorization
func (teamID TeamID) Rename(name string) error {
	_, err := db.Exec("UPDATE team SET name = ? WHERE teamID = ?", name, teamID)
	if err != nil {
		Log.Notice(err)
	}
	return err
}

// Delete removes the team identified by teamID
// does not check team ownership -- caller should take care of authorization
func (teamID TeamID) Delete() error {
	// do them one-at-a-time to take care of .rocks sync
	rows, err := db.Query("SELECT gid FROM agentteams WHERE teamID = ?", teamID)
	if err != nil {
		Log.Error(err)
		return err
	}

	var gid GoogleID
	defer rows.Close()
	for rows.Next() {
		err = rows.Scan(&gid)
		if err != nil {
			Log.Notice(err)
			continue
		}
		err = teamID.RemoveAgent(gid)
		if err != nil {
			Log.Notice(err)
			continue
		}
	}

	_, err = db.Exec("DELETE FROM team WHERE teamID = ?", teamID)
	if err != nil {
		Log.Notice(err)
		return err
	}
	return nil
}

// toGid takes anything (for small values of anything) and returns a Gid for it.
// depends on EnlID.Gid(), LocKey.Gid(), TelegramID.Gid(), and SearchAgentName.
// If you pass in a string, it will see if it looks like a gid, eid, lockey, or agent name and try that.
// XXX Move to model_agent.go
func toGid(in interface{}) (GoogleID, error) {
	var gid GoogleID
	var err error
	switch v := in.(type) {
	case LocKey:
		lockey := v
		gid, err = lockey.Gid()
		if err != nil && err == sql.ErrNoRows {
			err = fmt.Errorf("unknown lockey: %s", lockey)
			Log.Info(err)
			return "", err
		}
	case GoogleID:
		gid = v
	case EnlID:
		eid := v
		gid, err = eid.Gid()
		if err != nil && err == sql.ErrNoRows {
			err = fmt.Errorf("unknown EnlID: %s", eid)
			Log.Info(err)
			return "", err
		}
	case TelegramID:
		tid := v
		gid, _, err = tid.GidV()
		if err != nil && err == sql.ErrNoRows {
			err = fmt.Errorf("unknown TelegramID: %d", tid)
			Log.Info(err)
			return "", err
		}
	default:
		tmp := v.(string)
		if tmp == "" { // no need to look if it is empty
			return "", nil
		}
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
				gid, err = SearchAgentName(tmp)
				if err != nil {
					Log.Notice(err)
					return "", err
				}
				if gid == "" {
					err = fmt.Errorf("unknown agent: %s", tmp)
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

	_, err = db.Exec("INSERT IGNORE INTO agentteams (teamID, gid, state, color) VALUES (?, ?, 'Off', '00FF00')", teamID, gid)
	if err != nil {
		Log.Notice(err)
		return err
	}

	err = gid.AddToRemoteRocksCommunity(teamID)
	if err != nil {
		Log.Notice(err)
		// return (err)
	}
	return nil
}

// RemoveAgent removes a agent (identified by location share key, GoogleID, agent name, or EnlID) from a team.
func (teamID TeamID) RemoveAgent(in interface{}) error {
	gid, err := toGid(in)
	if err != nil {
		Log.Error(err)
		return err
	}

	_, err = db.Exec("DELETE FROM agentteams WHERE teamID = ? AND gid = ?", teamID, gid)
	if err != nil {
		Log.Notice(err)
		return (err)
	}

	err = gid.RemoveFromRemoteRocksCommunity(teamID)
	if err != nil {
		Log.Notice(err)
		// return (err)
	}
	return nil
}

// ClearPrimaryTeam sets any team marked as primary to "On" for a agent
func (gid GoogleID) ClearPrimaryTeam() error {
	_, err := db.Exec("UPDATE agentteams SET state = 'On' WHERE state = 'Primary' AND gid = ?", gid)
	if err != nil {
		Log.Notice(err)
		return (err)
	}
	return nil
}

// TeammatesNear identifies other agents who are on ANY mutual team within maxdistance km, returning at most maxresults
func (gid GoogleID) TeammatesNear(maxdistance, maxresults int, teamList *TeamData) error {
	var state, lat, lon, otdata string
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
		"FROM agentteams=x, agent=u, locations=l, otdata=o "+
		"WHERE x.teamID IN (SELECT teamID FROM agentteams WHERE gid = ? AND state IN ('On', 'Primary')) "+
		"AND x.state IN ('On', 'Primary') AND x.gid = u.gid AND x.gid = l.gid AND x.gid = o.gid AND l.upTime > SUBTIME(NOW(), '12:00:00') "+
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
		tmpU.State = isActive(state)
		tmpU.Lat, _ = strconv.ParseFloat(lat, 64)
		tmpU.Lon, _ = strconv.ParseFloat(lon, 64)
		tmpU.OwnTracks = json.RawMessage(otdata)
		teamList.Agent = append(teamList.Agent, tmpU)
	}
	return nil
}

// WaypointsNear returns any Waypoints and Markers near the specified gid, up to distance maxdistance, with a maximum of maxresults returned
// the Agents portion of the TeamData is uninitialized
func (gid GoogleID) WaypointsNear(maxdistance, maxresults int, td *TeamData) error {
	var lat, lon string
	var rows *sql.Rows
	var tmpW waypoint

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
		"WHERE teamID IN (SELECT teamID FROM agentteams WHERE gid = ? AND state IN ('On', 'Primary')) "+
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
		tmpW.Lat, _ = strconv.ParseFloat(lat, 64)
		tmpW.Lon, _ = strconv.ParseFloat(lon, 64)
		tmpW.Type = wpc
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

// SetRocks links a team to a community at enl.rocks.
// Does not check team ownership -- caller should take care of authorization.
// Local adds/deletes will be pushed to the community (API management must be enabled on the community at enl.rocks).
// adds/deletes at enl.rocks will be pushed here (onJoin/onLeave web hooks must be configured in the community at enl.rocks)
func (teamID TeamID) SetRocks(key, community string) error {
	_, err := db.Exec("UPDATE team SET rockskey = ?, rockscomm = ? WHERE teamID = ?", key, community, teamID)
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

	if _, err := db.Exec("UPDATE agentteams SET state = ? WHERE gid = ? AND teamID = ?", state, gid, teamID); err != nil {
		Log.Notice(err)
		return err
	}
	return nil
}

// PrimaryTeam is called to determine an agent's primary team -- which is where Waypoint data is saved
func (gid GoogleID) PrimaryTeam() (string, error) {
	var primary string
	err := db.QueryRow("SELECT teamID FROM agentteams WHERE gid = ? AND state = 'Primary'", gid).Scan(&primary)
	if err != nil && err == sql.ErrNoRows {
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

	err = db.QueryRow("SELECT u.gid, u.iname, u.level, u.VVerified, u.VBlacklisted, u.Vid, u.RocksVerified FROM agent=u WHERE u.gid = ?", gid).Scan(
		&agent.Gid, &agent.Name, &agent.Level, &agent.Verified, &agent.Blacklisted, &agent.EnlID, &agent.RocksVerified)
	if err != nil {
		Log.Error(err)
		return err
	}
	return nil
}

func isActive(state string) bool {
	switch state {
	case "On", "Primary":
		return true
	}
	return false
}

// Name returns a team's friendly name for a TeamID
func (teamID TeamID) Name() (string, error) {
	var name string
	err := db.QueryRow("SELECT name FROM team WHERE teamID = ?", teamID).Scan(&name)
	if err != nil {
		Log.Error(err)
		return "", err
	}
	return name, nil
}
