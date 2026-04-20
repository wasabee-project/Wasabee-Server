package rocks

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/wasabee-project/Wasabee-Server/config"
	"github.com/wasabee-project/Wasabee-Server/log"
	"github.com/wasabee-project/Wasabee-Server/messaging"
	"github.com/wasabee-project/Wasabee-Server/model"
)

// addToRemote adds an agent to a Rocks Community IF that community has API enabled.
func addToRemote(ctx context.Context, gid messaging.GoogleID, teamID messaging.TeamID) error {
	t := model.TeamID(teamID)
	// Pass ctx to the model call
	cid, err := t.RocksKey(ctx)
	if err != nil {
		log.Error(err)
		return err
	}

	if cid == "" || gid == "" {
		return nil
	}

	// Respect the passed context for rate limiting
	if err := limiter.Wait(ctx); err != nil {
		log.Infow("timeout waiting on .rocks rate limiter", "GID", gid)
		return err
	}

	c := config.Get().Rocks
	apiurl := fmt.Sprintf("%s/%s?key=%s", c.CommunityEndpoint, gid, cid)

	// Create request with context
	req, err := http.NewRequestWithContext(ctx, "POST", apiurl, nil)
	if err != nil {
		log.Error(err)
		return err
	}
	req.PostForm = url.Values{"Agent": {string(gid)}}

	client := &http.Client{
		Timeout: holdtime, // Keep the safety timeout
	}

	resp, err := client.Do(req)
	if err != nil {
		err := fmt.Errorf("error adding agent to rocks community")
		log.Errorw(err.Error(), "GID", gid)
		return err
	}
	defer resp.Body.Close()

	rr := rocksPushResponse{}
	if err := json.NewDecoder(resp.Body).Decode(&rr); err != nil {
		log.Error(err)
		return err
	}

	if !rr.Success {
		log.Errorw("unable to add to remote rocks team", "teamID", teamID, "gid", gid, "cid", cid, "error", rr.Error)

		owner, _ := t.Owner(ctx)
		msg := fmt.Sprintf("unable to add agent to rocks community for teamID: %s. Check that your community ID and api key are correct.", teamID)
		messaging.SendMessage(ctx, messaging.GoogleID(owner), msg)

		if rr.Error == "Invalid key" {
			community, _ := t.RocksCommunity(ctx)
			_ = t.SetRocks(ctx, "", community) // unlink
		}
	}
	return nil
}

// removeFromRemote removes an agent from a Rocks Community IF that community has API enabled.
func removeFromRemote(ctx context.Context, gid messaging.GoogleID, teamID messaging.TeamID) error {
	t := model.TeamID(teamID)
	cid, err := t.RocksKey(ctx)
	if err != nil {
		log.Error(err)
		return err
	}
	if cid == "" || gid == "" {
		return nil
	}

	if err := limiter.Wait(ctx); err != nil {
		log.Info(err)
	}

	c := config.Get().Rocks
	apiurl := fmt.Sprintf("%s/%s?key=%s", c.CommunityEndpoint, gid, cid)

	req, err := http.NewRequestWithContext(ctx, "DELETE", apiurl, nil)
	if err != nil {
		log.Error(err)
		return err
	}

	client := &http.Client{
		Timeout: holdtime,
	}
	resp, err := client.Do(req)
	if err != nil {
		err := fmt.Errorf("error removing agent from .rocks community")
		log.Errorw(err.Error(), "GID", gid)
		return err
	}
	defer resp.Body.Close()

	rr := rocksPushResponse{}
	if err := json.NewDecoder(resp.Body).Decode(&rr); err != nil {
		log.Error(err)
		return err
	}

	if !rr.Success {
		err = fmt.Errorf("%s", rr.Error)
		log.Error(err)
		if rr.Error == "Invalid key" {
			community, _ := t.RocksCommunity(ctx)
			_ = t.SetRocks(ctx, "", community) // unlink
		}
		return err
	}
	return nil
}
