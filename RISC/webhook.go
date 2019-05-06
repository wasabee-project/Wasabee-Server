package risc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/cloudkucooland/WASABI"
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

func riscRegisterWebhook(configfile string) error {
	data, err := ioutil.ReadFile(configfile)
	if err != nil {
		wasabi.Log.Error(err)
		return err
	}

	wasabi.Log.Notice("setting up RISC webhook with google")
	updateWebhook(data)
	googleLoadKeys()
	config.running = true
	ping(data)

	ticker := time.NewTicker(time.Hour)
	for tick := range ticker.C {
		wasabi.Log.Debug("updating webhook with google: ", tick)
		updateWebhook(data)
		googleLoadKeys()
		ping(data)
	}

	wasabi.Log.Debug("should never make it here")
	return nil
}

func updateWebhook(data []byte) error {
	creds, err := google.JWTAccessTokenSourceFromJSON(data, "https://risc.googleapis.com/google.identity.risc.v1beta.RiscManagementService")
	if err != nil {
		wasabi.Log.Fatal(err)
		return err
	}
	token, err := creds.Token()
	if err != nil {
		wasabi.Log.Fatal(err)
		return err
	}

	apiurl := "https://risc.googleapis.com/v1beta/stream:update"
	jmsg := map[string]interface{}{
		"delivery": map[string]string{
			"delivery_method": "https://schemas.openid.net/secevent/risc/delivery-method/push",
			"url":             "https://qbin.phtiv.com:8443/GoogleRISC", // XXX do not hardcode this!
		},
		"events_requested": []string{
			"https://schemas.openid.net/secevent/risc/event-type/account-credential-change-required",
			"https://schemas.openid.net/secevent/risc/event-type/account-disabled",
			"https://schemas.openid.net/secevent/risc/event-type/account-enabled",
			"https://schemas.openid.net/secevent/risc/event-type/account-purged",
			"https://schemas.openid.net/secevent/risc/event-type/account-credential-change-required",
			"https://schemas.openid.net/secevent/risc/event-type/sessions-revoked",
			"https://schemas.openid.net/secevent/risc/event-type/tokens-revoked",
			"https://schemas.openid.net/secevent/risc/event-type/verification",
		},
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

func ping(data []byte) error {
	creds, err := google.JWTAccessTokenSourceFromJSON(data, "https://risc.googleapis.com/google.identity.risc.v1beta.RiscManagementService")
	if err != nil {
		wasabi.Log.Fatal(err)
		return err
	}
	token, err := creds.Token()
	if err != nil {
		wasabi.Log.Fatal(err)
		return err
	}

	apiurl := "https://risc.googleapis.com/v1beta/stream:verify"
	jmsg := map[string]string{
		"state": "some random value",
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
