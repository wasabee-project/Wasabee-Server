package wasabeehttps

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	"github.com/wasabee-project/Wasabee-Server"
)

// this can probably be simplified now
func meShowRoute(res http.ResponseWriter, req *http.Request) {
	gid, err := getAgentID(req)
	if err != nil {
		wasabee.Log.Error(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	var ud wasabee.AgentData
	if err = gid.GetAgentData(&ud); err != nil {
		wasabee.Log.Error(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	ud.QueryToken = formValidationToken(req)

	if wantsJSON(req) {
		data, _ := json.Marshal(ud)
		res.Header().Add("Content-Type", jsonType)
		res.Header().Set("Cache-Control", "no-store")
		fmt.Fprint(res, string(data))
		return
	}

	// templateExecute runs the "me" template and outputs directly to the res
	if err = templateExecute(res, req, "me", ud); err != nil {
		wasabee.Log.Error(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
	}
}

// use this to verify that form data is sent from a client that requested it
func formValidationToken(req *http.Request) string {
	idx := strings.LastIndex(req.RemoteAddr, ":")
	if idx == -1 {
		idx = len(req.RemoteAddr)
	}
	ip := req.RemoteAddr[0:idx]
	toHash := fmt.Sprintf("%s %s %s", req.Header.Get("User-Agent"), ip, config.OauthConfig.ClientSecret)
	hasher := sha256.New()
	hasher.Write([]byte(toHash))
	return base64.URLEncoding.EncodeToString(hasher.Sum(nil))
}

// almost everything should return JSON now. The few things that do not redirect elsewhere.
func wantsJSON(req *http.Request) bool {
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
}

func meShowRouteJSON(res http.ResponseWriter, req *http.Request) {
	res.Header().Add("Content-Type", jsonType)
	gid, err := getAgentID(req)
	if err != nil {
		wasabee.Log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	var ud wasabee.AgentData
	if err = gid.GetAgentData(&ud); err != nil {
		wasabee.Log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	data, _ := json.Marshal(ud)
	res.Header().Set("Cache-Control", "no-store")
	fmt.Fprint(res, string(data))
}

func meToggleTeamRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Add("Content-Type", jsonType)
	gid, err := getAgentID(req)
	if err != nil {
		wasabee.Log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	team := wasabee.TeamID(vars["team"])
	state := vars["state"]

	if err = gid.SetTeamState(team, state); err != nil {
		wasabee.Log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	res.Header().Set("Cache-Control", "no-store")
	fmt.Fprint(res, jsonStatusOK)
}

func meToggleTeamWDShareRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Add("Content-Type", jsonType)
	gid, err := getAgentID(req)
	if err != nil {
		wasabee.Log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	team := wasabee.TeamID(vars["team"])
	state := vars["state"]

	if err = gid.SetWDShare(team, state); err != nil {
		wasabee.Log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	fmt.Fprint(res, jsonStatusOK)
}

func meToggleTeamWDLoadRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Add("Content-Type", jsonType)
	gid, err := getAgentID(req)
	if err != nil {
		wasabee.Log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	team := wasabee.TeamID(vars["team"])
	state := vars["state"]

	if err = gid.SetWDLoad(team, state); err != nil {
		wasabee.Log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	fmt.Fprint(res, jsonStatusOK)
}

func meRemoveTeamRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Add("Content-Type", jsonType)
	gid, err := getAgentID(req)
	if err != nil {
		wasabee.Log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	team := wasabee.TeamID(vars["team"])

	if err = team.RemoveAgent(gid); err != nil {
		wasabee.Log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	fmt.Fprint(res, jsonStatusOK)
}

func meSetAgentLocationRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Add("Content-Type", jsonType)
	gid, err := getAgentID(req)
	if err != nil {
		wasabee.Log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	lat := vars["lat"]
	lon := vars["lon"]

	// do the work
	if err = gid.AgentLocation(lat, lon); err != nil {
		wasabee.Log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	// send to the other servers
	gid.PSLocation(lat, lon)

	fmt.Fprint(res, jsonStatusOK)
}

func meDeleteRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Add("Content-Type", jsonType)
	gid, err := getAgentID(req)
	if err != nil {
		wasabee.Log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	qt := req.FormValue("qt")
	qtTest := formValidationToken(req)
	if qt != qtTest {
		err := fmt.Errorf("invalid form validation token")
		wasabee.Log.Errorw(err.Error(), "got", qt, "wanted", qtTest)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	// do the work
	wasabee.Log.Errorw("agent requested delete", "GID", gid.String())
	if err = gid.Delete(); err != nil {
		wasabee.Log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	// XXX delete the session cookie from the browser
	http.Redirect(res, req, "/", http.StatusPermanentRedirect)
}

func meStatusLocationRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Add("Content-Type", jsonType)
	gid, err := getAgentID(req)
	if err != nil {
		wasabee.Log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	sl := vars["sl"]

	if sl == "On" {
		_ = gid.StatusLocationEnable()
	} else {
		_ = gid.StatusLocationDisable()
	}
	fmt.Fprint(res, jsonStatusOK)
}

func meLogoutRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Add("Content-Type", jsonType)
	gid, err := getAgentID(req)
	if err != nil {
		wasabee.Log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	ses, err := config.store.Get(req, config.sessionName)
	delete(ses.Values, "nonce")
	delete(ses.Values, "id")
	delete(ses.Values, "loginReq")
	res.Header().Set("Connection", "close")

	if err != nil {
		wasabee.Log.Error(err)
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

	gid.Logout("user requested")
	res.Header().Add("Content-Type", jsonType)
	fmt.Fprint(res, jsonStatusOK)
}

func meFirebaseRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Add("Content-Type", jsonType)
	gid, err := getAgentID(req)
	if err != nil {
		wasabee.Log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	t, err := ioutil.ReadAll(req.Body)
	if err != nil {
		wasabee.Log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	token := string(t)
	// XXX limit to 152 char? 1k?

	if token == "" {
		err := fmt.Errorf("token empty")
		wasabee.Log.Warn(err)
		http.Error(res, jsonError(err), http.StatusNotAcceptable)
		return
	}
	err = gid.FirebaseInsertToken(token)
	if err != nil {
		wasabee.Log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	fmt.Fprint(res, jsonStatusOK)
}

func meFirebaseGenTokenRoute(res http.ResponseWriter, req *http.Request) {
	gid, err := getAgentID(req)
	if err != nil {
		wasabee.Log.Error(err)
		res.Header().Add("Content-Type", jsonType)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	token, err := gid.FirebaseCustomToken()
	if err != nil {
		wasabee.Log.Error(err)
		res.Header().Add("Content-Type", jsonType)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	res.Header().Add("Content-Type", "application/jwt")
	fmt.Fprint(res, token)
}

func meIntelIDRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Add("Content-Type", jsonType)
	gid, err := getAgentID(req)
	if err != nil {
		wasabee.Log.Error(err)
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
		wasabee.Log.Errorw(err.Error(), "got", qt, "wanted", qtTest)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	gid.SetIntelData(name, faction)
	gid.PSIntelData(name, faction)
	fmt.Fprint(res, jsonStatusOK)
}
