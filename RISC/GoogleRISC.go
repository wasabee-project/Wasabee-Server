package risc

import (
	// "bytes"
	"context"
	// "crypto/rsa"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	// "github.com/lestrrat-go/jwx/jwa"
	"github.com/lestrrat-go/jwx/jwk"
	"github.com/lestrrat-go/jwx/jwt"

	"github.com/wasabee-project/Wasabee-Server/auth"
	"github.com/wasabee-project/Wasabee-Server/config"
	"github.com/wasabee-project/Wasabee-Server/log"
	"github.com/wasabee-project/Wasabee-Server/model"
)

type riscConfig struct {
	Issuer      string   `json:"issuer"`
	JWKURI      string   `json:"jwks_uri"`
	Methods     []string `json:"delivery_methods_supported"`
	AddEndpoint string   `json:"add_subject_endpoint"`
	RemEndpoint string   `json:"remove_subject_endpoint"`
	running     bool
	clientemail string
	keys        jwk.Set
	authdata    []byte
}

// the token's event
// {"subject":{"email":"whoever@gmail.com","iss":"https://accounts.google.com","sub":"...gid...","subject_type":"id_token_claims"}, "reason": ""}
type event struct {
	Type    string `json:"subject_type"`
	Reason  string `json:"reason"` // wasabee change
	Issuer  string `json:"iss"`
	Subject string `json:"sub"`
	Email   string `json:"email"`
}

type riscmsg struct {
	Subject event  `json:"subject"`
	Reason  string `json:"reason"`
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
var c riscConfig

const riscHook = "/GoogleRISC"
const googleDiscoveryURL = "https://accounts.google.com/.well-known/risc-configuration"

// RISC sets up the data structures and starts the processing threads
func RISC(configfile string) {
	// load config from google
	if err := googleRiscDiscovery(); err != nil {
		log.Error(err)
		return
	}

	// load service-account info from JSON file
	// #nosec
	data, err := ioutil.ReadFile(configfile)
	if err != nil {
		log.Error(err)
		return
	}
	var sc serviceCreds
	err = json.Unmarshal(data, &sc)
	if err != nil {
		log.Error(err)
		return
	}
	c.clientemail = sc.ClientEmail
	c.authdata = data // Yeah, the consumers need the whole thing as bytes

	// make a channel to read for events
	riscchan = make(chan event, 1)

	risc := config.Subrouter(riscHook)
	risc.HandleFunc("", Webhook).Methods("POST")
	risc.HandleFunc("", WebhookStatus).Methods("GET")

	// start a thread for keeping the connection to Google fresh
	go riscRegisterWebhook()

	// this loops on the channel messsages
	for e := range riscchan {
		gid := model.GoogleID(e.Subject)
		switch e.Type {
		case "https://schemas.openid.net/secevent/risc/event-type/account-disabled":
			log.Errorw("locking account", "subsystem", "RISC", "GID", gid, "subject", e.Subject, "issuer", e.Issuer, "reason", e.Reason)
			_ = gid.Lock(e.Reason)
			gid.RemoveAllFirebaseTokens()
			auth.Logout(gid, e.Reason)
		case "https://schemas.openid.net/secevent/risc/event-type/account-enabled":
			log.Infow("unlocking account", "subsystem", "RISC", "GID", gid, "subject", e.Subject, "issuer", e.Issuer, "reason", e.Reason)
			_ = gid.Unlock(e.Reason)
		case "https://schemas.openid.net/secevent/risc/event-type/account-purged":
			log.Errorw("deleting account", "subsystem", "RISC", "GID", gid, "subject", e.Subject, "issuer", e.Issuer, "reason", e.Reason)
			auth.Logout(gid, e.Reason)
			_ = gid.Delete()
		case "https://schemas.openid.net/secevent/risc/event-type/account-credential-change-required":
			log.Warnw("logout", "subsystem", "RISC", "GID", gid, "issuer", e.Issuer, "subject", e.Subject, "reason", e.Reason)
			gid.RemoveAllFirebaseTokens()
			auth.Logout(gid, e.Reason)
		case "https://schemas.openid.net/secevent/risc/event-type/sessions-revoked":
			log.Warnw("logout", "subsystem", "RISC", "GID", gid, "issuer", e.Issuer, "subject", e.Subject, "reason", e.Reason)
			gid.RemoveAllFirebaseTokens()
			auth.Logout(gid, e.Reason)
		case "https://schemas.openid.net/secevent/risc/event-type/tokens-revoked":
			log.Warnw("logout", "subsystem", "RISC", "GID", gid, "issuer", e.Issuer, "subject", e.Subject, "reason", e.Reason)
			gid.RemoveAllFirebaseTokens()
			auth.Logout(gid, e.Reason)
		case "https://schemas.openid.net/secevent/risc/event-type/verification":
			// log.Debugw("verify", "subsystem", "RISC", "GID", gid,  "issuer", e.Issuer, "subject", e.Subject, "reason", e.Reason)
			// no need to do anything
		case "https://accounts.google.com/risc/event/sessions-revoked":
			log.Warnw("logout", "subsystem", "RISC", "GID", gid, "issuer", e.Issuer, "subject", e.Subject, "reason", e.Reason)
			gid.RemoveAllFirebaseTokens()
			auth.Logout(gid, e.Reason)
		default:
			log.Warnw("unknown event", "subsystem", "RISC", "type", e.Type, "reason", e.Reason)
		}
	}
}

// This is called from the webhook
// the JWT library has improved a lot, most of this can be simplified
func validateToken(rawjwt []byte) error {
	token, err := jwt.Parse(rawjwt, jwt.WithValidate(true), jwt.WithIssuer("https://accounts.google.com"), jwt.InferAlgorithmFromKey(true), jwt.UseDefaultKey(true), jwt.WithKeySet(c.keys))
	if err != nil {
		log.Errorw("RISC", "error", err.Error(), "subsystem", "RISC", "raw", string(rawjwt))
		return err
	}
	if token == nil {
		err := fmt.Errorf("unable to verify RISC event")
		log.Errorw(err.Error(), "subsystem", "RISC", "raw", string(rawjwt))
		return err
	}

	// XXX doe jwt.WithValidate(true) make this redundant?
	if err := jwt.Validate(token); err != nil {
		log.Infow("RISC jwt validate failed", "error", err)
		return err
	}

	events, ok := token.Get("events")
	if !ok {
		err := fmt.Errorf("unable to get events from token")
		log.Error(err)
		return err
	}

	// multiple events per message are possible
	for k, v := range events.(map[string]interface{}) {
		var r riscmsg
		r.Subject.Type = k

		// just respond to verification requests instantly
		if k == "https://schemas.openid.net/secevent/risc/event-type/verification" {
			r.Subject.Reason = "ping requsted"
			riscchan <- r.Subject
			continue
		}

		log.Infow("RISC event", "subsystem", "RISC", "type", k, "data", v, "message", "RISC event")

		// XXX this is ugly and brittle - it is JSON, just unmarshal it.
		x := v.(map[string]interface{})

		/* if err := json.Unmarshal(x, &r); err != nil {
			log.Errorw(err.Error(), "subsystem", "RISC", "message", err.Error(), "data", x)
			continue
		} */

		if x["reason"] != nil {
			r.Subject.Reason = x["reason"].(string)
		}
		y := x["subject"].(map[string]interface{})
		r.Subject.Issuer = y["iss"].(string)
		r.Subject.Subject = y["sub"].(string)
		r.Subject.Email = y["email"].(string)

		// r.Subject.Reason = r.Reason
		riscchan <- r.Subject
	}
	return nil
}

func googleRiscDiscovery() error {
	req, err := http.NewRequest("GET", googleDiscoveryURL, nil)
	if err != nil {
		log.Error(err)
		return err
	}
	client := &http.Client{
		Timeout: (3 * time.Second),
	}
	resp, err := client.Do(req)
	if err != nil {
		log.Error(err)
		return err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Error(err)
		return err
	}
	err = json.Unmarshal(body, &c)
	if err != nil {
		log.Error(err)
		return err
	}
	return nil
}

// called hourly to keep the key cache up-to-date
func googleLoadKeys() error {
	keys, err := jwk.Fetch(context.Background(), c.JWKURI)
	if err != nil {
		log.Error(err)
		return err
	}
	c.keys = keys
	return nil
}
