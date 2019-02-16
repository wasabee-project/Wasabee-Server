package PhDevHTTP

import (
	"fmt"
	"net/http"
	//	"strings"

	"github.com/cloudkucooland/PhDevBin"
	"github.com/gorilla/mux"
)

func meShowRoute(res http.ResponseWriter, req *http.Request) {
	id, err := GetUserID(req)
	if err != nil {
		PhDevBin.Log.Notice(err.Error())
		return
	}

	if id == "" {
		http.Redirect(res, req, "/login", http.StatusPermanentRedirect)
		return
	}

	var ud PhDevBin.UserData
	err = PhDevBin.GetUserData(id, &ud)
	if err != nil {
		PhDevBin.Log.Notice(err.Error())
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	PhDevBin.Log.Notice("/me: " + id)
	res.Header().Add("Content-Type", "text/plain")
	fmt.Fprint(res, "a screen full of data about me will be here.\n")
	fmt.Fprint(res, "user ID: " + id + "\n")
	fmt.Fprint(res, "google name: " + ud.GoogleName + " \n")
	fmt.Fprint(res, "ingress handle:  " + ud.IngressName + "\n")
	fmt.Fprint(res, "location share key:  " + ud.LocationKey + "\n")
	fmt.Fprint(res, "a list of all the tags I am in ... with options to remove/activate/deactivate\n")
}

func meToggleTagRoute(res http.ResponseWriter, req *http.Request) {
	id, err := GetUserID(req)
	if err != nil {
		PhDevBin.Log.Notice(err.Error())
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	if id == "" {
		http.Redirect(res, req, "/login", http.StatusPermanentRedirect)
	}

	vars := mux.Vars(req)
	tag := vars["tag"]
	state := vars["state"]

	// do the work
	PhDevBin.Log.Notice("toggle tag: " + id + " " + tag + " " + state)
    err = PhDevBin.SetUserTagState(id, tag, state)
	if err != nil {
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(res, req, "/me", http.StatusPermanentRedirect)
}

func meRemoveTagRoute(res http.ResponseWriter, req *http.Request) {
	id, err := GetUserID(req)
	if err != nil {
		PhDevBin.Log.Notice(err.Error())
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	if id == "" {
		http.Redirect(res, req, "/login", http.StatusPermanentRedirect)
	}

	vars := mux.Vars(req)
	tag := vars["tag"]

	// do the work
	PhDevBin.Log.Notice("remove me from tag: " + id + " " + tag)
    err = PhDevBin.RemoveUserFromTag(id, tag)
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
	}

	vars := mux.Vars(req)
	name := vars["name"]

	// do the work
	PhDevBin.Log.Notice("set ingress name: " + id + " " + name)
	err = PhDevBin.SetIngressName(id, name)
	if err != nil {
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(res, req, "/me", http.StatusPermanentRedirect)
}
