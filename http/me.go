package PhDevHTTP

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/cloudkucooland/PhDevBin"
	"github.com/gorilla/mux"
)

func meShowRoute(res http.ResponseWriter, req *http.Request) {
	gid, err := getUserID(req)
	if err != nil {
		PhDevBin.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	var ud PhDevBin.UserData
	err = gid.GetUserData(&ud)
	if err != nil {
		res.Header().Add("Cache-Control", "no-cache")
		PhDevBin.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	if strings.Contains(req.Referer(), "intel.ingress.com") {
		data, _ := json.MarshalIndent(ud, "", "\t")
		res.Header().Add("Content-Type", "text/json")
		fmt.Fprint(res, string(data))
		return
	}

	err = phDevBinHTTPSTemplateExecute(res, req, "me", ud)
	if err != nil {
		PhDevBin.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
	}
}

func meToggleTeamRoute(res http.ResponseWriter, req *http.Request) {
	gid, err := getUserID(req)
	if err != nil {
		PhDevBin.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	team := PhDevBin.TeamID(vars["team"])
	state := vars["state"]

	err = gid.SetUserTeamState(team, state)
	if err != nil {
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(res, req, "/me", http.StatusPermanentRedirect)
}

func meRemoveTeamRoute(res http.ResponseWriter, req *http.Request) {
	gid, err := getUserID(req)
	if err != nil {
		PhDevBin.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	team := PhDevBin.TeamID(vars["team"])

	// do the work
	PhDevBin.Log.Notice("remove me from team: " + gid.String() + " " + team.String())
	err = gid.RemoveUserFromTeam(team)
	if err != nil {
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(res, req, "/me", http.StatusPermanentRedirect)
}

func meSetIngressNameRoute(res http.ResponseWriter, req *http.Request) {
	gid, err := getUserID(req)
	if err != nil {
		PhDevBin.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	name := vars["name"]

	// do the work
	err = gid.SetIngressName(name)
	if err != nil {
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(res, req, "/me", http.StatusPermanentRedirect)
}

func meSetOwnTracksPWRoute(res http.ResponseWriter, req *http.Request) {
	gid, err := getUserID(req)
	if err != nil {
		PhDevBin.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	otpw := vars["otpw"]

	// do the work
	err = gid.SetOwnTracksPW(otpw)
	if err != nil {
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(res, req, "/me", http.StatusPermanentRedirect)
}

func meSetUserLocationRoute(res http.ResponseWriter, req *http.Request) {
	gid, err := getUserID(req)
	if err != nil {
		PhDevBin.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	lat := vars["lat"]
	lon := vars["lon"]

	// do the work
	err = gid.UserLocation(lat, lon, "https")
	if err != nil {
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(res, req, "/me", http.StatusPermanentRedirect)
}
