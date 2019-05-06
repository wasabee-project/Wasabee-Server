package risc

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/cloudkucooland/WASABI"
	"github.com/lestrrat-go/jwx/jwa"
	"github.com/lestrrat-go/jwx/jwt"
)

type riscConfig struct {
	Issuer      string   `json:"issuer"`
	JWKURI      string   `json:"jwks_uri"`
	Methods     []string `json:"delivery_methods_supported"`
	AddEndpoint string   `json:"add_subject_endpoint"`
	RemEndpoint string   `json:"remove_subject_endpoint"`
	running     bool
	clientemail string
	keys        keys
}

type event struct {
	Type    string
	Reason  string
	Issuer  string
	Subject string
}

type keys struct {
	Keys []key `json:"keys"`
}

type key struct {
	KeyID   string `json:"kid"`
	E       string `json:"e"`
	KeyType string `json:"kty"`
	Alg     string `json:"alg"`
	N       string `json:"n"`
	Use     string `json:"use"`
}

// Google probably has a type for this somewhere, maybe x/oauth/Google
type serviceCreds struct {
	Type            string `json:"type"`
	ProjectID       string `json:"project_id"`
	ProjectKeyID    string `json:"private_key_id"`
	PrivateKey      string `json:"private_key"`
	ClientEmail     string `json:"client_email"`
	ClientID        string `json:"client_id"`
	AuthURI         string `json:"auth_uri"`
	TokenURI        string `json:"token_uri"`
	ProviderCertURL string `json:"auth_provider_x509_cert_url"`
	ClientCertURL   string `json:"client_x509_cert_url"`
}

var riscchan chan event
var config riscConfig

// RISCinit sets up the data structures and starts the processing threads
func RISCinit(configfile string) error {
	// load config from google
	if err := googleRiscDiscovery(); err != nil {
		wasabi.Log.Error(err)
		return err
	}

	// load service-account info from JSON file
	data, err := ioutil.ReadFile(configfile)
	if err != nil {
		wasabi.Log.Error(err)
		return err
	}
	var sc serviceCreds
	err = json.Unmarshal(data, &sc)
	if err != nil {
		wasabi.Log.Error(err)
		return err
	}
	config.clientemail = sc.ClientEmail

	// make a channel to read for events
	riscchan = make(chan event, 2)

	// start a thread for keeping the connection to Google fresh
	go riscRegisterWebhook(configfile)

	for e := range riscchan {
		wasabi.Log.Notice("Received: ", e)
		// e.Subject is gid
		// lock the user out? set local blacklist flag?
		// just delete them outright (losing team membership and ops?)
	}

	return nil
}

// This is called from the webhook
func validateToken(rawjwt []byte) error {
	token, err := jwt.ParseBytes(rawjwt)
	if err != nil {
		wasabi.Log.Error(err)
		return err
	}

	var e event
	tmp, ok := token.Get("events")
	if !ok {
		wasabi.Log.Error(err)
		return err
	}
	for k, v := range tmp.(map[string]interface{}) {
		wasabi.Log.Debug("Event: ", k, v)
		switch k {
		// verification types are not signed, no KID, just respond instantly
		case "https://schemas.openid.net/secevent/risc/event-type/verification":
			wasabi.Log.Debug("processed ping response")
			return nil
		// XXX this will probably seg-fault, need to write some good test units
		default:
			wasabi.Log.Debug("trying to process type %s", k)
			e.Type = k
			x := v.(map[string]interface{})
			e.Reason = x["reason"].(string)
			y := x["subject"].(map[string]interface{})
			e.Issuer = y["iss"].(string)
			e.Subject = y["sub"].(string)
		}
	}

	kid, ok := token.Get("kid")
	if !ok {
		err = fmt.Errorf("no Key ID in JWT")
		wasabi.Log.Error(err)
		return err
	}

	key, err := googleMatchingKey(kid.(string))
	if err != nil {
		wasabi.Log.Error(err)
		return err
	}

	err = token.Verify(jwt.WithAudience(config.clientemail), jwt.WithAcceptableSkew(60), jwt.WithVerify(jwa.RS256, key))
	if err != nil {
		wasabi.Log.Error(err)
		return err
	}
	wasabi.Log.Debug("Pushing into channel: ", e)
	riscchan <- e

	return nil
}

func googleRiscDiscovery() error {
	discovery := "https://accounts.google.com/.well-known/risc-configuration"
	req, err := http.NewRequest("GET", discovery, nil)
	if err != nil {
		wasabi.Log.Error(err)
		return err
	}
	client := &http.Client{
		Timeout: 3 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		wasabi.Log.Error(err)
		return err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		wasabi.Log.Error(err)
		return err
	}
	err = json.Unmarshal(body, &config)
	if err != nil {
		wasabi.Log.Error(err)
		return err
	}
	return nil
}

// called hourly to keep them up-to-date
func googleLoadKeys() error {
	req, err := http.NewRequest("GET", config.JWKURI, nil)
	if err != nil {
		wasabi.Log.Error(err)
		return err
	}
	client := &http.Client{
		Timeout: 3 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		wasabi.Log.Error(err)
		return err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		wasabi.Log.Error(err)
		return err
	}

	// XXX should probably wrap this in a mutex
	err = json.Unmarshal(body, &config.keys)
	if err != nil {
		wasabi.Log.Error(err)
		return err
	}
	return nil
}

func googleMatchingKey(kid string) (key, error) {
	var x key

	// XXX use mutex from googleLoadKeys
	for _, v := range config.keys.Keys {
		if v.KeyID == kid {
			return v, nil
		}
	}

	err := fmt.Errorf("no matching keys found")
	return x, err
}
