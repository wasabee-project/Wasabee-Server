package PhDevBin

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
)

type vconfig struct {
	vAPIEndpoint string
	vAPIKey      string
	configured   bool
}

var vc vconfig

type Vresult struct {
	Status  string `json:"status"`
	Message string `json:"message,omitmissing"`
	Data    Vagent `json:"data"`
}

type Vagent struct {
	EnlID       string  `json:"enlid"`
	Vlevel      float64 `json:"vlevel"`
	Vpoints     float64 `json:"vpoints"`
	Agent       string  `json:"agent"`
	Level       float64 `json:"level"`
	Quarantine  bool    `json:"quarantine"`
	Active      bool    `json:"active"`
	Blacklisted bool    `json:"blacklisted"`
	Verified    bool    `json:"verified"`
	Flagged     bool    `json:"flagged"`
	Banned      bool    `json:"banned_by_nia"`
	Cellid      string  `json:"cellid"`
}

func SetVEnlOne(w string) {
	Log.Debugf("V.enl.one API Key: %s", w)
	vc.vAPIKey = w
	vc.vAPIEndpoint = "https://v.enl.one/api/v1"
	vc.configured = true
}

// for templates
func GetvEnlOne() bool {
	return vc.configured
}

// gid can be GoogleID, TelegramID or ENL-ID
func VSearchUser(gid string, res *Vresult) error {
	if vc.configured == false {
		return errors.New("V API key not configured")
	}
	url := fmt.Sprintf("%s/agent/%s/trust?apikey=%s", vc.vAPIEndpoint, gid, vc.vAPIKey)
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
	if res.Status != "ok" {
		err = errors.New(res.Message)
		Log.Info(err)
		return err
	}
	// Log.Debug(res.Data.Agent)
	return nil
}

func VUpdateUser(gid string, res *Vresult) error {
	if vc.configured == false {
		return errors.New("V API key not configured (VUpdateUser, should never happen)")
	}

	if res.Status == "ok" && res.Data.Agent != "" {
		Log.Debug("Updating V data for ", res.Data.Agent)
		_, err := db.Exec("UPDATE user SET iname = ?, VVerified = ?, VBlacklisted = ? WHERE gid = ?", res.Data.Agent, res.Data.Verified, res.Data.Blacklisted, gid)

		if err != nil {
			Log.Error(err)
			return err
		}
	}
	return nil
}
