package wasabihttps

import (
	"encoding/json"
	"fmt"
	"github.com/cloudkucooland/WASABI"
	"github.com/gorilla/mux"
	"net/http"
)

func getTeamRoute(res http.ResponseWriter, req *http.Request) {
	var teamList WASABI.TeamData

	gid, err := getAgentID(req)
	if err != nil {
		WASABI.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	team := WASABI.TeamID(vars["team"])

	safe, err := gid.AgentInTeam(team, false)
	if safe == false {
		http.Error(res, "Unauthorized", http.StatusUnauthorized)
		return
	}
	team.FetchTeam(&teamList, false)
	teamList.RocksComm = ""
	teamList.RocksKey = ""
	data, _ := json.MarshalIndent(teamList, "", "\t")
	s := string(data)
	res.Header().Add("Content-Type", "text/json")
	fmt.Fprintf(res, s)
	return
}

func newTeamRoute(res http.ResponseWriter, req *http.Request) {
	gid, err := getAgentID(req)
	if err != nil {
		WASABI.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	name := vars["name"]
	_, err = gid.NewTeam(name)
	if err != nil {
		WASABI.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(res, req, "/me", http.StatusPermanentRedirect)
}

func deleteTeamRoute(res http.ResponseWriter, req *http.Request) {
	gid, err := getAgentID(req)
	if err != nil {
		WASABI.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	team := WASABI.TeamID(vars["team"])
	safe, err := gid.OwnsTeam(team)
	if safe != true {
		http.Error(res, "Unauthorized", http.StatusUnauthorized)
		return
	}
	err = team.Delete()
	if err != nil {
		WASABI.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(res, req, "/me", http.StatusPermanentRedirect)
}

func editTeamRoute(res http.ResponseWriter, req *http.Request) {
	gid, err := getAgentID(req)
	if err != nil {
		WASABI.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	team := WASABI.TeamID(vars["team"])
	safe, err := gid.OwnsTeam(team)
	if safe != true {
		http.Error(res, "Unauthorized", http.StatusUnauthorized)
		return
	}
	var teamList WASABI.TeamData
	err = team.FetchTeam(&teamList, true)
	if err != nil {
		WASABI.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	err = wasabiHTTPSTemplateExecute(res, req, "edit", teamList)
	if err != nil {
		WASABI.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
	}
}

func addAgentToTeamRoute(res http.ResponseWriter, req *http.Request) {
	gid, err := getAgentID(req)
	if err != nil {
		WASABI.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	team := WASABI.TeamID(vars["team"])
	key := vars["key"] // Could be a lockkey, googleID, enlID or agent name, team.Addagent sorts it out for us

	safe, err := gid.OwnsTeam(team)
	if safe != true {
		http.Error(res, "Unauthorized", http.StatusUnauthorized)
		return
	}
	err = team.AddAgent(key)
	if err != nil {
		WASABI.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(res, req, "/"+config.apipath+"/team/"+team.String()+"/edit", http.StatusPermanentRedirect)
}

func delAgentFmTeamRoute(res http.ResponseWriter, req *http.Request) {
	gid, err := getAgentID(req)
	if err != nil {
		WASABI.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	team := WASABI.TeamID(vars["team"])
	key := WASABI.LocKey(vars["key"])
	safe, err := gid.OwnsTeam(team)
	if safe != true {
		http.Error(res, "Unauthorized", http.StatusUnauthorized)
		return
	}
	err = team.RemoveAgent(key)
	if err != nil {
		WASABI.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(res, req, "/"+config.apipath+"/team/"+team.String()+"/edit", http.StatusPermanentRedirect)
}
