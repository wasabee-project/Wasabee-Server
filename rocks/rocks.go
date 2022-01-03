package rocks

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"time"

	"golang.org/x/time/rate"

	"github.com/wasabee-project/Wasabee-Server/config"
	"github.com/wasabee-project/Wasabee-Server/log"
	"github.com/wasabee-project/Wasabee-Server/messaging"
	"github.com/wasabee-project/Wasabee-Server/model"
)

// communityNotice is sent from a community when an agent is added or removed
// consumed by RocksCommunitySync function below
type communityNotice struct {
	Community string           `json:"community"`
	Action    string           `json:"action"`
	User      agent            `json:"user"`
	TGId      model.TelegramID `json:"tg_id"`
	TGName    string           `json:"tg_user"`
}

// communityResponse is returned from a query request
type communityResponse struct {
	Community  string           `json:"community"`
	Title      string           `json:"title"`
	Members    []model.GoogleID `json:"members"`    // googleID
	Moderators []string         `json:"moderators"` // googleID
	User       agent            `json:"user"`       // (Members,Moderators || User) present, not both
	Error      string           `json:"error"`
}

// Agent is the data sent by enl.rocks -- the version sent in the communityResponse is different, but close enough for our purposes
type agent struct {
	Gid      model.GoogleID `json:"gid"`
	TGId     int64          `json:"tgid"`
	Agent    string         `json:"agentid"`
	Verified bool           `json:"verified"`
	Smurf    bool           `json:"smurf"`
	// Fullname string `json:"name"`
}

// sent by rocks on community pushes
type rocksPushResponse struct {
	Error   string `json:"error"`
	Success bool   `json:"success"`
}

var limiter *rate.Limiter

// Start is called from main() to initialize the config
func Start(ctx context.Context) {
	if config.Get().Rocks.APIKey == "" {
		log.Debug("Rocks not configured, not starting")
		return
	}

	limiter = rate.NewLimiter(rate.Limit(0.5), 10)

	// let the messaging susbsystem know we exist and how to use us
	messaging.RegisterMessageBus("Rocks", messaging.Bus{
		AddToRemote:      addToRemote,
		RemoveFromRemote: removeFromRemote,
	})
	config.SetRocksRunning(true)

	// there is no reason to stay running now -- this costs nothing
	<-ctx.Done()

	log.Infow("Shutdown", "message", "rocks shutting down")
	config.SetRocksRunning(false)
}

// Search checks a agent at enl.rocks and returns an Agent
func Search(id string) (*model.RocksAgent, error) {
	if !config.IsRocksRunning() {
		return &model.RocksAgent{}, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), (3 * time.Second))
	defer cancel()
	if err := limiter.Wait(ctx); err != nil {
		log.Warn(err)
		// just keep going
	}

	c := config.Get().Rocks
	apiurl := fmt.Sprintf("%s/%s?apikey=%s", c.StatusEndpoint, id, c.APIKey)
	req, err := http.NewRequest("GET", apiurl, nil)
	if err != nil {
		// do not leak API key to logs
		err := fmt.Errorf("error establishing .rocks request")
		log.Errorw(err.Error(), "search", id)
		return &model.RocksAgent{}, err
	}
	client := &http.Client{
		Timeout: (3 * time.Second),
	}
	resp, err := client.Do(req)
	if err != nil {
		// do not leak API key to logs
		err := fmt.Errorf("error executing .rocks request")
		log.Errorw(err.Error(), "search", id)
		return &model.RocksAgent{}, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Error(err)
		return &model.RocksAgent{}, err
	}

	var agent model.RocksAgent
	err = json.Unmarshal(body, &agent)
	if err != nil {
		log.Error(err)
		return &agent, err
	}
	return &agent, nil
}

// CommunitySync is called from the https server when it receives a push notification
func CommunitySync(msg json.RawMessage) error {
	log.Debug("rocks community request", "data", string(msg))

	// check the source? is the community key enough for this? I don't think so
	var rc communityNotice
	err := json.Unmarshal(msg, &rc)
	if err != nil {
		log.Error(err)
		return err
	}

	teamID, err := model.RocksCommunityToTeam(rc.Community)
	if err != nil {
		log.Error(err)
		return err
	}

	// already known?
	if !rc.User.Gid.Valid() {
		if err := rc.User.Gid.FirstLogin(); err != nil {
			log.Error(err)
			return err
		}
		_ = Authorize(rc.User.Gid)
	}

	if rc.Action == "onJoin" {
		if inteam, err := rc.User.Gid.AgentInTeam(teamID); inteam {
			return err
		}
		err := teamID.AddAgent(rc.User.Gid)
		if err != nil {
			log.Error(err)
			return err
		}
	} else {
		if err := teamID.RemoveAgent(rc.User.Gid); err != nil {
			log.Error(err)
			return err
		}
	}

	if rc.TGId > 0 && rc.TGName != "" {
		if err := rc.TGId.SetName(rc.TGName); err != nil {
			log.Error(err)
			return err
		}
	}

	return nil
}

// CommunityMemberPull grabs the member list from the associated community at enl.rocks and adds each agent to the team
func CommunityMemberPull(teamID model.TeamID) error {
	cid, err := teamID.RocksKey()
	if err != nil {
		log.Error(err)
		return err
	}
	if cid == "" {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), (3 * time.Second))
	defer cancel()
	if err := limiter.Wait(ctx); err != nil {
		log.Warn(err)
		// just keep going
	}

	c := config.Get().Rocks
	apiurl := fmt.Sprintf("%s?key=%s", c.CommunityEndpoint, cid)
	req, err := http.NewRequest("GET", apiurl, nil)
	if err != nil {
		err := fmt.Errorf("error establishing community pull request")
		log.Error(err)
		return err
	}
	client := &http.Client{
		Timeout: (3 * time.Second),
	}
	resp, err := client.Do(req)
	if err != nil {
		err := fmt.Errorf("error executing community pull request")
		log.Error(err)
		return err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Error(err)
		return err
	}

	var rr communityResponse
	err = json.Unmarshal(body, &rr)
	if err != nil {
		log.Error(err)
		return err
	}
	if rr.Error != "" {
		log.Error(rr.Error)
		return err
	}
	// log.Debugw("rocks sync", "response", rr)

	for _, gid := range rr.Members {
		if inteam, _ := gid.AgentInTeam(teamID); inteam {
			continue
		}
		log.Debugw("rocks sync", "adding", gid)
		if err := teamID.AddAgent(gid); err != nil {
			log.Info(err)
		}
	}
	return nil
}

// addToRemote adds an agent to a Rocks Community IF that community has API enabled.
func addToRemote(gid messaging.GoogleID, teamID messaging.TeamID) error {
	// log.Debug("add to remote rocks", "gid", gid, "teamID", teamID)
	t := model.TeamID(teamID)
	cid, err := t.RocksKey()
	if err != nil {
		log.Error(err)
		return err
	}

	if cid == "" || gid == "" {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), (3 * time.Second))
	defer cancel()
	if err := limiter.Wait(ctx); err != nil {
		log.Infow("timeout waiting on .rocks rate limiter", "GID", gid)
	}

	c := config.Get().Rocks
	client := &http.Client{
		Timeout: (3 * time.Second),
	}
	apiurl := fmt.Sprintf("%s/%s?key=%s", c.CommunityEndpoint, gid, cid)
	// #nosec
	resp, err := client.PostForm(apiurl, url.Values{"Agent": {string(gid)}})
	if err != nil {
		err := fmt.Errorf("error adding agent to rocks community")
		log.Errorw(err.Error(), "GID", gid)
		return err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Error(err)
		return err
	}

	var rr rocksPushResponse
	err = json.Unmarshal(body, &rr)
	if err != nil {
		log.Error(err)
		log.Debug(string(body))
	}
	if !rr.Success {
		log.Errorw("unable to add to remote rocks team", "teamID", teamID, "gid", gid, "cid", cid, "error", rr.Error)
		if rr.Error == "Invalid key" {
			c, _ := t.RocksCommunity()
			_ = t.SetRocks("", c) // unlink
		}
	}
	return nil
}

// removeFromRemote removes an agent from a Rocks Community IF that community has API enabled.
func removeFromRemote(gid messaging.GoogleID, teamID messaging.TeamID) error {
	// log.Debugw("remove from remote rocks", "gid", gid, "teamID", teamID)
	t := model.TeamID(teamID)
	cid, err := t.RocksKey()
	if err != nil {
		log.Error(err)
		return err
	}
	if cid == "" || gid == "" {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := limiter.Wait(ctx); err != nil {
		log.Info(err)
		// just keep going
	}

	c := config.Get().Rocks
	apiurl := fmt.Sprintf("%s/%s?key=%s", c.CommunityEndpoint, gid, cid)
	req, err := http.NewRequest("DELETE", apiurl, nil)
	if err != nil {
		log.Error(err)
		return err
	}
	client := &http.Client{
		Timeout: (3 * time.Second),
	}
	resp, err := client.Do(req)
	if err != nil {
		// default err leaks API key to logs
		err := fmt.Errorf("error removing agent from .rocks community")
		log.Errorw(err.Error(), "GID", gid)
		return err
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Error(err)
		return err
	}

	var rr rocksPushResponse
	err = json.Unmarshal(body, &rr)
	if err != nil {
		log.Error(err)
		return err
	}
	if !rr.Success {
		err = fmt.Errorf(rr.Error)
		log.Error(err)
		if rr.Error == "Invalid key" {
			c, _ := t.RocksCommunity()
			_ = t.SetRocks("", c) // unlink
		}
		return err
	}
	return nil
}

// Authorize checks Rocks to see if an agent is permitted to use Wasabee
// responses are cached for an hour
// unknown agents are permitted implicitly
// if an agent is marked as smurf at rocks, they are prohibited
func Authorize(gid model.GoogleID) bool {
	a, fetched, err := model.RocksFromDB(gid)
	if err != nil {
		log.Error(err)
		// do not block on db error
		return true
	}

	// log.Debugw("rocks from cache", "gid", gid, "data", a)
	if a.Agent == "" || fetched.Before(time.Now().Add(0-time.Hour)) {
		net, err := Search(string(gid))
		if err != nil {
			log.Error(err)
			return !a.Smurf // do not block on network error unless already listed as a smurf in the cache
		}
		// log.Debugw("rocks cache refreshed", "gid", gid, "data", net)
		if net.Gid == "" {
			// log.Debugw("Rocks returned a result without a GID, adding it", "gid", gid, "result", net)
			net.Gid = gid
		}
		err = model.RocksToDB(net)
		if err != nil {
			log.Error(err)
		}
		a = net
	}

	if a.Agent != "" && a.Smurf {
		log.Warnw("access denied", "GID", gid, "reason", "listed as smurf at enl.rocks")
		return false
	}

	// not in rocks is not sufficient to block
	return true
}
