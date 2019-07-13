package wasabee

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"
)

var enlioConfig struct {
	apikey     string
	configured bool
}

type enlioResult struct {
	Name string `json:"agent_name"`
}

const enlioAPI = "https://enl.io/api/whois?service=google"

// SetENLIO is called from main
func SetENLIO(w string) {
	enlioConfig.apikey = w
	enlioConfig.configured = true
}

// ENLIORunning is for templates
func ENLIORunning() bool {
	return enlioConfig.configured
}

// this is a last-ditch attempt to get an agent name if .rocks and V do not have it
func (gid GoogleID) enlioQuery() (string, error) {
	if !enlioConfig.configured {
		return "", nil
	}

	url := fmt.Sprintf("%s&id=%s&token=%s", enlioAPI, gid, enlioConfig.apikey)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		Log.Error(err)
		return "", err
	}
	client := &http.Client{
		Timeout: 3 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		Log.Error(err)
		return "", err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		Log.Error(err)
		return "", err
	}

	// rather than giving an error, they just return an empty file
	if len(body) == 0 {
		return "", nil
	}

	var n enlioResult
	err = json.Unmarshal(body, &n)
	if err != nil {
		Log.Error(err)
		return "", err
	}
	if n.Name == "<PRIVATE>" {
		Log.Debugf("%s marked <PRIVATE>", gid)
		return "", nil
	}

	return n.Name, nil
}
