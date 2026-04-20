package model

import (
	"context"
	"database/sql"
	"errors"

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
	RocksComm     string       `json:"rc,omitempty"`
	RocksKey      string       `json:"rk,omitempty"`
	JoinLinkToken string       `json:"jlt,omitempty"`
	TeamMembers   []TeamMember `json:"agents"`
}

// TeamMember is the light version of AgentData
type TeamMember struct {
	Gid           GoogleID `json:"id"`
	Name          string   `json:"name"`
	RocksName     string   `json:"rocksname,omitempty"`
	IntelName     string   `json:"intelname,omitempty"`
	PictureURL    string   `json:"pic,omitempty"`
	IntelFaction  string   `json:"intelfaction"`
	Comment       string   `json:"squad,omitempty"`
	Date          string   `json:"date"`
	Lat           float64  `json:"lat,omitempty"`
	Lon           float64  `json:"lng,omitempty"`
	Level         uint8    `json:"level,omitempty"`
	RocksVerified bool     `json:"rocks"`
	RocksSmurf    bool     `json:"smurf"`
	ShareLocation bool     `json:"state"`
	ShareWD       bool     `json:"shareWD"`
	LoadWD        bool     `json:"loadWD"`
}

// AgentInTeam checks to see if a agent is in a team and enabled.
func (gid GoogleID) AgentInTeam(ctx context.Context, team TeamID) (bool, error) {
	var count int
	err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM agentteams WHERE teamID = ? AND gid = ?", team, gid).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// FetchTeam populates an entire TeamData struct
func (teamID TeamID) FetchTeam(ctx context.Context) (*TeamData, error) {
	var teamList TeamData

	// Note: X() and Y() are legacy spatial functions, ensure your MariaDB version supports them or use ST_X()/ST_Y()
	rows, err := db.QueryContext(ctx, "SELECT agentteams.gid, agent.IntelName, rocks.Agent, agentteams.comment, agentteams.shareLoc, ST_Y(locations.loc), ST_X(locations.loc), locations.upTime, rocks.verified, rocks.smurf, agentteams.sharewd, agentteams.loadwd, agent.intelfaction, agent.picurl "+
		" FROM agentteams JOIN team ON agentteams.teamID = team.teamID JOIN agent ON agentteams.gid = agent.gid JOIN locations ON agentteams.gid = locations.gid LEFT JOIN rocks ON agentteams.gid = rocks.gid WHERE agentteams.teamID = ?", teamID)
	if err != nil {
		log.Error(err)
		return &teamList, err
	}
	defer rows.Close()

	for rows.Next() {
		agent := TeamMember{}
		var lat, lon sql.NullFloat64
		var faction IntelFaction
		var rocksverified, rockssmurf sql.NullBool
		var intelname, rocksname, picurl, comment sql.NullString

		err := rows.Scan(&agent.Gid, &intelname, &rocksname, &comment, &agent.ShareLocation, &lat, &lon, &agent.Date, &rocksverified, &rockssmurf, &agent.ShareWD, &agent.LoadWD, &faction, &picurl)
		if err != nil {
			log.Error(err)
			return &teamList, err
		}

		// Assume bestname will need ctx eventually if it hits DB
		agent.Name = agent.Gid.bestname(intelname, rocksname)

		if intelname.Valid {
			agent.IntelName = intelname.String
		}
		if rocksname.Valid {
			agent.RocksName = rocksname.String
		}
		if comment.Valid {
			agent.Comment = comment.String
		}
		if rocksverified.Valid {
			agent.RocksVerified = rocksverified.Bool
		}
		if rockssmurf.Valid {
			agent.RocksSmurf = rockssmurf.Bool
		}

		if agent.ShareLocation && lat.Valid && lon.Valid {
			agent.Lat = lat.Float64
			agent.Lon = lon.Float64
		}

		if picurl.Valid {
			agent.PictureURL = picurl.String
		}

		agent.IntelFaction = faction.String()
		teamList.TeamMembers = append(teamList.TeamMembers, agent)
	}

	var rockscomm, rockskey, joinlinktoken sql.NullString
	err = db.QueryRowContext(ctx, "SELECT name, rockscomm, rockskey, joinLinkToken FROM team WHERE teamID = ?", teamID).
		Scan(&teamList.Name, &rockscomm, &rockskey, &joinlinktoken)
	if err != nil {
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
func (teamID TeamID) Owner(ctx context.Context) (GoogleID, error) {
	var owner GoogleID
	err := db.QueryRowContext(ctx, "SELECT owner FROM team WHERE teamID = ?", teamID).Scan(&owner)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil
		}
		log.Error(err)
		return "", err
	}
	return owner, nil
}

// OwnsTeam returns true if the GoogleID owns the team
func (gid GoogleID) OwnsTeam(ctx context.Context, teamID TeamID) (bool, error) {
	var count int
	err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM team WHERE teamID = ? AND owner = ?", teamID, gid).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// NewTeam initializes a new team
func (gid GoogleID) NewTeam(ctx context.Context, name string) (TeamID, error) {
	team, err := GenerateSafeName(ctx)
	if err != nil {
		return "", err
	}

	name = util.Sanitize(name)
	if name == "" {
		name = team
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()

	if _, err = tx.ExecContext(ctx, "INSERT INTO team (teamID, owner, name, rockskey, rockscomm, vteam, vrole) VALUES (?,?,?,NULL,NULL,0,0)", team, gid, name); err != nil {
		return "", err
	}
	if _, err = tx.ExecContext(ctx, "INSERT INTO agentteams (teamID, gid, shareLoc, comment, shareWD, loadWD) VALUES (?,?,0,'owner',0,0)", team, gid); err != nil {
		return "", err
	}

	if err := tx.Commit(); err != nil {
		return "", err
	}
	return TeamID(team), nil
}

// Rename sets a new name for a teamID
func (teamID TeamID) Rename(ctx context.Context, name string) error {
	name = util.Sanitize(name)
	if name == "" {
		return errors.New("empty name on rename")
	}

	_, err := db.ExecContext(ctx, "UPDATE team SET name = ? WHERE teamID = ?", name, teamID)
	return err
}

// Delete removes the team identified by teamID
func (teamID TeamID) Delete(ctx context.Context) error {
	rows, err := db.QueryContext(ctx, "SELECT gid FROM agentteams WHERE teamID = ?", teamID)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var gid GoogleID
		if err = rows.Scan(&gid); err != nil {
			continue
		}
		// Passing context through to nested removal
		_ = teamID.RemoveAgent(ctx, gid)
	}

	// Transactions ensure consistency during deletion
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err = tx.ExecContext(ctx, "DELETE FROM permissions WHERE teamID = ?", teamID); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, "DELETE FROM team WHERE teamID = ?", teamID); err != nil {
		return err
	}

	return tx.Commit()
}

// AddAgent adds a agent to a team
func (teamID TeamID) AddAgent(ctx context.Context, in AgentID) error {
	gid, err := in.Gid(ctx)
	if err != nil {
		return err
	}

	_, err = db.ExecContext(ctx, "INSERT IGNORE INTO agentteams (teamID, gid, shareLoc, comment, shareWD, loadWD) VALUES (?, ?, 0, 'agents', 0, 0)", teamID, gid)
	if err != nil {
		return err
	}

	// Assuming messaging needs context later
	messaging.AddToRemote(ctx, messaging.GoogleID(gid), messaging.TeamID(teamID))
	return nil
}

// RemoveAgent removes an agent from a team
func (teamID TeamID) RemoveAgent(ctx context.Context, in AgentID) error {
	gid, err := in.Gid(ctx)
	if err != nil {
		return err
	}

	_, err = db.ExecContext(ctx, "DELETE FROM agentteams WHERE teamID = ? AND gid = ?", teamID, gid)
	if err != nil {
		return err
	}

	messaging.RemoveFromRemote(ctx, messaging.GoogleID(gid), messaging.TeamID(teamID))

	// Clean up permissions/assignments associated with this team/agent
	rows, err := db.QueryContext(ctx, "SELECT opID FROM permissions WHERE teamID = ?", teamID)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var opID OperationID
			if err := rows.Scan(&opID); err == nil {
				messaging.AgentDeleteOperation(ctx, messaging.GoogleID(gid), messaging.OperationID(opID))
			}
		}
	}

	// Remove team from ops the agent owns
	_, err = db.ExecContext(ctx, "DELETE FROM permissions WHERE teamID = ? AND opID IN (SELECT ID FROM operation WHERE gid = ?)", teamID, gid)
	return err
}

// Chown changes a team's ownership
func (teamID TeamID) Chown(ctx context.Context, to AgentID) error {
	gid, err := to.Gid(ctx)
	if err != nil {
		return err
	}

	_, err = db.ExecContext(ctx, "UPDATE team SET owner = ? WHERE teamID = ?", gid, teamID)
	return err
}

func (teamID TeamID) String() string {
	return string(teamID)
}

// SetTeamState updates the agent's shareLoc
func (gid GoogleID) SetTeamState(ctx context.Context, teamID TeamID, state bool) error {
	_, err := db.ExecContext(ctx, "UPDATE agentteams SET shareLoc = ? WHERE gid = ? AND teamID = ?", state, gid, teamID)
	return err
}

// SetWDShare updates the agent's willingness to share WD keys
func (gid GoogleID) SetWDShare(ctx context.Context, teamID TeamID, state bool) error {
	_, err := db.ExecContext(ctx, "UPDATE agentteams SET shareWD = ? WHERE gid = ? AND teamID = ?", state, gid, teamID)
	return err
}

// SetWDLoad updates the agent's desire to load WD keys
func (gid GoogleID) SetWDLoad(ctx context.Context, teamID TeamID, state bool) error {
	_, err := db.ExecContext(ctx, "UPDATE agentteams SET loadWD = ? WHERE gid = ? AND teamID = ?", state, gid, teamID)
	return err
}

// FetchAgent populates the minimal Agent struct
func FetchAgent(ctx context.Context, id AgentID, caller GoogleID) (*TeamMember, error) {
	var tm TeamMember

	gid, err := id.Gid(ctx)
	if err != nil {
		return nil, err
	}

	var rocksverified, rockssmurf sql.NullBool
	var rocksname, intelname, picurl sql.NullString
	var ifac IntelFaction

	err = db.QueryRowContext(ctx, "SELECT agent.gid, rocks.agent, agent.intelname, agent.intelfaction, rocks.verified, rocks.smurf, agent.picurl FROM agent LEFT JOIN rocks ON agent.gid = rocks.gid WHERE agent.gid = ?", gid).
		Scan(&tm.Gid, &rocksname, &intelname, &ifac, &rocksverified, &rockssmurf, &picurl)
	if err != nil {
		return nil, err
	}

	tm.Name = tm.Gid.bestname(intelname, rocksname)
	if intelname.Valid {
		tm.IntelName = intelname.String
	}
	if rocksname.Valid {
		tm.RocksName = rocksname.String
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

	// Check if they share location with the caller
	var count int
	_ = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM agentteams AS x JOIN agentteams AS y ON x.teamID = y.teamID WHERE x.gid = ? AND x.shareLoc = 1 AND y.gid = ?", gid, caller).Scan(&count)

	if count > 0 {
		var lat, lon sql.NullFloat64
		if err = db.QueryRowContext(ctx, "SELECT ST_Y(loc), ST_X(loc) FROM locations WHERE gid = ?", gid).Scan(&lat, &lon); err == nil {
			tm.Lat = lat.Float64
			tm.Lon = lon.Float64
		}
	}

	return &tm, nil
}

// Name returns a team's friendly name
func (teamID TeamID) Name(ctx context.Context) (string, error) {
	var name string
	err := db.QueryRowContext(ctx, "SELECT name FROM team WHERE teamID = ?", teamID).Scan(&name)
	return name, err
}

// TeamList returns a list of all an agent's teams
func (gid GoogleID) TeamList(ctx context.Context) []TeamID {
	var x []TeamID
	rows, err := db.QueryContext(ctx, "SELECT teamID FROM agentteams WHERE gid = ?", gid)
	if err != nil {
		return x
	}
	defer rows.Close()

	for rows.Next() {
		var tid TeamID
		if err := rows.Scan(&tid); err == nil {
			x = append(x, tid)
		}
	}
	return x
}

// TeamListEnabled returns a list of agent's enabled teams
func (gid GoogleID) TeamListEnabled(ctx context.Context) []TeamID {
	var x []TeamID
	rows, err := db.QueryContext(ctx, "SELECT teamID FROM agentteams WHERE gid = ? AND shareLoc = 1", gid)
	if err != nil {
		return x
	}
	defer rows.Close()

	for rows.Next() {
		var tid TeamID
		if err := rows.Scan(&tid); err == nil {
			x = append(x, tid)
		}
	}
	return x
}

// SetComment sets an agent's comment on a team
func (teamID TeamID) SetComment(ctx context.Context, gid GoogleID, comment string) error {
	c := makeNullString(util.Sanitize(comment))
	_, err := db.ExecContext(ctx, "UPDATE agentteams SET comment = ? WHERE teamID = ? AND gid = ?", c, teamID, gid)
	return err
}

// GenerateJoinToken sets a team's join link token
func (teamID TeamID) GenerateJoinToken(ctx context.Context) (string, error) {
	key, err := GenerateSafeName(ctx)
	if err != nil {
		return "", err
	}

	_, err = db.ExecContext(ctx, "UPDATE team SET joinLinkToken = ? WHERE teamID = ?", key, teamID)
	return key, err
}

// DeleteJoinToken removes a team's join link token
func (teamID TeamID) DeleteJoinToken(ctx context.Context) error {
	_, err := db.ExecContext(ctx, "UPDATE team SET joinLinkToken = NULL WHERE teamID = ?", teamID)
	return err
}

// JoinToken verifies a join link and adds the agent
func (teamID TeamID) JoinToken(ctx context.Context, gid GoogleID, key string) error {
	var count int
	err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM team WHERE teamID = ? AND joinLinkToken = ?", teamID, key).Scan(&count)
	if err != nil {
		return err
	}

	if count != 1 {
		return errors.New("invalid team join token")
	}

	if err = teamID.AddAgent(ctx, gid); err != nil {
		return err
	}
	return teamID.SetComment(ctx, gid, "joined via link")
}

func (teamID TeamID) FetchFBTokens(ctx context.Context) ([]string, error) {
	var tokens []string

	rows, err := db.QueryContext(ctx, "SELECT firebase.token FROM agentteams JOIN firebase ON firebase.gid = agentteams.gid WHERE agentteams.teamID = ?", teamID)
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

func GetAllTeams(ctx context.Context) ([]TeamID, error) {
	var teams []TeamID

	rows, err := db.QueryContext(ctx, "SELECT DISTINCT teamID FROM team")
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
func (teamID TeamID) Valid(ctx context.Context) bool {
	var count int
	err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM team WHERE teamID = ?", teamID).Scan(&count)
	return err == nil && count == 1
}
