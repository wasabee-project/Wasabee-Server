package risc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/lestrrat-go/jwx/v2/jwk"

	"github.com/wasabee-project/Wasabee-Server/config"
	"github.com/wasabee-project/Wasabee-Server/log"
	"github.com/wasabee-project/Wasabee-Server/model"
	"github.com/wasabee-project/Wasabee-Server/util"
)

const jsonType = "application/json; charset=UTF-8"
const apiBase = "https://risc.googleapis.com/v1beta/"
const jwtService = "https://risc.googleapis.com/google.identity.risc.v1beta.RiscManagementService"

// Webhook is the http route for receiving RISC updates
// pushes the updates into the RISC channel for processing
func webhook(res http.ResponseWriter, req *http.Request) {
	contentType := strings.Split(strings.Replace(strings.ToLower(req.Header.Get("Content-Type")), " ", "", -1), ";")[0]
	if contentType != "application/secevent+jwt" {
		err := fmt.Errorf("invalid request (needs to be application/secevent+jwt)")
		log.Errorw(err.Error(), "subsystem", "RISC")
		http.Error(res, err.Error(), http.StatusBadRequest)
		return
	}

	if !running {
		err := fmt.Errorf("RISC not configured, yet somehow a message was received")
		log.Errorw(err.Error(), "subsystem", "RISC")
		http.Error(res, err.Error(), http.StatusBadRequest)
		return
	}

	raw, err := io.ReadAll(req.Body)
	if err != nil {
		log.Errorw(err.Error(), "subsystem", "RISC")
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	if len(raw) == 0 {
		err = fmt.Errorf("empty JWT")
		log.Errorw(err.Error(), "subsystem", "RISC")
		http.Error(res, err.Error(), http.StatusBadRequest)
		return
	}

	// validateToken pushes the token to the handlers
	if err := validateToken(raw); err != nil {
		log.Errorw(err.Error(), "subsystem", "RISC")
		http.Error(res, err.Error(), http.StatusBadRequest)
		return
	}
	res.WriteHeader(http.StatusAccepted)
}

func registerWebhook(ctx context.Context) {
	// log.Infow("startup", "subsystem", "RISC", "message", "establishing RISC webhook with Google")

	// can these be pushed into registerWebhook?
	risc := config.Subrouter(config.Get().RISC.Webhook)
	risc.HandleFunc("", webhook).Methods("POST")

	ar := jwk.NewAutoRefresh(ctx)
	ar.Configure(googleConfig.JWKURI, jwk.WithMinRefreshInterval(60*time.Minute))
	tmp, err := ar.Refresh(ctx, googleConfig.JWKURI)
	if err != nil {
		log.Error(err)
		return
	}
	keys = tmp

	if err := updateWebhook(); err != nil {
		log.Errorw(err.Error(), "subsystem", "RISC")
		return
	}
	defer disableWebhook()

	running = true

	if err := ping(); err != nil {
		log.Errorw(err.Error(), "subsystem", "RISC")
		return
	}

	ticker := time.NewTicker(time.Hour)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			tmp, err := ar.Fetch(ctx, googleConfig.JWKURI)
			if err != nil {
				log.Error(err)
				return
			}
			keys = tmp

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
}

func updateWebhook() error {
	token, err := getToken()
	if err != nil {
		log.Errorw(err.Error(), "subsystem", "RISC")
		return err
	}

	apiurl := apiBase + "stream:update"
	webroot := config.Get().HTTP.Webroot
	// this is a static string, don't marshal it every time...
	jmsg := map[string]interface{}{
		"delivery": map[string]string{
			"delivery_method": "https://schemas.openid.net/secevent/risc/delivery-method/push",
			"url":             webroot + config.Get().RISC.Webhook,
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
		raw, _ := io.ReadAll(response.Body)
		log.Errorw(err.Error(), "subsystem", "RISC", "content", string(raw))
		return err
	}

	return nil
}

// disableWebhook tells Google to stop sending messages
func disableWebhook() {
	if !running {
		log.Infow("shutdown", "message", "RISC not running...not shutting down again")
		return
	}

	log.Infow("shutdown", "subsystem", "RISC", "message", "disabling RISC webhook with Google")

	token, err := getToken()
	if err != nil {
		log.Errorw(err.Error(), "subsystem", "RISC")
		return
	}

	apiurl := apiBase + "stream:update"
	webroot := config.Get().HTTP.Webroot
	jmsg := map[string]interface{}{
		"delivery": map[string]string{
			"delivery_method": "https://schemas.openid.net/secevent/risc/delivery-method/push",
			"url":             webroot + config.Get().RISC.Webhook,
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
		raw, _ = io.ReadAll(response.Body)
		log.Errorw("not OK status", "subsystem", "RISC", "content", string(raw))
	}

	close(riscchan)
	running = false
}

/* func checkWebhook() error {
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
	raw, _ := io.ReadAll(response.Body)
	log.Info(string(raw))

	return nil
} */

func ping() error {
	token, err := getToken()
	if err != nil {
		log.Errorw(err.Error(), "subsystem", "RISC")
		return err
	}

	apiurl := apiBase + "stream:verify"
	jmsg := map[string]string{
		"state": util.GenerateName(),
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
		raw, _ := io.ReadAll(response.Body)
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
			"iss":          googleConfig.Issuer,
			"sub":          gid.String(),
		},
		"verified": true,
	}
	raw, err := json.Marshal(jmsg)
	if err != nil {
		log.Errorw(err.Error(), "subsystem", "RISC")
		return err
	}

	log.Debugw("AddSubject", "subsystem", "RISC", "data", googleConfig.AddEndpoint, "raw", string(raw))

	client := http.Client{}
	req, err := http.NewRequest("POST", googleConfig.AddEndpoint, bytes.NewBuffer(raw))
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
		raw, _ := io.ReadAll(response.Body)
		log.Errorw("non OK status", "subsystem", "RISC", "content", string(raw))
	}

	return nil
}

func getToken() (*oauth2.Token, error) {
	creds, err := google.JWTAccessTokenSourceFromJSON(authdata, jwtService)
	if err != nil {
		log.Errorw(err.Error(), "subsystem", "RISC")
		return nil, err
	}
	return creds.Token()
}
