package rocks

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"golang.org/x/time/rate"

	"github.com/wasabee-project/Wasabee-Server/auth"
	"github.com/wasabee-project/Wasabee-Server/config"
	"github.com/wasabee-project/Wasabee-Server/log"
	"github.com/wasabee-project/Wasabee-Server/messaging"
	"github.com/wasabee-project/Wasabee-Server/model"
)

var holdtime = 3 * time.Second
var limiter *rate.Limiter

// Start is called from main() to initialize the config
func Start(ctx context.Context) {
	if config.Get().Rocks.APIKey == "" {
		log.Debugw("startup", "message", "Rocks not configured, not starting")
		return
	}

	limiter = rate.NewLimiter(rate.Limit(0.5), 10)

	rocks := config.Subrouter("/rocks")
	rocks.HandleFunc("", rocksCommunityRoute).Methods("POST")
	// rocks.NotFoundHandler = http.HandlerFunc(notFoundJSONRoute)
	// rocks.MethodNotAllowedHandler = http.HandlerFunc(notFoundJSONRoute)
	// rocks.PathPrefix("/rocks").HandlerFunc(notFoundJSONRoute)

	// let the messaging susbsystem know we exist and how to use us
	messaging.RegisterMessageBus("Rocks", messaging.Bus{
		AddToRemote:      addToRemote,
		RemoveFromRemote: removeFromRemote,
	})

	auth.RegisterAuthProvider(&Rocks{})

	config.SetRocksRunning(true)

	// there is no reason to stay running now -- this costs nothing
	<-ctx.Done()

	log.Infow("shutdown", "message", "rocks shutting down")
	config.SetRocksRunning(false)
}

// Search checks a agent at enl.rocks and returns an Agent
func Search(id string) (*model.RocksAgent, error) {
	if !config.IsRocksRunning() {
		return &model.RocksAgent{}, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), holdtime)
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
		Timeout: holdtime,
	}
	resp, err := client.Do(req)
	if err != nil {
		// do not leak API key to logs
		err := fmt.Errorf("error executing .rocks request")
		log.Errorw(err.Error(), "search", id)
		return &model.RocksAgent{}, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
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
		r := &Rocks{}
		_ = r.Authorize(rc.User.Gid)
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

	ctx, cancel := context.WithTimeout(context.Background(), holdtime)
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
		Timeout: holdtime,
	}
	resp, err := client.Do(req)
	if err != nil {
		err := fmt.Errorf("error executing community pull request")
		log.Error(err)
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
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
