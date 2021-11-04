package v

import (
	"encoding/json"
	"database/sql"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/wasabee-project/Wasabee-Server/log"
)

// interface AuthProvider
// Startup(apikey string) 
// Search(id string) (result, interface{})

// Vconfig contains configuration for calling v.enl.one APIs
type Vconfig struct {
	APIEndpoint    string
	APIKey         string
	StatusEndpoint string
	TeamEndpoint   string
	configured     bool
}

var vc = Vconfig{
	APIEndpoint:    "https://v.enl.one/api/v1",
	StatusEndpoint: "https://status.enl.one/api/location",
	TeamEndpoint:   "https://v.enl.one/api/v2/teams",
}

// Vresult is set by the V API
type Vresult struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
	Data    vagent `json:"data"`
}

// vagent is set by the V API
type vagent struct {
	EnlID       string     `json:"enlid"`
	Gid         string     `json:"gid"`
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
	TelegramID  string     `json:"telegramid"`
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
	EnlID string   `json:"enlid"`
	Gid   string   `json:"gid"`
}

// Version 2.0 of the team query
type VTeamResult struct {
	Status string   `json:"status"`
	Agents []vagent `json:"data"`
}

type role struct {
	ID   uint8  `json:"id"`
	Name string `json:"name"`
}

type AgentVTeams struct {
	Status string       `json:"status"`
	Teams  []AgentVTeam `json:"data"`
}

type AgentVTeam struct {
	TeamID int64  `json:"teamid"`
	Name   string `json:"team"`
	Roles  []role `json:"roles"`
	Admin  bool   `json:"admin"`
}

// Startup is called from main() to initialize the config
func Startup(key string) {
	if key == "" {
		log.Debugw("startup", "skipping", "api key not set, not enabling V support")
	}
	log.Debugw("startup", "V.enl.one API Key", key)
	vc.APIKey = key 
	vc.configured = true
}

// Search checks a agent at V and populates a Vresult
func Search(id string) (error, Vresult) {
	var vres Vesult
	if !vc.configured {
		return nil, vres
	}
	if id == "" {
		return fmt.Errorf("empty search value"), vres
	}

	url := fmt.Sprintf("%s/agent/%s/trust?apikey=%s", vc.APIEndpoint, id, vc.APIKey)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Error(err)
		return err, vres
	}
	client := &http.Client{
		Timeout: 3 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		newerr := strings.ReplaceAll(err.Error(), vc.APIKey, "...")
		err = fmt.Errorf("unable to request user info from V")
		log.Errorw(err.Error(), "GID", id, "message", newerr)
		return err, vres
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Error(err)
		return err, vres
	}

	// log.Debug(string(body))
	err = json.Unmarshal(body, &vres)
	if err != nil {
		log.Error(err)
		return err, vres
	}
	if vres.Status != "ok" && vres.Message != "Agent not found" {
		err = fmt.Errorf(vres.Message)
		log.Info(err)
		return err, vres
	}
	// log.Debug(vres.Data.Agent)
	return nil, vres
}


// VTeams pulls a list of teams the agent is on at V
func VTeams(gid string, key string) (AgentVTeams, error) {
	var v AgentVTeams
	
	if key == "" {
		err := fmt.Errorf("cannot get V teams if no V API key set")
		log.Error(err)
		return v, err
	}

	apiurl := fmt.Sprintf("%s?apikey=%s", vc.TeamEndpoint, key)
	req, err := http.NewRequest("GET", apiurl, nil)
	if err != nil {
		err := fmt.Errorf("error establishing agent's team pull request")
		log.Error(err)
		return v, err
	}
	client := &http.Client{
		Timeout: 3 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		err := fmt.Errorf("error executing team pull request")
		log.Error(err)
		return v, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Error(err)
		return v, err
	}

	err = json.Unmarshal(body, &v)
	if err != nil {
		log.Error(err)
		return v, err
	}
	return v, nil
}

func VGetTeam(vteamID int64, key string) (VTeamResult, error) {
	var vt VTeamResult
	if vteamID == 0 {
		return vt, nil
	}

	if key == "" {
		err := fmt.Errorf("cannot get V team if no V API key set")
		log.Error(err)
		return vt, err
	}

	apiurl := fmt.Sprintf("%s/%d?apikey=%s", vc.TeamEndpoint, vteamID, key)
	req, err := http.NewRequest("GET", apiurl, nil)
	if err != nil {
		// log.Error(err) // do not leak API key to logs
		err := fmt.Errorf("error establishing team pull request")
		log.Error(err)
		return vt, err
	}
	client := &http.Client{
		Timeout: 3 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		err := fmt.Errorf("error executing team pull request")
		log.Error(err)
		return vt, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Error(err)
		return vt, err
	}

	err = json.Unmarshal(body, &vt)
	if err != nil {
		log.Error(err)
		return vt, err
	}
	return vt, nil
}

// VSync pulls a team (and role) from V to sync with a Wasabee team
func Sync(teamID string, key string) error {
	vteamID, role, err := teamID.VTeam()
	if err != nil {
		return err
	}
	if vteamID == 0 {
		return nil
	}

	if key == "" {
		err := fmt.Errorf("cannot sync V team if no V API key set")
		log.Error(err)
		return err
	}

	vt, err := VGetTeam(vteamID, key)
	if err != nil {
		log.Error(err)
		return err
	}

	// do not remove the owner from the team
	owner, err := teamID.Owner()
	if err != nil {
		log.Error(err)
		return err
	}

	// a map to track added agents
	atv := make(map[string]bool)

	for _, agent := range vt.Agents {
		_, err = agent.Gid.IngressName()
		if err == sql.ErrNoRows {
			log.Infow("Importing previously unknown agent", "GID", agent.Gid)
			_, err = agent.Gid.InitAgent()
			if err != nil {
				log.Info(err)
				continue
			}
			// XXX we could setup telegram here, if available
		}
		if err != nil && err != sql.ErrNoRows {
			log.Info(err)
			continue
		}

		if role != 0 { // role 0 means "any"
			for _, r := range agent.Roles {
				if r.ID == role {
					atv[agent.Gid] = true
					break
				}
			}
		} else {
			atv[agent.Gid] = true
		}

		// don't re-add them if already in the team
		in, err := agent.Gid.AgentInTeam(teamID)
		if err != nil {
			log.Info(err)
			continue
		}
		if in {
			log.Debugw("ignoring agent already on team", "GID", agent.Gid, "team", teamID)
			continue
		}
		if _, ok := atv[agent.Gid]; ok {
			log.Debugw("adding agent to team via V pull", "GID", agent.Gid, "team", teamID)
			if err := teamID.AddAgent(agent.Gid); err != nil {
				log.Info(err)
				continue
			}
			// XXX set startlat, startlon, etc...
		}
	}

	// remove those not present at V
	var t TeamData
	err = teamID.FetchTeam(&t)
	if err != nil {
		log.Info(err)
		return err
	}
	for _, a := range t.Agent {
		if a.Gid == owner {
			continue
		}
		_, ok := atv[a.Gid]
		if !ok {
			err := fmt.Errorf("agent in wasabee team but not in V team/role, removing")
			log.Infow(err.Error(), "GID", a.Gid, "wteam", teamID, "vteam", vteamID, "role", role)
			err = teamID.RemoveAgent(a.Gid)
			if err != nil {
				log.Error(err)
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
		log.Error(err)
		return err
	}

	log.Infow("linking team to V", "teamID", teamID, "vteam", vteam, "role", role)

	_, err := db.Exec("UPDATE team SET vteam = ?, vrole = ? WHERE teamID = ?", vteam, role, teamID)
	if err != nil {
		log.Error(err)
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

func GetTeamsByVID(v int64) ([]TeamID, error) {
	var teams []TeamID

	row, err := db.Query("SELECT teamID FROM team WHERE vteam = ?", v)
	if err != nil {
		log.Error(err)
		return teams, err
	}
	defer row.Close()

	var teamID TeamID
	for row.Next() {
		err = row.Scan(&teamID)
		if err != nil {
			log.Error(err)
			continue
		}
		teams = append(teams, teamID)
	}
	return teams, nil
}
