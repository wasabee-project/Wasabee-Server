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
	GoogleID      GoogleID
	IngressName   string
	Level         int64
	LocationKey   LocKey
	VVerified     bool
	VBlacklisted  bool
	Vid           EnlID
	RocksVerified bool
	RAID          bool
	RISC          bool
	ProfileImage  string
	// XXX owned teams needs to go away, merge into teams
	OwnedTeams  []AdTeam
	Teams       []AdTeam
	Ops         []AdOperation
	Assignments []Assignment
	Telegram    struct {
		ID        int64
		Verified  bool
		Authtoken string
	}
}

// AdTeam is a sub-struct of AgentData
type AdTeam struct {
	ID        TeamID
	Name      string
	RocksComm string
	RocksKey  string
	State     string
	Owner     GoogleID
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

// Assignment is used for assigning targets
type Assignment struct {
	OpID          OperationID
	OperationName string
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

	if vdata.Data.Agent != "" {
		// if we got data, and the user already exists (not first login) update if necessary
		err = gid.VUpdate(&vdata)
		if err != nil {
			Log.Error(err)
			return false, err
		}
		if tmpName == "" {
			tmpName = vdata.Data.Agent
		}
		if vdata.Data.Quarantine {
			Log.Warnw("access denied", "GID", gid, "reason", "quarantined at V")
			Log.Debug(vdata.Data.Agent)
			authError = true
		}
		if vdata.Data.Flagged {
			Log.Warnw("access denied", "GID", gid, "reason", "flagged at V")
			Log.Debug(vdata.Data.Agent)
			authError = true
		}
		if vdata.Data.Blacklisted {
			Log.Warnw("access denied", "GID", gid, "reason", "blacklisted at V")
			Log.Debug(vdata.Data.Agent)
			authError = true
		}
		if vdata.Data.Banned {
			Log.Warnw("access denied", "GID", gid, "reason", "blacklisted at V")
			Log.Debug(vdata.Data.Agent)
			authError = true
		}
	}

	if rocks.Agent != "" {
		// if we got data, and the user already exists (not first login) update if necessary
		err = RocksUpdate(gid, &rocks)
		if err != nil {
			Log.Info(err)
			return false, err
		}
		if tmpName == "" {
			tmpName = rocks.Agent
		}
		if rocks.Smurf {
			Log.Warnw("access denied", "GID", gid, "reason", "listed as smurf at enl.rocks")
			Log.Debug(rocks.Agent)
			authError = true
		}
	}

	if authError {
		return false, fmt.Errorf("access denied")
	}

	// if the agent doesn't exist, prepopulate everything
	_, err = gid.IngressName()
	if err != nil && err == sql.ErrNoRows {
		Log.Infow("first login", "GID", gid.String(), "message", "first login for "+gid.String())

		// still no name? last resort
		if tmpName == "" {
			// triggered this in testing -- should never happen IRL
			length := 15
			if tmp := len(gid); tmp < length {
				length = tmp
			}
			tmpName = "UnverifiedAgent_" + gid.String()[:length]
			Log.Infow("using UnverifiedAgent name", "GID", gid.String(), "name", tmpName)
		}

		lockey, err := GenerateSafeName()
		if err != nil {
			Log.Error(err)
			return false, err
		}

		ad := AgentData{
			GoogleID:      gid,
			IngressName:   tmpName,
			LocationKey:   LocKey(lockey),
			Level:         vdata.Data.Level,
			VVerified:     vdata.Data.Verified,
			VBlacklisted:  vdata.Data.Blacklisted,
			Vid:           vdata.Data.EnlID,
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
		Log.Warnw(err.Error(), "GID", gid.String())
		return false, err
	}
	return true, nil
}

// Gid just satisfies the AgentID function
func (gid GoogleID) Gid() (GoogleID, error) {
	return gid, nil
}

// GetAgentData populates a AgentData struct based on the gid
func (gid GoogleID) GetAgentData(ad *AgentData) error {
	ad.GoogleID = gid
	var vid, lk, pic sql.NullString

	err := db.QueryRow("SELECT a.iname, a.level, a.lockey, a.VVerified, a.VBlacklisted, a.Vid, a.RocksVerified, a.RAID, a.RISC, e.picurl FROM agent=a, WHERE a.gid = ? LEFT JOIN agentextras=e ON a.gid = e.gid", gid).Scan(&ad.IngressName, &ad.Level, &lk, &ad.VVerified, &ad.VBlacklisted, &vid, &ad.RocksVerified, &ad.RAID, &ad.RISC, &pic)
	if err != nil && err == sql.ErrNoRows {
		err = fmt.Errorf("unknown GoogleID: %s", gid)
		return err
	}
	if err != nil {
		Log.Error(err)
		return err
	}

	if vid.Valid {
		ad.Vid = EnlID(vid.String)
	}

	if lk.Valid {
		ad.LocationKey = LocKey(lk.String)
	}

	if pic.Valid {
		ad.ProfileImage = pic.String
	}

	if err := gid.adOwnedTeams(ad); err != nil {
		return err
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

	if err = gid.adAssignments(ad); err != nil {
		return err
	}

	return nil
}

func (gid GoogleID) adTeams(ad *AgentData) error {
	rows, err := db.Query("SELECT t.teamID, t.name, x.state, t.rockscomm, t.rockskey, t.owner FROM team=t, agentteams=x WHERE x.gid = ? AND x.teamID = t.teamID ORDER BY t.name", gid)
	if err != nil {
		Log.Error(err)
		return err
	}
	defer rows.Close()

	var rc, rk sql.NullString
	var adteam AdTeam
	for rows.Next() {
		err := rows.Scan(&adteam.ID, &adteam.Name, &adteam.State, &rc, &rk, &adteam.Owner)
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
			adteam.RocksKey = rc.String
		} else {
			adteam.RocksKey = ""
		}
		ad.Teams = append(ad.Teams, adteam)
	}
	return nil
}

// deprecated - do not use
func (gid GoogleID) adOwnedTeams(ad *AgentData) error {
	row, err := db.Query("SELECT teamID, name, rockscomm, rockskey FROM team WHERE owner = ? ORDER BY name", gid)
	if err != nil {
		Log.Error(err)
		return err
	}
	defer row.Close()

	var rc, rockskey sql.NullString
	var ownedTeam AdTeam
	for row.Next() {
		err := row.Scan(&ownedTeam.ID, &ownedTeam.Name, &rc, &rockskey)
		if err != nil {
			Log.Error(err)
			return err
		}
		if rc.Valid {
			ownedTeam.RocksComm = rc.String
		} else {
			ownedTeam.RocksComm = ""
		}
		if rockskey.Valid {
			ownedTeam.RocksKey = rockskey.String
		} else {
			ownedTeam.RocksKey = ""
		}
		ownedTeam.State = "NA"
		ownedTeam.Owner = gid
		ad.OwnedTeams = append(ad.OwnedTeams, ownedTeam)
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

	rowTeam, err := db.Query("SELECT o.ID, o.Name, o.Color, p.teamID FROM operation=o, agentteams=x, opteams=p WHERE p.opID = o.ID AND x.gid = ? AND x.teamID = p.teamID AND x.state = 'On' ORDER BY o.Name", gid)
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
func (gid GoogleID) adAssignments(ad *AgentData) error {
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
}

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
	var iname string
	err := db.QueryRow("SELECT iname FROM agent WHERE gid = ?", gid).Scan(&iname)
	return iname, err
}

// IngressNameTeam returns the display name for an agent on a particular team, or the IngressName if not set
func (gid GoogleID) IngressNameTeam(teamID TeamID) (string, error) {
	var displayname sql.NullString
	err := db.QueryRow("SELECT displayname FROM agentteams WHERE teamID = ? AND gid = ?", teamID, gid).Scan(&displayname)
	if (err != nil && err == sql.ErrNoRows) || !displayname.Valid {
		return gid.IngressName()
	}
	if err != nil {
		Log.Error(err)
		return "", err
	}

	return displayname.String, nil
}

// IngressNameOperation returns an agent's display name on a given operation
// if the agent is on multiple teams associated with the op, the first one found is used -- this is non-deterministic
func (gid GoogleID) IngressNameOperation(o *Operation) (string, error) {
	var iname string

	err := o.PopulateTeams()
	if err != nil {
		Log.Error(err)
		return "", err
	}

	for _, t := range o.Teams {
		iname, err := gid.IngressNameTeam(t.TeamID)
		if err != nil && err != sql.ErrNoRows {
			Log.Error(err)
			// keep looking
		}
		if iname != "" {
			break
		}
	}

	return iname, nil
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
	defer rows.Close()

	var gid GoogleID
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

// SearchAgentName gets a GoogleID from an Agent's name
func SearchAgentName(agent string) (GoogleID, error) {
	var gid GoogleID

	// if it starts with an @ and not the placeholder, search tg
	if agent[0] == '@' && !strings.EqualFold("@unused", agent) {
		err := db.QueryRow("SELECT gid FROM telegram WHERE LOWER(telegramName) LIKE LOWER(?)", agent[1:]).Scan(&gid)
		if err != nil && err != sql.ErrNoRows {
			Log.Error(err)
			return "", err
		}
		if gid != "" {
			return gid, nil
		}
	}

	err := db.QueryRow("SELECT gid FROM agent WHERE LOWER(iname) LIKE LOWER(?)", agent).Scan(&gid)
	if err != nil && err != sql.ErrNoRows {
		Log.Error(err)
		return "", err
	}
	return gid, nil

	// XXX where else can we search by name? Rocks? V?
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
		Log.Debugw("clearing from logoutlist", "GID", gid)
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
	if _, err := db.Exec("REPLACE INTO agentextras (gid, picurl) VALUES (?,?) ", gid, picurl); err != nil {
		Log.Error(err)
		return err
	}
	return nil
}

// GetPicture returns the agent's Google Picture URL
func (gid GoogleID) GetPicture() string {
	// XXX don't hardcode this
	url := "https://cdn2.wasabee.rocks/android-chrome-512x512.png"

	err := db.QueryRow("SELECT picurl FROM agentextras WHERE gid = ?", gid).Scan(&url)
	if err == sql.ErrNoRows {
		// nothing
	} else if err != nil {
		Log.Error(err)
	}

	return url
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
		gid, err = SearchAgentName(in)
	}
	if err == sql.ErrNoRows || gid == "" {
		err = fmt.Errorf("agent [%s] not registered with this wasabee server", in)
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
	_, err := db.Exec("INSERT INTO agent (gid, iname, level, lockey, VVerified, VBlacklisted, Vid, RocksVerified, RAID, RISC) VALUES (?,?,?,?,?,?,?,?,?,0) ON DUPLICATE KEY UPDATE iname = ?, level = ?, VVerified = ?, VBlacklisted = ?, Vid = ?, RocksVerified = ?, RAID = ?, RISC = ?",
		ad.GoogleID, MakeNullString(ad.IngressName), ad.Level, MakeNullString(ad.LocationKey), ad.VVerified, ad.VBlacklisted, MakeNullString(ad.Vid), ad.RocksVerified, ad.RAID,
		MakeNullString(ad.IngressName), ad.Level, ad.VVerified, ad.VBlacklisted, MakeNullString(ad.Vid), ad.RocksVerified, ad.RAID, ad.RISC)
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

// OneTimeToken attempts to verify a submitted OTT and updates it if valid
func OneTimeToken(token LocKey) (GoogleID, error) {
	var gid GoogleID

	err := db.QueryRow("SELECT gid FROM agent WHERE LocKey = ?", token).Scan(&gid)
	if err != nil {
		Log.Error(err)
		return "", err
	}

	_, err = gid.NewLocKey()
	if err != nil {
		Log.Warn(err)
	}
	return gid, nil
}

// SetAgentName updates an agent's name -- used only for test scripts presently
func (gid GoogleID) SetAgentName(newname string) error {
	_, err := db.Exec("UPDATE agent SET iname = ? WHERE gid = ?", newname, gid)

	if err != nil {
		Log.Error(err)
		return err
	}
	return err
}
