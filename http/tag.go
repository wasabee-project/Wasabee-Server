package PhDevHTTP

import (
	"fmt"
	//	"io/ioutil"
	"net/http"
	//	"strings"
	"encoding/json"
	"github.com/cloudkucooland/PhDevBin"
	"github.com/gorilla/mux"
)

func getTagRoute(res http.ResponseWriter, req *http.Request) {
	var tmp PhDevBin.TagData
	var tagList []PhDevBin.TagData
	tagList = append(tagList, tmp)

	id, err := GetUserID(req)
	if err != nil {
		PhDevBin.Log.Notice(err.Error())
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	if id == "" {
		// unauthorized would be better
		http.Redirect(res, req, "/login", http.StatusPermanentRedirect)
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

func deleteTagRoute(res http.ResponseWriter, req *http.Request) {
	id, err := GetUserID(req)
	if err != nil {
		PhDevBin.Log.Notice(err.Error())
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	if id == "" {
		// unauthorized would be better
		http.Redirect(res, req, "/login", http.StatusPermanentRedirect)
		return
	}

	vars := mux.Vars(req)
	tag := vars["tag"]
	safe, err := PhDevBin.UserInTag(id, tag)
	if safe != true {
		return
	}
	return
}

func addUserToTagRoute(res http.ResponseWriter, req *http.Request) {
	id, err := GetUserID(req)
	if err != nil {
		PhDevBin.Log.Notice(err.Error())
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	if id == "" {
		// unauthorized would be better
		http.Redirect(res, req, "/login", http.StatusPermanentRedirect)
		return
	}

	vars := mux.Vars(req)
	tag := vars["tag"]
	safe, err := PhDevBin.UserInTag(id, tag)
	if safe != true {
		return
	}
	return
}

func delUserFmTagRoute(res http.ResponseWriter, req *http.Request) {
	id, err := GetUserID(req)
	if err != nil {
		PhDevBin.Log.Notice(err.Error())
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	if id == "" {
		// unauthorized would be better
		http.Redirect(res, req, "/login", http.StatusPermanentRedirect)
		return
	}

	vars := mux.Vars(req)
	tag := vars["tag"]
	safe, err := PhDevBin.UserInTag(id, tag)
	if safe != true {
		return
	}
	return
}
