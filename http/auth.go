package wasabeehttps

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/lestrrat-go/jwx/v3/jwa"
	"github.com/lestrrat-go/jwx/v3/jws"
	"github.com/lestrrat-go/jwx/v3/jwt"

	"github.com/wasabee-project/Wasabee-Server/Firebase"
	"github.com/wasabee-project/Wasabee-Server/auth"
	"github.com/wasabee-project/Wasabee-Server/config"
	"github.com/wasabee-project/Wasabee-Server/log"
	"github.com/wasabee-project/Wasabee-Server/model"
	"github.com/wasabee-project/Wasabee-Server/util"
)

// used in getAgentInfo and apTokenRoute -- takes a user's Oauth2 token and requests their info
func getOauthUserInfo(ctx context.Context, accessToken string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", config.Get().HTTP.OauthUserInfoURL, nil)
	if err != nil {
		log.Error(err)
		return nil, err
	}
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", accessToken))
	client := &http.Client{
		Timeout: (3 * time.Second),
	}
	resp, err := client.Do(req)
	if err != nil {
		log.Error(err)
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Error(err)
		return nil, err
	}
	return body, nil
}

// apTokenRoute receives a Google Oauth2 token from the Android/iOS app and sets the JWT
func apTokenRoute(res http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	var m struct {
		Gid model.GoogleID `json:"id"`
		Pic string         `json:"picture"`
	}

	// passed in from the clients
	type token struct {
		AccessToken string `json:"accessToken"`
		BadAT       string `json:"access_token"` // some APIs use this name, have it here for logging
	}
	var t token

	if !contentTypeIs(req, jsonTypeShort) {
		err := fmt.Errorf("invalid aptok send (needs to be application/json)")
		http.Error(res, jsonError(err), http.StatusNotAcceptable)
		log.Warn(err)
		return
	}

	jBlob, err := io.ReadAll(req.Body)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	if string(jBlob) == "" {
		err = fmt.Errorf("empty JSON in aptok route")
		log.Warn(err)
		http.Error(res, jsonStatusEmpty, http.StatusNotAcceptable)
		return
	}
	if err = json.Unmarshal(json.RawMessage(jBlob), &t); err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	contents, err := getOauthUserInfo(ctx, t.AccessToken)
	if err != nil {
		log.Info(err)
		err = fmt.Errorf("aptok failed getting agent info from Google")
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	if err = json.Unmarshal(contents, &m); err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	if m.Gid == "" {
		log.Errorw("bad aptok", "from client", t, "from google", m)
		err = fmt.Errorf("no GoogleID set")
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	authorized, err := auth.Authorize(ctx, m.Gid) // V & .rocks authorization takes place here
	if !authorized {
		err = fmt.Errorf("access denied: %v", err)
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	name, err := m.Gid.IngressName(ctx)
	if err != nil {
		log.Error(err)
	}

	agent, err := m.Gid.GetAgent(ctx)
	if err != nil {
		log.Error(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	agent.QueryToken = formValidationToken(req)
	agent.JWT, err = mintjwt(m.Gid)
	if err != nil {
		log.Error(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	data, err := json.Marshal(agent)
	if err != nil {
		log.Error(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	log.Infow("iitc/app login",
		"gid", m.Gid,
		"agent", name,
		"message", name+" login",
		"client", req.Header.Get("User-Agent"),
	)

	// notify other teams of agent login
	_ = wfb.AgentLogin(ctx, m.Gid.TeamListEnabled(ctx), m.Gid)

	// update picture
	_ = m.Gid.UpdatePicture(ctx, m.Pic)

	fmt.Fprint(res, string(data))
}

func mintjwt(gid model.GoogleID) (string, error) {
	sessionName := config.Get().HTTP.SessionName

	hostname, err := os.Hostname()
	if err != nil {
		return "", err
	}

	key, ok := config.JWSigningKeys().Key(0)
	if !ok {
		return "", fmt.Errorf("encryption jwk not set")
	}

	jwts, err := jwt.NewBuilder().
		IssuedAt(time.Now()).
		Subject(string(gid)).
		Issuer(hostname).
		JwtID(util.GenerateID(16)).
		Audience([]string{sessionName}).
		Expiration(time.Now().Add(time.Hour * 24 * 7)).
		Build()
	if err != nil {
		return "", err
	}

	hdrs := jws.NewHeaders()
	_ = hdrs.Set(jws.JWKSetURLKey, config.Get().JKU)

	signed, err := jwt.Sign(jwts, jwt.WithKey(jwa.RS256(), key, jws.WithProtectedHeaders(hdrs)))
	if err != nil {
		return "", err
	}

	return string(signed[:]), nil
}

func oneTimeTokenRoute(res http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	if !contentTypeIs(req, "multipart/form-data") {
		err := fmt.Errorf("invalid content-type (needs to be multipart/form-data)")
		log.Warn(err)
		http.Error(res, jsonError(err), http.StatusNotAcceptable)
		return
	}

	token := model.OneTimeToken(req.FormValue("token"))
	if token == "" {
		err := fmt.Errorf("token not set")
		log.Warn(err)
		http.Error(res, jsonError(err), http.StatusNotAcceptable)
		return
	}

	gid, err := token.Increment(ctx)
	if err != nil {
		incrementScanner(req)
		err := fmt.Errorf("invalid one-time token")
		log.Warn(err)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}

	authorized, err := auth.Authorize(ctx, gid)
	if !authorized {
		err = fmt.Errorf("access denied: %v", err)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	name, err := gid.IngressName(ctx)
	if err != nil {
		log.Error(err)
	}

	agent, err := gid.GetAgent(ctx)
	if err != nil {
		log.Error(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	agent.QueryToken = formValidationToken(req)

	agent.JWT, err = mintjwt(gid)
	if err != nil {
		log.Error(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	data, err := json.Marshal(agent)
	if err != nil {
		log.Error(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	log.Infow("oneTimeToken login",
		"GID", gid,
		"name", name,
		"message", name+" oneTimeToken login",
		"client", req.Header.Get("User-Agent"))

	if err := wfb.AgentLogin(ctx, gid.TeamListEnabled(ctx), gid); err != nil {
		log.Error(err)
	}

	fmt.Fprint(res, string(data))
}
