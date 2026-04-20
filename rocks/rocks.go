package rocks

import (
	"context"
	"encoding/json"
	"fmt"
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

// Start initializes the Rocks subsystem
func Start(ctx context.Context) {
	if config.Get().Rocks.APIKey == "" {
		log.Debugw("startup", "message", "Rocks not configured, not starting")
		return
	}

	// 0.5/sec = 1 request every 2 seconds, with a burst of 10
	limiter = rate.NewLimiter(rate.Limit(0.5), 10)

	// Messaging registration
	messaging.RegisterMessageBus("Rocks", messaging.Bus{
		AddToRemote:      addToRemote,
		RemoveFromRemote: removeFromRemote,
	})

	// Authorization registration
	auth.RegisterAuthProvider(&Rocks{})

	config.SetRocksRunning(true)

	<-ctx.Done()

	log.Infow("shutdown", "message", "rocks shutting down")
	config.SetRocksRunning(false)
}

// Search checks an agent at enl.rocks and returns an Agent
func Search(ctx context.Context, id string) (*model.RocksAgent, error) {
	agent := model.RocksAgent{}

	if !config.IsRocksRunning() {
		return &agent, nil
	}

	// Use the passed context for the rate limiter
	if err := limiter.Wait(ctx); err != nil {
		return &agent, err
	}

	c := config.Get().Rocks
	apiurl := fmt.Sprintf("%s/%s?apikey=%s", c.StatusEndpoint, id, c.APIKey)

	// Create request with the provided context
	req, err := http.NewRequestWithContext(ctx, "GET", apiurl, nil)
	if err != nil {
		return &agent, fmt.Errorf("error establishing .rocks request")
	}

	client := &http.Client{
		Timeout: holdtime,
	}
	resp, err := client.Do(req)
	if err != nil {
		return &agent, fmt.Errorf("error executing .rocks request")
	}
	defer resp.Body.Close()

	if err := json.NewDecoder(resp.Body).Decode(&agent); err != nil {
		return &agent, err
	}
	return &agent, nil
}

// CommunitySync is called when a push notification is received from enl.rocks
func CommunitySync(ctx context.Context, msg json.RawMessage) error {
	var rc communityNotice
	if err := json.Unmarshal(msg, &rc); err != nil {
		return err
	}

	teamID, err := model.RocksCommunityToTeam(ctx, rc.Community)
	if err != nil {
		return err
	}

	// Handle first-time login/discovery from sync notice
	if !rc.User.Gid.Valid(ctx) {
		if err := rc.User.Gid.FirstLogin(ctx); err != nil {
			return err
		}
		r := &Rocks{}
		_ = r.Authorize(ctx, rc.User.Gid)
	}

	if rc.Action == "onJoin" {
		inteam, err := rc.User.Gid.AgentInTeam(ctx, teamID)
		if err != nil || inteam {
			return err
		}
		if err := teamID.AddAgent(ctx, rc.User.Gid); err != nil {
			return err
		}

		// Notifications
		owner, _ := teamID.Owner(ctx)
		agent, _ := rc.User.Gid.IngressName(ctx)
		team, _ := teamID.Name(ctx)
		messaging.SendMessage(ctx, messaging.GoogleID(owner), fmt.Sprintf("added %s to %s via rocks community join", agent, team))
	} else {
		if err := teamID.RemoveAgent(ctx, rc.User.Gid); err != nil {
			return err
		}
	}

	if rc.TGId > 0 && rc.TGName != "" {
		_ = rc.TGId.SetName(ctx, rc.TGName)
	}

	return nil
}

// CommunityMemberPull syncs the entire Rocks community membership to the Wasabee team
func CommunityMemberPull(ctx context.Context, teamID model.TeamID) error {
	cid, err := teamID.RocksKey(ctx)
	if err != nil || cid == "" {
		return err
	}

	if err := limiter.Wait(ctx); err != nil {
		return err
	}

	c := config.Get().Rocks
	apiurl := fmt.Sprintf("%s?key=%s", c.CommunityEndpoint, cid)
	req, err := http.NewRequestWithContext(ctx, "GET", apiurl, nil)
	if err != nil {
		return err
	}

	client := &http.Client{Timeout: holdtime}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	rr := communityResponse{}
	if err := json.NewDecoder(resp.Body).Decode(&rr); err != nil {
		return err
	}

	for _, gid := range rr.Members {
		if inteam, _ := gid.AgentInTeam(ctx, teamID); !inteam {
			_ = teamID.AddAgent(ctx, gid)
		}
	}
	return nil
}
