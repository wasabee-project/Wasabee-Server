package wasabihttps

import (
	"encoding/json"
	"fmt"
	"github.com/cloudkucooland/WASABI"
	"github.com/gorilla/mux"
	"html"
	"net/http"
	"strings"
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
	if strings.Contains(req.Referer(), "intel.ingress.com") {
		data, err := json.MarshalIndent(teamList, "", "\t")
		if err != nil {
			wasabi.Log.Error(err)
			http.Error(res, err.Error(), http.StatusInternalServerError)
			return
		}
		res.Header().Add("Content-Type", jsonType)
		fmt.Fprint(res, string(data))
	} else {
		if err = templateExecute(res, req, "team", teamList); err != nil {
			wasabi.Log.Notice(err)
			http.Error(res, err.Error(), http.StatusInternalServerError)
		}
	}
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

func chownTeamRoute(res http.ResponseWriter, req *http.Request) {
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

	to, ok := vars["to"]
	if !ok { // this should not happen unless the router gets misconfigured
		err = fmt.Errorf("to unset")
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	togid, err := wasabi.ToGid(to)
	if err != nil {
		wasabi.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	if err = team.Chown(togid); err != nil {
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

	if err = templateExecute(res, req, "teamedit", teamList); err != nil {
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
	key := vars["key"]

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
		togid, err := wasabi.ToGid(key)
		if err != nil {
			wasabi.Log.Notice(err)
			http.Error(res, err.Error(), http.StatusInternalServerError)
			return
		}
		if err = team.AddAgent(togid); err != nil {
			wasabi.Log.Notice(err)
			http.Error(res, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	url := fmt.Sprintf("%s/team/%s/edit", apipath, team.String())
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
	togid, err := wasabi.ToGid(vars["key"])
	if err != nil {
		wasabi.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
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
	if err = team.RemoveAgent(togid); err != nil {
		wasabi.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	url := fmt.Sprintf("%s/team/%s/edit", apipath, team.String())
	http.Redirect(res, req, url, http.StatusPermanentRedirect)
}

func announceTeamRoute(res http.ResponseWriter, req *http.Request) {
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

	message := req.FormValue("m")
	if message == "" {
		message = "This is a toast notification"
	}
	err = team.SendAnnounce(gid, message)
	if err != nil {
		wasabi.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	res.Header().Add("Content-Type", jsonType)
	fmt.Fprintf(res, `{ "status": "ok" }`)
}
