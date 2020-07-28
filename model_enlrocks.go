package wasabee

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/time/rate"
)

// RocksCommunityNotice is sent from a community when an agent is added or removed
// consumed by RocksCommunitySync function below
type RocksCommunityNotice struct {
	Community string     `json:"community"`
	Action    string     `json:"action"`
	User      RocksAgent `json:"user"`
}

// RocksCommunityResponse is returned from a query request
type RocksCommunityResponse struct {
	Community  string     `json:"community"`
	Title      string     `json:"title"`
	Members    []GoogleID `json:"members"`
	Moderators []GoogleID `json:"moderators"`
	User       RocksAgent `json:"user"` // (Members,Moderators || User) present, not both
}

// RocksAgent is the data sent by enl.rocks -- the version sent in the CommunityResponse is different, but close enough for our purposes
type RocksAgent struct {
	Gid      GoogleID `json:"gid"`
	TGId     int64    `json:"tgid"`
	Agent    string   `json:"agentid"`
	Verified bool     `json:"verified"`
	Smurf    bool     `json:"smurf"`
	Fullname string   `json:"name"`
}

// Rocksconfig contains configuration for interacting with the enl.rocks APIs.
type Rocksconfig struct {
	// APIKey is the API Key for enl.rocks.
	APIKey string
	// CommunityEndpoint is the API endpoint for viewing community membership
	CommunityEndpoint string
	// StatusEndpoint is the API endpoint for getting user status
	StatusEndpoint string
	configured     bool
	limiter        *rate.Limiter
}

var rocks Rocksconfig

// SetEnlRocks is called from main() to initialize the config
func SetEnlRocks(input Rocksconfig) {
	Log.Debugf("enl.rocks API Key: %s", input.APIKey)
	rocks.APIKey = input.APIKey

	if len(input.CommunityEndpoint) != 0 {
		rocks.CommunityEndpoint = input.CommunityEndpoint
	} else {
		rocks.CommunityEndpoint = "https://enlightened.rocks/comm/api/membership"
	}

	if len(input.StatusEndpoint) != 0 {
		rocks.StatusEndpoint = input.StatusEndpoint
	} else {
		rocks.StatusEndpoint = "https://enlightened.rocks/api/user/status"
	}

	rocks.limiter = rate.NewLimiter(rate.Limit(0.5), 60)
	rocks.configured = true
}

// GetEnlRocks is used for templates to determine if .Rocks is enabled
func GetEnlRocks() bool {
	return rocks.configured
}

// RocksSearch checks a agent at enl.rocks and populates a RocksAgent
func RocksSearch(id AgentID, agent *RocksAgent) error {
	if !rocks.configured {
		return nil
	}

	searchID := id.String()
	return rockssearch(searchID, agent)
}

// rockssearch stands behind the wraper functions and checks a agent at enl.rocks and populates a RocksAgent
func rockssearch(searchID string, agent *RocksAgent) error {
	if !rocks.configured {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), GetTimeout(3*time.Second))
	defer cancel()
	if err := rocks.limiter.Wait(ctx); err != nil {
		Log.Notice(err)
		// just keep going
	}

	apiurl := fmt.Sprintf("%s/%s?apikey=%s", rocks.StatusEndpoint, searchID, rocks.APIKey)
	req, err := http.NewRequest("GET", apiurl, nil)
	if err != nil {
		Log.Error(err)
		return err
	}
	client := &http.Client{
		Timeout: GetTimeout(3 * time.Second),
	}
	resp, err := client.Do(req)
	if err != nil {
		Log.Error(err)
		return err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		Log.Error(err)
		return err
	}

	// Log.Debug(string(body))
	err = json.Unmarshal(body, &agent)
	if err != nil {
		Log.Error(err)
		return err
	}
	return nil
}

// RocksUpdate updates the database to reflect an agent's current status at enl.rocks.
// It should be called whenever a agent logs in via a new service (if appropriate); currently only https does.
func RocksUpdate(id AgentID, agent *RocksAgent) error {
	if !rocks.configured {
		return nil
	}
	gid, err := id.Gid()
	if err != nil {
		Log.Error(err)
		return nil
	}

	if agent.Agent != "" {
		// Log.Debug("Updating Rocks data for ", agent.Agent)
		_, err := db.Exec("UPDATE agent SET iname = ?, RocksVerified = ? WHERE gid = ?", agent.Agent, agent.Verified, gid)

		// doppelkeks error
		if err != nil && strings.Contains(err.Error(), "Error 1062") {
			iname := "%s-doppel"
			Log.Error("dupliate ingress agent name detected for [%], storing as [%]", agent.Agent, iname)
			if _, err := db.Exec("UPDATE agent SET iname = ?, RocksVerified = ? WHERE gid = ?", iname, agent.Verified, gid); err != nil {
				Log.Error(err)
				return err
			}
		} else if err != nil {
			Log.Error(err)
			return err
		}

		// we trust .rocks to verify telegram info; if it is not already set for a agent, just import it.
		if agent.TGId > 0 { // negative numbers are group chats, 0 is invalid
			if _, err := db.Exec("INSERT IGNORE INTO telegram (telegramID, telegramName, gid, verified) VALUES (?, 'unused', ?, 1)", agent.TGId, gid); err != nil {
				Log.Error(err)
				return err
			}

		}
	}
	return nil
}

// RocksCommunitySync is called from the https server when it receives a push notification
func RocksCommunitySync(msg json.RawMessage) error {
	// check the source? is the community key enough for this? I don't think so
	var rc RocksCommunityNotice
	err := json.Unmarshal(msg, &rc)
	if err != nil {
		Log.Error(err)
		return err
	}

	team, err := RocksTeamID(rc.Community)
	if err != nil {
		Log.Error(err)
		return err
	}
	if team == "" {
		return nil
	}

	if rc.Action == "onJoin" {
		_, err = rc.User.Gid.IngressName()
		if err != nil && err == sql.ErrNoRows {
			Log.Infof("Importing previously unknown agent: %s", rc.User.Gid)
			_, err = rc.User.Gid.InitAgent()
			if err != nil {
				Log.Error(err)
				return err
			}
		}

		err := team.AddAgent(rc.User.Gid)
		if err != nil {
			Log.Error(err)
			return err
		}
	} else {
		err := team.RemoveAgent(rc.User.Gid)
		if err != nil {
			Log.Error(err)
			return err
		}
	}

	return nil
}

// RocksCommunityMemberPull grabs the member list from the associated community at enl.rocks and adds each agent to the team
func (teamID TeamID) RocksCommunityMemberPull() error {
	if !rocks.configured {
		return nil
	}

	rc, err := teamID.rocksComm()
	if err != nil {
		return err
	}
	if rc == "" {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), GetTimeout(3*time.Second))
	defer cancel()
	if err := rocks.limiter.Wait(ctx); err != nil {
		Log.Notice(err)
		// just keep going
	}

	apiurl := fmt.Sprintf("%s?key=%s", rocks.CommunityEndpoint, rc)
	req, err := http.NewRequest("GET", apiurl, nil)
	if err != nil {
		Log.Error(err)
		return err
	}
	client := &http.Client{
		Timeout: GetTimeout(3 * time.Second),
	}
	resp, err := client.Do(req)
	if err != nil {
		Log.Error(err)
		return err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		Log.Error(err)
		return err
	}

	// Log.Debug(string(body))
	var rr RocksCommunityResponse
	err = json.Unmarshal(body, &rr)
	if err != nil {
		Log.Error(err)
		return err
	}

	for _, agent := range rr.Members {
		_, err = agent.IngressName()
		if err != nil && err == sql.ErrNoRows {
			Log.Infof("Importing previously unknown agent: %s", agent)
			_, err = agent.InitAgent() // add agent to system if they don't already exist
			if err != nil {
				Log.Notice(err)
				continue
			}
		}
		if err != nil && err != sql.ErrNoRows {
			Log.Notice(err)
			continue
		}

		err = teamID.AddAgent(agent)
		if err != nil {
			Log.Notice(err)
			continue
		}
	}
	return nil
}

// RocksTeamID takes a rocks community ID and returns an associated teamID
func RocksTeamID(rockscomm string) (TeamID, error) {
	var t TeamID
	err := db.QueryRow("SELECT teamID FROM team WHERE rockscomm = ?", rockscomm).Scan(&t)
	if err != nil && err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		Log.Error(err)
		return "", err
	}
	return t, nil
}

type rocksPushResponse struct {
	Error   string `json:"error"`
	Success bool   `json:"success"`
}

// AddToRemoteRocksCommunity adds an agent to a community at .rocks IF that community has API enabled.
func (gid GoogleID) AddToRemoteRocksCommunity(teamID TeamID) error {
	rc, err := teamID.rocksComm()
	if err != nil {
		return err
	}
	if rc == "" {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), GetTimeout(3*time.Second))
	defer cancel()
	if err := rocks.limiter.Wait(ctx); err != nil {
		Log.Notice(err)
		// just keep going
	}

	// XXX use NewRequest/client
	apiurl := fmt.Sprintf("%s/%s?key=%s", rocks.CommunityEndpoint, gid, rc)
	// #nosec
	resp, err := http.PostForm(apiurl, url.Values{"Agent": {gid.String()}})
	if err != nil {
		Log.Error(err)
		return err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		Log.Error(err)
		return err
	}

	var rr rocksPushResponse
	err = json.Unmarshal(body, &rr)
	if err != nil {
		Log.Error(err)
		Log.Error(string(body))
	}
	if !rr.Success {
		Log.Error(rr.Error)
	}
	return nil
}

// RemoveFromRemoteRocksCommunity removes an agent from a Rocks Community IF that community has API enabled.
func (gid GoogleID) RemoveFromRemoteRocksCommunity(teamID TeamID) error {
	rc, err := teamID.rocksComm()
	if err != nil {
		return err
	}
	if rc == "" {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := rocks.limiter.Wait(ctx); err != nil {
		Log.Notice(err)
		// just keep going
	}

	apiurl := fmt.Sprintf("%s/%s?key=%s", rocks.CommunityEndpoint, gid, rc)
	req, err := http.NewRequest("DELETE", apiurl, nil)
	if err != nil {
		Log.Error(err)
		return err
	}
	client := &http.Client{
		Timeout: GetTimeout(3 * time.Second),
	}
	resp, err := client.Do(req)
	if err != nil {
		Log.Error(err)
		return err
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		Log.Error(err)
		return err
	}

	var rr rocksPushResponse
	err = json.Unmarshal(body, &rr)
	if err != nil {
		Log.Error(err)
		return err
	}
	if !rr.Success {
		err = fmt.Errorf(rr.Error)
		Log.Error(err)
		return err
	}
	return nil
}

// rocksComm returns a rocks key for a TeamID
func (teamID TeamID) rocksComm() (string, error) {
	if !rocks.configured {
		return "", nil
	}

	var rc sql.NullString
	err := db.QueryRow("SELECT rockskey FROM team WHERE teamID = ?", teamID).Scan(&rc)
	if err != nil {
		Log.Error(err)
		return "", err
	}
	if !rc.Valid {
		return "", nil
	}
	return rc.String, nil
}
