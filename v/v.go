package v

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	// "strconv"
	"strings"
	"time"

	"github.com/wasabee-project/Wasabee-Server/log"
	"github.com/wasabee-project/Wasabee-Server/messaging"
	"github.com/wasabee-project/Wasabee-Server/model"
)

type vTeamID int64

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
	Status  string         `json:"status"`
	Message string         `json:"message,omitempty"`
	Agents  []model.VAgent `json:"data"`
}

type bulkResult struct {
	Status string                            `json:"status"`
	Agents map[model.TelegramID]model.VAgent `json:"data"`
}

// myTeams is what V returns when an agent's teams are requested
type myTeams struct {
	Status  string   `json:"status"`
	Teams   []myTeam `json:"data"`
	Message string   `json:"message,omitempty"`
}

type myTeam struct {
	TeamID vTeamID `json:"teamid"`
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

	messaging.RegisterMessageBus("v.enl.one", messaging.Bus{
		AddToRemote:      addToRemote,
		RemoveFromRemote: removeFromRemote,
	})
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

// geTeams pulls a list of teams the agent is on at V
func getTeams(gid model.GoogleID) (*myTeams, error) {
	var v myTeams

	key, err := gid.GetVAPIkey()
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

func (vteamID vTeamID) getTeamFromV(key string) (*teamResult, error) {
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
	// log.Debug(string(body))

	err = json.Unmarshal(body, &vt)
	if err != nil {
		log.Error(err)
		return &vt, err
	}
	if vt.Status != "ok" {
		err := fmt.Errorf(vt.Message)
		log.Error(err)
		return &vt, err
	}
	return &vt, nil
}

// Sync pulls a team (and role) from V to sync with a Wasabee team
func Sync(teamID model.TeamID, key string) error {
	x, role, err := teamID.VTeam()
	if err != nil {
		return err
	}
	vteamID := vTeamID(x)
	if vteamID == 0 {
		return nil
	}

	if key == "" {
		err := fmt.Errorf("cannot sync V team if no V API key set")
		log.Error(err)
		return err
	}

	vt, err := vteamID.getTeamFromV(key)
	if err != nil {
		log.Error(err)
		return err
	}
	// log.Debug("V Sync", "team", vt)

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
				continue
			}
			err = model.VToDB(&agent)
			if err != nil {
				log.Error(err)
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
			// log.Debugw("ignoring agent already on team", "GID", agent.Gid, "team", teamID)
			continue
		}
		if _, ok := atv[agent.Gid]; ok {
			log.Infow("adding agent to team via V pull", "GID", agent.Gid, "team", teamID)
			if err := teamID.AddAgent(agent.Gid); err != nil {
				log.Info(err)
				continue
			}
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

var rolenames = map[uint8]string{
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

// Authorize checks if an agent is permitted to use Wasabee based on V data
// data is cached per-agent for one hour
// if an agent is not known at V, they are implicitly permitted
// if an agent is banned, blacklisted, etc at V, they are prohibited
func Authorize(gid model.GoogleID) bool {
	a, fetched, err := model.VFromDB(gid)
	if err != nil {
		log.Error(err)
		// do not block on db error
		return true
	}

	if a.Agent == "" || fetched.Before(time.Now().Add(0-time.Hour)) {
		net, err := trustCheck(gid)
		if err != nil {
			log.Error(err)
			// do not block on network error unless already listed as blacklisted in DB
			return !a.Blacklisted
		}
		// log.Debugw("v cache refreshed", "gid", gid, "data", net.Data)
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

// BulkImport imports all teams of which the GoogleID is an admin
// mode determines how many teams are created
// team = one Wasabee Team per V team -- roles are ignored
// role = one Wasabee Team per V team/role pair
// team data is populated from V at creation
func BulkImport(gid model.GoogleID, mode string) error {
	log.Debug("v BulkImport")

	key, err := gid.GetVAPIkey()
	if err != nil {
		log.Error(err)
		return err
	}

	teamsfromv, err := getTeams(gid)
	if err != nil {
		log.Error(err)
		return err
	}
	log.Debugw("bulk import", "teams", teamsfromv)

	// this might take a while, let the client get on their way
	go bulkImportWorker(gid, key, mode, teamsfromv)

	return nil
}

func bulkImportWorker(gid model.GoogleID, key string, mode string, teamsfromv *myTeams) error {
	type teamToMake struct {
		ID   vTeamID
		Role uint8
		Name string
	}
	var tomake []teamToMake

	for _, t := range teamsfromv.Teams {
		if !t.Admin {
			continue // agent is not admin at V, we won't link
		}

		switch mode {
		case "role":
			// one Wasabee team per each V team/role pair in use
			vt, err := t.TeamID.getTeamFromV(key)
			if err != nil {
				log.Error(err)
				continue
			}

			roles := make(map[uint8]bool)

			for _, a := range vt.Agents {
				for _, r := range a.Roles {
					if !roles[r.ID] { // first time we've seen this team/role
						roles[r.ID] = true
						tomake = append(tomake, teamToMake{
							ID:   t.TeamID,
							Role: r.ID,
							Name: fmt.Sprintf("%s (%s)", t.Name, rolenames[r.ID]),
						})
					}
				}
			}
		case "team":
			// one Wasabee team per V team
			tomake = append(tomake, teamToMake{
				ID:   t.TeamID,
				Role: 0,
				Name: fmt.Sprintf("%s (all)", t.Name),
			})
		default:
			log.Info("unknown mode")
		}
	}

	for _, t := range tomake {
		exists, err := model.VTeamExists(int64(t.ID), uint8(t.Role), gid)
		if err != nil {
			log.Error(err)
			continue
			// return err
		}
		if exists {
			continue
		}

		log.Infow("Creating Wasabee team for V team", "v team", t.ID, "role", t.Role)
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

// TelegramSearch queries V for information about an agent by TelegramID
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

func addToRemote(gid messaging.GoogleID, teamID messaging.TeamID) error {
	// V's api doesn't support this?
	// log.Info("v add to remote not written")
	return nil
}

func removeFromRemote(gid messaging.GoogleID, teamID messaging.TeamID) error {
	// V's api doesn't support this?
	// log.Info("v remove from remote not written")
	return nil
}
