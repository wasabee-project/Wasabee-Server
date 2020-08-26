package risc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"
	"crypto/rsa"

	"github.com/lestrrat-go/jwx/jwa"
	"github.com/lestrrat-go/jwx/jwk"
	// "github.com/lestrrat-go/jwx/jws"
	"github.com/lestrrat-go/jwx/jwt"
	"github.com/wasabee-project/Wasabee-Server"
)

type riscConfig struct {
	Issuer      string   `json:"issuer"`
	JWKURI      string   `json:"jwks_uri"`
	Methods     []string `json:"delivery_methods_supported"`
	AddEndpoint string   `json:"add_subject_endpoint"`
	RemEndpoint string   `json:"remove_subject_endpoint"`
	running     bool
	clientemail string
	keys        *jwk.Set
	authdata    []byte
}

type event struct {
	Type    string
	Reason  string
	Issuer  string
	Subject string
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

const riscHook = "/GoogleRISC"
const googleDiscoveryURL = "https://accounts.google.com/.well-known/risc-configuration"

// RISC sets up the data structures and starts the processing threads
func RISC(configfile string) {
	// load config from google
	if err := googleRiscDiscovery(); err != nil {
		wasabee.Log.Error(err)
		return
	}

	// load service-account info from JSON file
	// #nosec
	data, err := ioutil.ReadFile(configfile)
	if err != nil {
		wasabee.Log.Error(err)
		return
	}
	var sc serviceCreds
	err = json.Unmarshal(data, &sc)
	if err != nil {
		wasabee.Log.Error(err)
		return
	}
	config.clientemail = sc.ClientEmail
	config.authdata = data // Yeah, the consumers need the whole thing as bytes

	// make a channel to read for events
	riscchan = make(chan event, 1)

	risc := wasabee.Subrouter(riscHook)
	risc.HandleFunc("", Webhook).Methods("POST")
	risc.HandleFunc("", WebhookStatus).Methods("GET")

	// start a thread for keeping the connection to Google fresh
	go riscRegisterWebhook()

	// this loops on the channel messsages
	for e := range riscchan {
		gid := wasabee.GoogleID(e.Subject)
		switch e.Type {
		case "https://schemas.openid.net/secevent/risc/event-type/account-disabled":
			wasabee.Log.Errorw("locking account", "subsystem", "RISC", "GID", gid, "subject", e.Subject, "issuer", e.Issuer, "reason", e.Reason)
			_ = gid.Lock(e.Reason)
			gid.FirebaseRemoveAllTokens()
			gid.Logout(e.Reason)
		case "https://schemas.openid.net/secevent/risc/event-type/account-enabled":
			wasabee.Log.Infow("unlocking account", "subsystem", "RISC", "GID", gid, "subject", e.Subject, "issuer", e.Issuer, "reason", e.Reason)
			_ = gid.Unlock(e.Reason)
		case "https://schemas.openid.net/secevent/risc/event-type/account-purged":
			wasabee.Log.Errorf("deleting account", "subsystem", "RISC", "GID", gid, "subject", e.Subject, "issuer", e.Issuer, "reason", e.Reason)
			gid.Logout(e.Reason)
			_ = gid.Delete()
		case "https://schemas.openid.net/secevent/risc/event-type/account-credential-change-required":
			wasabee.Log.Warnw("logout", "subsystem", "RISC", "GID", gid, "issuer", e.Issuer, "subject", e.Subject, "reason", e.Reason)
			gid.FirebaseRemoveAllTokens()
			gid.Logout(e.Reason)
		case "https://schemas.openid.net/secevent/risc/event-type/sessions-revoked":
			wasabee.Log.Warnw("logout", "subsystem", "RISC", "GID", gid, "issuer", e.Issuer, "subject", e.Subject, "reason", e.Reason)
			gid.FirebaseRemoveAllTokens()
			gid.Logout(e.Reason)
		case "https://schemas.openid.net/secevent/risc/event-type/tokens-revoked":
			wasabee.Log.Warnw("logout", "subsystem", "RISC", "GID", gid, "issuer", e.Issuer, "subject", e.Subject, "reason", e.Reason)
			gid.FirebaseRemoveAllTokens()
			gid.Logout(e.Reason)
		case "https://schemas.openid.net/secevent/risc/event-type/verification":
			// wasabee.Log.Debugw("verify", "subsystem", "RISC", "GID", gid,  "issuer", e.Issuer, "subject", e.Subject, "reason", e.Reason)
			// no need to do anything
		default:
			wasabee.Log.Warnw("unknown event", "subsystem", "RISC", "type", e.Type, "reason", e.Reason)
		}
	}
}

// This is called from the webhook
func validateToken(rawjwt []byte) error {
	var token jwt.Token
	var tokenOK bool
	for iter := config.keys.Iterate(context.TODO()); iter.Next(context.TODO()); {
		pair := iter.Pair()
		key := pair.Value.(jwk.RSAPublicKey)
		var pk rsa.PublicKey
		if err := key.Raw(&pk); err != nil {
			wasabee.Log.Errorw("unable to get public key from set", "error", err, "subsystem", "RISC", "message", "unable to get public key from set")
			continue
		}

		var err error
		token, err = jwt.Parse(bytes.NewReader(rawjwt), jwt.WithVerify(jwa.RS256, &pk), jwt.WithIssuer("https://accounts.google.com"))
		if err != nil {
			// silently try the next key
			token = nil
		} else {
			// found a good one
			tokenOK = true
			break
		}
	}

	// this can be removed now that we are getting verified above
	if !tokenOK {
		err := fmt.Errorf("unable to verify RISC event");
		wasabee.Log.Errorw(err.Error(), "subsystem", "RISC", "message", err.Error())
		return err
	}

	tmp, ok := token.Get("events")
	if !ok {
		err := fmt.Errorf("unable to get events from token")
		wasabee.Log.Error(err)
		return err
	}

	// multiple events per message are possible
	for k, v := range tmp.(map[string]interface{}) {
		var e event
		e.Type = k

		// just respond to verification requests instantly
		if k == "https://schemas.openid.net/secevent/risc/event-type/verification" {
			e.Reason = "ping requsted"
			riscchan <- e
			return nil
		}

		wasabee.Log.Infow("RISC event", "subsystem", "RISC", "type", k, "data", v, "message", "RISC event")

		// XXX this is ugly and brittle - use a map parser
		x := v.(map[string]interface{})
		if x["reason"] != nil {
			e.Reason = x["reason"].(string)
		}
		y := x["subject"].(map[string]interface{})
		e.Issuer = y["iss"].(string)
		e.Subject = y["sub"].(string)
		riscchan <- e
	}
	return nil
}

func googleRiscDiscovery() error {
	req, err := http.NewRequest("GET", googleDiscoveryURL, nil)
	if err != nil {
		wasabee.Log.Error(err)
		return err
	}
	client := &http.Client{
		Timeout: wasabee.GetTimeout(3 * time.Second),
	}
	resp, err := client.Do(req)
	if err != nil {
		wasabee.Log.Error(err)
		return err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		wasabee.Log.Error(err)
		return err
	}
	err = json.Unmarshal(body, &config)
	if err != nil {
		wasabee.Log.Error(err)
		return err
	}
	return nil
}

// called hourly to keep the key cache up-to-date
func googleLoadKeys() error {
	keys, err := jwk.Fetch(config.JWKURI)
	if err != nil {
		wasabee.Log.Error(err)
		return err
	}
	config.keys = keys
	return nil
}
