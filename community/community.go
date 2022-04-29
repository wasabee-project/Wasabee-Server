package community

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"golang.org/x/time/rate"

	"github.com/lestrrat-go/jwx/v2/jwa"
	"github.com/lestrrat-go/jwx/v2/jws"
	"github.com/lestrrat-go/jwx/v2/jwt"

	"github.com/wasabee-project/Wasabee-Server/config"
	"github.com/wasabee-project/Wasabee-Server/log"
	"github.com/wasabee-project/Wasabee-Server/model"
)

var limiter *rate.Limiter

func init() {
	limiter = rate.NewLimiter(rate.Limit(0.5), 10)
}

// the top-level data structure defined by the community website
type pull struct {
	Profile profile // the one we are concerrned with
	// the following are present on errors
	Code      uint16
	Exception string
	Class     string
	// all other fields are ignored
}

// the profile type defined by the community website
type profile struct {
	Name  string
	About string
	// all other fields are ignored
}

const profileURL = "https://community.ingress.com/en/profile"
const xgid = "x-gid" // custom claim name for GoogleID; not "sub" to prevent use as authorization
const xme = "x-me"   // custom claim name for "name"
const aud = "c2g"    // short for: community to GoogleID

// Validate checks the community website for the token and makes sure the token is correct
func Validate(gid model.GoogleID, name string) (bool, error) {
	profile, err := fetch(name)
	if err != nil {
		return false, err
	}

	if profile.Name != name {
		err := fmt.Errorf("requested name does not match profile name")
		log.Errorw(err.Error(), "requested", name, "profile", profile.Name)
		return false, nil // NotAcceptable
	}

	if err := checkJWT(strings.TrimSpace(profile.About), profile.Name, gid); err != nil {
		return false, nil // nil to trigger NotAcceptable rather than InternalServerError
	}

	if err := gid.SetCommunityName(profile.Name); err != nil {
		return false, err
	}
	message := fmt.Sprintf("validated community name for %s", name)
	log.Infow("validated", "gid", gid, "name", profile.Name, "requested", name, "message", message)
	return true, nil
}

func fetch(name string) (*profile, error) {
	p := pull{}

	ctx, cancel := context.WithTimeout(context.Background(), 5)
	defer cancel()
	if err := limiter.Wait(ctx); err != nil {
		log.Warn(err)
		// just keep going
	}

	apiurl := fmt.Sprintf("%s/%s.json", profileURL, name)
	req, err := http.NewRequest("GET", apiurl, nil)
	if err != nil {
		log.Errorw(err.Error(), "fetch", name)
		return &p.Profile, err
	}
	client := &http.Client{
		Timeout: (3 * time.Second),
	}
	resp, err := client.Do(req)
	if err != nil {
		log.Errorw(err.Error(), "fetch", name)
		return &p.Profile, err
	}

	if err := json.NewDecoder(resp.Body).Decode(&p); err != nil {
		log.Error(err)
		return &p.Profile, err
	}

	if p.Exception != "" {
		err := fmt.Errorf(p.Exception)
		log.Errorw(err.Error(), "code", p.Code, "class", p.Class)
		return &p.Profile, err
	}

	return &p.Profile, nil
}

// move the constants into the config package
func checkJWT(raw, name string, gid model.GoogleID) error {
	token, err := jwt.Parse(
		[]byte(raw),
		jwt.WithKeySet(config.JWParsingKeys(), jws.WithInferAlgorithmFromKey(true), jws.WithUseDefault(true)),
	)
	if err != nil {
		log.Errorw("community token parse failed", "err", err.Error(), "gid", gid, "name", name)
		return err
	}

	if err := jwt.Validate(token,
		jwt.WithAudience(aud),
		jwt.WithClaimValue(xme, name),
		jwt.WithClaimValue(xgid, string(gid))); err != nil {
		log.Errorw("community token validate failed", "err", err.Error(), "gid", gid, "name", name)
		return err
	}
	return nil
}

// BuildToken generates a token to be posted on the community site to verify the agent's name
func BuildToken(gid model.GoogleID, name string) (string, error) {
	t, err := model.CommunityNameToGID(name)
	if err != nil {
		log.Error(err)
		return "", err
	}
	if t == gid {
		err := fmt.Errorf("This wasabee account is already linked to this community name (%s) (%s)", name, t)
		log.Errorw(err.Error(), "gid", gid, "name", name, "owner", t)
		return "", err
	}
	if t != "" {
		err := fmt.Errorf("name '%s' already claimed by GID '%s'", name, t)
		log.Errorw(err.Error(), "gid", gid, "name", name, "owner", t)
		return "", err
	}

	key, ok := config.JWSigningKeys().Key(0)
	if !ok {
		err := fmt.Errorf("encryption jwk not set")
		log.Error(err)
		return "", err
	}

	jwts, err := jwt.NewBuilder().
		Claim(xgid, string(gid)).
		Claim(xme, name).
		Audience([]string{aud}).
		Build()
	if err != nil {
		log.Error(err)
		return "", err
	}

	hdrs := jws.NewHeaders()
	_ = hdrs.Set(jws.JWKSetURLKey, config.Get().JKU)

	signed, err := jwt.Sign(jwts, jwt.WithKey(jwa.RS256, key, jws.WithProtectedHeaders(hdrs)))
	if err != nil {
		log.Error(err)
		return "", err
	}
	return string(signed), nil
}
