package risc

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path"
	"time"

	"github.com/lestrrat-go/jwx/v3/jwk"
	"github.com/lestrrat-go/jwx/v3/jws"
	"github.com/lestrrat-go/jwx/v3/jwt"

	"github.com/wasabee-project/Wasabee-Server/auth"
	"github.com/wasabee-project/Wasabee-Server/config"
	"github.com/wasabee-project/Wasabee-Server/log"
	"github.com/wasabee-project/Wasabee-Server/model"
)

// internal channel
var riscchan chan event

var googleConfig struct {
	Issuer      string   `json:"issuer"`
	JWKURI      string   `json:"jwks_uri"`
	AddEndpoint string   `json:"add_subject_endpoint"`
	RemEndpoint string   `json:"remove_subject_endpoint"`
	Methods     []string `json:"delivery_methods_supported"`
}

var running bool
var authdata []byte
var keys jwk.Set

type event struct {
	Type    string          `json:"subject_type"`
	Reason  string          `json:"reason"`
	Issuer  string          `json:"iss"`
	Subject string          `json:"sub"`
	Email   string          `json:"email"`
	ctx     context.Context // Added to pass context to the processor
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

	tmp, err := os.ReadFile(full)
	if err != nil {
		log.Error(err)
		return
	}
	authdata = tmp

	if err := googleRiscDiscovery(ctx); err != nil {
		log.Error(err)
		return
	}

	riscchan = make(chan event, 1)

	// start a goroutine for keeping the connection to Google fresh
	go registerWebhook(ctx)

	for {
		select {
		case <-ctx.Done():
			log.Infow("shutting down RISC processor")
			return
		case e, ok := <-riscchan:
			if !ok {
				return
			}

			// Process each event using the system context
			// If we want it to be cancelable per-event, we could use e.ctx,
			// but e.ctx from the webhook will be canceled as soon as the HTTP response is sent.
			processEvent(ctx, e)
		}
	}
}

func processEvent(ctx context.Context, e event) {
	gid := model.GoogleID(e.Subject)
	switch e.Type {
	case "https://schemas.openid.net/secevent/risc/event-type/account-disabled":
		log.Errorw("locking account", "subsystem", "RISC", "GID", gid, "reason", e.Reason)
		_ = gid.Lock(ctx, e.Reason)
		_ = gid.RemoveAllFirebaseTokens(ctx)
		auth.Logout(gid, e.Reason)

	case "https://schemas.openid.net/secevent/risc/event-type/account-enabled":
		log.Infow("unlocking account", "subsystem", "RISC", "GID", gid, "reason", e.Reason)
		_ = gid.Unlock(ctx, e.Reason)

	case "https://schemas.openid.net/secevent/risc/event-type/account-purged":
		log.Errorw("deleting account", "subsystem", "RISC", "GID", gid, "reason", e.Reason)
		auth.Logout(gid, e.Reason)
		_ = gid.Delete(ctx)

	case "https://schemas.openid.net/secevent/risc/event-type/account-credential-change-required",
		"https://schemas.openid.net/secevent/risc/event-type/sessions-revoked",
		"https://schemas.openid.net/secevent/risc/event-type/tokens-revoked",
		"https://accounts.google.com/risc/event/sessions-revoked":
		log.Debugw("security event: revoking sessions", "subsystem", "RISC", "type", e.Type, "GID", gid)
		_ = gid.RemoveAllFirebaseTokens(ctx)
		auth.Logout(gid, e.Reason)

	case "https://schemas.openid.net/secevent/risc/event-type/verification":
		// no-op
	default:
		log.Warnw("unknown event", "subsystem", "RISC", "type", e.Type)
	}
}

// This is called from the webhook (which should pass req.Context())
func validateToken(ctx context.Context, rawjwt []byte) error {
	token, err := jwt.Parse(rawjwt,
		jwt.WithValidate(true),
		jwt.WithIssuer("https://accounts.google.com"),
		jwt.WithKeySet(keys, jws.WithInferAlgorithmFromKey(true), jws.WithUseDefault(true)),
		jwt.WithAcceptableSkew(20*time.Second))
	if err != nil {
		return err
	}

	var events interface{}
	if err := token.Get("events", &events); err != nil {
		return err
	}

	for k, v := range events.(map[string]interface{}) {
		var r riscmsg
		r.Subject.Type = k
		r.Subject.ctx = ctx // Attach context (though we use system ctx for DB)

		if k == "https://schemas.openid.net/secevent/risc/event-type/verification" {
			r.Subject.Reason = "ping requested"
			riscchan <- r.Subject
			continue
		}

		x := v.(map[string]interface{})
		if x["reason"] != nil {
			r.Subject.Reason = x["reason"].(string)
		}

		// Handle potential nil subjects/issuers safely
		if sub, ok := x["subject"].(map[string]interface{}); ok {
			r.Subject.Issuer, _ = sub["iss"].(string)
			r.Subject.Subject, _ = sub["sub"].(string)
			r.Subject.Email, _ = sub["email"].(string)
		}

		riscchan <- r.Subject
	}
	return nil
}

func googleRiscDiscovery(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", config.Get().RISC.Discovery, nil)
	if err != nil {
		return err
	}
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return json.NewDecoder(resp.Body).Decode(&googleConfig)
}
