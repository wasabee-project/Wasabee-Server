package risc

import (
	"bytes"
	"context"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

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
	Subject event `json:"subject"`
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
		case "https://accounts.google.com/risc/event/sessions-revoked":
			wasabee.Log.Warnw("logout", "subsystem", "RISC", "GID", gid, "issuer", e.Issuer, "subject", e.Subject, "reason", e.Reason)
			gid.FirebaseRemoveAllTokens()
			gid.Logout(e.Reason)
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
		token, err = jwt.Parse(bytes.NewReader(rawjwt),
		  // jwt.WithValidate(true), // coming soon
		  jwt.WithVerify(jwa.RS256, &pk),
		  jwt.WithIssuer("https://accounts.google.com"),
		)
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
		err := fmt.Errorf("unable to verify RISC event")
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
		var r riscmsg
		r.Subject.Type = k

		// just respond to verification requests instantly
		if k == "https://schemas.openid.net/secevent/risc/event-type/verification" {
			r.Subject.Reason = "ping requsted"
			riscchan <- r.Subject
			continue
		}

		wasabee.Log.Infow("RISC event", "subsystem", "RISC", "type", k, "data", v, "message", "RISC event")

		// XXX this is ugly and brittle - it is JSON, just unmarshal it.
		x := v.(map[string]interface{})
		
		/* if err := json.Unmarshal(x, &r); err != nil {
			wasabee.Log.Errorw(err.Error(), "subsystem", "RISC", "message", err.Error(), "data", x)
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
