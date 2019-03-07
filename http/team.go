package PhDevHTTP

import (
	"encoding/json"
	"fmt"
	"github.com/cloudkucooland/PhDevBin"
	"github.com/gorilla/mux"
	"net/http"
)

func getTeamRoute(res http.ResponseWriter, req *http.Request) {
	var teamList PhDevBin.TeamData

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
	team := vars["team"]

	safe, err := PhDevBin.UserInTeam(id, team, false)
	if safe {
		PhDevBin.FetchTeam(team, &teamList, false)
		data, _ := json.Marshal(teamList)
		s := string(data)
		res.Header().Add("Content-Type", "text/json")
		fmt.Fprintf(res, s)
		return
	}
	http.Error(res, "Unauthorized", http.StatusUnauthorized)
}

func newTeamRoute(res http.ResponseWriter, req *http.Request) {
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
	_, err = PhDevBin.NewTeam(name, id)
	if err != nil {
		PhDevBin.Log.Notice(err.Error())
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(res, req, "/me", http.StatusPermanentRedirect)
}

func deleteTeamRoute(res http.ResponseWriter, req *http.Request) {
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
	team := vars["team"]
	safe, err := PhDevBin.UserOwnsTeam(id, team)
	if safe != true {
		http.Error(res, "Unauthorized", http.StatusUnauthorized)
		return
	}
	err = PhDevBin.DeleteTeam(team)
	if err != nil {
		PhDevBin.Log.Notice(err.Error())
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(res, req, "/me", http.StatusPermanentRedirect)
}

func editTeamRoute(res http.ResponseWriter, req *http.Request) {
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
	team := vars["team"]
	safe, err := PhDevBin.UserOwnsTeam(id, team)
	if safe != true {
		http.Error(res, "Unauthorized", http.StatusUnauthorized)
		return
	}
	var teamList PhDevBin.TeamData
	err = PhDevBin.FetchTeam(team, &teamList, true)
	if err != nil {
		PhDevBin.Log.Notice(err.Error())
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	err = config.templateSet.ExecuteTemplate(res, "edit", teamList)
	if err != nil {
		PhDevBin.Log.Notice(err.Error())
		http.Error(res, err.Error(), http.StatusInternalServerError)
	}
}

func addUserToTeamRoute(res http.ResponseWriter, req *http.Request) {
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
	team := vars["team"]
	key := vars["key"]

	safe, err := PhDevBin.UserOwnsTeam(id, team)
	if safe != true {
		http.Error(res, "Unauthorized", http.StatusUnauthorized)
		return
	}
	err = PhDevBin.AddUserToTeam(team, key)
	if err != nil {
		PhDevBin.Log.Notice(err.Error())
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(res, req, "/team/"+team+"/edit", http.StatusPermanentRedirect)
}

func delUserFmTeamRoute(res http.ResponseWriter, req *http.Request) {
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
	team := vars["team"]
	key := vars["key"]
	safe, err := PhDevBin.UserOwnsTeam(id, team)
	if safe != true {
		http.Error(res, "Unauthorized", http.StatusUnauthorized)
		return
	}
	err = PhDevBin.DelUserFromTeam(team, key)
	if err != nil {
		PhDevBin.Log.Notice(err.Error())
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(res, req, "/team/"+team+"/edit", http.StatusPermanentRedirect)
}
