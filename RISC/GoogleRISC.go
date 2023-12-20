package risc

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"time"

	"github.com/lestrrat-go/jwx/v2/jwk"
	"github.com/lestrrat-go/jwx/v2/jws"
	"github.com/lestrrat-go/jwx/v2/jwt"

	"github.com/wasabee-project/Wasabee-Server/auth"
	"github.com/wasabee-project/Wasabee-Server/config"
	"github.com/wasabee-project/Wasabee-Server/log"
	"github.com/wasabee-project/Wasabee-Server/model"
)

// internal channel
var riscchan chan event

// the top-level configuration, populated by googleRiscDiscover()
var googleConfig struct {
	Issuer      string   `json:"issuer"`
	JWKURI      string   `json:"jwks_uri"`
	AddEndpoint string   `json:"add_subject_endpoint"`
	RemEndpoint string   `json:"remove_subject_endpoint"`
	Methods     []string `json:"delivery_methods_supported"`
}

// the flag to indicate if we are running or not
var running bool

// our credentials to Google
var authdata []byte

// the key set to validate/verify messages from Google
var keys jwk.Set

// the token's event
// {"subject":{"email":"whoever@gmail.com","iss":"https://accounts.google.com","sub":"...gid...","subject_type":"id_token_claims"}, "reason": ""}
type event struct {
	Type    string `json:"subject_type"`
	Reason  string `json:"reason"` // wasabee addition
	Issuer  string `json:"iss"`
	Subject string `json:"sub"`
	Email   string `json:"email"`
}

type riscmsg struct {
	Subject event  `json:"subject"`
	Reason  string `json:"reason"`
}

// Start sets up the data structures and starts the processing threads
func Start(ctx context.Context) {
	c := config.Get()
	if c.RISC.Cert == "" {
		log.Infow("startup", "message", "no configuration, not enabling RISC ")
		return
	}
	full := path.Join(c.Certs, c.RISC.Cert)

	if _, err := os.Stat(full); err != nil {
		log.Infow("startup", "message", "credentials do not exist, not enabling RISC", "credentials", full)
	}
	// #nosec
	tmp, err := ioutil.ReadFile(full)
	if err != nil {
		log.Error(err)
		return
	}
	authdata = tmp

	// load config from google
	if err := googleRiscDiscovery(); err != nil {
		log.Error(err)
		return
	}

	// make a channel to read for events
	riscchan = make(chan event, 1)

	// start a goroutine for keeping the connection to Google fresh
	go registerWebhook(ctx)

	// no need to wait for ctx.Done() since registerWebhook does and it calls disableWebhook, which closes riscchan...
	for e := range riscchan {
		gid := model.GoogleID(e.Subject)
		switch e.Type {
		case "https://schemas.openid.net/secevent/risc/event-type/account-disabled":
			log.Errorw("locking account", "subsystem", "RISC", "GID", gid, "subject", e.Subject, "issuer", e.Issuer, "reason", e.Reason)
			_ = gid.Lock(e.Reason)
			_ = gid.RemoveAllFirebaseTokens()
			auth.Logout(gid, e.Reason)
		case "https://schemas.openid.net/secevent/risc/event-type/account-enabled":
			log.Infow("unlocking account", "subsystem", "RISC", "GID", gid, "subject", e.Subject, "issuer", e.Issuer, "reason", e.Reason)
			_ = gid.Unlock(e.Reason)
		case "https://schemas.openid.net/secevent/risc/event-type/account-purged":
			log.Errorw("deleting account", "subsystem", "RISC", "GID", gid, "subject", e.Subject, "issuer", e.Issuer, "reason", e.Reason)
			auth.Logout(gid, e.Reason)
			_ = gid.Delete()
		case "https://schemas.openid.net/secevent/risc/event-type/account-credential-change-required":
			log.Debugw("credential change", "subsystem", "RISC", "GID", gid, "issuer", e.Issuer, "subject", e.Subject, "reason", e.Reason)
			_ = gid.RemoveAllFirebaseTokens()
			auth.Logout(gid, e.Reason)
		case "https://schemas.openid.net/secevent/risc/event-type/sessions-revoked":
			log.Debugw("sessions revoked", "subsystem", "RISC", "GID", gid, "issuer", e.Issuer, "subject", e.Subject, "reason", e.Reason)
			_ = gid.RemoveAllFirebaseTokens()
			auth.Logout(gid, e.Reason)
		case "https://schemas.openid.net/secevent/risc/event-type/tokens-revoked":
			log.Debugw("tokens revoked", "subsystem", "RISC", "GID", gid, "issuer", e.Issuer, "subject", e.Subject, "reason", e.Reason)
			_ = gid.RemoveAllFirebaseTokens()
			auth.Logout(gid, e.Reason)
		case "https://schemas.openid.net/secevent/risc/event-type/verification":
			// log.Debugw("verify", "subsystem", "RISC", "GID", gid,  "issuer", e.Issuer, "subject", e.Subject, "reason", e.Reason)
			// no need to do anything
		case "https://accounts.google.com/risc/event/sessions-revoked":
			log.Debugw("google sessions revoked", "subsystem", "RISC", "GID", gid, "issuer", e.Issuer, "subject", e.Subject, "reason", e.Reason)
			_ = gid.RemoveAllFirebaseTokens()
			auth.Logout(gid, e.Reason)
		default:
			log.Warnw("unknown event", "subsystem", "RISC", "type", e.Type, "reason", e.Reason)
		}
	}
}

// This is called from the webhook
func validateToken(rawjwt []byte) error {
	// log.Debugw("RISC token", "raw", rawjwt)
	token, err := jwt.Parse(rawjwt,
		jwt.WithValidate(true),
		jwt.WithIssuer("https://accounts.google.com"),
		jwt.WithKeySet(keys, jws.WithInferAlgorithmFromKey(true), jws.WithUseDefault(true)),
		jwt.WithAcceptableSkew(20*time.Second))
	if err != nil {
		log.Errorw("RISC", "error", err.Error(), "subsystem", "RISC")
		return err
	}
	if token == nil {
		err := fmt.Errorf("unable to verify RISC event")
		log.Errorw(err.Error(), "subsystem", "RISC")
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

		// it is JSON, just unmarshal it...
		x := v.(map[string]interface{})
		if x["reason"] != nil {
			r.Subject.Reason = x["reason"].(string)
		}
		y := x["subject"].(map[string]interface{})
		r.Subject.Issuer = y["iss"].(string)
		r.Subject.Subject = y["sub"].(string)
		r.Subject.Email = y["email"].(string)

		riscchan <- r.Subject
	}
	return nil
}

func googleRiscDiscovery() error {
	req, err := http.NewRequest("GET", config.Get().RISC.Discovery, nil)
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
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Error(err)
		return err
	}
	err = json.Unmarshal(body, &googleConfig)
	if err != nil {
		log.Error(err)
		return err
	}
	return nil
}
