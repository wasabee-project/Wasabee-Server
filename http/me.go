package wasabeehttps

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"

	"github.com/lestrrat-go/jwx/jwa"
	// "github.com/lestrrat-go/jwx/jwk"
	"github.com/lestrrat-go/jwx/jws"
	"github.com/lestrrat-go/jwx/jwt"

	"github.com/wasabee-project/Wasabee-Server/Firebase"
	"github.com/wasabee-project/Wasabee-Server/auth"
	"github.com/wasabee-project/Wasabee-Server/config"
	"github.com/wasabee-project/Wasabee-Server/log"
	"github.com/wasabee-project/Wasabee-Server/model"
)

// get the logged in agent
func meRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Add("Content-Type", jsonType)
	res.Header().Set("Cache-Control", "no-store")

	gid, err := getAgentID(req)
	if err != nil {
		log.Error(err)
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
	fmt.Fprint(res, string(data))
}

// use this to verify that form data is sent from a client that requested it
func formValidationToken(req *http.Request) string {
	idx := strings.LastIndex(req.RemoteAddr, ":")
	if idx == -1 {
		idx = len(req.RemoteAddr)
	}
	ip := req.RemoteAddr[0:idx]
	toHash := fmt.Sprintf("%s %s %s", req.Header.Get("User-Agent"), ip, c.OauthConfig.ClientSecret)
	hasher := sha256.New()
	hasher.Write([]byte(toHash))
	return base64.URLEncoding.EncodeToString(hasher.Sum(nil))
}

// almost everything should return JSON now. The few things that do not redirect elsewhere.
/* func wantsJSON(req *http.Request) bool {
	// if specified, use what is requested
	sendjson := req.FormValue("json")
	if sendjson == "y" {
		return true
	}
	if sendjson == "n" {
		return false
	}

	if strings.Contains(req.Referer(), "intel.ingress.com") {
		return true
	}

	return false
} */

func meToggleTeamRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Add("Content-Type", jsonType)
	res.Header().Set("Cache-Control", "no-store")

	gid, err := getAgentID(req)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	team := model.TeamID(vars["team"])
	state := vars["state"]

	if err = gid.SetTeamState(team, state); err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	fmt.Fprint(res, jsonStatusOK)
}

func meToggleTeamWDShareRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Add("Content-Type", jsonType)

	gid, err := getAgentID(req)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	team := model.TeamID(vars["team"])
	state := vars["state"]

	if err = gid.SetWDShare(team, state); err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	fmt.Fprint(res, jsonStatusOK)
}

func meToggleTeamWDLoadRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Add("Content-Type", jsonType)

	gid, err := getAgentID(req)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	team := model.TeamID(vars["team"])
	state := vars["state"]

	if err = gid.SetWDLoad(team, state); err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	fmt.Fprint(res, jsonStatusOK)
}

func meRemoveTeamRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Add("Content-Type", jsonType)

	gid, err := getAgentID(req)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	team := model.TeamID(vars["team"])

	if err = team.RemoveAgent(gid); err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	fmt.Fprint(res, jsonStatusOK)
}

func meSetAgentLocationRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Add("Content-Type", jsonType)
	gid, err := getAgentID(req)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	lat := vars["lat"]
	lon := vars["lon"]

	// do the work
	if err = gid.SetLocation(lat, lon); err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	// announce to teams with which this agent is sharing location information
	for _, teamID := range gid.TeamListEnabled() {
		wfb.AgentLocation(teamID)
	}

	fmt.Fprint(res, jsonStatusOK)
}

func meDeleteRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Add("Content-Type", jsonType)
	gid, err := getAgentID(req)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	qt := req.FormValue("qt")
	qtTest := formValidationToken(req)
	if qt != qtTest {
		err := fmt.Errorf("invalid form validation token")
		log.Errorw(err.Error(), "got", qt, "wanted", qtTest)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	// do the work
	log.Errorw("agent requested delete", "GID", gid.String())
	if err = gid.Delete(); err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	// XXX delete the session cookie from the browser
	fmt.Fprint(res, jsonStatusOK)
}

func meLogoutRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Add("Content-Type", jsonType)
	gid, err := getAgentID(req)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	ses, err := c.store.Get(req, c.sessionName)
	delete(ses.Values, "nonce")
	delete(ses.Values, "id")
	delete(ses.Values, "loginReq")
	res.Header().Set("Connection", "close")

	if err != nil {
		log.Error(err)
		_ = ses.Save(req, res)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	ses.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   -1,
		SameSite: http.SameSiteNoneMode,
		Secure:   true,
	}
	_ = ses.Save(req, res)

	auth.Logout(gid, "user requested")
	res.Header().Add("Content-Type", jsonType)
	fmt.Fprint(res, jsonStatusOK)
}

func meFirebaseRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Add("Content-Type", jsonType)
	gid, err := getAgentID(req)
	if err != nil {
		log.Error(err)
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
	// XXX limit to 152 char? 1k?

	if token == "" {
		err := fmt.Errorf("token empty")
		log.Warn(err)
		http.Error(res, jsonError(err), http.StatusNotAcceptable)
		return
	}
	err = gid.StoreFirebaseToken(token)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	fmt.Fprint(res, jsonStatusOK)
}

func meIntelIDRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Add("Content-Type", jsonType)
	gid, err := getAgentID(req)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	req.ParseMultipartForm(1024)
	qt := req.FormValue("qt")
	name := req.FormValue("name")
	faction := req.FormValue("faction")

	qtTest := formValidationToken(req)
	if qt != qtTest {
		err := fmt.Errorf("invalid form validation token")
		log.Errorw(err.Error(), "got", qt, "wanted", qtTest)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	gid.SetIntelData(name, faction)
	fmt.Fprint(res, jsonStatusOK)
}

func meVAPIkeyRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Add("Content-Type", jsonType)
	gid, err := getAgentID(req)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	v := req.FormValue("v")

	log.Infow("agent submitted V API token", "GID", gid.String())
	if err = gid.SetVAPIkey(v); err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	fmt.Fprint(res, jsonStatusOK)
}

func meJwtRefreshRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Add("Content-Type", jsonType)
	gid, err := getAgentID(req)
	if err != nil {
		log.Error(err)
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

	key, ok := config.Get().JWSigningKeys.Get(0)
	if !ok {
		err := fmt.Errorf("encryption jwk not set")
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	token, err := jwt.ParseRequest(req, jwt.InferAlgorithmFromKey(true), jwt.UseDefaultKey(true), jwt.WithKeySet(config.Get().JWParsingKeys))
	if err != nil {
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

	// let consumers know where to get the keys if they want to verify
	hdrs := jws.NewHeaders()
	hdrs.Set("jku", "https://cdn2.wasabee.rocks/.well-known/jwks.json")

	signed, err := jwt.Sign(jwts, jwa.RS256, key, jwt.WithHeaders(hdrs))
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	s := fmt.Sprintf("{\"status\":\"ok\", \"jwk\":\"%s\"}", string(signed[:]))
	// log.Infow("jwt", "refreshed", s)
	fmt.Fprint(res, s)
}
