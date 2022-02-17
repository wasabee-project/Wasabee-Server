package v

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/wasabee-project/Wasabee-Server/auth"
	"github.com/wasabee-project/Wasabee-Server/config"
	"github.com/wasabee-project/Wasabee-Server/log"
	"github.com/wasabee-project/Wasabee-Server/messaging"
	"github.com/wasabee-project/Wasabee-Server/model"
)

// Start is called start V integration
func Start(ctx context.Context) {
	if config.Get().V.APIKey == "" {
		log.Debugw("startup", "message", "V not configured, not starting")
		return
	}

	// v.enl.one posting when a team changes -- triggers a pull of all teams linked to the V team #
	v := config.Subrouter("/v")
	v.HandleFunc("/{teamID}", vTeamRoute).Methods("POST")
	// v.NotFoundHandler = http.HandlerFunc(notFoundJSONRoute)
	// v.MethodNotAllowedHandler = http.HandlerFunc(notFoundJSONRoute)
	// v.PathPrefix("/v").HandlerFunc(notFoundJSONRoute)

	messaging.RegisterMessageBus("v.enl.one", messaging.Bus{
		AddToRemote:      addToRemote,
		RemoveFromRemote: removeFromRemote,
	})

	auth.RegisterAuthProvider(&V{})

	config.SetVRunning(true)

	// there is no reason to stay running now -- this costs nothing
	<-ctx.Done()

	log.Infow("shutdown", "message", "v shutting down")
	config.SetVRunning(false)
}

// trustCheck checks a agent at V and populates a trustResult
func trustCheck(id model.GoogleID) (*trustResult, error) {
	tr := trustResult{}

	if !config.IsVRunning() {
		return &tr, nil
	}
	if id == "" {
		return &tr, fmt.Errorf("empty trustCheck value")
	}

	vc := config.Get().V
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

	if err := json.NewDecoder(resp.Body).Decode(&tr); err != nil {
		log.Error(err)
		return &tr, err
	}

	if tr.Status != "ok" && tr.Message != "Agent not found" {
		err = fmt.Errorf(tr.Message)
		log.Info(err)
		return &tr, err
	}
	tr.Data.Gid = id // V isn't sending the GIDs
	return &tr, nil
}

// geTeams pulls a list of teams the agent is on at V
func getTeams(gid model.GoogleID) (*myTeams, error) {
	v := myTeams{}

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

	vc := config.Get().V
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

	if err := json.NewDecoder(resp.Body).Decode(&v); err != nil {
		log.Error(err)
		return &v, err
	}
	return &v, nil
}

func (vteamID vTeamID) getTeamFromV(key string) (*teamResult, error) {
	vt := teamResult{}

	if vteamID == 0 {
		return &vt, nil
	}

	if key == "" {
		err := fmt.Errorf("cannot get V team if no V API key set")
		log.Error(err)
		return &vt, err
	}

	vc := config.Get().V
	apiurl := fmt.Sprintf("%s/%d?apikey=%s", vc.TeamEndpoint, vteamID, key)
	req, err := http.NewRequest("GET", apiurl, nil)
	if err != nil {
		// do not leak API key to logs
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

	if err := json.NewDecoder(resp.Body).Decode(&vt); err != nil {
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
func Sync(ctx context.Context, teamID model.TeamID, key string) error {
	// XXX put ctx.Done() checks in the loops....

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
			if err := agent.Gid.FirstLogin(); err != nil {
				log.Error(err)
				continue
			}
			// #nosec -- model.VToDB isn't async, aliasing doesn't matter here
			if err := model.VToDB(&agent); err != nil {
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
			// log.Infow("adding agent to team via V pull", "GID", agent.Gid, "team", teamID)
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

			if err = teamID.RemoveAgent(a.Gid); err != nil {
				log.Error(err)
			}
		}
	}
	return nil
}

// BulkImport imports all teams of which the GoogleID is an admin
// mode determines how many teams are created
// team = one Wasabee Team per V team -- roles are ignored
// role = one Wasabee Team per V team/role pair
// team data is populated from V at creation
func BulkImport(gid model.GoogleID, mode string) error {
	log.Infow("v BulkImport", "gid", gid, "mode", mode)

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
	// log.Debugw("bulk import", "teams", teamsfromv)

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
		err = Sync(context.Background(), teamID, key)
		if err != nil {
			log.Error(err)
			return err
		}
	}
	return nil
}

// TelegramSearch queries V for information about an agent by TelegramID
func TelegramSearch(tgid model.TelegramID) (*model.VAgent, error) {
	br := bulkResult{}

	if !config.IsVRunning() {
		return &model.VAgent{}, nil
	}

	vc := config.Get().V
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

	if err := json.NewDecoder(resp.Body).Decode(&br); err != nil {
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
