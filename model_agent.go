package wasabee

import (
	"database/sql"
	"fmt"
	"strconv"
)

// logoutlist is used by the RISC system
var logoutlist map[GoogleID]bool

// init is bad magic; need a proper constructor
func init() {
	logoutlist = make(map[GoogleID]bool)
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
	LocationKey   string
	VVerified     bool
	VBlacklisted  bool
	Vid           EnlID
	RocksVerified bool
	RAID          bool
	RISC          bool
	OwnedTeams    []AdOwnedTeam
	Teams         []AdTeam
	Ops           []AdOperation
	// OwnedOps is deprecated use Ops.IsOwner -- now empty, remove after client is updated
	OwnedOps []AdOperation
	Telegram struct {
		ID        int64
		Verified  bool
		Authtoken string
	}
	Assignments []Assignment
}

// AdOwnedTeam is a sub-struct of AgentData
type AdOwnedTeam struct {
	ID        string
	Name      string
	RocksComm string
	RocksKey  string
}

// AdTeam is a sub-struct of AgentData
type AdTeam struct {
	ID        string
	Name      string
	State     string
	RocksComm string
}

// AdOperation is a sub-struct of AgentData
type AdOperation struct {
	ID       string
	Name     string
	IsOwner  bool
	Color    string
	TeamName string
	TeamID   TeamID
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
	Type          string
}

// InitAgent is called from Oauth callback to set up a agent for the first time.
// It also checks and updates V and enl.rocks data. It returns true if the agent is authorized to continue, false if the agent is blacklisted or otherwise locked at V or enl.rocks.
func (gid GoogleID) InitAgent() (bool, error) {
	var authError error
	var tmpName string
	var err error
	var vdata Vresult
	var rocks RocksAgent

	// query both rocks and V at the same time
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
		Log.Notice(e1)
	}
	if e2 != nil {
		Log.Notice(e2)
	}

	if vdata.Data.Agent != "" {
		err = gid.VUpdate(&vdata)
		if err != nil {
			Log.Notice(err)
			return false, err
		}
		if tmpName == "" {
			tmpName = vdata.Data.Agent
		}
		if vdata.Data.Quarantine {
			authError = fmt.Errorf("%s %s quarantined at V", gid, vdata.Data.Agent)
			Log.Notice(authError)
		}
		if vdata.Data.Flagged {
			authError = fmt.Errorf("%s %s flagged at V", gid, vdata.Data.Agent)
			Log.Notice(authError)
		}
		if vdata.Data.Blacklisted {
			authError = fmt.Errorf("%s %s blacklisted at V", gid, vdata.Data.Agent)
			Log.Notice(authError)
		}
		if vdata.Data.Banned {
			authError = fmt.Errorf("%s %s banned at V", gid, vdata.Data.Agent)
			Log.Notice(authError)
		}
	}

	if rocks.Agent != "" {
		err = RocksUpdate(gid, &rocks)
		if err != nil {
			Log.Notice(err)
			return false, err
		}
		if tmpName == "" {
			tmpName = rocks.Agent
		}
		if rocks.Smurf {
			authError = fmt.Errorf("%s %s listed as a smurf at enl.rocks", gid, rocks.Agent)
			Log.Notice(authError)
		}
	}

	if authError != nil {
		return false, authError
	}

	if tmpName == "" {
		// use enlio only for agent name, only if .rocks and V fail
		tmpName, _ = gid.enlioQuery()
	}

	if tmpName == "" {
		// err := fmt.Errorf("gid %s not found at rocks or V", gid.String())
		// Log.Error(err)
		// return false, err
		tmpName = "UnverifiedAgent_" + gid.String()[:15]
	}

	// if the agent doesn't exist, prepopulate everything
	_, err = gid.IngressName()
	if err != nil && err == sql.ErrNoRows {
		lockey, err := GenerateSafeName()
		if err != nil {
			Log.Error(err)
			return false, err
		}
		_, err = db.Exec("INSERT IGNORE INTO agent (gid, iname, level, lockey, VVerified, VBlacklisted, Vid, RocksVerified, RAID, RISC) VALUES (?,?,?,?,?,?,?,?,?,0)",
			gid, MakeNullString(tmpName), vdata.Data.Level, lockey, vdata.Data.Verified, vdata.Data.Blacklisted, MakeNullString(vdata.Data.EnlID), rocks.Verified, 0)
		if err != nil {
			Log.Error(err)
			return false, err
		}
		_, err = db.Exec("INSERT IGNORE INTO locations (gid, upTime, loc) VALUES (?,NOW(),POINT(0,0))", gid)
		if err != nil {
			Log.Error(err)
			return false, err
		}
	} else if err != nil {
		Log.Error(err)
		return false, err
	}

	if gid.RISC() {
		err := fmt.Errorf("%s locked due to Google RISC", gid)
		Log.Error(err)
		return false, err
	}

	return true, nil
}

// Gid just satisfies the AgentID function
func (gid GoogleID) Gid() (GoogleID, error) {
	return gid, nil
}

// GetAgentData populates a AgentData struct based on the gid
func (gid GoogleID) GetAgentData(ud *AgentData) error {
	ud.GoogleID = gid

	var Vid sql.NullString
	err := db.QueryRow("SELECT u.iname, u.level, u.lockey, u.VVerified, u.VBlacklisted, u.Vid, u.RocksVerified, u.RAID, u.RISC FROM agent=u WHERE u.gid = ?", gid).Scan(&ud.IngressName, &ud.Level, &ud.LocationKey, &ud.VVerified, &ud.VBlacklisted, &Vid, &ud.RocksVerified, &ud.RAID, &ud.RISC)
	if err != nil && err == sql.ErrNoRows {
		// if you delete yourself and don't wait for your session cookie to expire to rejoin...
		err = fmt.Errorf("unknown GoogleID: [%s] try restarting your browser", gid)
		gid.Logout("broken cookie")
		return err
	}
	if err != nil {
		Log.Notice(err)
		return err
	}

	if Vid.Valid {
		ud.Vid = EnlID(Vid.String)
	}

	if err = gid.adTeams(ud); err != nil {
		Log.Error(err)
		return err
	}

	if err = gid.adOwnedTeams(ud); err != nil {
		Log.Error(err)
		return err
	}

	if err = gid.adTelegram(ud); err != nil {
		Log.Error(err)
		return err
	}

	if err = gid.adOps(ud); err != nil {
		Log.Error(err)
		return err
	}

	if err = gid.adOwnedOps(ud); err != nil {
		Log.Error(err)
		return err
	}

	if err = gid.adAssignments(ud); err != nil {
		Log.Error(err)
	}

	return nil
}

func (gid GoogleID) adTeams(ud *AgentData) error {
	rows, err := db.Query("SELECT t.teamID, t.name, x.state, t.rockscomm FROM team=t, agentteams=x WHERE x.gid = ? AND x.teamID = t.teamID ORDER BY t.name", gid)
	if err != nil {
		Log.Error(err)
		return err
	}
	defer rows.Close()
	var adteam AdTeam
	var rc sql.NullString
	for rows.Next() {
		err := rows.Scan(&adteam.ID, &adteam.Name, &adteam.State, &rc)
		if err != nil {
			Log.Error(err)
			return err
		}
		if rc.Valid {
			adteam.RocksComm = rc.String
		} else {
			adteam.RocksComm = ""
		}
		ud.Teams = append(ud.Teams, adteam)
	}
	return nil
}

func (gid GoogleID) adOwnedTeams(ud *AgentData) error {
	var ownedTeam AdOwnedTeam
	row, err := db.Query("SELECT teamID, name, rockscomm, rockskey FROM team WHERE owner = ? ORDER BY name", gid)
	if err != nil {
		Log.Error(err)
		return err
	}
	defer row.Close()
	var rc, rockskey sql.NullString
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
		ud.OwnedTeams = append(ud.OwnedTeams, ownedTeam)
	}
	return nil
}

func (gid GoogleID) adTelegram(ud *AgentData) error {
	var authtoken sql.NullString
	err := db.QueryRow("SELECT telegramID, verified, authtoken FROM telegram WHERE gid = ?", gid).Scan(&ud.Telegram.ID, &ud.Telegram.Verified, &authtoken)
	if err != nil && err == sql.ErrNoRows {
		ud.Telegram.ID = 0
		ud.Telegram.Verified = false
		ud.Telegram.Authtoken = ""
	} else if err != nil {
		Log.Error(err)
		return err
	}
	if authtoken.Valid {
		ud.Telegram.Authtoken = authtoken.String
	}
	return nil
}

func (gid GoogleID) adOps(ud *AgentData) error {
	var op AdOperation

	row2, err := db.Query("SELECT o.ID, o.Name, o.Color, t.Name, p.teamID FROM operation=o, team=t, agentteams=x, opteams=p WHERE p.opID = o.ID AND x.gid = ? AND x.teamID = p.teamID AND x.teamID = t.teamID AND x.state = 'On' ORDER BY o.Name, t.Name", gid)
	if err != nil {
		Log.Error(err)
		return err
	}
	defer row2.Close()
	for row2.Next() {
		err := row2.Scan(&op.ID, &op.Name, &op.Color, &op.TeamName, &op.TeamID)
		if err != nil {
			Log.Error(err)
			return err
		}
		ud.Ops = append(ud.Ops, op)
	}
	return nil
}

func (gid GoogleID) adOwnedOps(ud *AgentData) error {
	var op AdOperation

	row, err := db.Query("SELECT ID, Name, Color, 'owned', 'owned' FROM operation WHERE gid = ? ORDER BY Name", gid)
	if err != nil {
		Log.Error(err)
		return err
	}
	defer row.Close()
	for row.Next() {
		err := row.Scan(&op.ID, &op.Name, &op.Color, &op.TeamName, &op.TeamID)
		if err != nil {
			Log.Error(err)
			return err
		}
		op.IsOwner = true
		ud.Ops = append(ud.Ops, op)
	}
	return nil
}

func (gid GoogleID) adAssignments(ud *AgentData) error {
	var a Assignment

	a.Type = "Marker"
	row, err := db.Query("SELECT DISTINCT o.Name, o.ID FROM marker=m, operation=o WHERE m.gid = ? AND m.opID = o.ID ORDER BY o.Name", gid)
	if err != nil {
		Log.Error(err)
		return err
	}
	defer row.Close()
	for row.Next() {
		err := row.Scan(&a.OperationName, &a.OpID)
		if err != nil {
			Log.Error(err)
			return err
		}
		ud.Assignments = append(ud.Assignments, a)
	}

	a.Type = "Link"
	row2, err := db.Query("SELECT DISTINCT o.Name, o.ID FROM link=l, operation=o WHERE l.gid = ? AND l.opID = o.ID ORDER BY o.Name", gid)
	if err != nil {
		Log.Error(err)
		return err
	}
	defer row2.Close()
	for row2.Next() {
		err := row2.Scan(&a.OperationName, &a.OpID)
		if err != nil {
			Log.Error(err)
			return err
		}
		ud.Assignments = append(ud.Assignments, a)
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
		Log.Notice(err)
		flat = float64(0)
	}

	flon, err = strconv.ParseFloat(lon, 64)
	if err != nil {
		Log.Notice(err)
		flon = float64(0)
	}
	point := fmt.Sprintf("POINT(%s %s)", strconv.FormatFloat(flon, 'f', 7, 64), strconv.FormatFloat(flat, 'f', 7, 64))
	if _, err := db.Exec("UPDATE locations SET loc = PointFromText(?), upTime = NOW() WHERE gid = ?", point, gid); err != nil {
		Log.Notice(err)
		return err
	}

	gid.firebaseAgentLocation()
	return nil
}

// IngressName returns an agent's name for a given GoogleID.
// It returns err == sql.ErrNoRows if there is no such agent.
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
			Log.Notice(err)
		}
		if err = <-channel; err != nil {
			Log.Notice(err)
		}

		if err = gid.VUpdate(&v); err != nil {
			Log.Error(err)
		}

		if err = RocksUpdate(gid, &r); err != nil {
			Log.Error(err)
		}

		/* easy way to test the enl.io query
		rocksname, err := gid.enlioQuery()
		if err != nil {
			// Log.Error(err)
		} else {
			Log.Debugf("found %s for %s at enl.io", rocksname, gid)
		} */

	}
	return nil
}

// SearchAgentName gets a GoogleID from an Agent's name
func SearchAgentName(agent string) (GoogleID, error) {
	var gid GoogleID
	err := db.QueryRow("SELECT gid FROM agent WHERE LOWER(iname) LIKE LOWER(?)", agent).Scan(&gid)
	if err != nil && err != sql.ErrNoRows {
		Log.Notice(err)
		return "", err
	}
	return gid, nil
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
		Log.Notice(err)
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
	if gid == "" {
		err := fmt.Errorf("gid unset")
		Log.Error(err)
	}

	Log.Debugf("adding %s to logout list: %s", gid, reason)
	logoutlist[gid] = true
}

// CheckLogout looks to see if the user is on the force logout list
func (gid GoogleID) CheckLogout() bool {
	logout, ok := logoutlist[gid]
	if !ok { // not in the list
		return false
	}
	if logout {
		logoutlist[gid] = false
		Log.Debugf("clearing %s from logoutlist", gid)
		delete(logoutlist, gid)
	}
	return logout
}

// RISC checks to see if the user was marked as compromised by Google
func (gid GoogleID) RISC() bool {
	var RISC bool

	err := db.QueryRow("SELECT RISC FROM agent WHERE gid = ?", gid).Scan(&RISC)
	if err != nil {
		Log.Notice(err)
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
	var url string

	err := db.QueryRow("SELECT picurl FROM agentextras WHERE gid = ?", gid).Scan(&url)
	if err != nil {
		// Log.Info(err)
		wr, _ := GetWebroot()
		return fmt.Sprintf("%s/static/android-chrome-512x512.png", wr)
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
	if err == sql.ErrNoRows {
		err = fmt.Errorf("unknown agent: %s", in)
	}
	if err == nil && gid == "" {
		err = fmt.Errorf("unknown agent: %s", in)
	}
	if err != nil {
		Log.Info(err, in)
	}
	return gid, err
}
