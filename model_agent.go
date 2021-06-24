package wasabee

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"sync"
)

type logoutList struct {
	logoutlist map[GoogleID]bool
	mux        sync.Mutex
}

var ll logoutList

// init is bad magic; need a proper constructor
func init() {
	ll.logoutlist = make(map[GoogleID]bool)
}

// GoogleID is the primary location for interfacing with the agent type
type GoogleID string

// TeamID is the primary means for interfacing with teams
type TeamID string

// EnlID is a V EnlID
type EnlID string

// AgentData is the complete agent struct, used for the /me page.
type AgentData struct {
	GoogleID      GoogleID     `json:"GoogleID"`
	IngressName   string       `json:"name"`
	VName         string       `json:"vname"`
	RocksName     string       `json:"rocksname"`
	IntelName     string       `json:"intelname"`
	Level         int64        `json:"level"`
	OneTimeToken  OneTimeToken `json:"lockey"` // historical name, is this used by any clients?
	VVerified     bool         `json:"Vverified"`
	VBlacklisted  bool         `json:"blacklisted"`
	EnlID         EnlID        `json:"enlid"`
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

// AdTeam is a sub-struct of AgentData
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

// AdOperation is a sub-struct of AgentData
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

// InitAgent is called from Oauth callback to set up a agent for the first time or revalidate them on subsequent logins.
// It also updates the local V and enl.rocks data, if configured.
// Returns true if the agent is authorized to continue, false if the agent is blacklisted or otherwise locked.
func (gid GoogleID) InitAgent() (bool, error) {
	var authError bool
	var tmpName string
	var err error
	var vdata Vresult
	var rocks RocksAgent

	// If the ENL APIs are not configured/enabled
	// ask the pub/sub federation if anyone else knows about the agent
	// the data will be updated in the background if/when anyone responds
	if !GetvEnlOne() || !GetEnlRocks() {
		gid.PSRequest()
	}

	// query both rocks and V at the same time -- returns quickly if the API is not enabled
	channel := make(chan error, 2)
	go func() {
		channel <- VSearch(gid, &vdata)
	}()
	go func() {
		channel <- RocksSearch(gid, &rocks)
	}()
	defer close(channel)

	// would be better to start processing when either returned rather than waiting for both to be done, still better than serial calls
	e1, e2 := <-channel, <-channel
	if e1 != nil {
		Log.Error(e1)
	}
	if e2 != nil {
		Log.Error(e2)
	}

	// rocks agent names are less trustworthy than V, let V overwrite
	if rocks.Agent != "" {
		// if we got data, and the user already exists (not first login) update if necessary
		err = RocksUpdate(gid, &rocks)
		if err != nil {
			Log.Info(err)
			return false, err
		}
		tmpName = rocks.Agent
		if rocks.Smurf {
			Log.Warnw("access denied", "GID", gid, "reason", "listed as smurf at enl.rocks")
			authError = true
		}
	}

	if vdata.Data.Agent != "" {
		// if we got data, and the user already exists (not first login) update if necessary
		err = gid.VUpdate(&vdata)
		if err != nil {
			Log.Error(err)
			return false, err
		}
		// overwrite what we got from rocks
		tmpName = vdata.Data.Agent
		if vdata.Data.Quarantine {
			Log.Warnw("access denied", "GID", gid, "reason", "quarantined at V")
			authError = true
		}
		if vdata.Data.Flagged {
			Log.Warnw("access denied", "GID", gid, "reason", "flagged at V")
			authError = true
		}
		if vdata.Data.Blacklisted || vdata.Data.Banned {
			Log.Warnw("access denied", "GID", gid, "reason", "blacklisted at V")
			authError = true
		}
	}

	if authError {
		return false, fmt.Errorf("access denied")
	}

	// if the agent doesn't exist, prepopulate everything
	name, err := gid.IngressName()
	if err != nil && err == sql.ErrNoRows {
		Log.Infow("first login", "GID", gid.String(), "message", "first login for "+gid.String())

		if tmpName == "" {
			// triggered this in testing -- should never happen IRL
			length := 15
			if tmp := len(gid); tmp < length {
				length = tmp
			}
			tmpName = "UnverifiedAgent_" + gid.String()[:length]
			Log.Infow("using UnverifiedAgent name", "GID", gid.String(), "name", tmpName)
		}

		ott, err := GenerateSafeName()
		if err != nil {
			Log.Error(err)
			return false, err
		}

		ad := AgentData{
			GoogleID:      gid,
			IngressName:   tmpName,
			OneTimeToken:  OneTimeToken(ott),
			Level:         vdata.Data.Level,
			VVerified:     vdata.Data.Verified,
			VBlacklisted:  vdata.Data.Blacklisted,
			EnlID:         vdata.Data.EnlID,
			RocksVerified: rocks.Verified,
		}

		if err := ad.Save(); err != nil {
			Log.Error(err)
			return false, err
		}
	} else if err != nil {
		Log.Error(err)
		return false, err
	}

	if gid.RISC() {
		err := fmt.Errorf("account locked by Google RISC")
		Log.Warnw(err.Error(), "GID", gid.String(), "name", name)
		return false, err
	}

	if gid.IntelSmurf() {
		err := fmt.Errorf("intel account self-identified as RES")
		Log.Warnw(err.Error(), "GID", gid.String(), "name", name)
		return false, err
	}

	if tmpName != "" && strings.HasPrefix(name, "UnverifiedAgent_") {
		Log.Infow("updating agent name", "GID", gid.String(), "name", name, "new", tmpName)
		if err := gid.SetAgentName(tmpName); err != nil {
			Log.Warnw(err.Error(), "GID", gid.String(), "name", name, "new", tmpName)
			return true, nil
		}
	}

	return true, nil
}

// Gid just satisfies the AgentID interface
func (gid GoogleID) Gid() (GoogleID, error) {
	return gid, nil
}

// GetAgentData populates a AgentData struct based on the gid
func (gid GoogleID) GetAgentData(ad *AgentData) error {
	ad.GoogleID = gid
	var vid, pic, vapi sql.NullString
	var ifac IntelFaction

	err := db.QueryRow("SELECT a.name, a.Vname, a.Rocksname, a.intelname, a.level, a.OneTimeToken, a.VVerified, a.VBlacklisted, a.Vid, a.RocksVerified, a.RAID, a.RISC, a.intelfaction, e.picurl, e.VAPIkey FROM agent=a LEFT JOIN agentextras=e ON a.gid = e.gid WHERE a.gid = ?", gid).Scan(&ad.IngressName, &ad.VName, &ad.RocksName, &ad.IntelName, &ad.Level, &ad.OneTimeToken, &ad.VVerified, &ad.VBlacklisted, &vid, &ad.RocksVerified, &ad.RAID, &ad.RISC, &ifac, &pic, &vapi)
	if err != nil && err == sql.ErrNoRows {
		err = fmt.Errorf("unknown GoogleID: %s", gid)
		return err
	}
	if err != nil {
		Log.Error(err)
		return err
	}

	if vid.Valid {
		ad.EnlID = EnlID(vid.String)
	}

	if pic.Valid {
		ad.ProfileImage = pic.String
	}

	if err = gid.adTeams(ad); err != nil {
		return err
	}

	if err = gid.adTelegram(ad); err != nil {
		return err
	}

	if err = gid.adOps(ad); err != nil {
		return err
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

	return nil
}

func (gid GoogleID) adTeams(ad *AgentData) error {
	rows, err := db.Query("SELECT t.teamID, t.name, x.state, x.shareWD, x.loadWD, t.rockscomm, t.rockskey, t.owner, t.joinLinkToken, t.vteam, t.vrole FROM team=t, agentteams=x WHERE x.gid = ? AND x.teamID = t.teamID ORDER BY t.name", gid)
	if err != nil {
		Log.Error(err)
		return err
	}

	var rc, rk, jlt sql.NullString
	var adteam AdTeam
	defer rows.Close()
	for rows.Next() {
		err := rows.Scan(&adteam.ID, &adteam.Name, &adteam.State, &adteam.ShareWD, &adteam.LoadWD, &rc, &rk, &adteam.Owner, &jlt, &adteam.VTeam, &adteam.VTeamRole)
		if err != nil {
			Log.Error(err)
			return err
		}
		if rc.Valid {
			adteam.RocksComm = rc.String
		} else {
			adteam.RocksComm = ""
		}
		if rk.Valid && adteam.Owner == gid {
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

func (gid GoogleID) adTelegram(ad *AgentData) error {
	var authtoken sql.NullString
	err := db.QueryRow("SELECT telegramID, verified, authtoken FROM telegram WHERE gid = ?", gid).Scan(&ad.Telegram.ID, &ad.Telegram.Verified, &authtoken)
	if err != nil && err == sql.ErrNoRows {
		ad.Telegram.ID = 0
		ad.Telegram.Verified = false
		ad.Telegram.Authtoken = ""
	} else if err != nil {
		Log.Error(err)
		return err
	}
	if authtoken.Valid {
		ad.Telegram.Authtoken = authtoken.String
	}
	return nil
}

func (gid GoogleID) adOps(ad *AgentData) error {
	seen := make(map[OperationID]bool)

	rowOwned, err := db.Query("SELECT ID, Name, Color FROM operation WHERE gid = ? ORDER BY Name", gid)
	if err != nil {
		Log.Error(err)
		return err
	}
	defer rowOwned.Close()
	for rowOwned.Next() {
		var op AdOperation
		err := rowOwned.Scan(&op.ID, &op.Name, &op.Color)
		if err != nil {
			Log.Error(err)
			return err
		}
		op.IsOwner = true
		if seen[op.ID] {
			continue
		}
		ad.Ops = append(ad.Ops, op)
		seen[op.ID] = true
	}

	rowTeam, err := db.Query("SELECT o.ID, o.Name, o.Color, p.teamID FROM operation=o, agentteams=x, opteams=p WHERE p.opID = o.ID AND x.gid = ? AND x.teamID = p.teamID ORDER BY o.Name", gid)
	if err != nil {
		Log.Error(err)
		return err
	}
	defer rowTeam.Close()
	for rowTeam.Next() {
		var op AdOperation
		err := rowTeam.Scan(&op.ID, &op.Name, &op.Color, &op.TeamID)
		if err != nil {
			Log.Error(err)
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
/* func (gid GoogleID) adAssignments(ad *AgentData) error {
	assignMap := make(map[OperationID]string)

	var opID OperationID
	var opName string

	row, err := db.Query("SELECT DISTINCT o.Name, o.ID FROM marker=m, operation=o WHERE m.gid = ? AND m.opID = o.ID ORDER BY o.Name", gid)
	if err != nil {
		Log.Error(err)
		return err
	}
	defer row.Close()
	for row.Next() {
		err := row.Scan(&opName, &opID)
		if err != nil {
			Log.Error(err)
			return err
		}
		assignMap[opID] = opName
	}

	row2, err := db.Query("SELECT DISTINCT o.Name, o.ID FROM link=l, operation=o WHERE l.gid = ? AND l.opID = o.ID ORDER BY o.Name", gid)
	if err != nil {
		Log.Error(err)
		return err
	}
	defer row2.Close()
	for row2.Next() {
		err := row2.Scan(&opName, &opID)
		if err != nil {
			Log.Error(err)
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
		Log.Error(err)
		flat = float64(0)
	}

	flon, err = strconv.ParseFloat(lon, 64)
	if err != nil {
		Log.Error(err)
		flon = float64(0)
	}
	point := fmt.Sprintf("POINT(%s %s)", strconv.FormatFloat(flon, 'f', 7, 64), strconv.FormatFloat(flat, 'f', 7, 64))
	if _, err := db.Exec("UPDATE locations SET loc = PointFromText(?), upTime = UTC_TIMESTAMP() WHERE gid = ?", point, gid); err != nil {
		Log.Error(err)
		return err
	}

	gid.firebaseAgentLocation()
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

func (eid EnlID) String() string {
	return string(eid)
}

// RevalidateEveryone -- if the schema changes or another reason causes us to need to pull data from V and rocks, this is a function which does that
// V had bulk API functions we should use instead. This is good enough, and I hope we don't need it again.
func RevalidateEveryone() error {
	channel := make(chan error, 2)
	defer close(channel)

	rows, err := db.Query("SELECT gid FROM agent")
	if err != nil {
		Log.Error(err)
		return err
	}

	var gid GoogleID
	defer rows.Close()
	for rows.Next() {
		if err = rows.Scan(&gid); err != nil {
			Log.Error(err)
			continue
		}

		var v Vresult
		var r RocksAgent

		go func() {
			channel <- VSearch(gid, &v)
		}()
		go func() {
			channel <- RocksSearch(gid, &r)
		}()
		if err = <-channel; err != nil {
			Log.Error(err)
		}
		if err = <-channel; err != nil {
			Log.Error(err)
		}

		if err = gid.VUpdate(&v); err != nil {
			Log.Error(err)
		}

		if err = RocksUpdate(gid, &r); err != nil {
			Log.Error(err)
		}
	}
	return nil
}

// SearchAgentName gets a GoogleID from an Agent's name, searching local name, V name (if known), Rocks name (if known) and telegram name (if known)
// returns "" on no match
func SearchAgentName(agent string) (GoogleID, error) {
	var gid GoogleID

	// if it starts with an @ search tg
	if agent[0] == '@' {
		err := db.QueryRow("SELECT gid FROM telegram WHERE LOWER(telegramName) LIKE LOWER(?)", agent[1:]).Scan(&gid)
		if err != nil && err != sql.ErrNoRows {
			Log.Error(err)
			return "", err
		}
		if gid != "" {
			return gid, nil
		}
	}

	// agent.name has a unique key
	err := db.QueryRow("SELECT gid FROM agent WHERE LOWER(name) LIKE LOWER(?)", agent).Scan(&gid)
	if err != nil && err != sql.ErrNoRows {
		Log.Error(err)
		return "", err
	}
	if gid != "" { // found a match
		return gid, nil
	}

	// Vname does NOT have a unique key
	var count int
	err = db.QueryRow("SELECT COUNT(gid) FROM agent WHERE LOWER(Vname) LIKE LOWER(?)", agent).Scan(&count)
	if err != nil {
		Log.Error(err)
		return "", err
	}
	if count == 1 {
		err := db.QueryRow("SELECT gid FROM agent WHERE LOWER(Vname) LIKE LOWER(?)", gid).Scan(&gid)
		if err != nil {
			Log.Error(err)
			return "", err
		}
		return gid, nil
	}
	if count > 1 {
		err := fmt.Errorf("multiple V matches found, not using V results")
		Log.Error(err)
	}

	// rocks does NOT have a unique key
	err = db.QueryRow("SELECT COUNT(gid) FROM agent WHERE LOWER(rocksname) LIKE LOWER(?)", agent).Scan(&count)
	if err != nil {
		Log.Error(err)
		return "", err
	}
	if count == 1 {
		err := db.QueryRow("SELECT gid FROM agent WHERE LOWER(rocks) LIKE LOWER(?)", gid).Scan(&gid)
		if err != nil {
			Log.Error(err)
			return "", err
		}
		return gid, nil
	}
	if count > 1 {
		err := fmt.Errorf("multiple rocks matches found, not using V results")
		Log.Error(err)
	}

	// intelname does NOT have a unique key
	err = db.QueryRow("SELECT COUNT(gid) FROM agent WHERE LOWER(intelname) LIKE LOWER(?)", agent).Scan(&count)
	if err != nil {
		Log.Error(err)
		return "", err
	}
	if count == 1 {
		err := db.QueryRow("SELECT gid FROM agent WHERE LOWER(intelname) LIKE LOWER(?)", gid).Scan(&gid)
		if err != nil {
			Log.Error(err)
			return "", err
		}
		return gid, nil
	}
	if count > 1 {
		err := fmt.Errorf("multiple intelname matches found, not using V results")
		Log.Error(err)
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
		Log.Error(err)
		return err
	}
	defer rows.Close()
	for rows.Next() {
		err = rows.Scan(&teamID)
		if err != nil {
			Log.Error(err)
			continue
		}
		err = teamID.Delete()
		if err != nil {
			Log.Error(err)
			continue
		}
	}

	teamrows, err := db.Query("SELECT teamID FROM agentteams WHERE gid = ?", gid)
	if err != nil {
		Log.Error(err)
		return err
	}
	defer teamrows.Close()
	for teamrows.Next() {
		err := teamrows.Scan(&teamID)
		if err != nil {
			Log.Error(err)
			continue
		}
		_ = teamID.RemoveAgent(gid)
	}

	// brute force delete everyhing else
	_, err = db.Exec("DELETE FROM agent WHERE gid = ?", gid)
	if err != nil {
		Log.Error(err)
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
		Log.Error(err)
		return err
	}

	if _, err := db.Exec("UPDATE agent SET RISC = 1 WHERE gid = ?", gid); err != nil {
		Log.Error(err)
		return err
	}
	return nil
}

// Unlock enables a disabled account -- called by RISC system
func (gid GoogleID) Unlock(reason string) error {
	if gid == "" {
		err := fmt.Errorf("gid unset")
		Log.Error(err)
		return err
	}

	if _, err := db.Exec("UPDATE agent SET RISC = 0 WHERE gid = ?", gid); err != nil {
		Log.Error(err)
		return err
	}
	return nil
}

// Logout sets a temporary logout token - not stored in DB since logout cases are not critical
// and sessions are refreshed with google hourly
func (gid GoogleID) Logout(reason string) {
	Log.Infow("logout", "GID", gid.String(), "reason", reason, "message", gid.String()+" logout")
	ll.mux.Lock()
	defer ll.mux.Unlock()
	ll.logoutlist[gid] = true
}

// CheckLogout looks to see if the user is on the force logout list
func (gid GoogleID) CheckLogout() bool {
	ll.mux.Lock()
	defer ll.mux.Unlock()
	logout, ok := ll.logoutlist[gid]
	if !ok { // not in the list
		return false
	}
	if logout {
		ll.logoutlist[gid] = false
		// Log.Debugw("clearing from logoutlist", "GID", gid)
		delete(ll.logoutlist, gid)
	}
	return logout
}

// RISC checks to see if the user was marked as compromised by Google
func (gid GoogleID) RISC() bool {
	var RISC bool

	err := db.QueryRow("SELECT RISC FROM agent WHERE gid = ?", gid).Scan(&RISC)
	if err == sql.ErrNoRows {
		Log.Warnw("agent does not exist, checking RISC flag", "GID", gid)
	} else if err != nil {
		Log.Error(err)
	}
	return RISC
}

// UpdatePicture sets/updates the agent's google picture URL
func (gid GoogleID) UpdatePicture(picurl string) error {
	if _, err := db.Exec("INSERT INTO agentextras (gid, picurl) VALUES (?,?) ON DUPLICATE KEY UPDATE picurl = ?", gid, picurl, picurl); err != nil {
		Log.Error(err)
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
		Log.Error(err)
		return unset
	}

	return url.String
}

// ToGid takes a string and returns a Gid for it -- for reasonable values of a string; it must look like (GoogleID, EnlID) otherwise it defaults to agent name
func ToGid(in string) (GoogleID, error) {
	var gid GoogleID
	var err error

	switch len(in) {
	case 0:
		err = fmt.Errorf("empty agent request")
	case 40:
		gid, err = EnlID(in).Gid()
	case 21:
		gid = GoogleID(in)
	default:
		gid, err = SearchAgentName(in) // telegram @names covered here
	}
	if err == sql.ErrNoRows || gid == "" {
		// if you change this message, also change http/team.go
		err = fmt.Errorf("agent '%s' not registered with this wasabee server", in)
		Log.Infow(err.Error(), "search", in, "message", err.Error())
		return gid, err
	}
	if err != nil {
		Log.Errorw(err.Error(), "search", in, "message", err.Error())
		return gid, err
	}
	return gid, nil
}

// Save is called by InitAgent and from the Pub/Sub system to write a new agent
// also updates an existing agent from Pub/Sub
func (ad AgentData) Save() error {
	_, err := db.Exec("INSERT INTO agent (gid, name, vname, rocksname, level, OneTimeToken, VVerified, VBlacklisted, Vid, RocksVerified, RAID)  VALUES (?,?,?,?,?,?,?,?,?,?,?) ON DUPLICATE KEY UPDATE name = ?, vname = ?, rocksname = ? , level = ?, VVerified = ?, VBlacklisted = ?, Vid = ?, RocksVerified = ?, RAID = ?, RISC = ?",
		ad.GoogleID, ad.IngressName, ad.VName, ad.RocksName, ad.Level, ad.OneTimeToken, ad.VVerified, ad.VBlacklisted, MakeNullString(ad.EnlID), ad.RocksVerified, ad.RAID,
		ad.IngressName, ad.VName, ad.RocksName, ad.Level, ad.VVerified, ad.VBlacklisted, MakeNullString(ad.EnlID), ad.RocksVerified, ad.RAID, ad.RISC)
	if err != nil {
		Log.Error(err)
		return err
	}

	if _, err = db.Exec("INSERT IGNORE INTO locations (gid, upTime, loc) VALUES (?,UTC_TIMESTAMP(),POINT(0,0))", ad.GoogleID); err != nil {
		Log.Error(err)
		return err
	}

	if ad.Telegram.ID != 0 {
		if _, err := db.Exec("INSERT IGNORE INTO telegram (telegramID, gid, verified) VALUES (?, ?, ?)", ad.Telegram.ID, ad.GoogleID, ad.Telegram.Verified); err != nil {
			Log.Error(err)
			return err
		}
	}
	return nil
}

// SetAgentName updates an agent's name -- used only for test scripts presently
func (gid GoogleID) SetAgentName(newname string) error {
	_, err := db.Exec("UPDATE agent SET name = ? WHERE gid = ?", newname, gid)

	if err != nil {
		Log.Error(err)
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

	Log.Debugw("updating inteldata in db", "gid", gid, "name", name, "faction", ifac)

	_, err := db.Exec("UPDATE agent SET intelname = ?, intelfaction = ? WHERE GID = ?", name, ifac, gid)
	if err != nil {
		Log.Error(err)
		return err
	}

	if ifac == factionRes {
		Log.Errorw("self identified as RES", "sent name", name, "GID", gid)
		gid.Logout("self identified as RES")
	}
	return nil
}

// IntelSmurf checks to see if the agent has self-proclaimed to be a smurf (unset is OK)
func (gid GoogleID) IntelSmurf() bool {
	var ifac IntelFaction

	err := db.QueryRow("SELECT intelfaction FROM agent WHERE GID = ?", gid).Scan(&ifac)
	if err != nil {
		Log.Error(err)
		return false
	}
	if ifac == factionRes {
		return true
	}
	return false
}

// VAPIkey (gid GoogleID) loads an agents's V API key (this should be unusual); "" is "not set"
func (gid GoogleID) VAPIkey() (string, error) {
	var v sql.NullString

	err := db.QueryRow("SELECT VAPIkey FROM agentextras WHERE GID = ?", gid).Scan(&v)
	if err != nil {
		Log.Error(err)
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
		Log.Error(err)
		return err
	}
	return nil
}
