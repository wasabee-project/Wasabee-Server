package PhDevBin

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
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

// RocksAgent is a (minimal) version of the data sent by enl.rocks
type RocksAgent struct {
	Gid      GoogleID `json:"gid"`
	TGId     float64  `json:"tg_id"`
	TGUser   string   `json:"tg_user"`
	Agent    string   `json:"agentid"`
	Verified bool     `json:"verified"`
	Smurf    bool     `json:"smurf"`
}

type rocksconfig struct {
	rocksAPIEndpoint string
	rocksAPIKey      string
	configured       bool
	commAPIEndpoint  string
}

var rocks rocksconfig

// SetEnlRocks is called from main() to initialize the config
func SetEnlRocks(key string) {
	Log.Debugf("enl.rocks API Key: %s", key)
	rocks.rocksAPIKey = key
	rocks.rocksAPIEndpoint = "https://api.dfwenl.rocks/agent" // proxy for now
	rocks.commAPIEndpoint = "https://enlightened.rocks/comm/api/membership/"
	rocks.configured = true
}

// GetEnlRocks is used for templates to determine if .Rocks is enabled
func GetEnlRocks() bool {
	return rocks.configured
}

// RocksSearch checks a user at enl.rocks and populates a RocksAgent
// gid can be GoogleID, TelegramID or ENL-ID so this should be interface{} instead of GoogleID
func (gid GoogleID) RocksSearch(res *RocksAgent) error {
	return rockssearch(gid, res)
}

// RocksSearch checks a user at enl.rocks and populates a RocksAgent
func (eid EnlID) RocksSearch(res *RocksAgent) error {
	return rockssearch(eid, res)
}

// RocksSearch checks a user at enl.rocks and populates a RocksAgent
func (tgid TelegramID) RocksSearch(res *RocksAgent) error {
	id := strconv.Itoa(int(tgid))
	return rockssearch(id, res)
}

// rockssearch stands behind the wraper functions and checks a user at enl.rocks and populates a RocksAgent
func rockssearch(i interface{}, res *RocksAgent) error {
	var searchID string
	switch id := i.(type) {
	case GoogleID:
		searchID = id.String()
	case EnlID:
		searchID = id.String()
	case string:
		searchID = id
	default:
		searchID = ""
	}

	if rocks.configured == false {
		return errors.New("Rocks API key not configured")
	}
	url := fmt.Sprintf("%s/%s?key=%s", rocks.rocksAPIEndpoint, searchID, rocks.rocksAPIKey)
	resp, err := http.Get(url)
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
	err = json.Unmarshal(body, &res)
	if err != nil {
		Log.Error(err)
		return err
	}
	return nil
}

// RocksUpdate updates the database to reflect an agent's current status at enl.rocks.
// It should be called whenever a user logs in via a new service (if appropriate); currently only https does.
func (gid GoogleID) RocksUpdate(res *RocksAgent) error {
	if rocks.configured == false {
		return errors.New("Rocks API key not configured")
	}
	if res.Agent != "" {
		Log.Debug("Updating Rocks data for ", res.Agent)
		_, err := db.Exec("UPDATE user SET iname = ?, RocksVerified = ? WHERE gid = ?", res.Agent, res.Verified, gid)

		if err != nil {
			Log.Error(err)
			return err
		}
	}
	return nil
}

// RocksCommunitySync is called from the https server when it receives a push notification
// from the enl.rocks server; which it currently isn't sending even if enabled on a community.
// The calling function in the https server logs the request, so there is nothing for us to do here yet.
func RocksCommunitySync(msg json.RawMessage) error {
	// currently I can't get enl.rocks to send the data, nothing I can do here

	// check the source? is the community key enough for this? I don't think so
	var rc RocksCommunityNotice
	err := json.Unmarshal(msg, &rc)
	if err != nil {
		Log.Error(err)
		return err
	}

	_, err = rc.User.Gid.InitUser()
	if err != nil {
		Log.Error(err)
		return err
	}
	err = rc.User.Gid.RocksUpdate(&rc.User)
	if err != nil {
		Log.Notice(err)
	}

	team, err := RocksTeamID(rc.Community)
	if err != nil {
		Log.Error(err)
		return err
	}
	if rc.Action == "onJoin" {
		err := team.AddUser(rc.User.Gid)
		if err != nil {
			Log.Error(err)
			return err
		}
	} else {
		err := team.RemoveUser(rc.User.Gid)
		if err != nil {
			Log.Error(err)
			return err
		}
	}

	return nil
}

// RocksCommunityMemberPull grabs the member list from the associated community at enl.rocks and adds each user to the team
func (t TeamID) RocksCommunityMemberPull() error {
	if rocks.configured == false {
		return errors.New("Rocks API key not configured")
	}

	var rc sql.NullString
	err := db.QueryRow("SELECT rockskey FROM teams WHERE teamID = ?", t).Scan(&rc)
	if err != nil {
		Log.Error(err)
		return err
	}

	// rocks.commAPIEndpoint
	url := fmt.Sprintf("%s?key=%s", rocks.commAPIEndpoint, rc.String)
	Log.Debug(url)
	resp, err := http.Get(url)
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
	var rr RocksCommunityResponse
	err = json.Unmarshal(body, &rr)
	if err != nil {
		Log.Error(err)
		return err
	}

	for _, user := range rr.Members {
		// if tmp := user.LocKey(); tmp == "" { // XXX some test to see if the user is already in the system
		user.InitUser()
		// }
		err := t.AddUser(user)
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
	err := db.QueryRow("SELECT teamID FROM teams WHERE rockscomm = ?", rockscomm).Scan(&t)
	if err != nil {
		Log.Error(err)
		return "", err
	}
	return t, nil
}
