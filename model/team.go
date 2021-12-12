package model

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/wasabee-project/Wasabee-Server/log"
	"github.com/wasabee-project/Wasabee-Server/messaging"
)

// TeamID is the primary means for interfacing with teams
type TeamID string

// TeamData is the wrapper type containing all the team info
type TeamData struct {
	Name          string       `json:"name"`
	ID            TeamID       `json:"id"`
	TeamMembers   []TeamMember `json:"agents"`
	RocksComm     string       `json:"rc,omitempty"`
	RocksKey      string       `json:"rk,omitempty"`
	JoinLinkToken string       `json:"jlt,omitempty"`
	VTeam         int64        `json:"vt,omitempty"`
	VRole         int8         `json:"vr,omitempty"`
}

// TeamMember is the light version of AgentData, containing visible information exported to teams
type TeamMember struct {
	Gid           GoogleID `json:"id"`
	Name          string   `json:"name"`
	VName         string   `json:"vname"`
	RocksName     string   `json:"rocksname"`
	IntelName     string   `json:"intelname"`
	Level         uint8    `json:"level"`
	EnlID         string   `json:"enlid"`
	PictureURL    string   `json:"pic"`
	Verified      bool     `json:"Vverified"`
	Blacklisted   bool     `json:"blacklisted"`
	RocksVerified bool     `json:"rocks"`
	RocksSmurf    bool     `json:"smurf"`
	IntelFaction  string   `json:"intelfaction"`
	Comment       string   `json:"squad"`
	ShareLocation bool     `json:"state"`
	Lat           float64  `json:"lat"`
	Lon           float64  `json:"lng"`
	Date          string   `json:"date"`
	ShareWD       bool     `json:"shareWD"`
	LoadWD        bool     `json:"loadWD"`
	/* StartLat      float64  `json:"startlat,omitempty"` // unused
	StartLon      float64  `json:"startlng,omitempty"` // unused
	StartRadius   uint16   `json:"startradius,omitempty"` // unused
	ShareStart    bool     `json:"sharestart,omitempty"` // unused */
}

// AgentInTeam checks to see if a agent is in a team and enabled.
func (gid GoogleID) AgentInTeam(team TeamID) (bool, error) {
	var count string

	err := db.QueryRow("SELECT COUNT(*) FROM agentteams WHERE teamID = ? AND gid = ?", team, gid).Scan(&count)
	if err != nil {
		return false, err
	}
	i, err := strconv.ParseInt(count, 10, 32)
	if err != nil || i < 1 {
		return false, err
	}
	return true, nil
}

// FetchTeam populates an entire TeamData struct
func (teamID TeamID) FetchTeam() (*TeamData, error) {
	var teamList TeamData
	var rows *sql.Rows

	rows, err := db.Query("SELECT agentteams.gid, v.Agent, agent.IntelName, rocks.Agent, agentteams.squad, agentteams.state, Y(locations.loc), X(locations.loc), locations.upTime, v.Verified, v.Blacklisted, v.EnlID, rocks.verified, rocks.smurf, agentteams.sharewd, agentteams.loadwd, agent.intelfaction "+
		" FROM agentteams JOIN team ON agentteams.teamID = team.teamID JOIN agent ON agentteams.gid = agent.gid JOIN locations ON agentteams.gid = locations.gid LEFT JOIN v ON agentteams.gid = v.gid LEFT JOIN rocks ON agentteams.gid = rocks.gid WHERE agentteams.teamID = ?", teamID)
	if err != nil {
		log.Error(err)
		return &teamList, err
	}

	defer rows.Close()
	for rows.Next() {
		tmpU := TeamMember{}
		var state, lat, lon, sharewd, loadwd string
		var faction IntelFaction
		var vverified, vblacklisted, rocksverified, rockssmurf sql.NullBool
		var enlID, vname, rocksname sql.NullString

		err := rows.Scan(&tmpU.Gid, &vname, &tmpU.IntelName, &rocksname, &tmpU.Comment, &state, &lat, &lon, &tmpU.Date, &vverified, &vblacklisted, &enlID, &rocksverified, &rockssmurf, &sharewd, &loadwd, &faction)
		if err != nil {
			log.Error(err)
			return &teamList, err
		}

		tmpU.Name = fmt.Sprint("UnverifiedAgent_" + tmpU.Gid)

		if tmpU.IntelName != "" {
			tmpU.Name = tmpU.IntelName
		}

		if rocksname.Valid {
			tmpU.RocksName = rocksname.String
			if rocksname.String != "-hidden-" {
				tmpU.Name = rocksname.String
			}
		}

		if vname.Valid {
			tmpU.VName = vname.String
			tmpU.Name = vname.String
		}

		if enlID.Valid {
			tmpU.EnlID = enlID.String
		}

		if vverified.Valid {
			tmpU.Verified = vverified.Bool
		}

		if vblacklisted.Valid {
			tmpU.Verified = vblacklisted.Bool
		}

		if rocksverified.Valid {
			tmpU.Verified = rocksverified.Bool
		}

		if rockssmurf.Valid {
			tmpU.Verified = rockssmurf.Bool
		}

		if state == "On" {
			tmpU.ShareLocation = true
			tmpU.Lat, _ = strconv.ParseFloat(lat, 64)
			tmpU.Lon, _ = strconv.ParseFloat(lon, 64)
		} else {
			tmpU.ShareLocation = false
			tmpU.Lat = 0
			tmpU.Lon = 0
		}

		// XXX do not do a distinct lookup here, just rewrite the query to include a JOIN
		tmpU.PictureURL = tmpU.Gid.GetPicture()
		if sharewd == "On" {
			tmpU.ShareWD = true
		} else {
			tmpU.ShareWD = false
		}

		if loadwd == "On" {
			tmpU.LoadWD = true
		} else {
			tmpU.LoadWD = false
		}

		tmpU.IntelFaction = faction.String()

		teamList.TeamMembers = append(teamList.TeamMembers, tmpU)
	}

	var rockscomm, rockskey, joinlinktoken sql.NullString
	if err := db.QueryRow("SELECT name, rockscomm, rockskey, joinLinkToken, vteam, vrole FROM team WHERE teamID = ?", teamID).Scan(&teamList.Name, &rockscomm, &rockskey, &joinlinktoken, &teamList.VTeam, &teamList.VRole); err != nil {
		log.Error(err)
		return &teamList, err
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

	return &teamList, nil
}

func (teamID TeamID) Owner() (GoogleID, error) {
	var owner GoogleID

	err := db.QueryRow("SELECT owner FROM team WHERE teamID = ?", teamID).Scan(&owner)
	if err != nil && err == sql.ErrNoRows {
		log.Warnw("non-existent team ownership queried", "resource", teamID)
		return "", nil
	} else if err != nil {
		log.Error(err)
		return "", err
	}
	return owner, nil
}

// OwnsTeam returns true if the GoogleID owns the team identified by teamID
func (gid GoogleID) OwnsTeam(teamID TeamID) (bool, error) {
	owner, err := teamID.Owner()
	if err != nil {
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
		log.Error(err)
		return "", err
	}
	if name == "" {
		err = fmt.Errorf("attempting to create unnamed team: using team ID")
		log.Errorw(err.Error(), "GID", gid, "resource", team, "message", err.Error())
		name = team
	}

	_, err = db.Exec("INSERT INTO team (teamID, owner, name, rockskey, rockscomm, vteam, vrole) VALUES (?,?,?,NULL,NULL,0,0)", team, gid, name)
	if err != nil {
		log.Error(err)
		return "", err
	}
	_, err = db.Exec("INSERT INTO agentteams (teamID, gid, state, squad, displayname, shareWD, loadWD) VALUES (?,?,'On','operator',NULL, 'Off', 'Off')", team, gid)
	if err != nil {
		log.Error(err)
		return TeamID(team), err
	}
	return TeamID(team), nil
}

// Rename sets a new name for a teamID
// does not check team ownership -- caller should take care of authorization
func (teamID TeamID) Rename(name string) error {
	_, err := db.Exec("UPDATE team SET name = ? WHERE teamID = ?", name, teamID)
	if err != nil {
		log.Error(err)
	}
	return err
}

// Delete removes the team identified by teamID
// does not check team ownership -- caller should take care of authorization
func (teamID TeamID) Delete() error {
	// do them one-at-a-time to take care of rocks/v/firebase/telegram sync
	rows, err := db.Query("SELECT gid FROM agentteams WHERE teamID = ?", teamID)
	if err != nil {
		log.Error(err)
		return err
	}

	var gid GoogleID
	defer rows.Close()
	for rows.Next() {
		err = rows.Scan(&gid)
		if err != nil {
			log.Warn(err)
			continue
		}
		err = teamID.RemoveAgent(gid)
		if err != nil {
			log.Warn(err)
			continue
		}
	}

	_, err = db.Exec("DELETE FROM permissions WHERE teamID = ?", teamID)
	if err != nil {
		log.Error(err)
		return err
	}
	_, err = db.Exec("DELETE FROM team WHERE teamID = ?", teamID)
	if err != nil {
		log.Warn(err)
		return err
	}
	return nil
}

// AddAgent adds a agent to a team
func (teamID TeamID) AddAgent(in AgentID) error {
	gid, err := in.Gid()
	if err != nil {
		log.Error(err)
		return err
	}

	_, err = db.Exec("INSERT IGNORE INTO agentteams (teamID, gid, state, squad, shareWD, loadWD) VALUES (?, ?, 'Off', 'agents', 'Off', 'Off')", teamID, gid)
	if err != nil {
		log.Error(err)
		return err
	}

	messaging.AddToRemote(messaging.GoogleID(gid), messaging.TeamID(teamID))
	log.Infow("adding agent to team", "GID", gid, "resource", teamID, "message", "adding agent to team")
	return nil
}

// RemoveAgent removes a agent (identified by location share key, GoogleID, agent name, or EnlID) from a team.
func (teamID TeamID) RemoveAgent(in AgentID) error {
	gid, err := in.Gid()
	if err != nil {
		log.Error(err)
		return err
	}

	_, err = db.Exec("DELETE FROM agentteams WHERE teamID = ? AND gid = ?", teamID, gid)
	if err != nil {
		log.Error(err)
		return err
	}

	if err = messaging.RemoveFromRemote(messaging.GoogleID(gid), messaging.TeamID(teamID)); err != nil {
		log.Error(err)
	}

	// instruct the agent to delete all associated ops
	// this may get ops for which the agent has double-access, but they can just re-fetch them
	rows, err := db.Query("SELECT opID FROM permissions WHERE teamID = ?", teamID)
	if err != nil && err != sql.ErrNoRows {
		log.Error(err)
		return err
	}
	var opID OperationID
	defer rows.Close()
	for rows.Next() {
		err = rows.Scan(&opID)
		if err != nil {
			log.Error(err)
			// continue
		}
		messaging.AgentDeleteOperation(messaging.GoogleID(gid), messaging.OperationID(opID))
	}

	log.Debugw("removing agent from team", "GID", gid, "resource", teamID, "message", "removing agent from team")
	return nil
}

// Chown changes a team's ownership
// caller must verify permissions
func (teamID TeamID) Chown(to AgentID) error {
	gid, err := to.Gid()
	if err != nil {
		log.Error(err)
		return err
	}

	_, err = db.Exec("UPDATE team SET owner = ? WHERE teamID = ?", gid, teamID)
	if err != nil {
		log.Error(err)
		return (err)
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
		log.Error(err)
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
		log.Error(err)
		return err
	}
	return nil
}

// SetWDShare updates the agent's willingness to share WD keys with other agents on this team
func (gid GoogleID) SetWDShare(teamID TeamID, state string) error {
	if state != "On" {
		state = "Off"
	}

	if _, err := db.Exec("UPDATE agentteams SET shareWD = ? WHERE gid = ? AND teamID = ?", state, gid, teamID); err != nil {
		log.Error(err)
		return err
	}
	return nil
}

// SetWDLoad updates the agent's desire to load WD keys from other agents on this team
func (gid GoogleID) SetWDLoad(teamID TeamID, state string) error {
	if state != "On" {
		state = "Off"
	}

	if _, err := db.Exec("UPDATE agentteams SET loadWD = ? WHERE gid = ? AND teamID = ?", state, gid, teamID); err != nil {
		log.Error(err)
		return err
	}
	return nil
}

// FetchAgent populates the minimal Agent struct with data anyone can see
func FetchAgent(id AgentID, caller GoogleID) (*TeamMember, error) {
	var tm TeamMember

	var vverified, vblacklisted, rocksverified, rockssmurf sql.NullBool
	var level, enlID, vname, rocksname sql.NullString
	var ifac IntelFaction

	gid, err := id.Gid()
	if err != nil {
		log.Error(err)
		return nil, err
	}

	if err = db.QueryRow("SELECT agent.gid, v.agent, rocks.agent, agent.intelname, agent.intelfaction, v.level, v.verified, v.blacklisted, v.enlid, rocks.verified, rocks.smurf FROM agent LEFT JOIN v ON agent.gid = v.gid LEFT JOIN rocks ON agent.gid = rocks.gid WHERE agent.gid = ?", gid).Scan(
		&tm.Gid, &vname, &rocksname, &tm.IntelName, &ifac, &level, &vverified, &vblacklisted, &enlID, &rocksverified, &rockssmurf); err != nil {
		log.Error(err)
		return nil, err
	}

	tm.Name = fmt.Sprint("UnverifiedAgent_" + tm.Gid)

	if tm.IntelName != "" {
		tm.Name = tm.IntelName
	}

	if rocksname.Valid {
		tm.RocksName = rocksname.String
		if rocksname.String != "-hidden-" {
			tm.Name = rocksname.String
		}
	}

	if vname.Valid {
		tm.VName = vname.String
		tm.Name = vname.String
	}

	if enlID.Valid {
		tm.EnlID = enlID.String
	}

	if vverified.Valid {
		tm.Verified = vverified.Bool
	}

	if level.Valid {
		l, err := strconv.ParseInt(level.String, 10, 8)
		if err != nil {
			log.Error(err)
		}
		tm.Level = uint8(l)
	}

	if vblacklisted.Valid {
		tm.Verified = vblacklisted.Bool
	}

	if rocksverified.Valid {
		tm.Verified = rocksverified.Bool
	}

	if rockssmurf.Valid {
		tm.RocksSmurf = rockssmurf.Bool
	}

	tm.IntelFaction = ifac.String()

	//  XXX just JOIN the agentextras table, don't do another query
	tm.PictureURL = gid.GetPicture()

	// XXX make this a distinct function?
	var count int
	if err = db.QueryRow("SELECT COUNT(*) FROM agentteams=x, agentteams=y WHERE x.gid = ? AND x.state = 'On' AND y.gid = ?", id, caller).Scan(&count); err != nil {
		log.Error(err)
		return nil, err
	}

	// no sharing location with this agent
	if count < 1 {
		return &tm, nil
	}

	var lat, lon string
	if err = db.QueryRow("SELECT Y(loc), X(loc) FROM locations WHERE gid = ?", id).Scan(&lat, &lon); err != nil {
		log.Error(err)
		return nil, err
	}
	tm.Lat, _ = strconv.ParseFloat(lat, 64)
	tm.Lon, _ = strconv.ParseFloat(lon, 64)
	return &tm, nil
}

// Name returns a team's friendly name for a TeamID
func (teamID TeamID) Name() (string, error) {
	var name string
	err := db.QueryRow("SELECT name FROM team WHERE teamID = ?", teamID).Scan(&name)
	if err != nil {
		log.Error(err)
		return "", err
	}
	return name, nil
}

// teamList is used for getting a list of all an agent's teams
func (gid GoogleID) teamList() []TeamID {
	var tid TeamID
	var x []TeamID

	rows, err := db.Query("SELECT teamID FROM agentteams WHERE gid = ?", gid)
	if err != nil {
		log.Error(err)
		return x
	}

	defer rows.Close()
	for rows.Next() {
		if err := rows.Scan(&tid); err != nil {
			log.Error(err)
			continue
		}
		x = append(x, tid)
	}
	return x
}

// TeamListEnabled is used for getting a list of agent's enabled teams
func (gid GoogleID) TeamListEnabled() []TeamID {
	var tid TeamID
	var x []TeamID

	rows, err := db.Query("SELECT teamID FROM agentteams WHERE gid = ? AND state = 'On'", gid)
	if err != nil {
		log.Error(err)
		return x
	}

	defer rows.Close()
	for rows.Next() {
		if err := rows.Scan(&tid); err != nil {
			log.Error(err)
			continue
		}
		x = append(x, tid)
	}
	return x
}

// SetComment sets an agent's squad on a given team
func (teamID TeamID) SetComment(gid GoogleID, comment string) error {
	if comment == "" {
		comment = "agents"
	}
	_, err := db.Exec("UPDATE agentteams SET squad = ? WHERE teamID = ? and gid = ?", comment, teamID, gid)
	if err != nil {
		log.Error(err)
		return err
	}
	return nil
}

// GenerateJoinToken sets a team's join link token
func (teamID TeamID) GenerateJoinToken() (string, error) {
	key, err := GenerateSafeName()
	if err != nil {
		log.Error(err)
		return key, err
	}

	_, err = db.Exec("UPDATE team SET joinLinkToken = ? WHERE teamID = ?", key, teamID)
	if err != nil {
		log.Error(err)
		return key, err
	}
	return key, nil
}

// DeleteJoinToken removes a team's join link token
func (teamID TeamID) DeleteJoinToken() error {
	_, err := db.Exec("UPDATE team SET joinLinkToken = NULL WHERE teamID = ?", teamID)
	if err != nil {
		log.Error(err)
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

	i, err := strconv.ParseInt(count, 10, 32)
	if err != nil {
		return err
	}
	if i != 1 {
		err = fmt.Errorf("invalid team join token")
		log.Errorw(err.Error(), "resource", teamID, "GID", gid)
		return err
	}

	err = teamID.AddAgent(gid)
	if err != nil {
		return err
	}
	err = teamID.SetComment(gid, "joined via link")
	if err != nil {
		return err
	}

	return nil
}

// LinkToTelegramChat associates a telegram chat ID with the team, performs authorization
func (teamID TeamID) LinkToTelegramChat(chat TelegramID, opID OperationID) error {
	_, err := db.Exec("REPLACE INTO telegramteam (teamID, telegram, opID) VALUES (?,?,?)", teamID, chat, MakeNullString(opID))
	if err != nil {
		log.Error(err)
		return err
	}

	log.Infow("linked team to telegram", "resource", teamID, "chatID", chat, "opID", opID)
	return nil
}

// UnlinkFromTelegramChat disassociates a telegram chat ID from the team -- not authenticated since bot removal from chat is enough
func (teamID TeamID) UnlinkFromTelegramChat(chat int64) error {
	_, err := db.Exec("DELETE FROM telegramteam WHERE teamID = ? AND telegram = ?", teamID, chat)
	if err != nil {
		log.Error(err)
		return err
	}

	log.Infow("unlinked team from telegram", "resource", teamID, "chatID", chat)
	return nil
}

// TelegramChat returns the associated telegram chat ID for this team, if any
func (teamID TeamID) TelegramChat() (int64, error) {
	var chatID sql.NullInt64

	err := db.QueryRow("SELECT telegram FROM team WHERE teamID = ?", teamID).Scan(&chatID)
	if err != nil && err != sql.ErrNoRows {
		log.Error(err)
		return int64(0), err
	}
	if err == sql.ErrNoRows || !chatID.Valid {
		return int64(0), nil
	}
	return chatID.Int64, nil
}

// ChatToTeam takes a chatID and returns a linked teamID
func ChatToTeam(chat int64) (TeamID, error) {
	var t TeamID

	err := db.QueryRow("SELECT teamID FROM team WHERE telegram = ?", chat).Scan(&t)
	if err != nil && err != sql.ErrNoRows {
		log.Error(err)
		return t, err
	}
	if err == sql.ErrNoRows {
		err := fmt.Errorf("attempt to get teamID for non–linked chat")
		// log.Debug(err)
		return t, err
	}
	return t, nil
}

// GetAgentLocations is a fast-path to get all available agent locations
func (gid GoogleID) GetAgentLocations() (string, error) {
	type loc struct {
		Gid  GoogleID `json:"gid"`
		Lat  float64  `json:"lat"`
		Lon  float64  `json:"lng"`
		Date string   `json:"date"`
	}

	var list []loc
	var tmpL loc
	var lat, lon string

	var rows *sql.Rows
	rows, err := db.Query("SELECT x.gid, Y(l.loc), X(l.loc), l.upTime "+
		"FROM agentteams=x, locations=l "+
		"WHERE x.teamID IN (SELECT teamID FROM agentteams WHERE gid = ?) "+
		"AND x.state = 'On' AND x.gid = l.gid", gid)
	if err != nil {
		log.Error(err)
		return "", err
	}

	defer rows.Close()
	for rows.Next() {
		if err := rows.Scan(&tmpL.Gid, &lat, &lon, &tmpL.Date); err != nil {
			log.Error(err)
			return "", err
		}
		tmpL.Lat, _ = strconv.ParseFloat(lat, 64)
		tmpL.Lon, _ = strconv.ParseFloat(lon, 64)

		if tmpL.Lat == 0 || tmpL.Lon == 0 {
			continue
		}

		list = append(list, tmpL)
	}

	jList, err := json.Marshal(list)
	if err != nil {
		return "", err
	}
	return string(jList), nil
}
