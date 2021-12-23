package community

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/lestrrat-go/jwx/jwa"
	"github.com/lestrrat-go/jwx/jws"
	"github.com/lestrrat-go/jwx/jwt"

	"github.com/wasabee-project/Wasabee-Server/config"
	"github.com/wasabee-project/Wasabee-Server/log"
	"github.com/wasabee-project/Wasabee-Server/model"
)

type pull struct {
	Profile   profile
	Code      uint8
	Exception string
	Class     string
}

type profile struct {
	UserID   uint64
	Name     string
	Title    string
	Location string
	About    string
}

const profileURL = "https://community.ingress.com/en/profile"

func Validate(gid model.GoogleID, name string) (bool, error) {
	profile, err := fetch(name)
	if err != nil {
		return false, err
	}

	if err := validate(strings.TrimSpace(profile.About), name, gid); err != nil {
		return false, err
	}

	gid.SetCommunityName(name)
	log.Infow("validated niantic community name", "gid", gid, "name", name)
	return true, nil
}

func fetch(name string) (*profile, error) {
	p := pull{}

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
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Error(err)
		return &p.Profile, err
	}

	err = json.Unmarshal(body, &p)
	if err != nil {
		log.Error(err)
		return &p.Profile, err
	}

	if p.Exception != "" {
		err := fmt.Errorf(p.Exception)
		log.Errorw(err.Error(), "code", p.Code, "class", p.Class)
		return &p.Profile, nil
	}

	log.Debugw("community profile", "p", p.Profile)
	return &p.Profile, nil
}

// move the constants into the config package
// c2g, x-me, x-gid
func validate(raw, name string, gid model.GoogleID) error {
	token, err := jwt.Parse([]byte(raw), jwt.InferAlgorithmFromKey(true), jwt.UseDefaultKey(true), jwt.WithKeySet(config.Get().JWParsingKeys))
	if err != nil {
		log.Errorw("community token parse failed", "err", err.Error(), "gid", gid, "name", name)
		return err
	}

	if err := jwt.Validate(token, jwt.WithAudience("c2g"), jwt.WithClaimValue("x-me", name), jwt.WithClaimValue("x-gid", string(gid))); err != nil {
		log.Errorw("community token validate failed", "err", err.Error(), "gid", gid, "name", name)
		return err
	}
	return nil
}

func BuildToken(gid model.GoogleID, name string) (string, error) {
	key, ok := config.Get().JWSigningKeys.Get(0)
	if !ok {
		err := fmt.Errorf("encryption jwk not set")
		log.Error(err)
		return "", err
	}

	jwts, err := jwt.NewBuilder().
		Claim("x-gid", string(gid)).
		Claim("x-me", name).
		Audience([]string{"c2g"}).
		Build()
	if err != nil {
		log.Error(err)
		return "", err
	}

	hdrs := jws.NewHeaders()
	hdrs.Set("jku", "https://cdn2.wasabee.rocks/.well-known/jwks.json")

	signed, err := jwt.Sign(jwts, jwa.RS256, key, jwt.WithHeaders(hdrs))
	if err != nil {
		log.Error(err)
		return "", err
	}
	return string(signed), nil
}
