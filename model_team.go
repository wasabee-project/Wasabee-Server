package wasabee

import (
	"bytes"
	"database/sql"
	"fmt"
	"html/template"
	"strconv"
)

// TeamData is the wrapper type containing all the team info
type TeamData struct {
	Name      string  `json:"name"`
	ID        TeamID  `json:"id"`
	Agent     []Agent `json:"agents"`
	RocksComm string  `json:"rc,omitempty"`
	RocksKey  string  `json:"rk,omitempty"`
}

// Agent is the light version of AgentData, containing visible information exported to teams
type Agent struct {
	Gid           GoogleID `json:"id"`
	Name          string   `json:"name"`
	Level         int64    `json:"level"`
	EnlID         EnlID    `json:"enlid"`
	PictureURL    string   `json:"pic"`
	Verified      bool     `json:"Vverified,omitempty"`
	Blacklisted   bool     `json:"blacklisted"`
	RocksVerified bool     `json:"rocks,omitempty"`
	Color         string   `json:"color,omitempty"`
	State         bool     `json:"state,omitempty"`
	Lat           float64  `json:"lat,omitempty"`
	Lon           float64  `json:"lng,omitempty"`
	Date          string   `json:"date,omitempty"`
	Distance      float64  `json:"distance,omitempty"`
	CanSendTo     bool     `json:"cansendto,omitempty"`
}

// AgentInTeam checks to see if a agent is in a team and enabled.
// allowOff == true will report if a agent is in a team even if they are Off. That should ONLY be used to display lists of teams to the calling agent.
func (gid GoogleID) AgentInTeam(team TeamID, allowOff bool) (bool, error) {
	var count string

	var err error
	if allowOff {
		err = db.QueryRow("SELECT COUNT(*) FROM agentteams WHERE teamID = ? AND gid = ?", team, gid).Scan(&count)
	} else {
		err = db.QueryRow("SELECT COUNT(*) FROM agentteams WHERE teamID = ? AND gid = ? AND state = 'On'", team, gid).Scan(&count)
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
	var state, lat, lon string
	var tmpU Agent

	var err error
	var rows *sql.Rows
	if fetchAll {
		rows, err = db.Query("SELECT u.gid, u.iname, x.color, x.state, Y(l.loc), X(l.loc), l.upTime, u.VVerified, u.VBlacklisted, u.Vid "+
			"FROM team=t, agentteams=x, agent=u, locations=l "+
			"WHERE t.teamID = ? AND t.teamID = x.teamID AND x.gid = u.gid AND x.gid = l.gid ORDER BY x.state DESC, u.iname", teamID)
	} else {
		rows, err = db.Query("SELECT u.gid, u.iname, x.color, x.state, Y(l.loc), X(l.loc), l.upTime, u.VVerified, u.VBlacklisted, u.Vid "+
			"FROM team=t, agentteams=x, agent=u, locations=l "+
			"WHERE t.teamID = ? AND t.teamID = x.teamID AND x.gid = u.gid AND x.gid = l.gid "+
			"AND x.state = 'On' ORDER BY x.state DESC, u.iname", teamID)
	}
	if err != nil {
		Log.Error(err)
		return err
	}

	defer rows.Close()
	for rows.Next() {
		var enlID sql.NullString
		err := rows.Scan(&tmpU.Gid, &tmpU.Name, &tmpU.Color, &state, &lat, &lon, &tmpU.Date, &tmpU.Verified, &tmpU.Blacklisted, &enlID)
		if err != nil {
			Log.Error(err)
			return err
		}
		if state == "On" {
			tmpU.State = true
		} else {
			tmpU.State = false
		}
		if enlID.Valid {
			tmpU.EnlID = EnlID(enlID.String)
		} else {
			tmpU.EnlID = ""
		}
		tmpU.Lat, _ = strconv.ParseFloat(lat, 64)
		tmpU.Lon, _ = strconv.ParseFloat(lon, 64)
		tmpU.PictureURL = tmpU.Gid.GetPicture()
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
	defer rows.Close()

	var gid GoogleID
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

// AddAgent adds a agent to a team
func (teamID TeamID) AddAgent(in AgentID) error {
	gid, err := in.Gid()
	if err != nil {
		Log.Error(err)
		return err
	}

	_, err = db.Exec("INSERT IGNORE INTO agentteams (teamID, gid, state, color) VALUES (?, ?, 'Off', '00FF00')", teamID, gid)
	if err != nil {
		Log.Notice(err)
		return err
	}

	if err = gid.AddToRemoteRocksCommunity(teamID); err != nil {
		Log.Notice(err)
		// return err
	}
	return nil
}

// RemoveAgent removes a agent (identified by location share key, GoogleID, agent name, or EnlID) from a team.
func (teamID TeamID) RemoveAgent(in AgentID) error {
	gid, err := in.Gid()
	if err != nil {
		Log.Error(err)
		return err
	}

	_, err = db.Exec("DELETE FROM agentteams WHERE teamID = ? AND gid = ?", teamID, gid)
	if err != nil {
		Log.Notice(err)
		return err
	}

	if err = gid.RemoveFromRemoteRocksCommunity(teamID); err != nil {
		Log.Notice(err)
		// return err
	}
	return nil
}

// Chown changes a team's ownership
// caller must verify permissions
func (teamID TeamID) Chown(to AgentID) error {
	gid, err := to.Gid()
	if err != nil {
		Log.Error(err)
		return err
	}

	_, err = db.Exec("UPDATE team SET owner = ? WHERE teamID = ?", gid, teamID)
	if err != nil {
		Log.Notice(err)
		return (err)
	}
	return nil
}

// TeammatesNear identifies other agents who are on ANY mutual team within maxdistance km, returning at most maxresults
func (gid GoogleID) TeammatesNear(maxdistance, maxresults int, teamList *TeamData) error {
	var state, lat, lon string
	var tmpU Agent
	var rows *sql.Rows

	err := db.QueryRow("SELECT Y(loc), X(loc) FROM locations WHERE gid = ?", gid).Scan(&lat, &lon)
	if err != nil {
		Log.Error(err)
		return err
	}
	// Log.Debug("Teammates Near: " + gid.String() + " @ " + lat.String + "," + lon.String + " " + strconv.Itoa(maxdistance) + " " + strconv.Itoa(maxresults))

	// no ST_Distance_Sphere in MariaDB yet...
	rows, err = db.Query("SELECT DISTINCT u.iname, x.color, x.state, Y(l.loc), X(l.loc), l.upTime, u.VVerified, u.VBlacklisted, "+
		"ROUND(6371 * acos (cos(radians(?)) * cos(radians(Y(l.loc))) * cos(radians(X(l.loc)) - radians(?)) + sin(radians(?)) * sin(radians(Y(l.loc))))) AS distance "+
		"FROM agentteams=x, agent=u, locations=l "+
		"WHERE x.teamID IN (SELECT teamID FROM agentteams WHERE gid = ? AND state = 'On') "+
		"AND x.state = 'On' AND x.gid = u.gid AND x.gid = l.gid AND l.upTime > SUBTIME(NOW(), '12:00:00') "+
		"HAVING distance < ? AND distance > 0 ORDER BY distance LIMIT 0,?", lat, lon, lat, gid, maxdistance, maxresults)
	if err != nil {
		Log.Error(err)
		return err
	}

	defer rows.Close()
	for rows.Next() {
		err := rows.Scan(&tmpU.Name, &tmpU.Color, &state, &lat, &lon, &tmpU.Date, &tmpU.Verified, &tmpU.Blacklisted, &tmpU.Distance)
		if err != nil {
			Log.Error(err)
			return err
		}
		if state == "On" {
			tmpU.State = true
		} else {
			tmpU.State = false
		}
		tmpU.Lat, _ = strconv.ParseFloat(lat, 64)
		tmpU.Lon, _ = strconv.ParseFloat(lon, 64)
		teamList.Agent = append(teamList.Agent, tmpU)
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

// SetTeamState updates the agent's state on the team (Off|On)
func (gid GoogleID) SetTeamState(teamID TeamID, state string) error {
	if _, err := db.Exec("UPDATE agentteams SET state = ? WHERE gid = ? AND teamID = ?", state, gid, teamID); err != nil {
		Log.Notice(err)
		return err
	}
	if state == "On" {
		gid.firebaseSubscribeTeam(teamID)
	} else {
		gid.firebaseUnsubscribeTeam(teamID)
	}
	return nil
}

// FetchAgent populates the minimal Agent struct with data anyone can see
func FetchAgent(id AgentID, agent *Agent) error {
	var vid sql.NullString
	gid, err := id.Gid()
	if err != nil {
		Log.Error(err)
		return err
	}

	err = db.QueryRow("SELECT u.gid, u.iname, u.level, u.VVerified, u.VBlacklisted, u.Vid, u.RocksVerified FROM agent=u WHERE u.gid = ?", gid).Scan(
		&agent.Gid, &agent.Name, &agent.Level, &agent.Verified, &agent.Blacklisted, &vid, &agent.RocksVerified)
	if err != nil {
		Log.Error(err)
		return err
	}
	if vid.Valid {
		agent.EnlID = EnlID(vid.String)
	}
	return nil
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

// TeamMenu is used for html templates {{TeamMenu .Gid .TeamID}} .Gid is the user's GoogleID, TeamID is the current Op's teamID (for setting selected)
func TeamMenu(gid GoogleID, teamID TeamID) (template.HTML, error) {
	rows, err := db.Query("SELECT t.name, t.teamID FROM agentteams=x, team=t WHERE x.gid = ? AND x.teamID = t.teamID", gid)
	if err != nil {
		Log.Error(err)
		return "", err
	}

	defer rows.Close()

	var b bytes.Buffer
	var name string
	var tid string

	_, _ = b.WriteString(`<select name="team">`)
	for rows.Next() {
		err := rows.Scan(&name, &tid)
		if err != nil {
			Log.Error(err)
			continue
		}
		if tid == string(teamID) {
			_, _ = b.WriteString(fmt.Sprintf("<option value=\"%s\" selected=\"selected\">%s</option>", tid, name))
		} else {
			_, _ = b.WriteString(fmt.Sprintf("<option value=\"%s\">%s</option>", tid, name))
		}
	}
	_, _ = b.WriteString(`</select>`)
	// #nosec
	return template.HTML(b.String()), nil
}

// teamList is used for getting a list of all an agent's (active) teams
func (gid GoogleID) teamList() []TeamID {
	var tid TeamID
	var x []TeamID

	rows, err := db.Query("SELECT teamID FROM agentteams WHERE gid = ?", gid)
	if err != nil {
		Log.Error(err)
		return x
	}
	for rows.Next() {
		if err := rows.Scan(&tid); err != nil {
			Log.Error(err)
			continue
		}
		x = append(x, tid)
	}
	return x
}
