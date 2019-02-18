package PhDevHTTP

import (
	"encoding/json"
	"fmt"
	"github.com/cloudkucooland/PhDevBin"
	"github.com/gorilla/mux"
	"net/http"
)

func getTagRoute(res http.ResponseWriter, req *http.Request) {
	var tagList []PhDevBin.TagData

	id, err := GetUserID(req)
	if err != nil {
		PhDevBin.Log.Notice(err.Error())
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	if id == "" {
		http.Error(res, "Unauthorized", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(req)
	tag := vars["tag"]

	safe, err := PhDevBin.UserInTag(id, tag)
	if safe {
		PhDevBin.FetchTag(tag, &tagList)
		data, _ := json.Marshal(tagList)
		s := string(data)
		res.Header().Add("Content-Type", "text/json")
		fmt.Fprintf(res, s)
		return
	}
	http.Error(res, "Unauthorized", http.StatusUnauthorized)
}

func newTagRoute(res http.ResponseWriter, req *http.Request) {
	id, err := GetUserID(req)
	if err != nil {
		PhDevBin.Log.Notice(err.Error())
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	if id == "" {
		http.Error(res, "Unauthorized", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(req)
	name := vars["name"]
	_, err = PhDevBin.NewTag(name, id)
	if err != nil {
		PhDevBin.Log.Notice(err.Error())
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(res, req, "/me", http.StatusPermanentRedirect)
}

func deleteTagRoute(res http.ResponseWriter, req *http.Request) {
	id, err := GetUserID(req)
	if err != nil {
		PhDevBin.Log.Notice(err.Error())
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	if id == "" {
		http.Error(res, "Unauthorized", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(req)
	tag := vars["tag"]
	safe, err := PhDevBin.UserOwnsTag(id, tag)
	if safe != true {
		http.Error(res, "Unauthorized", http.StatusUnauthorized)
		return
	}
	err = PhDevBin.DeleteTag(tag)
	if err != nil {
		PhDevBin.Log.Notice(err.Error())
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(res, req, "/me", http.StatusPermanentRedirect)
}

func addUserToTagRoute(res http.ResponseWriter, req *http.Request) {
	id, err := GetUserID(req)
	if err != nil {
		PhDevBin.Log.Notice(err.Error())
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	if id == "" {
		http.Error(res, "Unauthorized", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(req)
	tag := vars["tag"]
	key := vars["key"]

	safe, err := PhDevBin.UserOwnsTag(id, tag)
	if safe != true {
		http.Error(res, "Unauthorized", http.StatusUnauthorized)
		return
	}
	err = PhDevBin.AddUserToTag(tag, key)
	if err != nil {
		PhDevBin.Log.Notice(err.Error())
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(res, req, "/tag/"+tag, http.StatusPermanentRedirect)
}

func delUserFmTagRoute(res http.ResponseWriter, req *http.Request) {
	id, err := GetUserID(req)
	if err != nil {
		PhDevBin.Log.Notice(err.Error())
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	if id == "" {
		http.Error(res, "Unauthorized", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(req)
	tag := vars["tag"]
	safe, err := PhDevBin.UserOwnsTag(id, tag)
	if safe != true {
		http.Error(res, "Unauthorized", http.StatusUnauthorized)
		return
	}
	// PhDevBin.DelUserFromTag(tag, key)
}
