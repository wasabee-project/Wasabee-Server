package PhDevBin

import (
	//	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
)

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
func SetEnlRocks(w string) {
	Log.Debugf("enl.rocks API Key: %s", w)
	rocks.rocksAPIKey = w
	rocks.rocksAPIEndpoint = "https://enlightened.rocks/comm/api"
	rocks.configured = true
}

// GetvEnlOne is used for templates to determine if V is enabled
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

// RocksUpdate updates the database to reflect an agent's current status at V.
// It should be called whenever a user logs in via a new service (if appropriate); currently only https does.
func (gid GoogleID) RocksUpdate(res *RocksResult) error {
	if rocks.configured == false {
		return errors.New("Rocks API key not configured")
	}

	if res.Status == "ok" && res.Data.Agent != "" {
		Log.Debug("Updating Rocks data for ", res.Data.Agent)
		_, err := db.Exec("UPDATE user SET iname = ?, level = ?, VVerified = ?, VBlacklisted = ?, Vid = ? WHERE gid = ?",
			res.Data.Agent, res.Data.Level, res.Data.Verified, res.Data.Blacklisted, res.Data.EnlID, gid)

		if err != nil {
			Log.Error(err)
			return err
		}
	}
	return nil
}
