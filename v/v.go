package v

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/wasabee-project/Wasabee-Server/log"
)

// wrappers for type safety
type TeamID string
type GoogleID string

var Callbacks struct {
	Team   func(teamID TeamID) (ID int64, roleNum uint8, err error)
	FromDB func(gid GoogleID) (a Agent, fetched time.Time, err error)
	ToDB   func(a Agent) error
}

// Config contains configuration for calling v.enl.one APIs
type config struct {
	APIKey         string
	APIEndpoint    string
	StatusEndpoint string
	TeamEndpoint   string
	configured     bool
}

var vc = config{
	APIEndpoint:    "https://v.enl.one/api/v1",
	StatusEndpoint: "https://status.enl.one/api/location",
	TeamEndpoint:   "https://v.enl.one/api/v2/teams",
}

// Result is set by the V API
type Result struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
	Data    Agent  `json:"data"`
}

// agent is set by the V API
type Agent struct {
	EnlID       string  `json:"enlid"`
	Gid         string  `json:"gid"`
	Vlevel      int64   `json:"vlevel"`
	Vpoints     int64   `json:"vpoints"`
	Agent       string  `json:"agent"`
	Level       int64   `json:"level"`
	Quarantine  bool    `json:"quarantine"`
	Active      bool    `json:"active"`
	Blacklisted bool    `json:"blacklisted"`
	Verified    bool    `json:"verified"`
	Flagged     bool    `json:"flagged"`
	Banned      bool    `json:"banned_by_nia"`
	Cellid      string  `json:"cellid"`
	TelegramID  string  `json:"telegramid"`
	Telegram    string  `json:"telegram"`
	Email       string  `json:"email"`
	StartLat    float64 `json:"lat"`
	StartLon    float64 `json:"lon"`
	Distance    int64   `json:"distance"`
	Roles       []role  `json:"roles"`
}

// v team is set by the V API
type team struct {
	Status string      `json:"status"`
	Agents []teamagent `json:"data"`
}

// keep it simple
type teamagent struct {
	EnlID string `json:"enlid"`
	Gid   string `json:"gid"`
}

// Version 2.0 of the team query
type TeamResult struct {
	Status string  `json:"status"`
	Agents []Agent `json:"data"`
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
	log.Debugw("startup", "V.enl.one API Key", key)
	vc.APIKey = key
	vc.configured = true
}

// search checks a agent at V and populates a Result
func search(id GoogleID) (Result, error) {
	var vres Result
	if !vc.configured {
		return vres, nil
	}
	if id == "" {
		return vres, fmt.Errorf("empty search value")
	}

	url := fmt.Sprintf("%s/agent/%s/trust?apikey=%s", vc.APIEndpoint, id, vc.APIKey)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Error(err)
		return vres, err
	}
	client := &http.Client{
		Timeout: 3 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		newerr := strings.ReplaceAll(err.Error(), vc.APIKey, "...")
		err = fmt.Errorf("unable to request user info from V")
		log.Errorw(err.Error(), "GID", id, "message", newerr)
		return vres, err
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Error(err)
		return vres, err
	}

	// log.Debug(string(body))
	err = json.Unmarshal(body, &vres)
	if err != nil {
		log.Error(err)
		return vres, err
	}
	if vres.Status != "ok" && vres.Message != "Agent not found" {
		err = fmt.Errorf(vres.Message)
		log.Info(err)
		return vres, err
	}
	// log.Debug(vres.Data.Agent)
	return vres, nil
}

// Teams pulls a list of teams the agent is on at V
func Teams(gid GoogleID, key string) (AgentVTeams, error) {
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

func GetTeam(vteamID int64, key string) (TeamResult, error) {
	var vt TeamResult
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

/*
// VSync pulls a team (and role) from V to sync with a Wasabee team
func Sync(teamID string, key string) error {
	vteamID, role, err := Callbacks.Team(teamID)
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

	vt, err := GetTeam(vteamID, key)
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
*/

var Roles = map[uint8]string{
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

func Authorize(gid GoogleID) bool {
	var a Agent

	fromdb, fetched, err := Callbacks.FromDB(gid)
	if err != nil {
		log.Error(err)
		return true
	}
	if fromdb.Agent == "" || fetched.Before(time.Now().Add(0-time.Hour)) {
		result, err := search(gid)
		if err != nil {
			log.Error(err)
			return true
		}
		err = Callbacks.ToDB(a)
		if err != nil {
			log.Error(err)
		}
		a = result.Data
	} else {
		a = fromdb
	}

	if a.Agent != "" {
		if a.Quarantine {
			log.Warnw("access denied", "GID", gid, "reason", "quarantined at V")
			return false
		}
		if a.Flagged {
			log.Warnw("access denied", "GID", gid, "reason", "flagged at V")
			return false
		}
		if a.Blacklisted || a.Banned {
			log.Warnw("access denied", "GID", gid, "reason", "blacklisted at V")
			return false
		}
	}

	return true
}
