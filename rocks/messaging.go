package rocks

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"

	"github.com/wasabee-project/Wasabee-Server/config"
	"github.com/wasabee-project/Wasabee-Server/log"
	"github.com/wasabee-project/Wasabee-Server/messaging"
	"github.com/wasabee-project/Wasabee-Server/model"
)

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

	ctx, cancel := context.WithTimeout(context.Background(), holdtime)
	defer cancel()
	if err := limiter.Wait(ctx); err != nil {
		log.Infow("timeout waiting on .rocks rate limiter", "GID", gid)
	}

	c := config.Get().Rocks
	client := &http.Client{
		Timeout: holdtime,
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

		// try to alert the team owner
		owner, _ := t.Owner()
		msg := fmt.Sprintf("unable to add agent to rocks community for teamID: %s. Check that your community ID and api key are correct.", teamID)
		messaging.SendMessage(messaging.GoogleID(owner), msg)

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

	ctx, cancel := context.WithTimeout(context.Background(), holdtime)
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
		Timeout: holdtime,
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
