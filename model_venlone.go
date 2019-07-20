package wasabee

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"
)

type vconfig struct {
	vAPIEndpoint   string
	vAPIKey        string
	statusEndpoint string
	configured     bool
}

var vc vconfig

// Vresult is set by the V API
type Vresult struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
	Data    vagent `json:"data"`
}

// vagent is set by the V API
type vagent struct {
	EnlID       EnlID  `json:"enlid"`
	Vlevel      int64  `json:"vlevel"`
	Vpoints     int64  `json:"vpoints"`
	Agent       string `json:"agent"`
	Level       int64  `json:"level"`
	Quarantine  bool   `json:"quarantine"`
	Active      bool   `json:"active"`
	Blacklisted bool   `json:"blacklisted"`
	Verified    bool   `json:"verified"`
	Flagged     bool   `json:"flagged"`
	Banned      bool   `json:"banned_by_nia"`
	Cellid      string `json:"cellid"`
}

// SetVEnlOne is called from main() to initialize the config
func SetVEnlOne(w string) {
	Log.Debugf("V.enl.one API Key: %s", w)
	vc.vAPIKey = w
	vc.vAPIEndpoint = "https://v.enl.one/api/v1"
	vc.statusEndpoint = "https://status.enl.one/api/location"
	vc.configured = true
}

// GetvEnlOne is used for templates to determine if V is enabled
func GetvEnlOne() bool {
	return vc.configured
}

// VSearch checks a agent at V and populates a Vresult
func VSearch(id AgentID, vres *Vresult) error {
	if !vc.configured {
		return nil
	}
	searchID := id.String()
	if searchID == "" {
		return fmt.Errorf("empty search value")
	}

	url := fmt.Sprintf("%s/agent/%s/trust?apikey=%s", vc.vAPIEndpoint, searchID, vc.vAPIKey)

	req, err := http.NewRequest("GET", url, nil)
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
	err = json.Unmarshal(body, &vres)
	if err != nil {
		Log.Error(err)
		return err
	}
	if vres.Status != "ok" && vres.Message != "Agent not found" {
		err = errors.New(vres.Message)
		Log.Info(err)
		return err
	}
	// Log.Debug(vres.Data.Agent)
	return nil
}

// VUpdate updates the database to reflect an agent's current status at V.
// It should be called whenever a agent logs in via a new service (if appropriate); currently only https does.
func (gid GoogleID) VUpdate(vres *Vresult) error {
	if !vc.configured {
		return nil
	}

	if vres.Status == "ok" && vres.Data.Agent != "" {
		// Log.Debug("Updating V data for ", vres.Data.Agent)
		_, err := db.Exec("UPDATE agent SET iname = ?, level = ?, VVerified = ?, VBlacklisted = ?, Vid = ? WHERE gid = ?",
			vres.Data.Agent, vres.Data.Level, vres.Data.Verified, vres.Data.Blacklisted, MakeNullString(vres.Data.EnlID), gid)

		if err != nil {
			Log.Error(err)
			return err
		}
	}
	return nil
}

type statusResponse struct {
	Status  int    `json:"status"`
	Message string `json:"message"`
	Lon     string `json:"lon"`
	Lat     string `json:"lat"`
	Time    string `json:"time"`
}

// StatusLocation attempts to check for location data from status.enl.one.
// The API documentation is scant, so this is provisional -- seems to work.
func (eid EnlID) StatusLocation() (string, string, error) {
	if !vc.configured {
		return "", "", errors.New("the V API key not configured")
	}
	url := fmt.Sprintf("%s/%s?apikey=%s", vc.statusEndpoint, eid, vc.vAPIKey)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		Log.Error(err)
		return "", "", err
	}
	client := &http.Client{
		Timeout: 3 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		Log.Error(err)
		return "", "", err
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		Log.Error(err)
		return "", "", err
	}

	var stat statusResponse
	err = json.Unmarshal(body, &stat)
	if err != nil {
		Log.Error(err)
		return "", "", err
	}
	if stat.Status != 0 {
		err := fmt.Errorf("polling %s returned message: %s", eid, stat.Message)
		_ = eid.StatusLocationDisable()
		return "", "", err
	}
	return stat.Lat, stat.Lon, nil
}

// StatusLocation attempts to check for location data from status.enl.one.
// The API documentation is scant, so this is provisional -- seems to work.
func (gid GoogleID) StatusLocation() (string, string, error) {
	e, err := gid.EnlID()
	if err != nil {
		Log.Error(err)
		return "", "", err
	}
	lat, lon, err := e.StatusLocation()
	return lat, lon, err
}

// StatusLocationEnable turns RAID/JEAH pulling on for the specified agent
func (eid EnlID) StatusLocationEnable() error {
	_, err := db.Exec("UPDATE agent SET RAID = 1 WHERE Vid = ?", eid)
	if err != nil {
		Log.Error(err)
		return err
	}
	return nil
}

// StatusLocationEnable turns RAID/JEAH pulling on for the specified agent
func (gid GoogleID) StatusLocationEnable() error {
	eid, _ := gid.EnlID()
	err := eid.StatusLocationEnable()
	return err
}

// StatusLocationDisable turns RAID/JEAH pulling off for the specified agent
func (eid EnlID) StatusLocationDisable() error {
	_, err := db.Exec("UPDATE agent SET RAID = 0 WHERE Vid = ?", eid)
	if err != nil {
		Log.Error(err)
		return err
	}
	return nil
}

// StatusLocationDisable turns RAID/JEAH pulling off for the specified agent
func (gid GoogleID) StatusLocationDisable() error {
	_, err := db.Exec("UPDATE agent SET RAID = 0 WHERE gid = ?", gid)
	if err != nil {
		Log.Error(err)
		return err
	}
	return nil
}

// EnlID returns the V EnlID for a agent if it is known.
func (gid GoogleID) EnlID() (EnlID, error) {
	var e EnlID
	err := db.QueryRow("SELECT Vid FROM agent WHERE gid = ?", gid).Scan(&e)
	if err != nil {
		Log.Debug(err)
	}
	return e, err
}

// StatusServerPoller starts up from main and requests any agents who are configured to use RAID/JEAH for location services from the status.enl.one server.
// It works, but more research is necessary on the settings required on the permissions.
func StatusServerPoller() {
	if !vc.configured {
		Log.Debug("V not configures: not polling status.enl.one")
		return
	}

	// loop forever
	Log.Info("Starting status.enl.one Poller")
	for {
		// get list of agents who say they use JEAH/RAID
		row, err := db.Query("SELECT gid, Vid FROM agent WHERE RAID = 1")
		if err != nil {
			Log.Error(err)
			return
		}
		defer row.Close()
		var gid GoogleID
		var vid sql.NullString

		for row.Next() {
			err = row.Scan(&gid, &vid)
			// XXX if the agent isn't active on any teams, ignore
			if err != nil {
				Log.Error(err)
				continue
			}
			// Log.Debugf("Polling status.enl.one for %s", gid.String)
			if !vid.Valid {
				Log.Info("agent requested RAID poll, but has not configured V")
				gid.StatusLocationDisable()
				continue
			}
			e := EnlID(vid.String)
			lat, lon, err := e.StatusLocation()
			if err != nil {
				// XXX add the agent to an exception list? purge the list every 12 hours?
				Log.Error(err)
				continue
			}
			err = gid.AgentLocation(lat, lon, "status.enl.one")
			if err != nil {
				Log.Error(err)
				continue
			}
		}
		// SCB: https://github.com/golang/go/issues/27707 -- sleep is fine
		time.Sleep(300 * time.Second)
	}
}

// Gid looks up a GoogleID from an EnlID
func (eid EnlID) Gid() (GoogleID, error) {
	var gid GoogleID
	err := db.QueryRow("SELECT gid FROM agent WHERE Vid = ?", eid).Scan(&gid)
	if err != nil {
		Log.Error(err)
		return "", err
	}
	return gid, nil
}
