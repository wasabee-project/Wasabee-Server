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
	id, err := GetUserID(req)
	if err != nil {
		res.Header().Add("Cache-Control", "no-cache")
		PhDevBin.Log.Notice(err.Error())
		return
	}
	if id == "" {
		res.Header().Add("Cache-Control", "no-cache")
		http.Redirect(res, req, "/login", http.StatusPermanentRedirect)
		return
	}

	var ud PhDevBin.UserData
	err = PhDevBin.GetUserData(id, &ud)
	if err != nil {
		res.Header().Add("Cache-Control", "no-cache")
		PhDevBin.Log.Notice(err.Error())
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	if strings.Contains(req.Referer(), "intel.ingress.com") {
		data, _ := json.MarshalIndent(ud, "", "\t")
		res.Header().Add("Content-Type", "text/json")
		fmt.Fprint(res, string(data))
		return
	}

	err = config.templateSet.ExecuteTemplate(res, "me", ud)
	if err != nil {
		PhDevBin.Log.Notice(err.Error())
		http.Error(res, err.Error(), http.StatusInternalServerError)
	}
}

func meToggleTeamRoute(res http.ResponseWriter, req *http.Request) {
	id, err := GetUserID(req)
	if err != nil {
		PhDevBin.Log.Notice(err.Error())
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	if id == "" {
		http.Redirect(res, req, "/login", http.StatusPermanentRedirect)
		return
	}

	vars := mux.Vars(req)
	team := vars["team"]
	state := vars["state"]

	err = PhDevBin.SetUserTeamState(id, team, state)
	if err != nil {
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(res, req, "/me", http.StatusPermanentRedirect)
}

func meRemoveTeamRoute(res http.ResponseWriter, req *http.Request) {
	id, err := GetUserID(req)
	if err != nil {
		PhDevBin.Log.Notice(err.Error())
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	if id == "" {
		http.Redirect(res, req, "/login", http.StatusPermanentRedirect)
		return
	}

	vars := mux.Vars(req)
	team := vars["team"]

	// do the work
	PhDevBin.Log.Notice("remove me from team: " + id + " " + team)
	err = PhDevBin.RemoveUserFromTeam(id, team)
	if err != nil {
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(res, req, "/me", http.StatusPermanentRedirect)
}

func meSetIngressNameRoute(res http.ResponseWriter, req *http.Request) {
	id, err := GetUserID(req)
	if err != nil {
		PhDevBin.Log.Notice(err.Error())
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	if id == "" {
		http.Redirect(res, req, "/login", http.StatusPermanentRedirect)
		return
	}

	vars := mux.Vars(req)
	name := vars["name"]

	// do the work
	err = PhDevBin.SetIngressName(id, name)
	if err != nil {
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(res, req, "/me", http.StatusPermanentRedirect)
}

func meSetOwnTracksPWRoute(res http.ResponseWriter, req *http.Request) {
	id, err := GetUserID(req)
	if err != nil {
		PhDevBin.Log.Notice(err.Error())
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	if id == "" {
		http.Redirect(res, req, "/login", http.StatusPermanentRedirect)
		return
	}

	vars := mux.Vars(req)
	otpw := vars["otpw"]

	// do the work
	err = PhDevBin.SetOwnTracksPW(id, otpw)
	if err != nil {
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(res, req, "/me", http.StatusPermanentRedirect)
}

func meSetUserLocationRoute(res http.ResponseWriter, req *http.Request) {
	id, err := GetUserID(req)
	if err != nil {
		PhDevBin.Log.Notice(err.Error())
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	if id == "" {
		http.Redirect(res, req, "/login", http.StatusPermanentRedirect)
		return
	}

	vars := mux.Vars(req)
	lat := vars["lat"]
	lon := vars["lon"]

	// do the work
	err = PhDevBin.UserLocation(id, lat, lon, "https")
	if err != nil {
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(res, req, "/me", http.StatusPermanentRedirect)
}

