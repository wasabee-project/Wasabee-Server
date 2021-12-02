package v

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/wasabee-project/Wasabee-Server/log"
)

// wrappers for type safety
type TeamID string
type GoogleID string
type VTeamID int64

type VT struct {
	Name   string
	TeamID VTeamID
	Role   uint8
}

var Callbacks struct {
	Team   func(teamID TeamID) (ID int64, roleNum uint8, err error)
	FromDB func(gid GoogleID) (a Agent, fetched time.Time, err error)
	ToDB   func(a Agent) error
	Agents func(VTeamID, string) ([]Agent, error)
}

// Config contains configuration for calling v.enl.one APIs
type Config struct {
	APIKey         string
	APIEndpoint    string
	StatusEndpoint string
	TeamEndpoint   string
	configured     bool
}

var vc = Config{
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
/* type team struct {
	Status string      `json:"status"`
	Agents []teamagent `json:"data"`
}

// keep it simple
type teamagent struct {
	EnlID string `json:"enlid"`
	Gid   string `json:"gid"`
} */

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
	TeamID VTeamID `json:"teamid"`
	Name   string  `json:"team"`
	Roles  []role  `json:"roles"`
	Admin  bool    `json:"admin"`
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

// Sync pulls a team (and role) from V to sync with a Wasabee team
func Sync(teamID TeamID, key string) error {
	vteamID, role, err := Callbacks.Team(teamID)
	log.Debug("V Sync", "team", vteamID, "role", role)
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
	log.Debug("V Sync", "team", vt)

	log.Error("Sync not refactored yet...")
	return nil
	/*
		// do not remove the owner from the team
		owner, err := teamID.Owner()
		if err != nil {
			log.Error(err)
			return err
		}

		// a map to track added agents
		atv := make(map[string]bool)

		for _, agent := range vt.Agents {
			if !agent.Gid.Valid() {
				log.Infow("Importing previously unknown agent", "GID", agent.Gid)
				_, err = agent.Gid.Authenticate()
				if err != nil {
					log.Info(err)
					continue
				}
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
	*/
}

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

func processTeams(data AgentVTeams, teams []VT) ([]teamToMake, error) {
	var m []teamToMake
	for _, t := range data.Teams {
		if !t.Admin {
			// log.Debugw("not admin of v team, not creating w team", "v team", t.TeamID)
			continue
		}

		// don't make duplicates
		already := false
		for _, adt := range teams {
			if adt.TeamID == t.TeamID && adt.Role == 0 {
				// log.Debugw("Wasabee team already exists for this V team", "v team", t.TeamID, "role", 0, "teamID", adt.ID)
				already = true
				break
			}
		}
		if already {
			continue
		}
		m = append(m, teamToMake{
			ID:   t.TeamID,
			Role: 0,
			Name: fmt.Sprintf("%s (all)", t.Name),
		})
	}

	return m, nil
}

func processRoleSingleTeam(t AgentVTeam, teams []VT, key string) ([]teamToMake, error) {
	var m []teamToMake

	if !t.Admin {
		// log.Debugw("not admin of v team, not creating w team", "v team", t.TeamID)
		return m, nil
	}
	roles := make(map[uint8]bool)

	vt, err := Callbacks.Agents(t.TeamID, key)
	if err != nil {
		return m, err
	}

	// for every role of every agent -- this logic order is better, update the processRoleTeams to use it...
	for _, a := range vt {
		for _, r := range a.Roles {
			ok := roles[r.ID]
			if !ok { // first time we've seen this team/role
				roles[r.ID] = true

				already := false
				for _, adt := range teams {
					if adt.TeamID == t.TeamID && adt.Role == r.ID {
						// log.Debugw("Wasabee team already exists for this V team/role", "v team", t.TeamID, "role", r.ID, "teamID", adt.ID)
						already = true
						break
					}
				}
				if already {
					continue
				}

				m = append(m, teamToMake{
					ID:   t.TeamID,
					Role: r.ID,
					Name: fmt.Sprintf("%s (%s)", t.Name, r.Name),
				})
			}
		}
	}

	return m, nil
}

type teamToMake struct {
	ID   VTeamID
	Role uint8
	Name string
}

func processRoleTeams(data AgentVTeams, teams []VT, key string) ([]teamToMake, error) {
	var m []teamToMake

	raw := make(map[VTeamID]map[uint8]bool)

	// for every team of which I am an admin
	for _, t := range data.Teams {
		if !t.Admin {
			// log.Debugw("not admin of v team, not creating w team", "v team", t.TeamID)
			continue
		}
		roles := make(map[uint8]bool)

		// load all agents
		vt, err := Callbacks.Agents(t.TeamID, key)
		if err != nil {
			return m, err
		}

		// for every role of every agent
		for _, a := range vt {
			for _, r := range a.Roles {
				// don't make duplicates
				already := false
				for _, adt := range teams {
					if adt.TeamID == t.TeamID && adt.Role == r.ID {
						// log.Debugw("Wasabee team already exists for this V team/role", "v team", t.TeamID, "role", r.ID, "teamID", adt.ID)
						already = true
						break
					}
				}
				if already {
					continue
				}

				ok := roles[r.ID]
				if !ok { // first time we've seen this team/role
					m = append(m, teamToMake{
						ID:   t.TeamID,
						Role: r.ID,
						Name: fmt.Sprintf("%s (%s)", t.Name, r.Name),
					})
					roles[r.ID] = true
				}
			}
		}
		raw[t.TeamID] = roles
	}

	return m, nil
}

func ProcessBulkImport(data AgentVTeams, teams []VT, key string, mode string) ([]teamToMake, error) {
	var teamstomake []teamToMake
	var err error

	switch mode {
	case "role":
		teamstomake, err = processRoleTeams(data, teams, key)
	case "team":
		teamstomake, err = processTeams(data, teams)
	default:
		id, err := strconv.ParseInt(mode, 10, 64)
		if err != nil {
			log.Error(err)
			return teamstomake, err
		}
		for _, t := range data.Teams {
			if int64(t.TeamID) == id {
				teamstomake, err = processRoleSingleTeam(t, teams, key)
				if err != nil {
					return teamstomake, err
				}
				break
			}
		}
	}
	return teamstomake, err
}
