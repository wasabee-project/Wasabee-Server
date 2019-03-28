package PhDevBin

import (
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
	Community string    `json:"community"`
	Action    string    `json:"action"`
	User      RocksUser `json:"user"`
}

// RocksUser is a (minimal) version of the data sent by enl.rocks
type RocksUser struct {
	Gid    GoogleID `json:"gid"`
	TGId   float64  `json:"tg_id"`
	TGUser string   `json:"tg_user"`
	Agent  string   `json:"agentid"`
	Smurf  bool     `json:"smurf"`
}

type rocksconfig struct {
	rocksAPIEndpoint string
	rocksAPIKey      string
	configured       bool
}

var rocks rocksconfig

// RocksResult is set by the enl.rocks API
type RocksResult struct {
	Status  string `json:"status"`
	Message string `json:"message,omitmissing"`
	Data    Vagent `json:"data"`
}

// RocksAgent is set by the Rocks API
// NOT STARTED
type RocksAgent struct {
	EnlID EnlID `json:"enlid"`
}

// SetEnlRocks is called from main() to initialize the config
func SetEnlRocks(key string) {
	Log.Debugf("enl.rocks API Key: %s", key)
	rocks.rocksAPIKey = key
	rocks.rocksAPIEndpoint = "https://enlightened.rocks/comm/api"
	rocks.configured = true
}

// GetEnlRocks is used for templates to determine if .Rocks is enabled
func GetEnlRocks() bool {
	return rocks.configured
}

// RocksSearch checks a user at enl.rocks and populates a RocksAgent
// gid can be GoogleID, TelegramID or ENL-ID so this should be interface{} instead of GoogleID
func (gid GoogleID) RocksSearch(res *RocksResult) error {
	return rockssearch(gid, res)
}

// RocksSearch checks a user at enl.rocks and populates a RocksAgent
func (eid EnlID) RocksSearch(res *RocksResult) error {
	return rockssearch(eid, res)
}

// RocksSearch checks a user at enl.rocks and populates a RocksAgent
func (tgid TelegramID) RocksSearch(res *RocksResult) error {
	id := strconv.Itoa(int(tgid))
	return rockssearch(id, res)
}

// rockssearch stands behind the wraper functions and checks a user at enl.rocks and populates a RocksResult
func rockssearch(i interface{}, res *RocksResult) error {
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
	url := fmt.Sprintf("%s/membership/%s?key=%s", rocks.rocksAPIEndpoint, searchID, rocks.rocksAPIKey)
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
	/* err = json.Unmarshal(body, &res)
	if err != nil {
		Log.Error(err)
		return err
	}
	if res.Status != "ok" {
		err = errors.New(res.Message)
		Log.Info(err)
		return err
	} */
	// Log.Debug(res.Data.Agent)
	return nil
}

// RocksUpdate updates the database to reflect an agent's current status at enl.rocks.
// It should be called whenever a user logs in via a new service (if appropriate); currently only https does.
// XXX on hold until I can get an API key
func (gid GoogleID) RocksUpdate(res *RocksUser) error {
	if rocks.configured == false {
		return errors.New("Rocks API key not configured")
	}
	Log.Debug("RocksUpdate doing nothing")

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
