package model

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"

	"github.com/wasabee-project/Wasabee-Server/config"
	"github.com/wasabee-project/Wasabee-Server/log"
	"github.com/wasabee-project/Wasabee-Server/messaging"
	"github.com/wasabee-project/Wasabee-Server/util"
)

// GoogleID is the primary location for interfacing with the agent type
type GoogleID string

// Agent is the complete agent struct, used for the /me page.
type Agent struct {
	GoogleID     GoogleID     `json:"GoogleID"`
	Name         string       `json:"name,omitempty"`
	RocksName    string       `json:"rocksname,omitempty"`
	IntelName    string       `json:"intelname,omitempty"`
	OneTimeToken OneTimeToken `json:"lockey,omitempty"`
	ProfileImage string       `json:"pic,omitempty"`
	IntelFaction string       `json:"intelfaction,omitempty"`
	QueryToken   string       `json:"querytoken,omitempty"`
	JWT          string       `json:"jwt,omitempty"`
	Teams        []AdTeam
	Ops          []AdOperation
	Telegram     struct {
		Name      string `json:"name,omitempty"`
		Authtoken string `json:"Authtoken,omitempty"`
		ID        int64  `json:"ID,omitempty"`
		Verified  bool   `json:"Verified,omitempty"`
	}
	Level         uint8 `json:"level,omitempty"` // from v
	RocksVerified bool  `json:"rocks,omitempty"`
	RISC          bool  `json:"RISC,omitempty"`
}

// AdTeam is a sub-struct of Agent
type AdTeam struct {
	ID            TeamID
	Name          string `json:"Name,omitempty"`
	RocksComm     string `json:"RocksComm,omitempty"`
	RocksKey      string `json:"RocksKey,omitempty"`
	JoinLinkToken string `json:"JoinLinkToken,omitempty"`
	ShareLoc      string `json:"State"`
	ShareWD       string
	LoadWD        string
	Owner         GoogleID
}

// AdOperation is a sub-struct of Agent
type AdOperation struct {
	ID         OperationID
	Name       string
	Color      string
	TeamID     TeamID
	Modified   string
	LastEditID string
	IsOwner    bool
}

// AgentID is anything that can be converted to a GoogleID or a string
type AgentID interface {
	Gid(ctx context.Context) (GoogleID, error)
	fmt.Stringer
}

// AgentLocation is the lite version used for the team location pull
type AgentLocation struct {
	Gid  GoogleID `json:"gid"`
	Date string   `json:"date"`
	Lat  float64  `json:"lat"`
	Lon  float64  `json:"lng"`
}

// Gid just satisfies the AgentID interface
func (gid GoogleID) Gid(ctx context.Context) (GoogleID, error) {
	return gid, nil
}

// GetAgent populates an Agent struct based on the gid
func (gid GoogleID) GetAgent(ctx context.Context) (*Agent, error) {
	var a Agent
	a.GoogleID = gid
	var level, pic, intelname, rocksname sql.NullString
	var rocksverified sql.NullBool
	var ifac IntelFaction

	err := db.QueryRowContext(ctx, "SELECT rocks.agent AS Rocksname, a.intelname, a.OneTimeToken, rocks.verified AS RockVerified, a.RISC, a.intelfaction, a.picurl FROM agent=a LEFT JOIN rocks ON a.gid = rocks.gid WHERE a.gid = ?", gid).Scan(&rocksname, &intelname, &a.OneTimeToken, &rocksverified, &a.RISC, &ifac, &pic)
	if err != nil && errors.Is(err, sql.ErrNoRows) {
		return &a, errors.New(ErrUnknownGID)
	}
	if err != nil {
		log.Error(err)
		return &a, err
	}

	a.Name = gid.bestname(intelname, rocksname)

	if intelname.Valid {
		a.IntelName = intelname.String
	}

	if rocksname.Valid {
		a.RocksName = rocksname.String
	}

	if pic.Valid {
		a.ProfileImage = pic.String
	}

	if rocksverified.Valid {
		a.RocksVerified = rocksverified.Bool
	}

	if level.Valid {
		l, err := strconv.ParseInt(level.String, 10, 8)
		if err != nil {
			log.Error(err)
		}
		a.Level = uint8(l)
	}

	if err = adTeams(ctx, &a); err != nil {
		return &a, err
	}

	if err = adTelegram(ctx, &a); err != nil {
		return &a, err
	}

	if err = adOps(ctx, &a); err != nil {
		return &a, err
	}

	a.IntelFaction = ifac.String()

	return &a, nil
}

func adTeams(ctx context.Context, ad *Agent) error {
	rows, err := db.QueryContext(ctx, "SELECT x.teamID, team.name, x.shareLoc, x.shareWD, x.loadWD, team.rockscomm, team.rockskey, team.owner, team.joinLinkToken FROM agentteams=x JOIN team ON x.teamID = team.teamID WHERE x.gid = ?", ad.GoogleID)
	if err != nil {
		log.Error(err)
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var team AdTeam
		var shareLoc, shareWD, loadWD bool
		var rc, rk, jlt sql.NullString

		err := rows.Scan(&team.ID, &team.Name, &shareLoc, &shareWD, &loadWD, &rc, &rk, &team.Owner, &jlt)
		if err != nil {
			log.Error(err)
			return err
		}

		if rc.Valid {
			team.RocksComm = rc.String
		}

		if rk.Valid && team.Owner == ad.GoogleID {
			team.RocksKey = rk.String
		}

		if jlt.Valid {
			team.JoinLinkToken = jlt.String
		}

		team.ShareLoc = "Off"
		if shareLoc {
			team.ShareLoc = "On"
		}

		team.ShareWD = "Off"
		if shareWD {
			team.ShareWD = "On"
		}

		team.LoadWD = "Off"
		if loadWD {
			team.LoadWD = "On"
		}

		ad.Teams = append(ad.Teams, team)
	}
	return nil
}

func adTelegram(ctx context.Context, ad *Agent) error {
	var authtoken sql.NullString
	err := db.QueryRowContext(ctx, "SELECT telegramID, telegramName, verified, authtoken FROM telegram WHERE gid = ?", ad.GoogleID).Scan(&ad.Telegram.ID, &ad.Telegram.Name, &ad.Telegram.Verified, &authtoken)
	if err != nil && errors.Is(err, sql.ErrNoRows) {
		ad.Telegram.ID = 0
		ad.Telegram.Name = ""
		ad.Telegram.Verified = false
		ad.Telegram.Authtoken = ""
		return nil
	} else if err != nil {
		log.Error(err)
		return err
	}
	if authtoken.Valid {
		ad.Telegram.Authtoken = authtoken.String
	}
	return nil
}

func adOps(ctx context.Context, ad *Agent) error {
	seen := make(map[OperationID]bool)

	rowOwned, err := db.QueryContext(ctx, "SELECT ID, Name, Color, modified, lasteditid FROM operation WHERE gid = ?", ad.GoogleID)
	if err != nil {
		log.Error(err)
		return err
	}
	defer rowOwned.Close()

	for rowOwned.Next() {
		var op AdOperation
		err := rowOwned.Scan(&op.ID, &op.Name, &op.Color, &op.Modified, &op.LastEditID)
		if err != nil {
			log.Error(err)
			return err
		}
		op.IsOwner = true
		if seen[op.ID] {
			continue
		}
		ad.Ops = append(ad.Ops, op)
		seen[op.ID] = true
	}

	rowTeam, err := db.QueryContext(ctx, "SELECT operation.ID, operation.Name, operation.Color, permissions.teamID, operation.modified, operation.lasteditid FROM agentteams JOIN permissions ON agentteams.teamID = permissions.teamID JOIN operation ON permissions.opID = operation.ID WHERE agentteams.gid = ?", ad.GoogleID)
	if err != nil {
		log.Error(err)
		return err
	}
	defer rowTeam.Close()

	for rowTeam.Next() {
		var op AdOperation
		err := rowTeam.Scan(&op.ID, &op.Name, &op.Color, &op.TeamID, &op.Modified, &op.LastEditID)
		if err != nil {
			log.Error(err)
			return err
		}
		if seen[op.ID] {
			continue
		}
		ad.Ops = append(ad.Ops, op)
		seen[op.ID] = true
	}
	return nil
}

// SetLocation updates the database to reflect an agent's current location
func (gid GoogleID) SetLocation(ctx context.Context, lat, lon string) error {
	if lat == "" || lon == "" {
		return nil
	}

	flat, err := strconv.ParseFloat(lat, 64)
	if err != nil {
		log.Error(err)
		flat = 0
	}

	flon, err := strconv.ParseFloat(lon, 64)
	if err != nil {
		log.Error(err)
		flon = 0
	}

	point := fmt.Sprintf("POINT(%s %s)", strconv.FormatFloat(flon, 'f', 7, 64), strconv.FormatFloat(flat, 'f', 7, 64))
	if _, err := db.ExecContext(ctx, "UPDATE locations SET loc = PointFromText(?), upTime = UTC_TIMESTAMP() WHERE gid = ?", point, gid); err != nil {
		log.Error(err)
		return err
	}

	return nil
}

// IngressName returns an agent's name for a given GoogleID.
func (gid GoogleID) IngressName(ctx context.Context) (string, error) {
	var intelname, rocksname sql.NullString
	err := db.QueryRowContext(ctx, "SELECT rocks.agent, agent.intelname FROM agent LEFT JOIN rocks ON agent.gid = rocks.gid WHERE agent.gid = ?", gid).Scan(&rocksname, &intelname)

	if err != nil && errors.Is(err, sql.ErrNoRows) {
		log.Error("getting ingressname for unknown gid")
		return "Unknown Agent", nil
	}
	if err != nil {
		log.Error(err)
		return string(gid), err
	}

	return gid.bestname(intelname, rocksname), nil
}

// IngressName is used for templates
// Note: This helper might need special care if the templates can't pass context yet.
func IngressName(ctx context.Context, g messaging.GoogleID) string {
	name, _ := GoogleID(string(g)).IngressName(ctx)
	return name
}

func (gid GoogleID) bestname(intel, rocks sql.NullString) string {
	if rocks.Valid && rocks.String != "-hidden-" {
		return rocks.String
	}
	if intel.Valid {
		return intel.String
	}
	return fmt.Sprint("UnverifiedAgent_", gid)
}

// Valid returns "true" if the GoogleID is known to wasabee
func (gid GoogleID) Valid(ctx context.Context) bool {
	var count int
	err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM agent WHERE gid = ?", gid).Scan(&count)
	if err != nil {
		log.Error(err)
		return false
	}
	return count == 1
}

func (gid GoogleID) String() string {
	return string(gid)
}

// SearchAgentName gets a GoogleID from an Agent's name
func SearchAgentName(ctx context.Context, agent string) (GoogleID, error) {
	var gid GoogleID
	var count int

	if len(agent) > 0 && agent[0] == '@' {
		err := db.QueryRowContext(ctx, "SELECT gid FROM telegram WHERE LOWER(telegramName) = LOWER(?)", agent[1:]).Scan(&gid)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			log.Error(err)
			return "", err
		}
		if gid != "" {
			return gid, nil
		}
	}

	err := db.QueryRowContext(ctx, "SELECT gid FROM agent WHERE LOWER(communityname) = LOWER(?)", agent).Scan(&gid)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		log.Error(err)
		return "", err
	}
	if !errors.Is(err, sql.ErrNoRows) && gid != "" {
		return gid, nil
	}

	err = db.QueryRowContext(ctx, "SELECT COUNT(gid) FROM rocks WHERE LOWER(agent) = LOWER(?)", agent).Scan(&count)
	if err != nil {
		log.Error(err)
		return "", err
	}
	if count == 1 {
		err := db.QueryRowContext(ctx, "SELECT gid FROM rocks WHERE LOWER(agent) = LOWER(?)", agent).Scan(&gid)
		if err != nil {
			log.Error(err)
			return "", err
		}
		return gid, nil
	}
	if count > 1 {
		log.Error(errors.New(ErrMultipleRocks))
	}

	err = db.QueryRowContext(ctx, "SELECT COUNT(gid) FROM agent WHERE LOWER(intelname) = LOWER(?)", agent).Scan(&count)
	if err != nil {
		log.Error(err)
		return "", err
	}
	if count == 1 {
		err := db.QueryRowContext(ctx, "SELECT gid FROM agent WHERE LOWER(intelname) = LOWER(?)", agent).Scan(&gid)
		if err != nil {
			log.Error(err)
			return "", err
		}
		return gid, nil
	}
	if count > 1 {
		log.Error(errors.New(ErrMultipleIntelname))
	}

	return "", nil
}

// Delete removes an agent and all associated data
func (gid GoogleID) Delete(ctx context.Context) error {
	var teamID TeamID
	rows, err := db.QueryContext(ctx, "SELECT teamID FROM team WHERE owner = ?", gid)
	if err != nil {
		log.Error(err)
		return err
	}
	defer rows.Close()

	for rows.Next() {
		if err = rows.Scan(&teamID); err != nil {
			log.Error(err)
			continue
		}
		if err = teamID.Delete(ctx); err != nil {
			log.Error(err)
			continue
		}
	}

	teamrows, err := db.QueryContext(ctx, "SELECT teamID FROM agentteams WHERE gid = ?", gid)
	if err != nil {
		log.Error(err)
		return err
	}
	defer teamrows.Close()
	for teamrows.Next() {
		if err := teamrows.Scan(&teamID); err != nil {
			log.Error(err)
			continue
		}
		_ = teamID.RemoveAgent(ctx, gid)
	}

	if _, err = db.ExecContext(ctx, "DELETE FROM agent WHERE gid = ?", gid); err != nil {
		log.Error(err)
		return err
	}

	_, _ = db.ExecContext(ctx, "DELETE FROM locations WHERE gid = ?", gid)
	_, _ = db.ExecContext(ctx, "DELETE FROM telegram WHERE gid = ?", gid)
	_, _ = db.ExecContext(ctx, "DELETE FROM firebase WHERE gid = ?", gid)
	_, _ = db.ExecContext(ctx, "DELETE FROM rocks WHERE gid = ?", gid)

	return nil
}

// Lock disables an account
func (gid GoogleID) Lock(ctx context.Context, reason string) error {
	log.Infow("RISC locking", "gid", gid, "reason", reason)
	if _, err := db.ExecContext(ctx, "UPDATE agent SET RISC = 1 WHERE gid = ?", gid); err != nil {
		log.Error(err)
		return err
	}
	return nil
}

// Unlock enables a disabled account
func (gid GoogleID) Unlock(ctx context.Context, reason string) error {
	log.Infow("RISC unlocking", "gid", gid, "reason", reason)
	if _, err := db.ExecContext(ctx, "UPDATE agent SET RISC = 0 WHERE gid = ?", gid); err != nil {
		log.Error(err)
		return err
	}
	return nil
}

// RISC checks the compromised flag
func (gid GoogleID) RISC(ctx context.Context) bool {
	var RISC bool
	err := db.QueryRowContext(ctx, "SELECT RISC FROM agent WHERE gid = ?", gid).Scan(&RISC)
	if errors.Is(err, sql.ErrNoRows) {
		log.Warnw("agent does not exist, checking RISC flag", "GID", gid)
	} else if err != nil {
		log.Error(err)
	}
	return RISC
}

// UpdatePicture sets/updates the agent's google picture URL
func (gid GoogleID) UpdatePicture(ctx context.Context, picurl string) error {
	if _, err := db.ExecContext(ctx, "UPDATE agent SET picurl = ? WHERE gid = ?", picurl, gid); err != nil {
		log.Error(err)
		return err
	}
	return nil
}

// GetPicture returns the agent's Google Picture URL
func (gid GoogleID) GetPicture(ctx context.Context) string {
	var url sql.NullString
	err := db.QueryRowContext(ctx, "SELECT picurl FROM agent WHERE gid = ?", gid).Scan(&url)
	if errors.Is(err, sql.ErrNoRows) || !url.Valid || url.String == "" {
		return config.Get().DefaultPictureURL
	}
	if err != nil {
		log.Error(err)
		return config.Get().DefaultPictureURL
	}
	return url.String
}

// ToGid takes a string and returns a Gid
func ToGid(ctx context.Context, in string) (GoogleID, error) {
	var gid GoogleID
	var err error

	switch len(in) {
	case 0:
		err = errors.New(ErrEmptyAgent)
	case 21:
		gid = GoogleID(in)
	default:
		gid, err = SearchAgentName(ctx, in)
	}
	if errors.Is(err, sql.ErrNoRows) || gid == "" {
		err = errors.New(ErrAgentNotFound)
		log.Infow(err.Error(), "search", in, "message", err.Error())
		return gid, err
	}
	if err != nil {
		log.Errorw(err.Error(), "search", in, "message", err.Error())
		return gid, err
	}
	return gid, nil
}

// SetIntelData sets the untrusted data from IITC
func (gid GoogleID) SetIntelData(ctx context.Context, name, faction string) error {
	name = util.Sanitize(name)
	if name == "" {
		return nil
	}

	ifac := FactionFromString(faction)
	_, err := db.ExecContext(ctx, "UPDATE agent SET intelname = LEFT(?, 15), intelfaction = ? WHERE GID = ?", name, ifac, gid)
	if err != nil {
		log.Error(err)
		return err
	}

	if ifac == FactionRes {
		log.Errorw("self identified as RES", "sent name", name, "GID", gid)
	}
	return nil
}

// IntelSmurf checks for self-proclaimed smurfs
func (gid GoogleID) IntelSmurf(ctx context.Context) bool {
	var ifac IntelFaction
	if err := db.QueryRowContext(ctx, "SELECT intelfaction FROM agent WHERE GID = ?", gid).Scan(&ifac); err != nil {
		log.Error(err)
		return false
	}
	return ifac == FactionRes
}

// FirstLogin sets the required database records for a new agent
func (gid GoogleID) FirstLogin(ctx context.Context) error {
	log.Infow("first login", "GID", gid, "message", "first login for "+gid)

	ott, err := GenerateSafeName(ctx)
	if err != nil {
		log.Error(err)
		return err
	}

	if _, err := db.ExecContext(ctx, "INSERT IGNORE INTO agent (gid, OneTimeToken) VALUES (?,?)", gid, ott); err != nil {
		log.Error(err)
		return err
	}

	if _, err = db.ExecContext(ctx, "INSERT IGNORE INTO locations (gid, upTime, loc) VALUES (?,UTC_TIMESTAMP(),POINT(0,0))", gid); err != nil {
		log.Error(err)
		return err
	}

	return nil
}

// GetAgentLocations is a fast-path to get all available agent locations
func (gid GoogleID) GetAgentLocations(ctx context.Context) ([]AgentLocation, error) {
	var list []AgentLocation
	var tmpL AgentLocation
	var lat, lon string

	rows, err := db.QueryContext(ctx, "SELECT x.gid, Y(l.loc), X(l.loc), l.upTime "+
		"FROM agentteams=x, locations=l "+
		"WHERE x.teamID IN (SELECT teamID FROM agentteams WHERE gid = ?) "+
		"AND x.shareLoc= 1 AND x.gid = l.gid", gid)
	if err != nil {
		log.Error(err)
		return list, err
	}

	defer rows.Close()
	for rows.Next() {
		if err := rows.Scan(&tmpL.Gid, &lat, &lon, &tmpL.Date); err != nil {
			log.Error(err)
			return list, err
		}
		tmpL.Lat, _ = strconv.ParseFloat(lat, 64)
		tmpL.Lon, _ = strconv.ParseFloat(lon, 64)

		if tmpL.Lat == 0 || tmpL.Lon == 0 {
			continue
		}

		list = append(list, tmpL)
	}
	return list, nil
}
