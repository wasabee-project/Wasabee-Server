package model

import (
	"database/sql"
	"fmt"
	"strconv"

	"github.com/wasabee-project/Wasabee-Server/log"
	"github.com/wasabee-project/Wasabee-Server/messaging"
	"github.com/wasabee-project/Wasabee-Server/util"
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
	VName         string   `json:"vname,omitempty"`
	RocksName     string   `json:"rocksname,omitempty"`
	IntelName     string   `json:"intelname,omitempty"`
	CommunityName string   `json:"communityname,omitempty"`
	Level         uint8    `json:"level,omitempty"`
	EnlID         string   `json:"enlid,omitempty"`
	PictureURL    string   `json:"pic,omitempty"`
	Verified      bool     `json:"Vverified"`
	Blacklisted   bool     `json:"blacklisted"`
	RocksVerified bool     `json:"rocks"`
	RocksSmurf    bool     `json:"smurf"`
	IntelFaction  string   `json:"intelfaction"`
	Comment       string   `json:"squad,omitempty"`
	ShareLocation bool     `json:"state"`
	Lat           float64  `json:"lat,omitempty"`
	Lon           float64  `json:"lng,omitempty"`
	Date          string   `json:"date"`
	ShareWD       bool     `json:"shareWD"`
	LoadWD        bool     `json:"loadWD"`
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
	// var rows *sql.Rows

	rows, err := db.Query("SELECT agentteams.gid, v.Agent, agent.IntelName, rocks.Agent, agentteams.comment, agentteams.shareLoc, Y(locations.loc), X(locations.loc), locations.upTime, v.Verified, v.Blacklisted, v.EnlID, rocks.verified, rocks.smurf, agentteams.sharewd, agentteams.loadwd, agent.intelfaction, agent.communityname, agent.picurl "+
		" FROM agentteams JOIN team ON agentteams.teamID = team.teamID JOIN agent ON agentteams.gid = agent.gid JOIN locations ON agentteams.gid = locations.gid LEFT JOIN v ON agentteams.gid = v.gid LEFT JOIN rocks ON agentteams.gid = rocks.gid WHERE agentteams.teamID = ?", teamID)
	if err != nil {
		log.Error(err)
		return &teamList, err
	}
	defer rows.Close()

	for rows.Next() {
		agent := TeamMember{}
		var lat, lon string
		var faction IntelFaction
		var vverified, vblacklisted, rocksverified, rockssmurf sql.NullBool
		var intelname, communityname, enlID, vname, rocksname, picurl, comment sql.NullString

		err := rows.Scan(&agent.Gid, &vname, &intelname, &rocksname, &comment, &agent.ShareLocation, &lat, &lon, &agent.Date, &vverified, &vblacklisted, &enlID, &rocksverified, &rockssmurf, &agent.ShareWD, &agent.LoadWD, &faction, &communityname, &picurl)
		if err != nil {
			log.Error(err)
			return &teamList, err
		}

		agent.Name = agent.Gid.bestname(intelname, vname, rocksname, communityname)

		if intelname.Valid {
			agent.IntelName = intelname.String
		}

		if communityname.Valid {
			agent.CommunityName = communityname.String
		}

		if vname.Valid {
			agent.VName = vname.String
		}

		if rocksname.Valid {
			agent.RocksName = rocksname.String
		}

		if comment.Valid {
			agent.Comment = comment.String
		}

		if enlID.Valid {
			agent.EnlID = enlID.String
		}

		if vverified.Valid {
			agent.Verified = vverified.Bool
		}

		if vblacklisted.Valid {
			agent.Blacklisted = vblacklisted.Bool
		}

		if rocksverified.Valid {
			agent.RocksVerified = rocksverified.Bool
		}

		if rockssmurf.Valid {
			agent.RocksSmurf = rockssmurf.Bool
		}

		if agent.ShareLocation {
			agent.Lat, _ = strconv.ParseFloat(lat, 64)
			agent.Lon, _ = strconv.ParseFloat(lon, 64)
		} else {
			agent.Lat = 0
			agent.Lon = 0
		}

		if picurl.Valid {
			agent.PictureURL = picurl.String
		}

		agent.IntelFaction = faction.String()
		teamList.TeamMembers = append(teamList.TeamMembers, agent)
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

// Owner returns the owner of the team
func (teamID TeamID) Owner() (GoogleID, error) {
	var owner GoogleID

	err := db.QueryRow("SELECT owner FROM team WHERE teamID = ?", teamID).Scan(&owner)
	if err != nil && err == sql.ErrNoRows {
		// log.Warnw("non-existent team ownership queried", "resource", teamID)
		return "", nil
	} else if err != nil {
		log.Error(err)
		return "", err
	}
	return owner, nil
}

// OwnsTeam returns true if the GoogleID owns the team identified by teamID
func (gid GoogleID) OwnsTeam(teamID TeamID) (bool, error) {
	var count int

	err := db.QueryRow("SELECT COUNT(*) FROM team WHERE teamID = ? AND owner = ?", teamID, gid).Scan(&count)
	if err != nil {
		return false, err
	}
	if count < 1 {
		return false, nil
	}
	return true, nil
}

// NewTeam initializes a new team and returns a teamID
// the creating gid is added and enabled on that team by default
func (gid GoogleID) NewTeam(name string) (TeamID, error) {
	team, err := GenerateSafeName()
	if err != nil {
		log.Error(err)
		return "", err
	}

	name = util.Sanitize(name)
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
	_, err = db.Exec("INSERT INTO agentteams (teamID, gid, shareLoc, comment, shareWD, loadWD) VALUES (?,?,0,'owner',0,0)", team, gid)
	if err != nil {
		log.Error(err)
		return TeamID(team), err
	}
	return TeamID(team), nil
}

// Rename sets a new name for a teamID
// does not check team ownership -- caller should take care of authorization
func (teamID TeamID) Rename(name string) error {
	name = util.Sanitize(name)
	if name == "" {
		err := fmt.Errorf("empty name on rename")
		log.Errorw(err.Error(), "resource", teamID, "message", err.Error())
		return err
	}

	if _, err := db.Exec("UPDATE team SET name = ? WHERE teamID = ?", name, teamID); err != nil {
		log.Error(err)
		return err
	}
	return nil
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
	defer rows.Close()

	for rows.Next() {
		var gid GoogleID
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

	_, err = db.Exec("INSERT IGNORE INTO agentteams (teamID, gid, shareLoc, comment, shareWD, loadWD) VALUES (?, ?, 0, 'agents', 0, 0)", teamID, gid)
	if err != nil {
		log.Error(err)
		return err
	}

	messaging.AddToRemote(messaging.GoogleID(gid), messaging.TeamID(teamID))
	// log.Infow("adding agent to team", "GID", gid, "resource", teamID, "message", "adding agent to team")
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

	messaging.RemoveFromRemote(messaging.GoogleID(gid), messaging.TeamID(teamID))

	// instruct the agent to delete all associated ops
	// this may get ops for which the agent has double-access, but they can just re-fetch them
	rows, err := db.Query("SELECT opID FROM permissions WHERE teamID = ?", teamID)
	if err != nil && err != sql.ErrNoRows {
		log.Error(err)
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var opID OperationID
		err = rows.Scan(&opID)
		if err != nil {
			log.Error(err)
			// continue
		}
		messaging.AgentDeleteOperation(messaging.GoogleID(gid), messaging.OperationID(opID))
	}

	// log.Debugw("removing agent from team", "GID", gid, "resource", teamID, "message", "removing agent from team")
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

func (teamID TeamID) String() string {
	return string(teamID)
}

// SetTeamState updates the agent's shareLoc the team
func (gid GoogleID) SetTeamState(teamID TeamID, state bool) error {
	if _, err := db.Exec("UPDATE agentteams SET shareLoc = ? WHERE gid = ? AND teamID = ?", state, gid, teamID); err != nil {
		log.Error(err)
		return err
	}
	return nil
}

// SetWDShare updates the agent's willingness to share WD keys with other agents on this team
func (gid GoogleID) SetWDShare(teamID TeamID, state bool) error {
	if _, err := db.Exec("UPDATE agentteams SET shareWD = ? WHERE gid = ? AND teamID = ?", state, gid, teamID); err != nil {
		log.Error(err)
		return err
	}
	return nil
}

// SetWDLoad updates the agent's desire to load WD keys from other agents on this team
func (gid GoogleID) SetWDLoad(teamID TeamID, state bool) error {
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
	var level, enlID, vname, rocksname, intelname, communityname, picurl sql.NullString
	var ifac IntelFaction

	gid, err := id.Gid()
	if err != nil {
		log.Error(err)
		return nil, err
	}

	if err = db.QueryRow("SELECT agent.gid, v.agent, rocks.agent, agent.intelname, agent.intelfaction, v.level, v.verified, v.blacklisted, v.enlid, rocks.verified, rocks.smurf, agent.communityname, agent.picurl FROM agent LEFT JOIN v ON agent.gid = v.gid LEFT JOIN rocks ON agent.gid = rocks.gid WHERE agent.gid = ?", gid).Scan(
		&tm.Gid, &vname, &rocksname, &intelname, &ifac, &level, &vverified, &vblacklisted, &enlID, &rocksverified, &rockssmurf, &communityname, &picurl); err != nil {
		log.Error(err)
		return nil, err
	}

	tm.Name = tm.Gid.bestname(intelname, vname, rocksname, communityname)

	if intelname.Valid {
		tm.IntelName = intelname.String
	}

	if communityname.Valid {
		tm.CommunityName = communityname.String
	}

	if vname.Valid {
		tm.VName = vname.String
	}

	if rocksname.Valid {
		tm.RocksName = rocksname.String
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
		tm.Blacklisted = vblacklisted.Bool
	}

	if rocksverified.Valid {
		tm.RocksVerified = rocksverified.Bool
	}

	if rockssmurf.Valid {
		tm.RocksSmurf = rockssmurf.Bool
	}

	tm.IntelFaction = ifac.String()

	if picurl.Valid {
		tm.PictureURL = picurl.String
	}

	// XXX make this a distinct function?
	var count int
	if err = db.QueryRow("SELECT COUNT(*) FROM agentteams=x, agentteams=y WHERE x.gid = ? AND x.shareLoc = 1 AND y.gid = ?", id, caller).Scan(&count); err != nil {
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

	rows, err := db.Query("SELECT teamID FROM agentteams WHERE gid = ? AND shareLoc = 1", gid)
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

// SetComment sets an agent's comment on a given team
func (teamID TeamID) SetComment(gid GoogleID, comment string) error {
	c := makeNullString(util.Sanitize(comment))

	_, err := db.Exec("UPDATE agentteams SET comment = ? WHERE teamID = ? AND gid = ?", c, teamID, gid)
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

func (teamID TeamID) FetchFBTokens() ([]string, error) {
	var tokens []string

	rows, err := db.Query("SELECT firebase.token FROM agentteams JOIN firebase ON firebase.gid = agentteams.gid WHERE agentteams.teamID = ?", teamID)
	if err != nil && err != sql.ErrNoRows {
		log.Error(err)
		return tokens, err
	}
	defer rows.Close()

	for rows.Next() {
		var token string
		if err = rows.Scan(&token); err != nil {
			log.Error(err)
			continue
		}
		tokens = append(tokens, token)
	}

	return tokens, nil
}

func GetAllTeams() ([]TeamID, error) {
	var teams []TeamID

	rows, err := db.Query("SELECT DISTINCT teamID FROM team")
	if err != nil && err != sql.ErrNoRows {
		log.Error(err)
		return teams, err
	}
	defer rows.Close()

	for rows.Next() {
		var teamID TeamID
		if err = rows.Scan(&teamID); err != nil {
			log.Error(err)
			continue
		}
		teams = append(teams, teamID)
	}

	return teams, nil
}

// Valid checks to see if team exists.
func (teamID TeamID) Valid() bool {
	var i uint8

	err := db.QueryRow("SELECT COUNT(*) FROM team WHERE teamID = ?", teamID).Scan(&i)
	if err != nil || i != 1 {
		return false
	}

	return true
}
