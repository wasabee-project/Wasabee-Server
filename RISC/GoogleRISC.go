package risc

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/lestrrat-go/jwx/jwa"
	"github.com/lestrrat-go/jwx/jwk"
	"github.com/lestrrat-go/jwx/jws"
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
			wasabee.Log.Criticalf("locking %s because %s said: %s", e.Subject, e.Issuer, e.Reason)
			_ = gid.Lock(e.Reason)
			gid.FirebaseRemoveAllTokens()
			gid.Logout(e.Reason)
		case "https://schemas.openid.net/secevent/risc/event-type/account-enabled":
			wasabee.Log.Criticalf("unlocking %s because %s said: %s", e.Subject, e.Issuer, e.Reason)
			_ = gid.Unlock(e.Reason)
		case "https://schemas.openid.net/secevent/risc/event-type/account-purged":
			wasabee.Log.Criticalf("deleting %s because %s said: %s", e.Subject, e.Issuer, e.Reason)
			gid.Logout(e.Reason)
			_ = gid.Delete()
		case "https://schemas.openid.net/secevent/risc/event-type/account-credential-change-required":
			wasabee.Log.Noticef("%s requested logout for %s: %s", e.Issuer, e.Subject, e.Reason)
			gid.FirebaseRemoveAllTokens()
			gid.Logout(e.Reason)
		case "https://schemas.openid.net/secevent/risc/event-type/sessions-revoked":
			wasabee.Log.Noticef("%s requested logout for %s: %s", e.Issuer, e.Subject, e.Reason)
			gid.FirebaseRemoveAllTokens()
			gid.Logout(e.Reason)
		case "https://schemas.openid.net/secevent/risc/event-type/tokens-revoked":
			wasabee.Log.Noticef("%s requested logout for %s: %s", e.Issuer, e.Subject, e.Reason)
			gid.FirebaseRemoveAllTokens()
			gid.Logout(e.Reason)
		case "https://schemas.openid.net/secevent/risc/event-type/verification":
			// no need to do anything
		default:
			wasabee.Log.Noticef("Unknown event %s (%s)", e.Type, e.Reason)
		}
	}
}

// This is called from the webhook
func validateToken(rawjwt []byte) error {
	token, err := jwt.ParseBytes(rawjwt)
	if err != nil {
		wasabee.Log.Error(err)
		return err
	}

	kid, keyOK := token.Get("kid")
	// if it is signed, verify it
	if keyOK {
		key := config.keys.LookupKeyID(kid.(string))
		if len(key) == 0 {
			err = fmt.Errorf("no matching key")
			wasabee.Log.Error(err)
			return err
		}
		if len(key) != 1 {
			// pick the first that is RS256?
			err = fmt.Errorf("multiple matching keys found, using the first")
			wasabee.Log.Notice(err)
		}
		r, err := key[0].Materialize()
		if err != nil {
			wasabee.Log.Error(err)
			return err
		}

		// this checks iss, iat, aud and others
		err = token.Verify(jwt.WithAudience(config.clientemail), jwt.WithAcceptableSkew(60), jwt.WithIssuer(config.Issuer), jwt.WithVerify(jwa.RS256, r))
		if err != nil {
			wasabee.Log.Error(err)
			return err
		}
		// this checks the signature
		_, err = jws.Verify(rawjwt, jwa.RS256, r)
		if err != nil {
			wasabee.Log.Error(err)
			return err
		}
	}

	tmp, ok := token.Get("events")
	if !ok {
		wasabee.Log.Error(err)
		return err
	}

	// multiple events per message are possible
	for k, v := range tmp.(map[string]interface{}) {
		var e event
		e.Type = k

		// verification types are not signed, no KID, just respond instantly
		if k == "https://schemas.openid.net/secevent/risc/event-type/verification" {
			e.Reason = "ping requsted"
			riscchan <- e
		} else if keyOK {
			wasabee.Log.Noticef("verified RISC event: (%s) %s", k, v)

			// XXX this is ugly and brittle - use a map parser
			x := v.(map[string]interface{})
			e.Reason = x["reason"].(string)
			y := x["subject"].(map[string]interface{})
			e.Issuer = y["iss"].(string)
			e.Subject = y["sub"].(string)
			riscchan <- e
		} else {
			wasabee.Log.Criticalf("non-verified request: (%s) %s", k, v)

			// XXX this is ugly and brittle - use a map parser
			x := v.(map[string]interface{})
			e.Reason = x["reason"].(string)
			y := x["subject"].(map[string]interface{})
			e.Issuer = y["iss"].(string)
			e.Subject = y["sub"].(string)
			wasabee.Log.Debug(e)
			riscchan <- e
		}
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
