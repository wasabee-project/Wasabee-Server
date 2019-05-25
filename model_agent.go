package wasabi

import (
	"database/sql"
	"fmt"
)

// logoutlist is used by the RISC system
var logoutlist map[GoogleID]bool

func init() {
	logoutlist = make(map[GoogleID]bool)
}

// GoogleID is the primary location for interfacing with the agent type
type GoogleID string

// TeamID is the primary means for interfacing with teams
type TeamID string

// LocKey is the location share key
type LocKey string

// EnlID is a V EnlID
type EnlID string

// AgentData is the complete agent struct, used for the /me page.
type AgentData struct {
	GoogleID      GoogleID
	IngressName   string
	Level         int64
	LocationKey   string
	OwnTracksPW   string
	VVerified     bool
	VBlacklisted  bool
	Vid           EnlID
	OwnTracksJSON string
	RocksVerified bool
	RAID          bool
	RISC          bool
	OwnedTeams    []AdOwnedTeam
	Teams         []AdTeam
	Ops           []AdOperation
	OwnedOps      []AdOperation
	Telegram      struct {
		UserName  string
		ID        int64
		Verified  bool
		Authtoken string
	}
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
	Color    string
	TeamName string
}

// AgentID is anything that can be converted to a GoogleID or a string
type AgentID interface {
	Gid() (GoogleID, error)
	fmt.Stringer
}

// InitAgent is called from Oauth callback to set up a agent for the first time.
// It also checks and updates V and enl.rocks data. It returns true if the agent is authorized to continue, false if the agent is blacklisted or otherwise locked at V or enl.rocks.
func (gid GoogleID) InitAgent() (bool, error) {
	var authError error // delay reporting authorization problems until after INSERT/Vupdate/RocksUpdate
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
			authError = fmt.Errorf("%s quarantined at V", vdata.Data.Agent)
		}
		if vdata.Data.Flagged {
			authError = fmt.Errorf("%s flagged at V", vdata.Data.Agent)
		}
		if vdata.Data.Blacklisted {
			authError = fmt.Errorf("%s blacklisted at V", vdata.Data.Agent)
		}
		if vdata.Data.Banned {
			authError = fmt.Errorf("%s banned at V", vdata.Data.Agent)
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
			authError = fmt.Errorf("%s listed as a smurf at enl.rocks", rocks.Agent)
		}
	}

	// if the agent doesn't exist, prepopulate everything
	_, err = gid.IngressName()
	if err != nil && err == sql.ErrNoRows {
		if tmpName == "" {
			tmpName = "UnverifiedAgent_" + gid.String()[:15]
		}
		lockey, err := GenerateSafeName()
		if err != nil {
			Log.Error(err)
			return false, err
		}
		_, err = db.Exec("INSERT IGNORE INTO agent (gid, iname, level, lockey, OTpassword, VVerified, VBlacklisted, Vid, RocksVerified, RAID, RISC) VALUES (?,?,?,?,NULL,?,?,?,?,?,0)",
			gid, tmpName, vdata.Data.Level, lockey, vdata.Data.Verified, vdata.Data.Blacklisted, vdata.Data.EnlID, rocks.Verified, 0)
		if err != nil {
			Log.Error(err)
			return false, err
		}
		_, err = db.Exec("INSERT IGNORE INTO locations (gid, upTime, loc) VALUES (?,NOW(),POINT(0,0))", gid)
		if err != nil {
			Log.Error(err)
			return false, err
		}
		_, err = db.Exec("INSERT IGNORE INTO otdata (gid, otdata) VALUES (?,'{ }')", gid)
		if err != nil {
			Log.Error(err)
			return false, err
		}
		_ = gid.ownTracksExternalUpdate("0", "0", "reaper")
	} else if err != nil {
		Log.Error(err)
		return false, err
	}

	if authError != nil || gid.RISC() {
		Log.Notice(authError)
		return false, authError
	}
	return true, nil
}

// SetIngressName is called to update the agent's ingress name in the database.
// The ingress name cannot be updated if V or Rocks verification has taken place.
// ZZZ Do we even want to allow this any longer since V and rocks are giving us data? Unverified agents can just live with the Agent_XXXXXX name.
func (gid GoogleID) SetIngressName(name string) error {
	// if VVerified or RocksVerified: ignore name changes -- let the V/Rocks functions take care of that
	// XXX doesn't take care of the case where they are in .rocks but not verified
	_, err := db.Exec("UPDATE agent SET iname = ? WHERE gid = ? AND VVerified = 0 AND RocksVerified = 0", name, gid)
	if err != nil {
		Log.Notice(err)
	}
	return err
}

// Gid converts a location share key to a agent's gid
func (lockey LocKey) Gid() (GoogleID, error) {
	var gid GoogleID

	err := db.QueryRow("SELECT gid FROM agent WHERE lockey = ?", lockey).Scan(&gid)
	if err != nil {
		Log.Notice(err)
		return "", err
	}

	return gid, nil
}

// Gid just satisifes the AgentID funciton
func (gid GoogleID) Gid() (GoogleID, error) {
	return gid, nil
}

// GetAgentData populates a AgentData struct based on the gid
func (gid GoogleID) GetAgentData(ud *AgentData) error {
	ud.GoogleID = gid

	var ot sql.NullString
	err := db.QueryRow("SELECT u.iname, u.level, u.lockey, u.OTpassword, u.VVerified, u.VBlacklisted, u.Vid, u.RocksVerified, u.RAID, u.RISC, ot.otdata FROM agent=u, otdata=ot WHERE u.gid = ? AND ot.gid = u.gid", gid).Scan(&ud.IngressName, &ud.Level, &ud.LocationKey, &ot, &ud.VVerified, &ud.VBlacklisted, &ud.Vid, &ud.RocksVerified, &ud.RAID, &ud.RISC, &ud.OwnTracksJSON)
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
	if ot.Valid {
		ud.OwnTracksPW = ot.String
	}

	err = adTeams(gid, ud)
	if err != nil {
		Log.Error(err)
		return err
	}

	err = adOwnedTeams(gid, ud)
	if err != nil {
		Log.Error(err)
		return err
	}

	err = adTelegram(gid, ud)
	if err != nil {
		Log.Error(err)
		return err
	}

	err = adOps(gid, ud)
	if err != nil {
		Log.Error(err)
		return err
	}

	return nil
}

func adTeams(gid GoogleID, ud *AgentData) error {
	rows, err := db.Query("SELECT t.teamID, t.name, x.state, t.rockscomm FROM team=t, agentteams=x WHERE x.gid = ? AND x.teamID = t.teamID", gid)
	if err != nil {
		Log.Error(err)
		return err
	}
	defer rows.Close()
	var adteam AdTeam
	var teamname, rc sql.NullString
	for rows.Next() {
		err := rows.Scan(&adteam.ID, &teamname, &adteam.State, &rc)
		if err != nil {
			Log.Error(err)
			return err
		}
		// teamname can be null
		if teamname.Valid {
			adteam.Name = teamname.String
		} else {
			adteam.Name = ""
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

func adOwnedTeams(gid GoogleID, ud *AgentData) error {
	var ownedTeam AdOwnedTeam
	row, err := db.Query("SELECT teamID, name, rockscomm, rockskey FROM team WHERE owner = ?", gid)
	if err != nil {
		Log.Error(err)
		return err
	}
	defer row.Close()
	var rc, teamname, rockskey sql.NullString
	for row.Next() {
		err := row.Scan(&ownedTeam.ID, &teamname, &rc, &rockskey)
		if err != nil {
			Log.Error(err)
			return err
		}
		// can be null -- but should change schema to disallow that
		if teamname.Valid {
			ownedTeam.Name = teamname.String
		} else {
			ownedTeam.Name = ""
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

func adTelegram(gid GoogleID, ud *AgentData) error {
	var authtoken sql.NullString
	err := db.QueryRow("SELECT telegramName, telegramID, verified, authtoken FROM telegram WHERE gid = ?", gid).Scan(&ud.Telegram.UserName, &ud.Telegram.ID, &ud.Telegram.Verified, &authtoken)
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

func adOps(gid GoogleID, ud *AgentData) error {
	var op AdOperation
	row, err := db.Query("SELECT o.ID, o.Name, o.Color, t.Name FROM operation=o, team=t WHERE o.gid = ? AND o.teamID = t.teamID ORDER BY o.Name", gid)
	if err != nil {
		Log.Error(err)
		return err
	}
	defer row.Close()
	for row.Next() {
		err := row.Scan(&op.ID, &op.Name, &op.Color, &op.TeamName)
		if err != nil {
			Log.Error(err)
			return err
		}
		ud.OwnedOps = append(ud.OwnedOps, op)
	}

	row2, err := db.Query("SELECT o.ID, o.Name, o.Color, t.Name FROM operation=o, team=t, agentteams=x WHERE x.gid = ? AND x.teamID = o.teamID AND x.teamID = t.teamID AND x.state IN ('On', 'Primary')", gid)
	if err != nil {
		Log.Error(err)
		return err
	}
	defer row2.Close()
	for row2.Next() {
		err := row2.Scan(&op.ID, &op.Name, &op.Color, &op.TeamName)
		if err != nil {
			Log.Error(err)
			return err
		}
		ud.Ops = append(ud.Ops, op)
	}
	return nil
}

// AgentLocation updates the database to reflect a agent's current location
// OwnTracks data is updated to reflect the change
// TODO: react based on the location
func (gid GoogleID) AgentLocation(lat, lon, source string) error {
	// sanity checing on bounds?
	// YES, store lon,lat -- the ST_ functions expect it this way
	point := fmt.Sprintf("POINT(%s %s)", lon, lat)
	if _, err := db.Exec("UPDATE locations SET loc = PointFromText(?), upTime = NOW() WHERE gid = ?", point, gid); err != nil {
		Log.Notice(err)
		return err
	}

	if source != "OwnTracks" {
		err := gid.ownTracksExternalUpdate(lat, lon, source)
		if err != nil {
			Log.Notice(err)
			return err
		}
		// XXX put it out onto MQTT
	}

	// XXX check for waypoints/markers in range -- spin off into go routine which sends notifications

	return nil
}

// ResetLocKey updates the database with a new OwnTracks ID for a given agent
func (gid GoogleID) ResetLocKey() error {
	newlockey, _ := GenerateSafeName()
	_, err := db.Exec("UPDATE agent SET lockey = ? WHERE gid = ?", newlockey, gid)
	if err != nil {
		Log.Notice(err)
	}
	return err
}

// IngressName returns an agent's name for a GoogleID
func (gid GoogleID) IngressName() (string, error) {
	var iname string
	err := db.QueryRow("SELECT iname FROM agent WHERE gid = ?", gid).Scan(&iname)
	return iname, err
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
	_, _ = db.Exec("DELETE FROM otdata WHERE gid = ?", gid)
	_, _ = db.Exec("DELETE FROM locations WHERE gid = ?", gid)
	_, _ = db.Exec("DELETE FROM telegram WHERE gid = ?", gid)

	return nil
}

// Lock disables an account -- called by RISC system
func (gid GoogleID) Lock(reason string) error {
	if _, err := db.Exec("UPDATE agent SET RISC = 1 WHERE gid = ?", gid); err != nil {
		Log.Error(err)
		return err
	}
	return nil
}

// Unlock enables a disabled account -- called by RISC system
func (gid GoogleID) Unlock(reason string) error {
	if _, err := db.Exec("UPDATE agent SET RISC = 0 WHERE gid = ?", gid); err != nil {
		Log.Error(err)
		return err
	}
	return nil
}

// Logout sets a temporary logout token - not stored in DB since logout cases are not critical
// and sessions are refreshed with google hourly
func (gid GoogleID) Logout(reason string) {
	Log.Debugf("adding %s to logout list: %s", gid, reason)
	logoutlist[gid] = true
}

// CheckLogout looks to see if the user is on the force logout list
func (gid GoogleID) CheckLogout() bool {
	logout, ok := logoutlist[gid]
	if !ok { // not in the list
		return false
	}
	Log.Debugf("clearing %s from logoutlist", gid)
	logoutlist[gid] = false // now that they've been checked, clear them
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
