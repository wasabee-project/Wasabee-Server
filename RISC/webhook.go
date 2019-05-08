package risc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/cloudkucooland/WASABI"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"io/ioutil"
	"net/http"
	"strings"
	"time"
)

// Webhook is the http route for receiving RISC updates
// pushes the updates into the RISC channel for processing
func Webhook(res http.ResponseWriter, req *http.Request) {
	var err error

	contentType := strings.Split(strings.Replace(strings.ToLower(req.Header.Get("Content-Type")), " ", "", -1), ";")[0]
	if contentType != "application/secevent+jwt" {
		err = fmt.Errorf("invalid request (needs to be application/secevent+jwt)")
		wasabi.Log.Error(err)
		http.Error(res, err.Error(), http.StatusNotAcceptable)
		return
	}

	if !config.running {
		err = fmt.Errorf("RISC not configured, yet somehow a message was received")
		wasabi.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusNotAcceptable)
		return
	}

	raw, err := ioutil.ReadAll(req.Body)
	if err != nil {
		wasabi.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	if string(raw) == "" {
		err = fmt.Errorf("empty JWT")
		wasabi.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusNotAcceptable)
		return
	}
	if err := validateToken(raw); err != nil {
		wasabi.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusNotAcceptable)
		return
	}
	res.WriteHeader(202)
}

func riscRegisterWebhook() {
	wasabi.Log.Notice("establishing RISC webhook with Google")
	err := googleLoadKeys()
	if err != nil {
		wasabi.Log.Error(err)
		return
	}

	err = updateWebhook()
	if err != nil {
		wasabi.Log.Error(err)
		return
	}

	defer disableWebhook()

	// if a secevent comes in between establishing the hook and loading the keys?
	config.running = true

	err = ping()
	if err != nil {
		wasabi.Log.Error(err)
		return
	}

	ticker := time.NewTicker(time.Hour)
	for tick := range ticker.C {
		wasabi.Log.Debug("updating RISC webhook with Google: ", tick)
		err = googleLoadKeys()
		if err != nil {
			wasabi.Log.Error(err)
			return
		}
		err = updateWebhook()
		if err != nil {
			wasabi.Log.Error(err)
			return
		}
		err = ping()
		if err != nil {
			wasabi.Log.Error(err)
			return
		}
	}
}

func updateWebhook() error {
	token, err := getToken()
	if err != nil {
		wasabi.Log.Error(err)
		return err
	}

	apiurl := "https://risc.googleapis.com/v1beta/stream:update"
	webroot, _ := wasabi.GetWebroot()
	jmsg := map[string]interface{}{
		"delivery": map[string]string{
			"delivery_method": "https://schemas.openid.net/secevent/risc/delivery-method/push",
			"url":             webroot + "/GoogleRISC",
		},
		"events_requested": []string{
			"https://schemas.openid.net/secevent/risc/event-type/account-credential-change-required",
			"https://schemas.openid.net/secevent/risc/event-type/account-disabled",
			"https://schemas.openid.net/secevent/risc/event-type/account-enabled",
			"https://schemas.openid.net/secevent/risc/event-type/account-purged",
			"https://schemas.openid.net/secevent/risc/event-type/sessions-revoked",
			"https://schemas.openid.net/secevent/risc/event-type/tokens-revoked",
			"https://schemas.openid.net/secevent/risc/event-type/verification",
		},
		"status": "enabled",
	}
	raw, err := json.Marshal(jmsg)
	if err != nil {
		wasabi.Log.Error(err)
		return err
	}
	client := http.Client{}
	req, err := http.NewRequest("POST", apiurl, bytes.NewBuffer(raw))
	if err != nil {
		wasabi.Log.Error(err)
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	req.Header.Set("Content-Type", "application/json; charset=UTF-8")

	response, err := client.Do(req)
	if err != nil {
		wasabi.Log.Error(err)
		return err
	}
	if response.StatusCode != 200 {
		raw, _ := ioutil.ReadAll(response.Body)
		wasabi.Log.Notice(string(raw))
	}

	return nil
}

func disableWebhook() error {
	wasabi.Log.Info("disabling RISC webhook with Google")

	token, err := getToken()
	if err != nil {
		wasabi.Log.Error(err)
		return err
	}

	apiurl := "https://risc.googleapis.com/v1beta/stream:update"
	webroot, _ := wasabi.GetWebroot()
	jmsg := map[string]interface{}{
		"delivery": map[string]string{
			"delivery_method": "https://schemas.openid.net/secevent/risc/delivery-method/push",
			"url":             webroot + "/GoogleRISC",
		},
		"status": "disabled",
	}
	raw, err := json.Marshal(jmsg)
	if err != nil {
		wasabi.Log.Error(err)
		return err
	}
	client := http.Client{}
	req, err := http.NewRequest("POST", apiurl, bytes.NewBuffer(raw))
	if err != nil {
		wasabi.Log.Error(err)
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	req.Header.Set("Content-Type", "application/json; charset=UTF-8")

	response, err := client.Do(req)
	if err != nil {
		wasabi.Log.Error(err)
		return err
	}
	if response.StatusCode != 200 {
		raw, _ = ioutil.ReadAll(response.Body)
		wasabi.Log.Notice(string(raw))
	}
	config.running = false

	return nil
}

func checkWebhook() error {
	token, err := getToken()
	if err != nil {
		wasabi.Log.Error(err)
		return err
	}

	apiurl := "https://risc.googleapis.com/v1beta/stream"
	if err != nil {
		wasabi.Log.Error(err)
		return err
	}
	client := http.Client{}
	req, err := http.NewRequest("GET", apiurl, nil)
	if err != nil {
		wasabi.Log.Error(err)
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	req.Header.Set("Content-Type", "application/json; charset=UTF-8")

	response, err := client.Do(req)
	if err != nil {
		wasabi.Log.Error(err)
		return err
	}
	raw, _ := ioutil.ReadAll(response.Body)
	wasabi.Log.Notice(string(raw))

	return nil
}

func ping() error {
	token, err := getToken()
	if err != nil {
		wasabi.Log.Error(err)
		return err
	}

	apiurl := "https://risc.googleapis.com/v1beta/stream:verify"
	jmsg := map[string]string{
		"state": wasabi.GenerateName(),
	}
	raw, err := json.Marshal(jmsg)
	if err != nil {
		wasabi.Log.Error(err)
		return err
	}
	client := http.Client{}
	req, err := http.NewRequest("POST", apiurl, bytes.NewBuffer(raw))
	if err != nil {
		wasabi.Log.Error(err)
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	req.Header.Set("Content-Type", "application/json; charset=UTF-8")

	response, err := client.Do(req)
	if err != nil {
		wasabi.Log.Error(err)
		return err
	}
	if response.StatusCode != 200 {
		raw, _ := ioutil.ReadAll(response.Body)
		wasabi.Log.Debug(string(raw))
	}

	return nil
}

// AddSubject puts the GID into the list of subjects we are concerned with.
// Google lists the endpoint in .well-known/, but doesn't do anything with it. It just 404s at the moment
func AddSubject(gid wasabi.GoogleID) error {
	token, err := getToken()
	if err != nil {
		wasabi.Log.Error(err)
		return err
	}

	// wasabi.Log.Debug(config.AddEndpoint)
	jmsg := map[string]interface{}{
		"subject": map[string]string{
			"subject_type": "iss-sub",
			"iss":          config.Issuer,
			"sub":          gid.String(),
		},
		"verified": true,
	}
	raw, err := json.Marshal(jmsg)
	if err != nil {
		wasabi.Log.Error(err)
		return err
	}

	wasabi.Log.Debug(config.AddEndpoint)
	wasabi.Log.Debug(string(raw))

	client := http.Client{}
	req, err := http.NewRequest("POST", config.AddEndpoint, bytes.NewBuffer(raw))
	if err != nil {
		wasabi.Log.Error(err)
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	req.Header.Set("Content-Type", "application/json; charset=UTF-8")

	response, err := client.Do(req)
	if err != nil {
		wasabi.Log.Error(err)
		return err
	}
	if response.StatusCode != 200 {
		raw, _ := ioutil.ReadAll(response.Body)
		wasabi.Log.Debug(string(raw))
	}

	return nil
}

func getToken() (*oauth2.Token, error) {
	creds, err := google.JWTAccessTokenSourceFromJSON(config.authdata, "https://risc.googleapis.com/google.identity.risc.v1beta.RiscManagementService")
	if err != nil {
		wasabi.Log.Fatal(err)
		return nil, err
	}
	return creds.Token()
}
