package WASABIhttps

import (
	"encoding/json"
	"fmt"
	"github.com/cloudkucooland/WASABI"
	"github.com/gorilla/mux"
	"net/http"
	"strings"
)

func getTeamRoute(res http.ResponseWriter, req *http.Request) {
	var teamList WASABI.TeamData

	gid, err := getUserID(req)
	if err != nil {
		WASABI.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	team := WASABI.TeamID(vars["team"])

	safe, err := gid.UserInTeam(team, false)
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
	gid, err := getUserID(req)
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
	gid, err := getUserID(req)
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
	gid, err := getUserID(req)
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

func addUserToTeamRoute(res http.ResponseWriter, req *http.Request) {
	gid, err := getUserID(req)
	if err != nil {
		WASABI.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	team := WASABI.TeamID(vars["team"])
	tmp := vars["key"] // Could be a lockkey, googleID, enlID or agent name
	var key interface{}
	switch len(tmp) { // length gives us a guess, presence of a - makes us certain
	case 40:
		if strings.IndexByte(tmp, '-') != -1 {
			key = WASABI.LocKey(tmp) // Looks like a GoogleID
		} else {
			key = WASABI.EnlID(tmp)
		}
	case 21:
		if strings.IndexByte(tmp, '-') != -1 {
			key = WASABI.LocKey(tmp)
		} else {
			key = WASABI.GoogleID(tmp) // Looks like a GoogleID
		}
	default:
		if strings.IndexByte(tmp, '-') != -1 {
			key = WASABI.LocKey(tmp)
		} else {
			key = string(tmp) // trigger a search by AgentID
		}
	}

	safe, err := gid.OwnsTeam(team)
	if safe != true {
		http.Error(res, "Unauthorized", http.StatusUnauthorized)
		return
	}
	err = team.AddUser(key)
	if err != nil {
		WASABI.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(res, req, "/"+config.apipath+"/team/"+team.String()+"/edit", http.StatusPermanentRedirect)
}

func delUserFmTeamRoute(res http.ResponseWriter, req *http.Request) {
	gid, err := getUserID(req)
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
	err = team.RemoveUser(key)
	if err != nil {
		WASABI.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(res, req, "/"+config.apipath+"/team/"+team.String()+"/edit", http.StatusPermanentRedirect)
}
