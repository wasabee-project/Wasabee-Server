package wasabeehttps

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
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

// get the logged in agent
func meRoute(res http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	gid, err := getAgentID(req)
	if err != nil {
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	agent, err := gid.GetAgent(ctx)
	if err != nil {
		log.Error(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	agent.QueryToken = formValidationToken(req)

	res.Header().Set("Cache-Control", "no-store")
	json.NewEncoder(res).Encode(&agent)
}

// use this to verify that form data is sent from a client that requested it
func formValidationToken(req *http.Request) string {
	ip, _, _ := net.SplitHostPort(req.RemoteAddr)
	toHash := fmt.Sprintf("%s %s %s", req.Header.Get("User-Agent"), ip, config.GetOauthConfig().ClientSecret)
	hasher := sha256.New()
	_, _ = hasher.Write([]byte(toHash))
	return base64.URLEncoding.EncodeToString(hasher.Sum(nil))
}

func meToggleTeamRoute(res http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	gid, err := getAgentID(req)
	if err != nil {
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	team := model.TeamID(req.PathValue("team"))
	state := req.PathValue("state")
	b := false
	if state == "On" || state == "on" {
		b = true
	}

	if err = gid.SetTeamState(ctx, team, b); err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	fmt.Fprint(res, jsonStatusOK)
}

func meToggleTeamWDShareRoute(res http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	gid, err := getAgentID(req)
	if err != nil {
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	team := model.TeamID(req.PathValue("team"))
	state := req.PathValue("state")
	b := false
	if state == "On" || state == "on" {
		b = true
	}

	if err = gid.SetWDShare(ctx, team, b); err != nil {
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	fmt.Fprint(res, jsonStatusOK)
}

func meToggleTeamWDLoadRoute(res http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	gid, err := getAgentID(req)
	if err != nil {
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	team := model.TeamID(req.PathValue("team"))
	state := req.PathValue("state")
	b := false
	if state == "On" || state == "on" {
		b = true
	}

	if err = gid.SetWDLoad(ctx, team, b); err != nil {
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	fmt.Fprint(res, jsonStatusOK)
}

func meRemoveTeamRoute(res http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	gid, err := getAgentID(req)
	if err != nil {
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	team := model.TeamID(req.PathValue("team"))

	if err = team.RemoveAgent(ctx, gid); err != nil {
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	fmt.Fprint(res, jsonStatusOK)
}

func meSetAgentLocationRoute(res http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	gid, err := getAgentID(req)
	if err != nil {
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	lat := req.PathValue("lat")
	lon := req.PathValue("lon")

	if err = gid.SetLocation(ctx, lat, lon); err != nil {
		http.Error(res, jsonError(err), http.StatusNotAcceptable)
		return
	}

	// announce to teams with which this agent is sharing location information
	// fire-and-forget: use context.Background()
	go func() {
		wfb.AgentLocation(context.Background(), gid)
	}()

	fmt.Fprint(res, jsonStatusOK)
}

func meDeleteRoute(res http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	gid, err := getAgentID(req)
	if err != nil {
		http.Error(res, jsonError(err), http.StatusUnauthorized)
		return
	}

	qt := req.FormValue("qt")
	qtTest := formValidationToken(req)
	if qt != qtTest {
		err := fmt.Errorf("invalid form validation token")
		log.Errorw(err.Error(), "got", qt, "wanted", qtTest)
		http.Error(res, jsonError(err), http.StatusNotAcceptable)
		return
	}

	log.Warnw("agent requested delete", "GID", gid.String())
	if err := gid.Delete(ctx); err != nil {
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	fmt.Fprint(res, jsonStatusOK)
}

func meLogoutRoute(res http.ResponseWriter, req *http.Request) {
	gid, err := getAgentID(req)
	if err != nil {
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	auth.Logout(gid, "user requested")
	fmt.Fprint(res, jsonStatusOK)
}

func meFirebaseRoute(res http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	gid, err := getAgentID(req)
	if err != nil {
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	t, err := io.ReadAll(req.Body)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	token := string(t)
	if token == "" {
		err := fmt.Errorf("firebase token empty")
		log.Debugw(err.Error(), "gid", gid)
		http.Error(res, jsonError(err), http.StatusNotAcceptable)
		return
	}
	if err := gid.StoreFirebaseToken(ctx, token); err != nil {
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	fmt.Fprint(res, jsonStatusOK)
}

func meIntelIDRoute(res http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	gid, err := getAgentID(req)
	if err != nil {
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	if err := req.ParseMultipartForm(1024); err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusNotAcceptable)
		return
	}
	qt := req.FormValue("qt")
	name := util.Sanitize(req.FormValue("name"))
	faction := req.FormValue("faction")

	qtTest := formValidationToken(req)
	if qt != qtTest {
		err := fmt.Errorf("invalid form validation token")
		log.Errorw(err.Error(), "got", qt, "wanted", qtTest)
		http.Error(res, jsonError(err), http.StatusNotAcceptable)
		return
	}

	if err := gid.SetIntelData(ctx, name, faction); err != nil {
		http.Error(res, jsonError(err), http.StatusNotAcceptable)
		return
	}

	fmt.Fprint(res, jsonStatusOK)
}

func meJwtRefreshRoute(res http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	gid, err := getAgentID(req)
	if err != nil {
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	ok, err := auth.Authorize(ctx, gid)
	if !ok {
		err := fmt.Errorf("account disabled")
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	key, ok := config.JWSigningKeys().Key(0)
	if !ok {
		err := fmt.Errorf("encryption jwk not set")
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	token, err := jwt.ParseRequest(req, jwt.WithKeySet(config.JWParsingKeys(), jws.WithInferAlgorithmFromKey(true), jws.WithUseDefault(true)))
	if err != nil {
		log.Info(err)
		http.Error(res, err.Error(), http.StatusUnauthorized)
		return
	}

	sessionName := config.Get().HTTP.SessionName
	if err := jwt.Validate(token, jwt.WithAudience(sessionName)); err != nil {
		log.Info(err)
		http.Error(res, err.Error(), http.StatusUnauthorized)
		return
	}

	jwtid, ok := token.JwtID()
	if !ok {
		err := fmt.Errorf("missing token ID")
		log.Info(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	hostname, err := os.Hostname()
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	jwts, err := jwt.NewBuilder().
		IssuedAt(time.Now()).
		Subject(string(gid)).
		Issuer(hostname).
		JwtID(jwtid).
		Audience([]string{"wasabee"}).
		Expiration(time.Now().Add(time.Hour * 24 * 7)).
		Build()
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	hdrs := jws.NewHeaders()
	_ = hdrs.Set(jws.JWKSetURLKey, config.Get().JKU)

	signed, err := jwt.Sign(jwts, jwt.WithKey(jwa.RS256(), key, jws.WithProtectedHeaders(hdrs)))
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	log.Infow("jwt Refresh", "gid", gid, "token ID", jwtid, "message", "jwt Token refreshed for "+gid)

	json.NewEncoder(res).Encode(struct {
		Status string `json:"status"`
		JWT    string `json:"jwt"`
	}{
		Status: "ok",
		JWT:    string(signed),
	})
}
