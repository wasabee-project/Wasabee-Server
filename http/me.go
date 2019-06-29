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
	if err = gid.GetAgentData(&ud); err != nil {
		wasabi.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	if strings.Contains(req.Referer(), "intel.ingress.com") || strings.Contains(req.Header.Get("User-Agent"), appUserAgent) {
		data, _ := json.MarshalIndent(ud, "", "\t")
		res.Header().Add("Content-Type", jsonType)
		fmt.Fprint(res, string(data))
		return
	}

	// templateExecute runs the "me" template and outputs directly to the res
	if err = templateExecute(res, req, "me", ud); err != nil {
		wasabi.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
	}
}

func meSettingsRoute(res http.ResponseWriter, req *http.Request) {
	gid, err := getAgentID(req)
	if err != nil {
		wasabi.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	var ud wasabi.AgentData
	if err = gid.GetAgentData(&ud); err != nil {
		wasabi.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	if err = templateExecute(res, req, "settings", ud); err != nil {
		wasabi.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
	}
}

func meOperationsRoute(res http.ResponseWriter, req *http.Request) {
	gid, err := getAgentID(req)
	if err != nil {
		wasabi.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	// XXX this gets too much -- but works for now; if it is slow, then use the deeper calls
	var ud wasabi.AgentData
	if err = gid.GetAgentData(&ud); err != nil {
		wasabi.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	if strings.Contains(req.Referer(), "intel.ingress.com") || strings.Contains(req.Header.Get("User-Agent"), appUserAgent) {
		data, _ := json.MarshalIndent(ud.Ops, "", "\t")
		res.Header().Add("Content-Type", jsonType)
		fmt.Fprint(res, string(data))
		return
	}

	if err = templateExecute(res, req, "operations", ud.Ops); err != nil {
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

	if err = gid.SetTeamState(team, state); err != nil {
		wasabi.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	if strings.Contains(req.Referer(), "intel.ingress.com") || strings.Contains(req.Header.Get("User-Agent"), appUserAgent) {
		res.Header().Add("Content-Type", jsonType)
		fmt.Fprintf(res, `{ "status": "OK"}`)
		return
	}
	http.Redirect(res, req, me, http.StatusPermanentRedirect)
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

	if err = team.RemoveAgent(gid); err != nil {
		wasabi.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	if strings.Contains(req.Referer(), "intel.ingress.com") || strings.Contains(req.Header.Get("User-Agent"), appUserAgent) {
		res.Header().Add("Content-Type", jsonType)
		fmt.Fprintf(res, `{ "status": "OK"}`)
		return
	}
	http.Redirect(res, req, me, http.StatusPermanentRedirect)
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
	if err = gid.SetIngressName(name); err != nil {
		wasabi.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	if strings.Contains(req.Referer(), "intel.ingress.com") || strings.Contains(req.Header.Get("User-Agent"), appUserAgent) {
		res.Header().Add("Content-Type", jsonType)
		fmt.Fprintf(res, `{ "status": "OK"}`)
		return
	}
	http.Redirect(res, req, me, http.StatusPermanentRedirect)
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
	if err = gid.SetOwnTracksPW(otpw); err != nil {
		wasabi.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	if strings.Contains(req.Referer(), "intel.ingress.com") || strings.Contains(req.Header.Get("User-Agent"), appUserAgent) {
		res.Header().Add("Content-Type", jsonType)
		fmt.Fprintf(res, `{ "status": "OK"}`)
		return
	}
	http.Redirect(res, req, me, http.StatusPermanentRedirect)
}

func meSetLocKeyRoute(res http.ResponseWriter, req *http.Request) {
	gid, err := getAgentID(req)
	if err != nil {
		wasabi.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	if err = gid.ResetLocKey(); err != nil {
		wasabi.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	if strings.Contains(req.Referer(), "intel.ingress.com") || strings.Contains(req.Header.Get("User-Agent"), appUserAgent) {
		res.Header().Add("Content-Type", jsonType)
		fmt.Fprintf(res, `{ "status": "OK"}`)
		return
	}
	http.Redirect(res, req, me, http.StatusPermanentRedirect)
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
	if err = gid.AgentLocation(lat, lon, "https"); err != nil {
		wasabi.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	if strings.Contains(req.Referer(), "intel.ingress.com") || strings.Contains(req.Header.Get("User-Agent"), appUserAgent) {
		res.Header().Add("Content-Type", jsonType)
		fmt.Fprintf(res, `{ "status": "OK"}`)
		return
	}
	http.Redirect(res, req, me, http.StatusPermanentRedirect)
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
	if err = gid.Delete(); err != nil {
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
		_ = gid.StatusLocationEnable()
	} else {
		_ = gid.StatusLocationDisable()
	}
	http.Redirect(res, req, me, http.StatusPermanentRedirect)
}

func meLogoutRoute(res http.ResponseWriter, req *http.Request) {
	gid, err := getAgentID(req)
	if err != nil {
		wasabi.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	gid.Logout("user requested logout")
	if strings.Contains(req.Referer(), "intel.ingress.com") || strings.Contains(req.Header.Get("User-Agent"), appUserAgent) {
		res.Header().Add("Content-Type", jsonType)
		fmt.Fprintf(res, `{ "status": "OK"}`)
		return
	}
	http.Redirect(res, req, "/", http.StatusPermanentRedirect)
}
