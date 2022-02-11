package wasabeehttps

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"

	"github.com/lestrrat-go/jwx/jwa"
	// "github.com/lestrrat-go/jwx/jwk"
	"github.com/lestrrat-go/jwx/jws"
	"github.com/lestrrat-go/jwx/jwt"

	"github.com/wasabee-project/Wasabee-Server/Firebase"
	"github.com/wasabee-project/Wasabee-Server/auth"
	"github.com/wasabee-project/Wasabee-Server/community"
	"github.com/wasabee-project/Wasabee-Server/config"
	"github.com/wasabee-project/Wasabee-Server/federation"
	"github.com/wasabee-project/Wasabee-Server/log"
	"github.com/wasabee-project/Wasabee-Server/model"
	"github.com/wasabee-project/Wasabee-Server/util"
)

// get the logged in agent
func meRoute(res http.ResponseWriter, req *http.Request) {
	gid, err := getAgentID(req)
	if err != nil {
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	agent, err := gid.GetAgent()
	if err != nil {
		log.Error(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	agent.QueryToken = formValidationToken(req)

	data, _ := json.Marshal(agent)
	res.Header().Set("Cache-Control", "no-store")
	fmt.Fprint(res, string(data))
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
	gid, err := getAgentID(req)
	if err != nil {
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	team := model.TeamID(vars["team"])
	state := vars["state"]
	b := false
	if state == "On" || state == "on" {
		b = true
	}

	if err = gid.SetTeamState(team, b); err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	fmt.Fprint(res, jsonStatusOK)
}

func meToggleTeamWDShareRoute(res http.ResponseWriter, req *http.Request) {
	gid, err := getAgentID(req)
	if err != nil {
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	team := model.TeamID(vars["team"])
	state := vars["state"]
	b := false
	if state == "On" || state == "on" {
		b = true
	}

	if err = gid.SetWDShare(team, b); err != nil {
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	fmt.Fprint(res, jsonStatusOK)
}

func meToggleTeamWDLoadRoute(res http.ResponseWriter, req *http.Request) {
	gid, err := getAgentID(req)
	if err != nil {
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	team := model.TeamID(vars["team"])
	state := vars["state"]
	b := false
	if state == "On" || state == "on" {
		b = true
	}

	if err = gid.SetWDLoad(team, b); err != nil {
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	fmt.Fprint(res, jsonStatusOK)
}

func meRemoveTeamRoute(res http.ResponseWriter, req *http.Request) {
	gid, err := getAgentID(req)
	if err != nil {
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	team := model.TeamID(vars["team"])

	if err = team.RemoveAgent(gid); err != nil {
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	fmt.Fprint(res, jsonStatusOK)
}

func meSetAgentLocationRoute(res http.ResponseWriter, req *http.Request) {
	gid, err := getAgentID(req)
	if err != nil {
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	lat := vars["lat"]
	lon := vars["lon"]

	if err = gid.SetLocation(lat, lon); err != nil {
		http.Error(res, jsonError(err), http.StatusNotAcceptable)
		return
	}

	// announce to teams with which this agent is sharing location information
	go wfb.AgentLocation(gid)

	flat, err := strconv.ParseFloat(lat, 32)
	if err != nil {
		log.Error(err)
		flat = float64(0)
	}

	flon, err := strconv.ParseFloat(lon, 32)
	if err != nil {
		log.Error(err)
		flon = float64(0)
	}

	go federation.SetAgentLocation(context.Background(), gid, float32(flat), float32(flon))

	fmt.Fprint(res, jsonStatusOK)
}

func meDeleteRoute(res http.ResponseWriter, req *http.Request) {
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
	if err := gid.Delete(); err != nil {
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
	ses, err := store.Get(req, config.Get().HTTP.SessionName)
	if err != nil {
		log.Error(err)
		_ = ses.Save(req, res)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	delete(ses.Values, "nonce")
	delete(ses.Values, "id")
	delete(ses.Values, "loginReq")
	res.Header().Set("Connection", "close")

	ses.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   -1,
		SameSite: http.SameSiteNoneMode,
		Secure:   true,
	}
	_ = ses.Save(req, res)

	auth.Logout(gid, "user requested")
	fmt.Fprint(res, jsonStatusOK)
}

func meFirebaseRoute(res http.ResponseWriter, req *http.Request) {
	gid, err := getAgentID(req)
	if err != nil {
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	t, err := ioutil.ReadAll(req.Body)
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
	err = gid.StoreFirebaseToken(token)
	if err != nil {
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	go federation.AddFirebaseToken(context.Background(), gid, token)

	fmt.Fprint(res, jsonStatusOK)
}

func meIntelIDRoute(res http.ResponseWriter, req *http.Request) {
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

	if err := gid.SetIntelData(name, faction); err != nil {
		http.Error(res, jsonError(err), http.StatusNotAcceptable)
		return
	}

	go federation.SetIntelData(context.Background(), gid, name, faction)

	fmt.Fprint(res, jsonStatusOK)
}

func meVAPIkeyRoute(res http.ResponseWriter, req *http.Request) {
	gid, err := getAgentID(req)
	if err != nil {
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	v := req.FormValue("v")

	if err = gid.SetVAPIkey(v); err != nil {
		http.Error(res, jsonError(err), http.StatusNotAcceptable)
		return
	}

	fmt.Fprint(res, jsonStatusOK)
}

func meJwtRefreshRoute(res http.ResponseWriter, req *http.Request) {
	gid, err := getAgentID(req)
	if err != nil {
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	log.Debugw("refrehing JWT", "GID", gid)

	ok, err := auth.Authorize(gid)
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

	key, ok := config.JWSigningKeys().Get(0)
	if !ok {
		err := fmt.Errorf("encryption jwk not set")
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	token, err := jwt.ParseRequest(req, jwt.InferAlgorithmFromKey(true), jwt.UseDefaultKey(true), jwt.WithKeySet(config.JWParsingKeys()))
	if err != nil {
		log.Info(err)
		http.Error(res, err.Error(), http.StatusUnauthorized)
		return
	}

	// this was already done in authMW, but double-check it here
	sessionName := config.Get().HTTP.SessionName
	if err := jwt.Validate(token, jwt.WithAudience(sessionName)); err != nil {
		log.Info(err)
		http.Error(res, err.Error(), http.StatusUnauthorized)
		return
	}

	jwtid := token.JwtID()

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

	// get refreshed count from token
	// increment and set refreshed count

	// let consumers know where to get the keys if they want to verify
	hdrs := jws.NewHeaders()
	_ = hdrs.Set(jws.JWKSetURLKey, config.Get().JKU)

	signed, err := jwt.Sign(jwts, jwa.RS256, key, jwt.WithHeaders(hdrs))
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	log.Infow("jwt Refresh", "gid", gid, "token ID", jwtid, "message", "jwt Token refreshed for "+gid)
	s := fmt.Sprintf("{\"status\":\"ok\", \"jwt\":\"%s\"}", string(signed[:]))
	fmt.Fprint(res, s)
}

func meCommProofRoute(res http.ResponseWriter, req *http.Request) {
	gid, err := getAgentID(req)
	if err != nil {
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	name := util.Sanitize(vars["name"])
	if name == "" {
		err := fmt.Errorf("name unset")
		log.Debug(err)
		http.Error(res, jsonError(err), http.StatusNotAcceptable)
		return
	}

	signed, err := community.BuildToken(gid, name)
	if err != nil {
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	s := fmt.Sprintf("{\"status\":\"ok\", \"jwt\":\"%s\"}", signed)
	fmt.Fprint(res, s)
}

func meCommVerifyRoute(res http.ResponseWriter, req *http.Request) {
	gid, err := getAgentID(req)
	if err != nil {
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	name := util.Sanitize(vars["name"])

	if name == "" {
		err := fmt.Errorf("name unset")
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusNotAcceptable)
		return
	}

	ok, err := community.Validate(gid, name)
	if err != nil {
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	if !ok {
		err := fmt.Errorf("unable to verify")
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusNotAcceptable)
		return
	}

	go federation.SetCommunityID(context.Background(), gid, name)

	fmt.Fprint(res, jsonStatusOK)
}

func meCommClearRoute(res http.ResponseWriter, req *http.Request) {
	gid, err := getAgentID(req)
	if err != nil {
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	if err := gid.ClearCommunityName(); err != nil {
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	fmt.Fprint(res, jsonStatusOK)
}
