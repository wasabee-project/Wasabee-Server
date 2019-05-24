package wasabi

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"golang.org/x/time/rate"
	"io/ioutil"
	"net/http"
	"net/url"
	"time"
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

type rocksconfig struct {
	rocksAPIKey string
	configured  bool
	limiter     *rate.Limiter
}

const commAPIEndpoint = "https://enlightened.rocks/comm/api/membership/"
const rocksAPIEndpoint = "https://enlightened.rocks/api/user/status/"

var rocks rocksconfig

// SetEnlRocks is called from main() to initialize the config
func SetEnlRocks(key string) {
	Log.Debugf("enl.rocks API Key: %s", key)
	rocks.rocksAPIKey = key

	rocks.limiter = rate.NewLimiter(rate.Limit(0.5), 60)
	rocks.configured = true
}

// GetEnlRocks is used for templates to determine if .Rocks is enabled
func GetEnlRocks() bool {
	return rocks.configured
}

// RocksSearch checks a agent at enl.rocks and populates a RocksAgent
// gid can be GoogleID, TelegramID or ENL-ID so this should be interface{} instead of GoogleID
func (gid GoogleID) RocksSearch(agent *RocksAgent) error {
	return rockssearch(gid, agent)
}

// RocksSearch checks a agent at enl.rocks and populates a RocksAgent
func (eid EnlID) RocksSearch(agent *RocksAgent) error {
	return rockssearch(eid, agent)
}

// RocksSearch checks a agent at enl.rocks and populates a RocksAgent
func (tgid TelegramID) RocksSearch(agent *RocksAgent) error {
	return rockssearch(tgid, agent)
}

// rockssearch stands behind the wraper functions and checks a agent at enl.rocks and populates a RocksAgent
func rockssearch(i interface{}, agent *RocksAgent) error {
	if !rocks.configured {
		return nil
	}

	var searchID string
	switch id := i.(type) {
	case GoogleID:
		searchID = id.String()
	case EnlID:
		searchID = id.String()
	case TelegramID:
		searchID = id.String()
	case string:
		searchID = id
	default:
		searchID = ""
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := rocks.limiter.Wait(ctx); err != nil {
		Log.Notice(err)
		// just keep going
	}

	apiurl := fmt.Sprintf("%s%s?apikey=%s", rocksAPIEndpoint, searchID, rocks.rocksAPIKey)
	req, err := http.NewRequest("GET", apiurl, nil)
	if err != nil {
		Log.Error(err)
		return err
	}
	client := &http.Client{
		Timeout: 3 * time.Second,
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

	err = json.Unmarshal(body, &agent)
	if err != nil {
		Log.Error(err)
		Log.Debug(string(body))
		return err
	}
	return nil
}

// RocksUpdate updates the database to reflect an agent's current status at enl.rocks.
// It should be called whenever a agent logs in via a new service (if appropriate); currently only https does.
func (gid GoogleID) RocksUpdate(agent *RocksAgent) error {
	if !rocks.configured {
		return nil
	}

	if agent.Agent != "" {
		// Log.Debug("Updating Rocks data for ", agent.Agent)
		_, err := db.Exec("UPDATE agent SET iname = ?, RocksVerified = ? WHERE gid = ?", agent.Agent, agent.Verified, gid)

		if err != nil {
			Log.Error(err)
			return err
		}

		// we trust .rocks to verify telegram info; if it is not already set for a agent, just import it.
		// XXX using agent name for telegramName isn't right in many cases -- need to reverify later
		if agent.TGId > 0 { // negative numbers are group chats, 0 is invalid
			_, err := db.Exec("INSERT IGNORE INTO telegram (telegramID, telegramName, gid, verified) VALUES (?, ?, ?, 1)", agent.TGId, agent.Agent, gid)
			if err != nil {
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
			Log.Debugf("Importing previously unknown agent: %s", rc.User.Gid)
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

	rc, err := teamID.RocksComm()
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

	apiurl := fmt.Sprintf("%s?key=%s", commAPIEndpoint, rc)
	req, err := http.NewRequest("GET", apiurl, nil)
	if err != nil {
		Log.Error(err)
		return err
	}
	client := &http.Client{
		Timeout: 3 * time.Second,
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
			Log.Debugf("Importing previously unknown agent: %s", agent)
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
	rc, err := teamID.RocksComm()
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

	// XXX use NewRequest/client
	apiurl := fmt.Sprintf("%s%s?key=%s", commAPIEndpoint, gid, rc)
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
	}
	if !rr.Success {
		Log.Error(rr.Error)
	}
	return nil
}

// RemoveFromRemoteRocksCommunity removes an agent from a Rocks Community IF that community has API enabled.
func (gid GoogleID) RemoveFromRemoteRocksCommunity(teamID TeamID) error {
	rc, err := teamID.RocksComm()
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

	apiurl := fmt.Sprintf("%s%s?key=%s", commAPIEndpoint, gid, rc)
	Log.Debug(apiurl)
	req, err := http.NewRequest("DELETE", apiurl, nil)
	if err != nil {
		Log.Error(err)
		return err
	}
	client := &http.Client{
		Timeout: 3 * time.Second,
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

	Log.Debug(string(body))
	var rr rocksPushResponse
	err = json.Unmarshal(body, &rr)
	if err != nil {
		Log.Error(err)
		return err
	}
	if !rr.Success {
		Log.Error(rr.Error)
		return errors.New(rr.Error)
	}
	return nil
}

func (teamID TeamID) RocksComm() (string, error) {
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
