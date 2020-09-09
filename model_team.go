package wasabee

import (
	"database/sql"
	"fmt"
	"strconv"
)

// TeamData is the wrapper type containing all the team info
type TeamData struct {
	Name          string  `json:"name"`
	ID            TeamID  `json:"id"`
	Agent         []Agent `json:"agents"`
	RocksComm     string  `json:"rc,omitempty"`
	RocksKey      string  `json:"rk,omitempty"`
	JoinLinkToken string  `json:"jlt,omitempty"`
	// telegramChannel int64
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
	Squad         string   `json:"squad,omitempty"`
	State         bool     `json:"state,omitempty"`
	Lat           float64  `json:"lat,omitempty"`
	Lon           float64  `json:"lng,omitempty"`
	Date          string   `json:"date,omitempty"`
	Distance      float64  `json:"distance,omitempty"`
	CanSendTo     bool     `json:"cansendto,omitempty"`
	DisplayName   string   `json:"displayname,omitempty"`
}

// AgentInTeam checks to see if a agent is in a team and enabled.
func (gid GoogleID) AgentInTeam(team TeamID) (bool, error) {
	var count string

	var err error
	err = db.QueryRow("SELECT COUNT(*) FROM agentteams WHERE teamID = ? AND gid = ?", team, gid).Scan(&count)
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
func (teamID TeamID) FetchTeam(teamList *TeamData) error {
	var state, lat, lon string
	var tmpU Agent

	var err error
	var rows *sql.Rows
	rows, err = db.Query("SELECT u.gid, u.iname, x.color, x.state, Y(l.loc), X(l.loc), l.upTime, u.VVerified, u.VBlacklisted, u.Vid, x.displayname "+
		"FROM team=t, agentteams=x, agent=u, locations=l "+
		"WHERE t.teamID = ? AND t.teamID = x.teamID AND x.gid = u.gid AND x.gid = l.gid ORDER BY u.iname", teamID)
	if err != nil {
		Log.Error(err)
		return err
	}

	defer rows.Close()
	for rows.Next() {
		var enlID, dn sql.NullString
		err := rows.Scan(&tmpU.Gid, &tmpU.Name, &tmpU.Squad, &state, &lat, &lon, &tmpU.Date, &tmpU.Verified, &tmpU.Blacklisted, &enlID, &dn)
		if err != nil {
			Log.Error(err)
			return err
		}
		if state == "On" {
			tmpU.State = true
			tmpU.Lat, _ = strconv.ParseFloat(lat, 64)
			tmpU.Lon, _ = strconv.ParseFloat(lon, 64)
		} else {
			tmpU.State = false
			tmpU.Lat = 0
			tmpU.Lon = 0
		}
		if enlID.Valid {
			tmpU.EnlID = EnlID(enlID.String)
		} else {
			tmpU.EnlID = ""
		}
		tmpU.PictureURL = tmpU.Gid.GetPicture()
		if dn.Valid {
			tmpU.Name = dn.String
			tmpU.DisplayName = dn.String
		} else {
			tmpU.DisplayName = ""
		}
		teamList.Agent = append(teamList.Agent, tmpU)
	}

	var rockscomm, rockskey, joinlinktoken sql.NullString
	if err := db.QueryRow("SELECT name, rockscomm, rockskey, joinLinkToken FROM team WHERE teamID = ?", teamID).Scan(&teamList.Name, &rockscomm, &rockskey, &joinlinktoken); err != nil {
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
	if joinlinktoken.Valid {
		teamList.JoinLinkToken = joinlinktoken.String
	}

	return nil
}

// OwnsTeam returns true if the GoogleID owns the team identified by teamID
func (gid GoogleID) OwnsTeam(teamID TeamID) (bool, error) {
	var owner GoogleID

	err := db.QueryRow("SELECT owner FROM team WHERE teamID = ?", teamID).Scan(&owner)
	if err != nil && err == sql.ErrNoRows {
		Log.Warnw("non-existent team ownership queried", "resource", teamID, "GID", gid)
		return false, nil
	} else if err != nil {
		Log.Error(err)
		return false, err
	}
	if gid != owner {
		return false, nil
	}
	return true, nil
}

// NewTeam initializes a new team and returns a teamID
// the creating gid is added and enabled on that team by default
func (gid GoogleID) NewTeam(name string) (TeamID, error) {
	var err error
	team, err := GenerateSafeName()
	if err != nil {
		Log.Error(err)
		return "", err
	}
	if name == "" {
		err = fmt.Errorf("attempting to create unnamed team: using team ID")
		Log.Errorw(err.Error(), "GID", gid, "resource", team, "message", err.Error())
		name = team
	}

	_, err = db.Exec("INSERT INTO team (teamID, owner, name, rockskey, rockscomm, telegram) VALUES (?,?,?,NULL,NULL,NULL)", team, gid, name)
	if err != nil {
		Log.Error(err)
		return "", err
	}
	_, err = db.Exec("INSERT INTO agentteams (teamID, gid, state, color, displayname) VALUES (?,?,'On','operator',NULL)", team, gid)
	if err != nil {
		Log.Error(err)
		return TeamID(team), err
	}
	return TeamID(team), nil
}

// Rename sets a new name for a teamID
// does not check team ownership -- caller should take care of authorization
func (teamID TeamID) Rename(name string) error {
	_, err := db.Exec("UPDATE team SET name = ? WHERE teamID = ?", name, teamID)
	if err != nil {
		Log.Error(err)
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
			Log.Warn(err)
			continue
		}
		err = teamID.RemoveAgent(gid)
		if err != nil {
			Log.Warn(err)
			continue
		}
	}

	_, err = db.Exec("DELETE FROM opteams WHERE teamID = ?", teamID)
	if err != nil {
		Log.Error(err)
		return err
	}
	_, err = db.Exec("DELETE FROM team WHERE teamID = ?", teamID)
	if err != nil {
		Log.Warn(err)
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

	_, err = db.Exec("INSERT IGNORE INTO agentteams (teamID, gid, state, color, displayname) VALUES (?, ?, 'Off', 'boots', NULL)", teamID, gid)
	if err != nil {
		Log.Error(err)
		return err
	}

	if err = gid.AddToRemoteRocksCommunity(teamID); err != nil {
		Log.Error(err)
		// return err
	}

	Log.Infow("adding agent to team", "GID", gid, "resource", teamID, "message", "adding agent to team")
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
		Log.Error(err)
		return err
	}

	if err = gid.RemoveFromRemoteRocksCommunity(teamID); err != nil {
		Log.Error(err)
		// return err
	}

	// instruct the agent to delete all associated ops
	// this may get ops for which the agent has double-access, but they can just re-fetch them
	rows, err := db.Query("SELECT opID FROM opteams WHERE teamID = ?", teamID)
	if err != nil && err != sql.ErrNoRows {
		Log.Error(err)
		return err
	}
	defer rows.Close()
	var opID OperationID
	for rows.Next() {
		err = rows.Scan(&opID)
		if err != nil {
			Log.Error(err)
			// continue
		}
		gid.FirebaseDeleteOp(opID)
	}

	Log.Infow("removing agent from team", "GID", gid, "resource", teamID, "message", "removing agent from team")
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
		Log.Error(err)
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
		"AND x.state = 'On' AND x.gid = u.gid AND x.gid = l.gid AND l.upTime > SUBTIME(UTC_TIMESTAMP(), '12:00:00') "+
		"HAVING distance < ? AND distance > 0 ORDER BY distance LIMIT 0,?", lat, lon, lat, gid, maxdistance, maxresults)
	if err != nil {
		Log.Error(err)
		return err
	}

	defer rows.Close()
	for rows.Next() {
		err := rows.Scan(&tmpU.Name, &tmpU.Squad, &state, &lat, &lon, &tmpU.Date, &tmpU.Verified, &tmpU.Blacklisted, &tmpU.Distance)
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
		Log.Error(err)
	}
	return err
}

func (teamID TeamID) String() string {
	return string(teamID)
}

// SetTeamState updates the agent's state on the team (Off|On)
func (gid GoogleID) SetTeamState(teamID TeamID, state string) error {
	if state != "On" {
		state = "Off"
	}

	if _, err := db.Exec("UPDATE agentteams SET state = ? WHERE gid = ? AND teamID = ?", state, gid, teamID); err != nil {
		Log.Error(err)
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

// teamList is used for getting a list of all an agent's active teams
func (gid GoogleID) teamList() []TeamID {
	var tid TeamID
	var x []TeamID

	rows, err := db.Query("SELECT teamID FROM agentteams WHERE gid = ? AND state = 'On'", gid)
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

// SetSquad sets an agent's squad on a given team
func (teamID TeamID) SetSquad(gid GoogleID, squad string) error {
	_, err := db.Exec("UPDATE agentteams SET color = ? WHERE teamID = ? and gid = ?", MakeNullString(squad), teamID, gid)
	if err != nil {
		Log.Error(err)
		return err
	}
	return nil
}

// SetDisplayname sets an agent's display name on a given team
func (teamID TeamID) SetDisplayname(gid GoogleID, displayname string) error {
	_, err := db.Exec("UPDATE agentteams SET displayname = ? WHERE teamID = ? and gid = ?", MakeNullString(displayname), teamID, gid)
	if err != nil {
		Log.Error(err)
		return err
	}
	return nil
}

// GenerateJoinToken sets a team's join link token
func (teamID TeamID) GenerateJoinToken() (string, error) {
	key, err := GenerateSafeName()
	if err != nil {
		Log.Error(err)
		return key, err
	}

	_, err = db.Exec("UPDATE team SET joinLinkToken = ? WHERE teamID = ?", key, teamID)
	if err != nil {
		Log.Error(err)
		return key, err
	}
	return key, nil
}

// DeleteJoinToken removes a team's join link token
func (teamID TeamID) DeleteJoinToken() error {
	_, err := db.Exec("UPDATE team SET joinLinkToken = NULL WHERE teamID = ?", teamID)
	if err != nil {
		Log.Error(err)
		return err
	}
	return nil
}

// JoinToken verifies a join link
func (teamID TeamID) JoinToken(gid GoogleID, key string) error {
	var count string

	err := db.QueryRow("SELECT COUNT(*) FROM team WHERE teamID = ? AND joinLinkToken= ?", teamID, key).Scan(&count)
	if err != nil {
		return err
	}

	i, err := strconv.Atoi(count)
	if err != nil {
		return err
	}
	if i != 1 {
		err = fmt.Errorf("invalid team join token")
		Log.Errorw(err.Error(), "resource", teamID, "GID", gid)
		return err
	}

	err = teamID.AddAgent(gid)
	if err != nil {
		return err
	}
	err = teamID.SetSquad(gid, "joined via link")
	if err != nil {
		return err
	}

	return nil
}

// LinkToTelegramChat associates a telegram chat ID with the team, performs authorization
func (teamID TeamID) LinkToTelegramChat(chat int64, gid GoogleID) error {
	owns, err := gid.OwnsTeam(teamID)
	if err != nil {
		Log.Error(err)
		return err
	}
	if !owns {
		err = fmt.Errorf("only team owner can set telegram link")
		Log.Error(err)
		return err
	}

	_, err = db.Exec("UPDATE team SET telegram = ? WHERE teamID = ?", chat, teamID)
	if err != nil {
		Log.Error(err)
		return err
	}

	Log.Infow("linked team to telegram", "GID", gid, "resource", teamID, "chatID", chat)
	return nil
}

// TelegramChat returns the associated telegram chat ID for this team, if any
func (teamID TeamID) TelegramChat() (int64, error) {
	var chatID int64

	err := db.QueryRow("SELECT telegram FROM team WHERE teamID = ?", teamID).Scan(&chatID)
	if err != nil && err != sql.ErrNoRows {
		Log.Error(err)
		return int64(0), err
	}
	if err == sql.ErrNoRows {
		Log.Debug("attempt to get telegram chatID for non-existant team")
		return int64(0), nil
	}
	return chatID, nil
}

// ChatToTeam takes a chatID and returns a linked teamID
func ChatToTeam(chat int64) (TeamID, error) {
	var t TeamID

	err := db.QueryRow("SELECT teamID FROM team WHERE telegram = ?", chat).Scan(&t)
	if err != nil && err != sql.ErrNoRows {
		Log.Error(err)
		return t, err
	}
	if err == sql.ErrNoRows {
		err := fmt.Errorf("attempt to get teamID for non-linked chat")
		Log.Debug(err)
		return t, err
	}
	return t, nil
}
