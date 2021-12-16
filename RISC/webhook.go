package risc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/wasabee-project/Wasabee-Server/config"
	"github.com/wasabee-project/Wasabee-Server/generatename"
	"github.com/wasabee-project/Wasabee-Server/log"
	"github.com/wasabee-project/Wasabee-Server/model"
)

const jsonType = "application/json; charset=UTF-8"
const apiBase = "https://risc.googleapis.com/v1beta/"
const jwtService = "https://risc.googleapis.com/google.identity.risc.v1beta.RiscManagementService"

// Webhook is the http route for receiving RISC updates
// pushes the updates into the RISC channel for processing
func Webhook(res http.ResponseWriter, req *http.Request) {
	// var err error

	contentType := strings.Split(strings.Replace(strings.ToLower(req.Header.Get("Content-Type")), " ", "", -1), ";")[0]
	if contentType != "application/secevent+jwt" {
		err := fmt.Errorf("invalid request (needs to be application/secevent+jwt)")
		log.Errorw(err.Error(), "subsystem", "RISC")
		http.Error(res, err.Error(), http.StatusBadRequest)
		return
	}

	if !c.running {
		err := fmt.Errorf("RISC not configured, yet somehow a message was received")
		log.Errorw(err.Error(), "subsystem", "RISC")
		http.Error(res, err.Error(), http.StatusBadRequest)
		return
	}

	raw, err := ioutil.ReadAll(req.Body)
	if err != nil {
		log.Errorw(err.Error(), "subsystem", "RISC")
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	if string(raw) == "" {
		err = fmt.Errorf("empty JWT")
		log.Errorw(err.Error(), "subsystem", "RISC")
		http.Error(res, err.Error(), http.StatusBadRequest)
		return
	}
	if err := validateToken(raw); err != nil {
		log.Errorw(err.Error(), "subsystem", "RISC")
		http.Error(res, err.Error(), http.StatusBadRequest)
		return
	}
	res.WriteHeader(http.StatusAccepted)
}

// WebhookStatus exposes the running RISC status to the HTTP process
func WebhookStatus(res http.ResponseWriter, req *http.Request) {
	if err := checkWebhook(); err != nil {
		log.Errorw(err.Error(), "subsystem", "RISC")
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	fmt.Fprint(res, "RISC listener service is Running")
}

func riscRegisterWebhook() {
	log.Infow("startup", "subsystem", "RISC", "message", "establishing RISC webhook with Google")
	if err := googleLoadKeys(); err != nil {
		log.Errorw(err.Error(), "subsystem", "RISC")
		return
	}

	if err := updateWebhook(); err != nil {
		log.Errorw(err.Error(), "subsystem", "RISC")
		return
	}

	defer DisableWebhook()

	// if a secevent comes in between establishing the hook and loading the keys?
	c.running = true

	if err := ping(); err != nil {
		log.Errorw(err.Error(), "subsystem", "RISC")
		return
	}
	// checkWebhook()

	ticker := time.NewTicker(time.Hour)
	for range ticker.C {
		if err := googleLoadKeys(); err != nil {
			log.Errorw(err.Error(), "subsystem", "RISC")
			return
		}
		if err := updateWebhook(); err != nil {
			log.Errorw(err.Error(), "subsystem", "RISC")
			return
		}
		if err := ping(); err != nil {
			log.Errorw(err.Error(), "subsystem", "RISC")
			return
		}
	}
}

func updateWebhook() error {
	token, err := getToken()
	if err != nil {
		log.Errorw(err.Error(), "subsystem", "RISC")
		return err
	}

	apiurl := apiBase + "stream:update"
	webroot := config.GetWebroot()
	jmsg := map[string]interface{}{
		"delivery": map[string]string{
			"delivery_method": "https://schemas.openid.net/secevent/risc/delivery-method/push",
			"url":             webroot + riscHook,
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
		log.Errorw(err.Error(), "subsystem", "RISC")
		return err
	}
	client := http.Client{}
	req, err := http.NewRequest("POST", apiurl, bytes.NewBuffer(raw))
	if err != nil {
		log.Errorw(err.Error(), "subsystem", "RISC")
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	req.Header.Set("Content-Type", jsonType)

	response, err := client.Do(req)
	if err != nil {
		log.Errorw(err.Error(), "subsystem", "RISC")
		return err
	}
	if response.StatusCode != http.StatusOK {
		err := fmt.Errorf("non-OK status")
		raw, _ := ioutil.ReadAll(response.Body)
		log.Errorw(err.Error(), "subsystem", "RISC", "content", string(raw))
		return err
	}

	return nil
}

// DisableWebhook tells Google to stop sending messages
func DisableWebhook() {
	log.Infow("shutdown", "subsystem", "RISC", "message", "disabling RISC webhook with Google")

	token, err := getToken()
	if err != nil {
		log.Errorw(err.Error(), "subsystem", "RISC")
		return
	}

	apiurl := apiBase + "stream:update"
	webroot := config.GetWebroot()
	jmsg := map[string]interface{}{
		"delivery": map[string]string{
			"delivery_method": "https://schemas.openid.net/secevent/risc/delivery-method/push",
			"url":             webroot + riscHook,
		},
		"status": "disabled",
	}
	raw, err := json.Marshal(jmsg)
	if err != nil {
		log.Errorw(err.Error(), "subsystem", "RISC")
		return
	}
	client := http.Client{}
	req, err := http.NewRequest("POST", apiurl, bytes.NewBuffer(raw))
	if err != nil {
		log.Errorw(err.Error(), "subsystem", "RISC")
		return
	}
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	req.Header.Set("Content-Type", jsonType)

	response, err := client.Do(req)
	if err != nil {
		log.Errorw(err.Error(), "subsystem", "RISC")
		return
	}
	if response.StatusCode != http.StatusOK {
		raw, _ = ioutil.ReadAll(response.Body)
		log.Errorw("not OK status", "subsystem", "RISC", "content", string(raw))
	}
	c.running = false
}

func checkWebhook() error {
	token, err := getToken()
	if err != nil {
		log.Errorw(err.Error(), "subsystem", "RISC")
		return err
	}

	apiurl := apiBase + "stream"
	client := http.Client{}
	req, err := http.NewRequest("GET", apiurl, nil)
	if err != nil {
		log.Errorw(err.Error(), "subsystem", "RISC")
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	req.Header.Set("Content-Type", jsonType)

	response, err := client.Do(req)
	if err != nil {
		log.Errorw(err.Error(), "subsystem", "RISC")
		return err
	}
	raw, _ := ioutil.ReadAll(response.Body)
	log.Info(string(raw))

	return nil
}

func ping() error {
	token, err := getToken()
	if err != nil {
		log.Errorw(err.Error(), "subsystem", "RISC")
		return err
	}

	apiurl := apiBase + "stream:verify"
	jmsg := map[string]string{
		"state": generatename.GenerateName(),
	}
	raw, err := json.Marshal(jmsg)
	if err != nil {
		log.Errorw(err.Error(), "subsystem", "RISC")
		return err
	}
	client := http.Client{}
	req, err := http.NewRequest("POST", apiurl, bytes.NewBuffer(raw))
	if err != nil {
		log.Errorw(err.Error(), "subsystem", "RISC")
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	req.Header.Set("Content-Type", jsonType)

	response, err := client.Do(req)
	if err != nil {
		log.Errorw(err.Error(), "subsystem", "RISC")
		return err
	}
	if response.StatusCode != http.StatusOK {
		raw, _ := ioutil.ReadAll(response.Body)
		log.Errorw("non OK status", "subsystem", "RISC", "content", string(raw))
	}

	return nil
}

// AddSubject puts the GID into the list of subjects we are concerned with.
// Google lists the endpoint in .well-known/, but doesn't do anything with it. It just 404s at the moment
func AddSubject(gid model.GoogleID) error {
	token, err := getToken()
	if err != nil {
		log.Errorw(err.Error(), "subsystem", "RISC")
		return err
	}

	jmsg := map[string]interface{}{
		"subject": map[string]string{
			"subject_type": "iss-sub",
			"iss":          c.Issuer,
			"sub":          gid.String(),
		},
		"verified": true,
	}
	raw, err := json.Marshal(jmsg)
	if err != nil {
		log.Errorw(err.Error(), "subsystem", "RISC")
		return err
	}

	log.Debugw("AddSubject", "subsystem", "RISC", "data", c.AddEndpoint, "raw", string(raw))

	client := http.Client{}
	req, err := http.NewRequest("POST", c.AddEndpoint, bytes.NewBuffer(raw))
	if err != nil {
		log.Errorw(err.Error(), "subsystem", "RISC")
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	req.Header.Set("Content-Type", jsonType)

	response, err := client.Do(req)
	if err != nil {
		log.Errorw(err.Error(), "subsystem", "RISC")
		return err
	}
	if response.StatusCode != http.StatusOK {
		raw, _ := ioutil.ReadAll(response.Body)
		log.Errorw("non OK status", "subsystem", "RISC", "content", string(raw))
	}

	return nil
}

func getToken() (*oauth2.Token, error) {
	creds, err := google.JWTAccessTokenSourceFromJSON(c.authdata, jwtService)
	if err != nil {
		log.Errorw(err.Error(), "subsystem", "RISC")
		return nil, err
	}
	return creds.Token()
}
