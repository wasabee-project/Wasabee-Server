package wasabee

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"
)

// Vconfig contains configuration for calling v.enl.one APIs
type Vconfig struct {
	APIEndpoint    string
	APIKey         string
	StatusEndpoint string
	TeamEndpoint   string
	configured     bool
}

var vc Vconfig

// Vresult is set by the V API
type Vresult struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
	Data    vagent `json:"data"`
}

// vagent is set by the V API
type vagent struct {
	EnlID       EnlID      `json:"enlid"`
	Gid         GoogleID   `json:"gid"`
	Vlevel      int64      `json:"vlevel"`
	Vpoints     int64      `json:"vpoints"`
	Agent       string     `json:"agent"`
	Level       int64      `json:"level"`
	Quarantine  bool       `json:"quarantine"`
	Active      bool       `json:"active"`
	Blacklisted bool       `json:"blacklisted"`
	Verified    bool       `json:"verified"`
	Flagged     bool       `json:"flagged"`
	Banned      bool       `json:"banned_by_nia"`
	Cellid      string     `json:"cellid"`
	TelegramID  TelegramID `json:"telegramid"`
	Telegram    string     `json:"telegram"`
	Email       string     `json:"email"`
	StartLat    float64    `json:"lat"`
	StartLon    float64    `json:"lon"`
	Distance    int64      `json:"distance"`
	Roles       []role     `json:"roles"`
}

// v team is set by the V API
type vteam struct {
	Status string    `json:"status"`
	Agents []vtagent `json:"data"`
}

// keep it simple
type vtagent struct {
	EnlID EnlID    `json:"enlid"`
	Gid   GoogleID `json:"gid"`
}

// Version 2.0 of the team query
type VTeamResult struct {
	Status string   `json:"status"`
	Agents []vagent `json:"data"`
}

type role struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// SetVEnlOne is called from main() to initialize the config
func SetVEnlOne(w Vconfig) {
	if w.APIKey == "" {
		Log.Infow("startup", "skipping", "api key not set, not enabling V support")
	}
	Log.Debugw("startup", "V.enl.one API Key", w.APIKey)
	vc.APIKey = w.APIKey

	if w.APIEndpoint != "" {
		vc.APIEndpoint = w.APIEndpoint
	} else {
		vc.APIEndpoint = "https://v.enl.one/api/v1"
	}

	if w.StatusEndpoint != "" {
		vc.StatusEndpoint = w.StatusEndpoint
	} else {
		vc.StatusEndpoint = "https://status.enl.one/api/location"
	}

	if w.TeamEndpoint != "" {
		vc.TeamEndpoint = w.TeamEndpoint
	} else {
		vc.TeamEndpoint = "https://v.enl.one/api/v2/teams" //teams/{teamid}?apikey={apikey}
	}

	vc.configured = true
}

// GetvEnlOne is used for templates to determine if V is enabled
func GetvEnlOne() bool {
	return vc.configured
}

// VSearch checks a agent at V and populates a Vresult
func VSearch(id AgentID, vres *Vresult) error {
	if !vc.configured {
		return nil
	}
	searchID := id.String()
	if searchID == "" {
		return fmt.Errorf("empty search value")
	}

	url := fmt.Sprintf("%s/agent/%s/trust?apikey=%s", vc.APIEndpoint, searchID, vc.APIKey)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		Log.Error(err)
		return err
	}
	client := &http.Client{
		Timeout: GetTimeout(3 * time.Second),
	}
	resp, err := client.Do(req)
	if err != nil {
		newerr := strings.ReplaceAll(err.Error(), vc.APIKey, "...")
		err = fmt.Errorf("unable to request user info from V")
		Log.Errorw(err.Error(), "GID", searchID, "message", newerr)
		return err
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		Log.Error(err)
		return err
	}

	// Log.Debug(string(body))
	err = json.Unmarshal(body, &vres)
	if err != nil {
		Log.Error(err)
		return err
	}
	if vres.Status != "ok" && vres.Message != "Agent not found" {
		err = fmt.Errorf(vres.Message)
		Log.Info(err)
		return err
	}
	// Log.Debug(vres.Data.Agent)
	return nil
}

// VUpdate updates the database to reflect an agent's current status at V.
// It should be called whenever a agent logs in via a new service (if appropriate); currently only https does.
func (gid GoogleID) VUpdate(vres *Vresult) error {
	if !vc.configured {
		return nil
	}

	if vres.Status == "ok" && vres.Data.Agent != "" {
		_, err := db.Exec("UPDATE agent SET Vname = ?, level = ?, VVerified = ?, VBlacklisted = ?, Vid = ? WHERE gid = ?",
			vres.Data.Agent, vres.Data.Level, vres.Data.Verified, vres.Data.Blacklisted, MakeNullString(vres.Data.EnlID), gid)

		// doppelkeks error
		if err != nil && strings.Contains(err.Error(), "Error 1062") {
			vname := fmt.Sprintf("%s-%s", vres.Data.Agent, gid)
			Log.Warnw("dupliate ingress agent name detected at v", "GID", vres.Data.Agent, "new name", vname)
			if _, err := db.Exec("UPDATE agent SET Vname = ?, level = ?, VVerified = ?, VBlacklisted = ?, Vid = ? WHERE gid = ?",
				vname, vres.Data.Level, vres.Data.Verified, vres.Data.Blacklisted, MakeNullString(vres.Data.EnlID), gid); err != nil {
				Log.Error(err)
				return err
			}
		} else if err != nil {
			Log.Error(err)
			return err
		}

	}
	return nil
}

type statusResponse struct {
	Status  int    `json:"status"`
	Message string `json:"message"`
	Lon     string `json:"lon"`
	Lat     string `json:"lat"`
	Time    string `json:"time"`
}

// StatusLocation attempts to check for location data from status.enl.one.
// The API documentation is scant, so this is provisional -- seems to work.
func (eid EnlID) StatusLocation() (string, string, error) {
	if !vc.configured {
		return "", "", fmt.Errorf("the V API key not configured")
	}
	url := fmt.Sprintf("%s/%s?apikey=%s", vc.StatusEndpoint, eid, vc.APIKey)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		Log.Error(err)
		return "", "", err
	}
	client := &http.Client{
		Timeout: GetTimeout(3 * time.Second),
	}
	resp, err := client.Do(req)
	if err != nil {
		Log.Error(err)
		return "", "", err
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		Log.Error(err)
		return "", "", err
	}

	var stat statusResponse
	err = json.Unmarshal(body, &stat)
	if err != nil {
		Log.Error(err)
		return "", "", err
	}
	if stat.Status != 0 {
		err := fmt.Errorf("polling %s returned message: %s", eid, stat.Message)
		_ = eid.StatusLocationDisable()
		return "", "", err
	}
	return stat.Lat, stat.Lon, nil
}

// StatusLocation attempts to check for location data from status.enl.one.
// The API documentation is scant, so this is provisional -- seems to work.
func (gid GoogleID) StatusLocation() (string, string, error) {
	e, err := gid.EnlID()
	if err != nil {
		Log.Error(err)
		return "", "", err
	}
	lat, lon, err := e.StatusLocation()
	return lat, lon, err
}

// StatusLocationEnable turns RAID/JEAH pulling on for the specified agent
func (eid EnlID) StatusLocationEnable() error {
	_, err := db.Exec("UPDATE agent SET RAID = 1 WHERE Vid = ?", eid)
	if err != nil {
		Log.Error(err)
		return err
	}
	return nil
}

// StatusLocationEnable turns RAID/JEAH pulling on for the specified agent
func (gid GoogleID) StatusLocationEnable() error {
	eid, _ := gid.EnlID()
	err := eid.StatusLocationEnable()
	return err
}

// StatusLocationDisable turns RAID/JEAH pulling off for the specified agent
func (eid EnlID) StatusLocationDisable() error {
	_, err := db.Exec("UPDATE agent SET RAID = 0 WHERE Vid = ?", eid)
	if err != nil {
		Log.Error(err)
		return err
	}
	return nil
}

// StatusLocationDisable turns RAID/JEAH pulling off for the specified agent
func (gid GoogleID) StatusLocationDisable() error {
	_, err := db.Exec("UPDATE agent SET RAID = 0 WHERE gid = ?", gid)
	if err != nil {
		Log.Error(err)
		return err
	}
	return nil
}

// EnlID returns the V EnlID for a agent if it is known.
func (gid GoogleID) EnlID() (EnlID, error) {
	var vid sql.NullString
	err := db.QueryRow("SELECT Vid FROM agent WHERE gid = ?", gid).Scan(&vid)
	if err != nil {
		Log.Error(err)
		return "", err
	}
	if !vid.Valid {
		return "", nil
	}
	e := EnlID(vid.String)

	return e, err
}

// StatusServerPoller starts up from main and requests any agents who are configured to use RAID/JEAH for location services from the status.enl.one server.
// It works, but more research is necessary on the settings required on the permissions.
func StatusServerPoller() {
	if !vc.configured {
		Log.Infow("startup", "status", "V not configures: not polling status.enl.one")
		return
	}

	// loop forever
	Log.Infow("startup", "status", "Starting status.enl.one Poller")
	for {
		// get list of agents who say they use JEAH/RAID
		row, err := db.Query("SELECT gid, Vid FROM agent WHERE RAID = 1")
		if err != nil {
			Log.Error(err)
			return
		}
		defer row.Close()
		var gid GoogleID
		var vid sql.NullString

		for row.Next() {
			err = row.Scan(&gid, &vid)
			// XXX if the agent isn't active on any teams, ignore
			if err != nil {
				Log.Error(err)
				continue
			}
			if !vid.Valid {
				Log.Errorw("agent requested RAID poll, but has not configured V", "GID", gid.String())
				_ = gid.StatusLocationDisable()
				continue
			}
			e := EnlID(vid.String)
			lat, lon, err := e.StatusLocation()
			if err != nil {
				// XXX add the agent to an exception list? purge the list every 12 hours?
				Log.Error(err)
				continue
			}
			err = gid.AgentLocation(lat, lon)
			if err != nil {
				Log.Error(err)
				continue
			}
		}
		// SCB: https://github.com/golang/go/issues/27707 -- sleep is fine
		time.Sleep(300 * time.Second)
	}
}

// Gid looks up a GoogleID from an EnlID
func (eid EnlID) Gid() (GoogleID, error) {
	var gid GoogleID
	err := db.QueryRow("SELECT gid FROM agent WHERE Vid = ?", eid).Scan(&gid)
	if err != nil {
		Log.Error(err)
		return "", err
	}
	return gid, nil
}

// VTeam gets the confimgured V team information for a teamID
func (teamID TeamID) VTeam() (int64, int64, error) {
	var team, role int64
	err := db.QueryRow("SELECT vteam, vrole FROM team WHERE teamID = ?", teamID).Scan(&team, &role)
	if err != nil {
		Log.Error(err)
		return 0, 0, err
	}
	return team, role, nil
}

// VSync pulls a team (and role) from V to sync with a Wasabee team
func (teamID TeamID) VSync(key string) error {
	vteamID, role, err := teamID.VTeam()
	if err != nil {
		return err
	}
	if vteamID == 0 {
		return nil
	}

	apiurl := fmt.Sprintf("%s/%d?apikey=%s", vc.TeamEndpoint, vteamID, key)
	req, err := http.NewRequest("GET", apiurl, nil)
	if err != nil {
		// Log.Error(err) // do not leak API key to logs
		err := fmt.Errorf("error establishing team pull request")
		Log.Error(err)
		return err
	}
	client := &http.Client{
		Timeout: GetTimeout(3 * time.Second),
	}
	resp, err := client.Do(req)
	if err != nil {
		err := fmt.Errorf("error executing team pull request")
		Log.Error(err)
		return err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		Log.Error(err)
		return err
	}

	var vt VTeamResult
	err = json.Unmarshal(body, &vt)
	if err != nil {
		Log.Error(err)
		return err
	}

	// a map to track added agents
	atv := make(map[GoogleID]bool)

	for _, agent := range vt.Agents {
		_, err = agent.Gid.IngressName()
		if err == sql.ErrNoRows {
			Log.Infow("Importing previously unknown agent", "GID", agent.Gid)
			_, err = agent.Gid.InitAgent()
			if err != nil {
				Log.Info(err)
				continue
			}
			// XXX we could setup telegram here, if available
		}
		if err != nil && err != sql.ErrNoRows {
			Log.Info(err)
			continue
		}

		if role != 0 { // role 0 means "any"
			thisrole := false
			for _, r := range agent.Roles {
				if r.ID == role {
					thisrole = true
					break
				}
			}
			if !thisrole {
				// agent is not in the proper role to be on this team -- remove if necessary
				in, err := agent.Gid.AgentInTeam(teamID)
				if err != nil {
					Log.Info(err)
					continue
				}
				if in {
					Log.Debugf("%s no longer in role %d", agent.Gid, role)
					err = teamID.RemoveAgent(agent.Gid)
					if err != nil {
						Log.Error(err)
					}
				}
				return nil
			}
		}

		// team is set to all roles (0) or the role is present for this agent: add them
		atv[agent.Gid] = true

		// don't re-add them if already in the team
		in, err := agent.Gid.AgentInTeam(teamID)
		if err != nil {
			Log.Info(err)
			continue
		}
		if in {
			Log.Debugw("ignoring agent already on team", "GID", agent.Gid, "team", teamID)
			continue
		}

		Log.Debugw("adding agent to team via V pull", "GID", agent.Gid, "team", teamID)
		if err := teamID.AddAgent(agent.Gid); err != nil {
			Log.Info(err)
			continue
		}
		// XXX set startlat, startlon, etc...
	}

	// remove those not present at V
	var t TeamData
	err = teamID.FetchTeam(&t)
	if err != nil {
		Log.Info(err)
		return err
	}
	for _, a := range t.Agent {
		Log.Debugf("checking agent for delete: %s", a.Gid)
		_, ok := atv[a.Gid]
		if !ok {
			err := fmt.Errorf("agent in wasabee team but not in V team/role, removing")
			Log.Infow(err.Error(), "GID", a.Gid, "wteam", teamID, "vteam", vteamID, "role", role)
			err = teamID.RemoveAgent(a.Gid)
			if err != nil {
				Log.Error(err)
			}
		}
	}

	return nil
}

// VConfigure sets V connection for a Wasabee team -- caller should verify ownership
func (teamID TeamID) VConfigure(vteam int64, role uint8) error {
	_, ok := vroles[role]
	if !ok {
		err := fmt.Errorf("invalid role")
		Log.Error(err)
		return err
	}

	Log.Infow("linking team to V", "teamID", teamID, "vteam", vteam, "role", role)

	_, err := db.Exec("UPDATE team SET vteam = ?, vrole = ? WHERE teamID = ?", vteam, role, teamID)
	if err != nil {
		Log.Error(err)
		return err
	}
	return nil
}

var vroles = map[uint8]string{
	0:   "All",
	1:   "Planner",
	2:   "Operator",
	3:   "Linker",
	4:   "Keyfarming",
	5:   "Cleaner",
	6:   "Field Agent",
	7:   "Item Sponsor",
	8:   "Key Transport",
	9:   "Recharging",
	10:  "Software Support",
	11:  "Anomaly TL",
	12:  "Team Lead",
	13:  "Other",
	100: "Team-0",
	101: "Team-1",
	102: "Team-2",
	103: "Team-3",
	104: "Team-4",
	105: "Team-5",
	106: "Team-6",
	107: "Team-7",
	108: "Team-8",
	109: "Team-9",
	110: "Team-10",
	111: "Team-11",
	112: "Team-12",
	113: "Team-13",
	114: "Team-14",
	115: "Team-15",
	116: "Team-16",
	117: "Team-17",
	118: "Team-18",
	119: "Team-19",
}
