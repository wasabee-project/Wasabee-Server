package wasabihttps

import (
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"strings"

	"github.com/cloudkucooland/WASABI"
	"github.com/gorilla/mux"
)

func meShowRoute(res http.ResponseWriter, req *http.Request) {
	gid, err := getAgentID(req)
	if err != nil {
		wasabi.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	var ud wasabi.AgentData
	err = gid.GetAgentData(&ud)
	if err != nil {
		res.Header().Add("Cache-Control", "no-cache")
		wasabi.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	if strings.Contains(req.Referer(), "intel.ingress.com") {
		data, _ := json.MarshalIndent(ud, "", "\t")
		res.Header().Add("Content-Type", "text/json")
		fmt.Fprint(res, string(data))
		return
	}

	err = wasabiHTTPSTemplateExecute(res, req, "me", ud)
	if err != nil {
		wasabi.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
	}
}

func meToggleTeamRoute(res http.ResponseWriter, req *http.Request) {
	gid, err := getAgentID(req)
	if err != nil {
		wasabi.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	team := wasabi.TeamID(vars["team"])
	state := vars["state"]

	err = gid.SetTeamState(team, state)
	if err != nil {
		wasabi.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(res, req, "/me", http.StatusPermanentRedirect)
}

func meRemoveTeamRoute(res http.ResponseWriter, req *http.Request) {
	gid, err := getAgentID(req)
	if err != nil {
		wasabi.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	team := wasabi.TeamID(vars["team"])

	// wasabi.Log.Debug("remove me from team: " + gid.String() + " " + team.String())
	err = team.RemoveAgent(gid)
	if err != nil {
		wasabi.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(res, req, "/me", http.StatusPermanentRedirect)
}

func meSetIngressNameRoute(res http.ResponseWriter, req *http.Request) {
	gid, err := getAgentID(req)
	if err != nil {
		wasabi.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	name := html.EscapeString(vars["name"])

	// do the work
	err = gid.SetIngressName(name)
	if err != nil {
		wasabi.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(res, req, "/me", http.StatusPermanentRedirect)
}

func meSetOwnTracksPWRoute(res http.ResponseWriter, req *http.Request) {
	gid, err := getAgentID(req)
	if err != nil {
		wasabi.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	otpw := vars["otpw"]

	// do the work
	err = gid.SetOwnTracksPW(otpw)
	if err != nil {
		wasabi.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(res, req, "/me", http.StatusPermanentRedirect)
}

func meSetLocKeyRoute(res http.ResponseWriter, req *http.Request) {
	gid, err := getAgentID(req)
	if err != nil {
		wasabi.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	err = gid.ResetLocKey()
	if err != nil {
		wasabi.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(res, req, "/me", http.StatusPermanentRedirect)
}

func meSetAgentLocationRoute(res http.ResponseWriter, req *http.Request) {
	gid, err := getAgentID(req)
	if err != nil {
		wasabi.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	lat := vars["lat"]
	lon := vars["lon"]

	// do the work
	err = gid.AgentLocation(lat, lon, "https")
	if err != nil {
		wasabi.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(res, req, "/me", http.StatusPermanentRedirect)
}

func meDeleteRoute(res http.ResponseWriter, req *http.Request) {
	gid, err := getAgentID(req)
	if err != nil {
		wasabi.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	// do the work
	wasabi.Log.Noticef("Agent requested delete: %s", gid.String())
	err = gid.Delete()
	if err != nil {
		wasabi.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	// XXX delete the session cookie from the browser
	http.Redirect(res, req, "/", http.StatusPermanentRedirect)
}

func meStatusLocationRoute(res http.ResponseWriter, req *http.Request) {
	gid, err := getAgentID(req)
	if err != nil {
		wasabi.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	sl := vars["sl"]

	if sl == "On" {
		gid.StatusLocationEnable()
	} else {
		gid.StatusLocationDisable()
	}
	http.Redirect(res, req, "/me", http.StatusPermanentRedirect)
}
