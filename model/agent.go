package model

import (
	"database/sql"
	"fmt"
	"strconv"

	"github.com/wasabee-project/Wasabee-Server/Firebase"
	"github.com/wasabee-project/Wasabee-Server/log"
)

// GoogleID is the primary location for interfacing with the agent type
type GoogleID string

// Agent is the complete agent struct, used for the /me page.
type Agent struct {
	GoogleID      GoogleID     `json:"GoogleID"`
	IngressName   string       `json:"name"`
	VName         string       `json:"vname"`
	RocksName     string       `json:"rocksname"`
	IntelName     string       `json:"intelname"`
	Level         int64        `json:"level"`
	OneTimeToken  OneTimeToken `json:"lockey"` // historical name, is this used by any clients?
	VVerified     bool         `json:"Vverified"`
	VBlacklisted  bool         `json:"blacklisted"`
	EnlID         string       `json:"enlid"`
	RocksVerified bool         `json:"rocks"`
	RAID          bool         `json:"RAID"`
	RISC          bool         `json:"RISC"`
	ProfileImage  string       `json:"pic"`
	Teams         []AdTeam
	Ops           []AdOperation
	Telegram      struct {
		ID        int64
		Verified  bool
		Authtoken string
	}
	IntelFaction string `json:"intelfaction"`
	QueryToken   string `json:"querytoken"`
	VAPIkey      string `json:"vapi"`
}

// AdTeam is a sub-struct of Agent
type AdTeam struct {
	ID            TeamID
	Name          string
	RocksComm     string
	RocksKey      string
	JoinLinkToken string
	State         string
	ShareWD       string
	LoadWD        string
	Owner         GoogleID
	VTeam         int64
	VTeamRole     uint8
}

// AdOperation is a sub-struct of Agent
type AdOperation struct {
	ID      OperationID
	Name    string
	IsOwner bool
	Color   string
	TeamID  TeamID
}

// AgentID is anything that can be converted to a GoogleID or a string
type AgentID interface {
	Gid() (GoogleID, error)
	fmt.Stringer
}

// Gid just satisfies the AgentID interface
func (gid GoogleID) Gid() (GoogleID, error) {
	return gid, nil
}

// GetAgent populates an Agent struct based on the gid
func (gid GoogleID) GetAgent() (Agent, error) {
	var a Agent
	a.GoogleID = gid
	var vid, pic, vapi sql.NullString
	var ifac IntelFaction

	ad := &a

	err := db.QueryRow("SELECT a.name, a.Vname, a.Rocksname, a.intelname, a.level, a.OneTimeToken, a.VVerified, a.VBlacklisted, a.Vid, a.RocksVerified, a.RAID, a.RISC, a.intelfaction, e.picurl, e.VAPIkey FROM agent=a LEFT JOIN agentextras=e ON a.gid = e.gid WHERE a.gid = ?", gid).Scan(&ad.IngressName, &ad.VName, &ad.RocksName, &ad.IntelName, &ad.Level, &ad.OneTimeToken, &ad.VVerified, &ad.VBlacklisted, &vid, &ad.RocksVerified, &ad.RAID, &ad.RISC, &ifac, &pic, &vapi)
	if err != nil && err == sql.ErrNoRows {
		err = fmt.Errorf("unknown GoogleID: %s", gid)
		return a, err
	}
	if err != nil {
		log.Error(err)
		return a, err
	}

	if vid.Valid {
		ad.EnlID = vid.String
	}

	if pic.Valid {
		ad.ProfileImage = pic.String
	}

	if err = adTeams(ad); err != nil {
		return a, err
	}

	if err = adTelegram(ad); err != nil {
		return a, err
	}

	if err = adOps(ad); err != nil {
		return a, err
	}

	ad.IntelFaction = ifac.String()

	if vapi.Valid {
		// if the user set a short string... don't panic
		len := len(vapi.String)
		if len > 6 {
			len = 6
		}
		ad.VAPIkey = vapi.String[0:len] + "..." // never show the full thing
	}

	return a, nil
}

func adTeams(ad *Agent) error {
	rows, err := db.Query("SELECT t.teamID, t.name, x.state, x.shareWD, x.loadWD, t.rockscomm, t.rockskey, t.owner, t.joinLinkToken, t.vteam, t.vrole FROM team=t, agentteams=x WHERE x.gid = ? AND x.teamID = t.teamID ORDER BY t.name", ad.GoogleID)
	if err != nil {
		log.Error(err)
		return err
	}

	var rc, rk, jlt sql.NullString
	var adteam AdTeam
	defer rows.Close()
	for rows.Next() {
		err := rows.Scan(&adteam.ID, &adteam.Name, &adteam.State, &adteam.ShareWD, &adteam.LoadWD, &rc, &rk, &adteam.Owner, &jlt, &adteam.VTeam, &adteam.VTeamRole)
		if err != nil {
			log.Error(err)
			return err
		}
		if rc.Valid {
			adteam.RocksComm = rc.String
		} else {
			adteam.RocksComm = ""
		}
		if rk.Valid && adteam.Owner == ad.GoogleID {
			// only share RocksKey with owner
			adteam.RocksKey = rk.String
		} else {
			adteam.RocksKey = ""
		}
		if jlt.Valid {
			adteam.JoinLinkToken = jlt.String
		} else {
			adteam.JoinLinkToken = ""
		}
		ad.Teams = append(ad.Teams, adteam)
	}
	return nil
}

func adTelegram(ad *Agent) error {
	var authtoken sql.NullString
	err := db.QueryRow("SELECT telegramID, verified, authtoken FROM telegram WHERE gid = ?", ad.GoogleID).Scan(&ad.Telegram.ID, &ad.Telegram.Verified, &authtoken)
	if err != nil && err == sql.ErrNoRows {
		ad.Telegram.ID = 0
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

	rowOwned, err := db.Query("SELECT ID, Name, Color FROM operation WHERE gid = ? ORDER BY Name", ad.GoogleID)
	if err != nil {
		log.Error(err)
		return err
	}
	defer rowOwned.Close()
	for rowOwned.Next() {
		var op AdOperation
		err := rowOwned.Scan(&op.ID, &op.Name, &op.Color)
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

	rowTeam, err := db.Query("SELECT o.ID, o.Name, o.Color, p.teamID FROM operation=o, agentteams=x, opteams=p WHERE p.opID = o.ID AND x.gid = ? AND x.teamID = p.teamID ORDER BY o.Name", ad.GoogleID)
	if err != nil {
		log.Error(err)
		return err
	}
	defer rowTeam.Close()
	for rowTeam.Next() {
		var op AdOperation
		err := rowTeam.Scan(&op.ID, &op.Name, &op.Color, &op.TeamID)
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

// adAssignments lists operations in which one has assignments
/* func (gid GoogleID) adAssignments(ad *Agent) error {
	assignMap := make(map[OperationID]string)

	var opID OperationID
	var opName string

	row, err := db.Query("SELECT DISTINCT o.Name, o.ID FROM marker=m, operation=o WHERE m.gid = ? AND m.opID = o.ID ORDER BY o.Name", gid)
	if err != nil {
		log.Error(err)
		return err
	}
	defer row.Close()
	for row.Next() {
		err := row.Scan(&opName, &opID)
		if err != nil {
			log.Error(err)
			return err
		}
		assignMap[opID] = opName
	}

	row2, err := db.Query("SELECT DISTINCT o.Name, o.ID FROM link=l, operation=o WHERE l.gid = ? AND l.opID = o.ID ORDER BY o.Name", gid)
	if err != nil {
		log.Error(err)
		return err
	}
	defer row2.Close()
	for row2.Next() {
		err := row2.Scan(&opName, &opID)
		if err != nil {
			log.Error(err)
			return err
		}
		assignMap[opID] = opName
	}

	for k, v := range assignMap {
		ad.Assignments = append(ad.Assignments, Assignment{
			OpID:          k,
			OperationName: v,
		})
	}
	return nil
} */

// AgentLocation updates the database to reflect a agent's current location
func (gid GoogleID) AgentLocation(lat, lon string) error {
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

	// announce to teams with which this agent is sharing location information
	for teamID := range gid.teamListEnabled() {
		wfb.AgentLocation(wfb.TeamID(teamID))
	}
	return nil
}

// IngressName returns an agent's name for a given GoogleID.
// returns err == sql.ErrNoRows if there is no such agent.
func (gid GoogleID) IngressName() (string, error) {
	var name string // , rocksname, vname, intelname string
	// err := db.QueryRow("SELECT name, rocksname, vname, intelname FROM agent WHERE gid = ?", gid).Scan(&name, &rocksname, &vname, &intelname)
	err := db.QueryRow("SELECT name FROM agent WHERE gid = ?", gid).Scan(&name)
	return name, err
}

func (gid GoogleID) String() string {
	return string(gid)
}

// SearchAgentName gets a GoogleID from an Agent's name, searching local name, V name (if known), Rocks name (if known) and telegram name (if known)
// returns "" on no match
func SearchAgentName(agent string) (GoogleID, error) {
	var gid GoogleID

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

	// agent.name has a unique key
	err := db.QueryRow("SELECT gid FROM agent WHERE LOWER(name) = LOWER(?)", agent).Scan(&gid)
	if err != nil && err != sql.ErrNoRows {
		log.Error(err)
		return "", err
	}
	if gid != "" { // found a match
		return gid, nil
	}

	// Vname does NOT have a unique key
	var count int
	err = db.QueryRow("SELECT COUNT(gid) FROM agent WHERE LOWER(Vname) = LOWER(?)", agent).Scan(&count)
	if err != nil {
		log.Error(err)
		return "", err
	}
	if count == 1 {
		err := db.QueryRow("SELECT gid FROM agent WHERE LOWER(Vname) = LOWER(?)", agent).Scan(&gid)
		if err != nil {
			log.Error(err)
			return "", err
		}
		return gid, nil
	}
	if count > 1 {
		err := fmt.Errorf("multiple V matches found, not using V results")
		log.Error(err)
	}

	// rocks does NOT have a unique key
	err = db.QueryRow("SELECT COUNT(gid) FROM agent WHERE LOWER(rocksname) = LOWER(?)", agent).Scan(&count)
	if err != nil {
		log.Error(err)
		return "", err
	}
	if count == 1 {
		err := db.QueryRow("SELECT gid FROM agent WHERE LOWER(rocks) = LOWER(?)", agent).Scan(&gid)
		if err != nil {
			log.Error(err)
			return "", err
		}
		return gid, nil
	}
	if count > 1 {
		err := fmt.Errorf("multiple rocks matches found, not using V results")
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
		err := fmt.Errorf("multiple intelname matches found, not using V results")
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
	_, _ = db.Exec("DELETE FROM agentextras WHERE gid = ?", gid)
	_, _ = db.Exec("DELETE FROM firebase WHERE gid = ?", gid)

	return nil
}

// Lock disables an account -- called by RISC system
func (gid GoogleID) Lock(reason string) error {
	if gid == "" {
		err := fmt.Errorf("gid unset")
		log.Error(err)
		return err
	}

	if _, err := db.Exec("UPDATE agent SET RISC = 1 WHERE gid = ?", gid); err != nil {
		log.Error(err)
		return err
	}
	return nil
}

// Unlock enables a disabled account -- called by RISC system
func (gid GoogleID) Unlock(reason string) error {
	if gid == "" {
		err := fmt.Errorf("gid unset")
		log.Error(err)
		return err
	}

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
	if _, err := db.Exec("INSERT INTO agentextras (gid, picurl) VALUES (?,?) ON DUPLICATE KEY UPDATE picurl = ?", gid, picurl, picurl); err != nil {
		log.Error(err)
		return err
	}
	return nil
}

// GetPicture returns the agent's Google Picture URL
func (gid GoogleID) GetPicture() string {
	// XXX don't hardcode this
	unset := "https://cdn2.wasabee.rocks/android-chrome-512x512.png"
	var url sql.NullString

	err := db.QueryRow("SELECT picurl FROM agentextras WHERE gid = ?", gid).Scan(&url)
	if err == sql.ErrNoRows || !url.Valid || url.String == "" {
		return unset
	}
	if err != nil {
		log.Error(err)
		return unset
	}

	return url.String
}

// ToGid takes a string and returns a Gid for it -- for reasonable values of a string; it must look like a GoogleID otherwise it defaults to agent name
func ToGid(in string) (GoogleID, error) {
	var gid GoogleID
	var err error

	switch len(in) {
	case 0:
		err = fmt.Errorf("empty agent request")
	case 21:
		gid = GoogleID(in)
	default:
		gid, err = SearchAgentName(in) // telegram @names covered here
	}
	if err == sql.ErrNoRows || gid == "" {
		// if you change this message, also change http/team.go
		err = fmt.Errorf("agent '%s' not registered with this wasabee server", in)
		log.Infow(err.Error(), "search", in, "message", err.Error())
		return gid, err
	}
	if err != nil {
		log.Errorw(err.Error(), "search", in, "message", err.Error())
		return gid, err
	}
	return gid, nil
}

// Save is called by InitAgent and from the Pub/Sub system to write a new agent
// also updates an existing agent from Pub/Sub
func (ad Agent) Save() error {
	_, err := db.Exec("INSERT INTO agent (gid, name, vname, rocksname, level, OneTimeToken, VVerified, VBlacklisted, Vid, RocksVerified, RAID)  VALUES (?,?,?,?,?,?,?,?,?,?,?) ON DUPLICATE KEY UPDATE name = ?, vname = ?, rocksname = ? , level = ?, VVerified = ?, VBlacklisted = ?, Vid = ?, RocksVerified = ?, RAID = ?, RISC = ?",
		ad.GoogleID, ad.IngressName, ad.VName, ad.RocksName, ad.Level, ad.OneTimeToken, ad.VVerified, ad.VBlacklisted, MakeNullString(ad.EnlID), ad.RocksVerified, ad.RAID,
		ad.IngressName, ad.VName, ad.RocksName, ad.Level, ad.VVerified, ad.VBlacklisted, MakeNullString(ad.EnlID), ad.RocksVerified, ad.RAID, ad.RISC)
	if err != nil {
		log.Error(err)
		return err
	}

	if _, err = db.Exec("INSERT IGNORE INTO locations (gid, upTime, loc) VALUES (?,UTC_TIMESTAMP(),POINT(0,0))", ad.GoogleID); err != nil {
		log.Error(err)
		return err
	}

	if ad.Telegram.ID != 0 {
		if _, err := db.Exec("INSERT IGNORE INTO telegram (telegramID, gid, verified) VALUES (?, ?, ?)", ad.Telegram.ID, ad.GoogleID, ad.Telegram.Verified); err != nil {
			log.Error(err)
			return err
		}
	}
	return nil
}

// SetAgentName updates an agent's name -- used only for test scripts presently
func (gid GoogleID) SetAgentName(newname string) error {
	_, err := db.Exec("UPDATE agent SET name = ? WHERE gid = ?", newname, gid)

	if err != nil {
		log.Error(err)
		return err
	}
	return err
}

// Stores the untrusted data from IITC - do not depend on these values for authorization
// but if someone says they are a smurf, who are we to ignore their self-identity?
func (gid GoogleID) SetIntelData(name, faction string) error {
	if name == "" {
		return nil
	}

	ifac := FactionFromString(faction)

	_, err := db.Exec("UPDATE agent SET intelname = ?, intelfaction = ? WHERE GID = ?", name, ifac, gid)
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

	err := db.QueryRow("SELECT intelfaction FROM agent WHERE GID = ?", gid).Scan(&ifac)
	if err != nil {
		log.Error(err)
		return false
	}
	if ifac == factionRes {
		return true
	}
	return false
}

// VAPIkey (gid GoogleID) loads an agents's V API key (this should be unusual); "" is "not set"
func (gid GoogleID) GetVAPIkey() (string, error) {
	var v sql.NullString

	err := db.QueryRow("SELECT VAPIkey FROM agentextras WHERE GID = ?", gid).Scan(&v)
	if err != nil {
		log.Error(err)
		return "", nil
	}
	if !v.Valid {
		return "", nil
	}
	return v.String, nil
}

// SetVAPIkey stores
func (gid GoogleID) SetVAPIkey(key string) error {
	if _, err := db.Exec("INSERT INTO agentextras (gid, VAPIkey) VALUES (?,?) ON DUPLICATE KEY UPDATE VAPIkey = ? ", gid, key, key); err != nil {
		log.Error(err)
		return err
	}
	return nil
}