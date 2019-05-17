package wasabihttps

import (
	"encoding/json"
	"fmt"
	"github.com/cloudkucooland/WASABI"
	"github.com/gorilla/mux"
	"html"
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
	err = team.FetchTeam(&teamList, false)
	if err != nil {
		wasabi.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	teamList.RocksComm = ""
	teamList.RocksKey = ""
	data, err := json.MarshalIndent(teamList, "", "\t")
	if err != nil {
		wasabi.Log.Error(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	res.Header().Add("Content-Type", jsonType)
	fmt.Fprint(res, string(data))
}

func newTeamRoute(res http.ResponseWriter, req *http.Request) {
	gid, err := getAgentID(req)
	if err != nil {
		wasabi.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	name := html.EscapeString(vars["name"])

	_, err = gid.NewTeam(name)
	if err != nil {
		wasabi.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(res, req, me, http.StatusPermanentRedirect)
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
	if err = team.Delete(); err != nil {
		wasabi.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(res, req, me, http.StatusPermanentRedirect)
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
	if err = team.FetchTeam(&teamList, true); err != nil {
		wasabi.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	if err = wasabiHTTPSTemplateExecute(res, req, "teamedit", teamList); err != nil {
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
		if err = team.AddAgent(key); err != nil {
			wasabi.Log.Notice(err)
			http.Error(res, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	url := fmt.Sprintf("/%s/team/%s/edit", config.apipath, team.String())
	http.Redirect(res, req, url, http.StatusPermanentRedirect)
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
	if err = team.RemoveAgent(key); err != nil {
		wasabi.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	url := fmt.Sprintf("/%s/team/%s/edit", config.apipath, team.String())
	http.Redirect(res, req, url, http.StatusPermanentRedirect)
}
