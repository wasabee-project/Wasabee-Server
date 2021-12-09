package v

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/wasabee-project/Wasabee-Server/log"
	"github.com/wasabee-project/Wasabee-Server/model"
)

type VTeamID int64

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

// Result is set by the V trust API
type trustResult struct {
	Status  string       `json:"status"`
	Message string       `json:"message,omitempty"`
	Data    model.VAgent `json:"data"`
}

// Version 2.0 of the team query
type teamResult struct {
	Status string         `json:"status"`
	Agents []model.VAgent `json:"data"`
}

type bulkResult struct {
	Status string                            `json:"status"`
	Agents map[model.TelegramID]model.VAgent `json:"data"`
}

// myTeams is what V returns when an agent's teams are requested
type myTeams struct {
	Status string   `json:"status"`
	Teams  []myTeam `json:"data"`
}

type myTeam struct {
	TeamID VTeamID `json:"teamid"`
	Name   string  `json:"team"`
	Roles  []struct {
		ID   uint8  `json:"id"`
		Name string `json:"name"`
	} `json:"roles"`
	Admin bool `json:"admin"`
}

// Startup is called from main() to initialize the config
func Startup(key string) {
	log.Debugw("startup", "V.enl.one API Key", key)
	vc.APIKey = key
	vc.configured = true
}

// trustCheck checks a agent at V and populates a trustResult
func trustCheck(id model.GoogleID) (*trustResult, error) {
	var tr trustResult
	if !vc.configured {
		return &tr, nil
	}
	if id == "" {
		return &tr, fmt.Errorf("empty trustCheck value")
	}

	url := fmt.Sprintf("%s/agent/%s/trust?apikey=%s", vc.APIEndpoint, id, vc.APIKey)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Error(err)
		return &tr, err
	}
	client := &http.Client{
		Timeout: 3 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		log.Debug(err)
		err = fmt.Errorf("unable to request user info from V")
		return &tr, err
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Error(err)
		return &tr, err
	}

	// log.Debug(string(body))
	err = json.Unmarshal(body, &tr)
	if err != nil {
		log.Error(err)
		return &tr, err
	}
	if tr.Status != "ok" && tr.Message != "Agent not found" {
		err = fmt.Errorf(tr.Message)
		log.Info(err)
		return &tr, err
	}
	// log.Debug(tr)
	tr.Data.Gid = id // V isn't sending the GIDs
	return &tr, nil
}

// GetMyTeams pulls a list of teams the agent is on at V
func GetMyTeams(gid model.GoogleID) (*myTeams, error) {
	log.Debug("v get my team", "gid", gid)
	var v myTeams

	key, err := gid.VAPIkey()
	if err != nil {
		log.Error(err)
		return &v, err
	}
	if key == "" {
		err := fmt.Errorf("cannot get V teams if no V API key set")
		log.Error(err)
		return &v, err
	}

	apiurl := fmt.Sprintf("%s?apikey=%s", vc.TeamEndpoint, key)
	req, err := http.NewRequest("GET", apiurl, nil)
	if err != nil {
		err := fmt.Errorf("error establishing agent's team pull request")
		log.Error(err)
		return &v, err
	}
	client := &http.Client{
		Timeout: 3 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		err := fmt.Errorf("error executing team pull request")
		log.Error(err)
		return &v, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Error(err)
		return &v, err
	}

	err = json.Unmarshal(body, &v)
	if err != nil {
		log.Error(err)
		return &v, err
	}
	return &v, nil
}

func (vteamID VTeamID) GetTeamFromV(key string) (*teamResult, error) {
	log.Debug("get team from v")

	var vt teamResult
	if vteamID == 0 {
		return &vt, nil
	}

	if key == "" {
		err := fmt.Errorf("cannot get V team if no V API key set")
		log.Error(err)
		return &vt, err
	}

	apiurl := fmt.Sprintf("%s/%d?apikey=%s", vc.TeamEndpoint, vteamID, key)
	req, err := http.NewRequest("GET", apiurl, nil)
	if err != nil {
		// log.Error(err) // do not leak API key to logs
		err := fmt.Errorf("error establishing team pull request")
		log.Error(err)
		return &vt, err
	}
	client := &http.Client{
		Timeout: 3 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		err := fmt.Errorf("error executing team pull request")
		log.Error(err)
		return &vt, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Error(err)
		return &vt, err
	}

	err = json.Unmarshal(body, &vt)
	if err != nil {
		log.Error(err)
		return &vt, err
	}
	return &vt, nil
}

// Sync pulls a team (and role) from V to sync with a Wasabee team
func Sync(teamID model.TeamID, key string) error {
	log.Debug("v sync")

	x, role, err := teamID.VTeam()
	vteamID := VTeamID(x)

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

	vt, err := vteamID.GetTeamFromV(key)
	if err != nil {
		log.Error(err)
		return err
	}
	log.Debug("V Sync", "team", vt)

	// do not remove the owner from the team
	owner, err := teamID.Owner()
	if err != nil {
		log.Error(err)
		return err
	}

	// a map to track added agents
	atv := make(map[model.GoogleID]bool)

	for _, agent := range vt.Agents {
		if !agent.Gid.Valid() {
			log.Infow("Importing previously unknown agent", "GID", agent.Gid)
			err := agent.Gid.FirstLogin()
			if err != nil {
				log.Error(err)
				return err
			}
		}

		/*
			if role != 0 { // role 0 means "any"
				for _, r := range agent.Roles {
					if r == role {
						atv[agent.Gid] = true
						break
					}
				}
			} else {
				atv[agent.Gid] = true
			} */

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
	t, err := teamID.FetchTeam()
	if err != nil {
		log.Info(err)
		return err
	}
	for _, a := range t.TeamMembers {
		if a.Gid == owner {
			continue
		}
		if !atv[a.Gid] {
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

func Authorize(gid model.GoogleID) bool {
	a, fetched, err := model.VFromDB(gid)
	if err != nil {
		log.Error(err)
		// do not block on db error
		return true
	}

	log.Debugw("v from cache", "gid", gid, "data", a, "fetched", fetched)

	if a.Agent == "" || fetched.Before(time.Now().Add(0-time.Hour)) {
		net, err := trustCheck(gid)
		if err != nil {
			log.Error(err)
			// do not block on network error unless already listed as blacklisted in DB
			return !a.Blacklisted
		}
		log.Debugw("v cache refreshed", "gid", gid, "data", net.Data)
		err = model.VToDB(&net.Data)
		if err != nil {
			log.Error(err)
		}
		a = &net.Data // use the network result now that it is saved
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

func processTeams(data myTeams) ([]teamToMake, error) {
	log.Debug("v processTeams")

	var m []teamToMake
	for _, t := range data.Teams {
		if !t.Admin {
			// log.Debugw("not admin of v team, not creating w team", "v team", t.TeamID)
			continue
		}

		/* don't make duplicates
		already := false
		for _, adt := range teams {
			if adt.TeamID == t.TeamID && adt.Role == 0 {
				log.Debugw("Wasabee team already exists for this V team", "v team", t.TeamID, "role", 0, "teamID", adt.ID)
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
		*/
	}

	return m, nil
}

// internal type
type teamToMake struct {
	ID   VTeamID
	Role uint8
	Name string
}

func processRoleSingleTeam(t myTeam, teams []vt, key string) ([]teamToMake, error) {
	log.Debug("v processRoleSingleTeam")
	var m []teamToMake

	/*
		if !t.Admin {
			log.Debugw("not admin of v team, not creating w team", "v team", t.TeamID)
			return m, nil
		}
		roles := make(map[uint8]bool)

		vteamID, role, err := t.TeamID.VTeam()
		if err != nil {
			log.Error(err)
			return m, err
		}

		vt, err := vteamID.GetTeamFromV(key)
		if err != nil {
			log.Error(err)
			return m, err
		}

		// for every role of every agent -- this logic order is better, update the processRoleTeams to use it...
		for _, a := range vt {
			for _, r := range a.Roles {
				if !roles[r.ID] { // first time we've seen this team/role
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
	*/
	return m, nil
}

func processRoleTeams(data *myTeams) ([]teamToMake, error) {
	log.Debug("v processRoleTeams")
	var m []teamToMake

	// raw := make(map[VTeamID]map[uint8]bool)

	log.Error("rewrite processRoleTeams")

	return m, nil
}

type vt struct {
	Name   string
	TeamID VTeamID
	Role   uint8
}

func BulkImport(gid model.GoogleID, mode string) error {
	log.Debug("v BuklImport")
	var teamstomake []teamToMake

	key, err := gid.VAPIkey()
	if err != nil {
		log.Error(err)
		return err
	}

	agent, err := gid.GetAgent()
	if err != nil {
		log.Error(err)
		return err
	}

	var vteams []vt

	for _, t := range agent.Teams {
		vteams = append(vteams, vt{
			Name:   t.Name,
			TeamID: VTeamID(t.VTeam),
			Role:   t.VTeamRole,
		})
	}

	teamsfromv, err := GetMyTeams(gid)
	if err != nil {
		log.Error(err)
		return err
	}

	switch mode {
	case "role":
		teamstomake, err = processRoleTeams(teamsfromv)
	case "team":
		teamstomake, err = processTeams(*teamsfromv)
	default:
		/*
			id, err := strconv.ParseInt(mode, 10, 64)
			if err != nil {
				log.Error(err)
				return err
			}
			for _, t := range teamsfromv.Teams {
				if int64(t.TeamID) == id {
					teamstomake, err = processRoleSingleTeam(t, teams, key)
					if err != nil {
						return teamstomake, err
					}
					break
				}
			}
		*/
	}
	if err != nil {
		log.Error(err)
	}

	for _, t := range teamstomake {
		log.Debugw("Creating Wasabee team for V team", "v team", t.ID, "role", t.Role)
		teamID, err := gid.NewTeam(t.Name)
		if err != nil {
			log.Error(err)
			return err
		}
		err = teamID.VConfigure(int64(t.ID), uint8(t.Role))
		if err != nil {
			log.Error(err)
			return err
		}
		err = Sync(teamID, key)
		if err != nil {
			log.Error(err)
			return err
		}
	}

	return nil
}

func TelegramSearch(tgid model.TelegramID) (*model.VAgent, error) {
	var br bulkResult
	if !vc.configured {
		return &model.VAgent{}, nil
	}

	url := fmt.Sprintf("%s/bulk/agent/info/telegramid?apikey=%s", vc.APIEndpoint, vc.APIKey)
	postdata := fmt.Sprintf("[%d]", tgid)

	req, err := http.NewRequest("POST", url, strings.NewReader(postdata))
	if err != nil {
		log.Error(err)
		return &model.VAgent{}, err
	}
	client := &http.Client{
		Timeout: 3 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		log.Debug(err)
		err = fmt.Errorf("unable to search at V")
		return &model.VAgent{}, err
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Error(err)
		return &model.VAgent{}, err
	}

	log.Debug(string(body))
	err = json.Unmarshal(body, &br)
	if err != nil {
		log.Error(err)
		return &model.VAgent{}, err
	}
	if br.Status != "ok" {
		err = fmt.Errorf(br.Status)
		log.Info(err)
		return &model.VAgent{}, err
	}
	a := br.Agents[tgid]
	return &a, nil
}
