package model

import (
	"database/sql"
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
	OneTimeToken OneTimeToken `json:"lockey,omitempty"` // historical name, is this used by any clients?
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
	Gid() (GoogleID, error)
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
func (gid GoogleID) Gid() (GoogleID, error) {
	return gid, nil
}

// GetAgent populates an Agent struct based on the gid
func (gid GoogleID) GetAgent() (*Agent, error) {
	var a Agent
	a.GoogleID = gid
	var level, vname, pic, intelname, rocksname sql.NullString
	var rocksverified sql.NullBool
	var ifac IntelFaction

	err := db.QueryRow("SELECT v.agent AS Vname, rocks.agent AS Rocksname, a.intelname, a.OneTimeToken, rocks.verified AS RockVerified, a.RISC, a.intelfaction, a.picurl FROM agent=a LEFT JOIN rocks ON a.gid = rocks.gid LEFT JOIN v ON a.gid = v.gid WHERE a.gid = ?", gid).Scan(&vname, &rocksname, &intelname, &a.OneTimeToken, &rocksverified, &a.RISC, &ifac, &pic)
	if err != nil && err == sql.ErrNoRows {
		err = fmt.Errorf(ErrUnknownGID)
		return &a, err
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

	if err = adTeams(&a); err != nil {
		return &a, err
	}

	if err = adTelegram(&a); err != nil {
		return &a, err
	}

	if err = adOps(&a); err != nil {
		return &a, err
	}

	a.IntelFaction = ifac.String()

	return &a, nil
}

func adTeams(ad *Agent) error {
	rows, err := db.Query("SELECT x.teamID, team.name, x.shareLoc, x.shareWD, x.loadWD, team.rockscomm, team.rockskey, team.owner, team.joinLinkToken FROM agentteams=x JOIN team ON x.teamID = team.teamID WHERE x.gid = ?", ad.GoogleID)
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
			// only share RocksKey with owner
			team.RocksKey = rk.String
		}

		if jlt.Valid {
			team.JoinLinkToken = jlt.String
		}

		if shareLoc {
			team.ShareLoc = "On"
		} else {
			team.ShareLoc = "Off"
		}

		if shareWD {
			team.ShareWD = "On"
		} else {
			team.ShareWD = "Off"
		}

		if loadWD {
			team.LoadWD = "On"
		} else {
			team.LoadWD = "Off"
		}

		ad.Teams = append(ad.Teams, team)
	}
	return nil
}

func adTelegram(ad *Agent) error {
	var authtoken sql.NullString
	err := db.QueryRow("SELECT telegramID, telegramName, verified, authtoken FROM telegram WHERE gid = ?", ad.GoogleID).Scan(&ad.Telegram.ID, &ad.Telegram.Name, &ad.Telegram.Verified, &authtoken)
	if err != nil && err == sql.ErrNoRows {
		ad.Telegram.ID = 0
		ad.Telegram.Name = ""
		ad.Telegram.Verified = false
		ad.Telegram.Authtoken = ""
	} else if err != nil {
		log.Error(err)
		return err
	}
	if authtoken.Valid {
		ad.Telegram.Authtoken = authtoken.String
	}
	return nil
}

func adOps(ad *Agent) error {
	seen := make(map[OperationID]bool)

	rowOwned, err := db.Query("SELECT ID, Name, Color, modified, lasteditid FROM operation WHERE gid = ?", ad.GoogleID)
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

	rowTeam, err := db.Query("SELECT operation.ID, operation.Name, operation.Color, permissions.teamID, operation.modified, operation.lasteditid FROM agentteams JOIN permissions ON agentteams.teamID = permissions.teamID JOIN operation ON permissions.opID = operation.ID WHERE agentteams.gid = ?", ad.GoogleID)
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

// SetLocation updates the database to reflect a agent's current location
func (gid GoogleID) SetLocation(lat, lon string) error {
	if lat == "" || lon == "" {
		return nil
	}

	// convert to float64 and back to reduce the garbage input
	var flat, flon float64

	flat, err := strconv.ParseFloat(lat, 64)
	if err != nil {
		log.Error(err)
		flat = float64(0)
	}

	flon, err = strconv.ParseFloat(lon, 64)
	if err != nil {
		log.Error(err)
		flon = float64(0)
	}

	point := fmt.Sprintf("POINT(%s %s)", strconv.FormatFloat(flon, 'f', 7, 64), strconv.FormatFloat(flat, 'f', 7, 64))
	if _, err := db.Exec("UPDATE locations SET loc = PointFromText(?), upTime = UTC_TIMESTAMP() WHERE gid = ?", point, gid); err != nil {
		log.Error(err)
		return err
	}

	return nil
}

// IngressName returns an agent's name for a given GoogleID.
// returns err == sql.ErrNoRows if there is no such agent.
func (gid GoogleID) IngressName() (string, error) {
	var intelname, rocksname, vname, communityname sql.NullString
	err := db.QueryRow("SELECT rocks.agent, v.agent, agent.intelname, agent.communityname FROM agent LEFT JOIN rocks ON agent.gid = rocks.gid LEFT JOIN v ON agent.gid = v.gid WHERE agent.gid = ?", gid).Scan(&rocksname, &vname, &intelname, &communityname)

	if err != nil && err == sql.ErrNoRows {
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
func IngressName(g messaging.GoogleID) string {
	name, _ := GoogleID(string(g)).IngressName()
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
func (gid GoogleID) Valid() bool {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM agent WHERE gid = ?", gid).Scan(&count)
	if err != nil {
		log.Error(err)
		return false
	}
	return count == 1
}

func (gid GoogleID) String() string {
	return string(gid)
}

// SearchAgentName gets a GoogleID from an Agent's name, searching local name, V name (if known), Rocks name (if known) and telegram name (if known)
// returns "" on no match
func SearchAgentName(agent string) (GoogleID, error) {
	var gid GoogleID
	var count int

	// if it starts with an @ search tg
	if agent[0] == '@' {
		err := db.QueryRow("SELECT gid FROM telegram WHERE LOWER(telegramName) = LOWER(?)", agent[1:]).Scan(&gid)
		if err != nil && err != sql.ErrNoRows {
			log.Error(err)
			return "", err
		}
		if gid != "" {
			return gid, nil
		}
	}

	err := db.QueryRow("SELECT gid FROM agent WHERE LOWER(communityname) = LOWER(?)", agent).Scan(&gid)
	if err != nil && err != sql.ErrNoRows {
		log.Error(err)
		return "", err
	}
	if err != sql.ErrNoRows && gid != "" {
		return gid, nil
	}

	// v.agent does NOT have a unique key
	err = db.QueryRow("SELECT COUNT(gid) FROM v WHERE LOWER(agent) = LOWER(?)", agent).Scan(&count)
	if err != nil {
		log.Error(err)
		return "", err
	}
	if count == 1 {
		err := db.QueryRow("SELECT gid FROM v WHERE LOWER(agent) = LOWER(?)", agent).Scan(&gid)
		if err != nil {
			log.Error(err)
			return "", err
		}
		return gid, nil
	}
	if count > 1 {
		err := fmt.Errorf(ErrMultipleV)
		log.Error(err)
	}

	// rocks.agent does NOT have a unique key
	err = db.QueryRow("SELECT COUNT(gid) FROM rocks WHERE LOWER(agent) = LOWER(?)", agent).Scan(&count)

	if err != nil {
		log.Error(err)
		return "", err
	}
	if count == 1 {
		err := db.QueryRow("SELECT gid FROM rocks WHERE LOWER(agent) = LOWER(?)", agent).Scan(&gid)
		if err != nil {
			log.Error(err)
			return "", err
		}
		return gid, nil
	}
	if count > 1 {
		err := fmt.Errorf(ErrMultipleRocks)
		log.Error(err)
	}

	// intelname does NOT have a unique key
	err = db.QueryRow("SELECT COUNT(gid) FROM agent WHERE LOWER(intelname) = LOWER(?)", agent).Scan(&count)
	if err != nil {
		log.Error(err)
		return "", err
	}
	if count == 1 {
		err := db.QueryRow("SELECT gid FROM agent WHERE LOWER(intelname) = LOWER(?)", agent).Scan(&gid)
		if err != nil {
			log.Error(err)
			return "", err
		}
		return gid, nil
	}
	if count > 1 {
		err := fmt.Errorf(ErrMultipleIntelname)
		log.Error(err)
	}

	// no match found, return ""
	return "", nil
}

// Delete removes an agent and all associated data
func (gid GoogleID) Delete() error {
	// teams require special attention since they might be linked to .rocks communities
	var teamID TeamID
	rows, err := db.Query("SELECT teamID FROM team WHERE owner = ?", gid)
	if err != nil {
		log.Error(err)
		return err
	}
	defer rows.Close()

	for rows.Next() {
		err = rows.Scan(&teamID)
		if err != nil {
			log.Error(err)
			continue
		}
		err = teamID.Delete()
		if err != nil {
			log.Error(err)
			continue
		}
	}

	teamrows, err := db.Query("SELECT teamID FROM agentteams WHERE gid = ?", gid)
	if err != nil {
		log.Error(err)
		return err
	}
	defer teamrows.Close()
	for teamrows.Next() {
		err := teamrows.Scan(&teamID)
		if err != nil {
			log.Error(err)
			continue
		}
		_ = teamID.RemoveAgent(gid)
	}

	// brute force delete everyhing else
	_, err = db.Exec("DELETE FROM agent WHERE gid = ?", gid)
	if err != nil {
		log.Error(err)
		return err
	}

	// the foreign key constraints should take care of these, but just in case...
	_, _ = db.Exec("DELETE FROM locations WHERE gid = ?", gid)
	_, _ = db.Exec("DELETE FROM telegram WHERE gid = ?", gid)
	_, _ = db.Exec("DELETE FROM firebase WHERE gid = ?", gid)
	_, _ = db.Exec("DELETE FROM v WHERE gid = ?", gid)
	_, _ = db.Exec("DELETE FROM rocks WHERE gid = ?", gid)

	return nil
}

// Lock disables an account -- called by RISC system
func (gid GoogleID) Lock(reason string) error {
	log.Infow("RISC locking", "gid", gid, "reason", reason)
	if _, err := db.Exec("UPDATE agent SET RISC = 1 WHERE gid = ?", gid); err != nil {
		log.Error(err)
		return err
	}
	return nil
}

// Unlock enables a disabled account -- called by RISC system
func (gid GoogleID) Unlock(reason string) error {
	log.Infow("RISC unlocking", "gid", gid, "reason", reason)
	if _, err := db.Exec("UPDATE agent SET RISC = 0 WHERE gid = ?", gid); err != nil {
		log.Error(err)
		return err
	}
	return nil
}

// RISC checks to see if the user was marked as compromised by Google
func (gid GoogleID) RISC() bool {
	var RISC bool

	err := db.QueryRow("SELECT RISC FROM agent WHERE gid = ?", gid).Scan(&RISC)
	if err == sql.ErrNoRows {
		log.Warnw("agent does not exist, checking RISC flag", "GID", gid)
	} else if err != nil {
		log.Error(err)
	}
	return RISC
}

// UpdatePicture sets/updates the agent's google picture URL
func (gid GoogleID) UpdatePicture(picurl string) error {
	if _, err := db.Exec("UPDATE agent SET picurl = ? WHERE gid = ?", picurl, gid); err != nil {
		log.Error(err)
		return err
	}
	return nil
}

// GetPicture returns the agent's Google Picture URL
func (gid GoogleID) GetPicture() string {
	var url sql.NullString

	err := db.QueryRow("SELECT picurl FROM agent WHERE gid = ?", gid).Scan(&url)
	if err == sql.ErrNoRows || !url.Valid || url.String == "" {
		return config.Get().DefaultPictureURL
	}
	if err != nil {
		log.Error(err)
		return config.Get().DefaultPictureURL
	}

	return url.String
}

// ToGid takes a string and returns a Gid for it -- for reasonable values of a string; it must look like a GoogleID otherwise it defaults to agent name
func ToGid(in string) (GoogleID, error) {
	var gid GoogleID
	var err error

	switch len(in) {
	case 0:
		err = fmt.Errorf(ErrEmptyAgent)
	case 21:
		gid = GoogleID(in)
	default:
		gid, err = SearchAgentName(in) // telegram @names covered here
	}
	if err == sql.ErrNoRows || gid == "" {
		err = fmt.Errorf(ErrAgentNotFound)
		log.Infow(err.Error(), "search", in, "message", err.Error())
		return gid, err
	}
	if err != nil {
		log.Errorw(err.Error(), "search", in, "message", err.Error())
		return gid, err
	}
	return gid, nil
}

// SetIntelData sets the untrusted data from IITC - do not depend on these values for authorization
// but if someone says they are a smurf, who are we to deny their self-identity?
func (gid GoogleID) SetIntelData(name, faction string) error {
	name = util.Sanitize(name) // don't trust this too much

	if name == "" {
		return nil
	}

	if len(name) > 15 {
		log.Infow("intel name too long", "gid", gid, "name", name)
	}

	ifac := FactionFromString(faction)

	_, err := db.Exec("UPDATE agent SET intelname = LEFT(?, 15), intelfaction = ? WHERE GID = ?", name, ifac, gid)
	if err != nil {
		log.Error(err)
		return err
	}

	if ifac == factionRes {
		log.Errorw("self identified as RES", "sent name", name, "GID", gid)
	}
	return nil
}

// IntelSmurf checks to see if the agent has self-proclaimed to be a smurf (unset is OK)
func (gid GoogleID) IntelSmurf() bool {
	var ifac IntelFaction

	if err := db.QueryRow("SELECT intelfaction FROM agent WHERE GID = ?", gid).Scan(&ifac); err != nil {
		log.Error(err)
		return false
	}
	if ifac == factionRes {
		return true
	}
	return false
}

// FirstLogin sets the required database records for a new agent
func (gid GoogleID) FirstLogin() error {
	log.Infow("first login", "GID", gid, "message", "first login for "+gid)

	ott, err := GenerateSafeName()
	if err != nil {
		log.Error(err)
		return err
	}

	if _, err := db.Exec("INSERT IGNORE INTO agent (gid, OneTimeToken) VALUES (?,?)", gid, ott); err != nil {
		log.Error(err)
		return err
	}

	if _, err = db.Exec("INSERT IGNORE INTO locations (gid, upTime, loc) VALUES (?,UTC_TIMESTAMP(),POINT(0,0))", gid); err != nil {
		log.Error(err)
		return err
	}

	return nil
}

// GetAgentLocations is a fast-path to get all available agent locations
func (gid GoogleID) GetAgentLocations() ([]AgentLocation, error) {
	var list []AgentLocation
	var tmpL AgentLocation
	var lat, lon string

	var rows *sql.Rows
	rows, err := db.Query("SELECT x.gid, Y(l.loc), X(l.loc), l.upTime "+
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

// SetCommunityName sets the name the agent is known as on the Niantic Community -- this is the most trustworthy source of agent identity
func (gid GoogleID) SetCommunityName(name string) error {
	if name == "" {
		return gid.ClearCommunityName()
	}

	if len(name) > 15 {
		log.Infow("community name too long", "gid", gid, "name", name)
	}

	if _, err := db.Exec("UPDATE agent SET communityname = LEFT(?,15) WHERE gid = ?", name, gid); err != nil {
		log.Error(err)
		return err
	}
	return nil
}

// CommunityNameToGID takes a community name and returns a GoogleID
func CommunityNameToGID(name string) (GoogleID, error) {
	var gid GoogleID

	err := db.QueryRow("SELECT gid FROM agent WHERE communityname = ?", name).Scan(&gid)
	if err != nil && err != sql.ErrNoRows {
		log.Error(err)
		return "", err
	}
	return gid, nil
}

// ClearCommunityName removes an agent's community name verification
func (gid GoogleID) ClearCommunityName() error {
	if _, err := db.Exec("UPDATE agent SET communityname = NULL WHERE gid = ?", gid); err != nil {
		log.Error(err)
		return err
	}
	return nil
}
