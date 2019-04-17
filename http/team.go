package wasabihttps

import (
	"encoding/json"
	"fmt"
	"github.com/cloudkucooland/WASABI"
	"github.com/gorilla/mux"
	"net/http"
)

func getTeamRoute(res http.ResponseWriter, req *http.Request) {
	var teamList wasabi.TeamData

	gid, err := getAgentID(req)
	if err != nil {
		wasabi.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	team := wasabi.TeamID(vars["team"])

	safe, err := gid.AgentInTeam(team, false)
	if err != nil {
		wasabi.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	if !safe {
		http.Error(res, "unauthorized", http.StatusUnauthorized)
		return
	}
	team.FetchTeam(&teamList, false)
	teamList.RocksComm = ""
	teamList.RocksKey = ""
	data, _ := json.MarshalIndent(teamList, "", "\t")
	s := string(data)
	res.Header().Add("Content-Type", "text/json")
	fmt.Fprint(res, s)
}

func newTeamRoute(res http.ResponseWriter, req *http.Request) {
	gid, err := getAgentID(req)
	if err != nil {
		wasabi.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	name := vars["name"]
	_, err = gid.NewTeam(name)
	if err != nil {
		wasabi.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(res, req, "/me", http.StatusPermanentRedirect)
}

func deleteTeamRoute(res http.ResponseWriter, req *http.Request) {
	gid, err := getAgentID(req)
	if err != nil {
		wasabi.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	team := wasabi.TeamID(vars["team"])
	safe, err := gid.OwnsTeam(team)
	if err != nil {
		wasabi.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	if !safe {
		http.Error(res, "Unauthorized", http.StatusUnauthorized)
		return
	}
	err = team.Delete()
	if err != nil {
		wasabi.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(res, req, "/me", http.StatusPermanentRedirect)
}

func editTeamRoute(res http.ResponseWriter, req *http.Request) {
	gid, err := getAgentID(req)
	if err != nil {
		wasabi.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	team := wasabi.TeamID(vars["team"])
	safe, err := gid.OwnsTeam(team)
	if err != nil {
		wasabi.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	if !safe {
		http.Error(res, "Unauthorized", http.StatusUnauthorized)
		return
	}
	var teamList wasabi.TeamData
	err = team.FetchTeam(&teamList, true)
	if err != nil {
		wasabi.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	err = wasabiHTTPSTemplateExecute(res, req, "edit", teamList)
	if err != nil {
		wasabi.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
	}
}

func addAgentToTeamRoute(res http.ResponseWriter, req *http.Request) {
	gid, err := getAgentID(req)
	if err != nil {
		wasabi.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	team := wasabi.TeamID(vars["team"])
	key := vars["key"] // Could be a lockkey, googleID, enlID or agent name, team.Addagent sorts it out for us

	safe, err := gid.OwnsTeam(team)
	if err != nil {
		wasabi.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	if !safe {
		http.Error(res, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if key != "" { // prevents a bit of log spam
		err = team.AddAgent(key)
		if err != nil {
			wasabi.Log.Notice(err)
			http.Error(res, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	http.Redirect(res, req, "/"+config.apipath+"/team/"+team.String()+"/edit", http.StatusPermanentRedirect)
}

func delAgentFmTeamRoute(res http.ResponseWriter, req *http.Request) {
	gid, err := getAgentID(req)
	if err != nil {
		wasabi.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	team := wasabi.TeamID(vars["team"])
	key := wasabi.LocKey(vars["key"])
	safe, err := gid.OwnsTeam(team)
	if err != nil {
		wasabi.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	if !safe {
		http.Error(res, "Unauthorized", http.StatusUnauthorized)
		return
	}
	err = team.RemoveAgent(key)
	if err != nil {
		wasabi.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(res, req, "/"+config.apipath+"/team/"+team.String()+"/edit", http.StatusPermanentRedirect)
}
