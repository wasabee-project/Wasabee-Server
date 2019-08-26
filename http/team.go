package wasabeehttps

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/wasabee-project/Wasabee-Server"
	"html"
	"net/http"
	"strings"
)

func getTeamRoute(res http.ResponseWriter, req *http.Request) {
	var teamList wasabee.TeamData

	gid, err := getAgentID(req)
	if err != nil {
		wasabee.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	team := wasabee.TeamID(vars["team"])

	safe, err := gid.AgentInTeam(team, false)
	if err != nil {
		wasabee.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	if !safe {
		http.Error(res, "unauthorized: enable the team to access it", http.StatusUnauthorized)
		return
	}
	err = team.FetchTeam(&teamList, false)
	if err != nil {
		wasabee.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	teamList.RocksComm = ""
	teamList.RocksKey = ""
	if strings.Contains(req.Referer(), "intel.ingress.com") || strings.Contains(req.Header.Get("User-Agent"), appUserAgent) {
		res.Header().Add("Content-Type", jsonType)
		data, err := json.MarshalIndent(teamList, "", "\t")
		if err != nil {
			wasabee.Log.Error(err)
			http.Error(res, jsonError(err), http.StatusInternalServerError)
			return
		}
		fmt.Fprint(res, string(data))
		return
	}

	if err = templateExecute(res, req, "team", teamList); err != nil {
		wasabee.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
	}
}

func newTeamRoute(res http.ResponseWriter, req *http.Request) {
	gid, err := getAgentID(req)
	if err != nil {
		wasabee.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	name := html.EscapeString(vars["name"])

	_, err = gid.NewTeam(name)
	if err != nil {
		wasabee.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(res, req, me, http.StatusPermanentRedirect)
}

func deleteTeamRoute(res http.ResponseWriter, req *http.Request) {
	gid, err := getAgentID(req)
	if err != nil {
		wasabee.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	team := wasabee.TeamID(vars["team"])
	safe, err := gid.OwnsTeam(team)
	if err != nil {
		wasabee.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	if !safe {
		http.Error(res, "Unauthorized", http.StatusUnauthorized)
		return
	}
	if err = team.Delete(); err != nil {
		wasabee.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(res, req, me, http.StatusPermanentRedirect)
}

func chownTeamRoute(res http.ResponseWriter, req *http.Request) {
	gid, err := getAgentID(req)
	if err != nil {
		wasabee.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	team := wasabee.TeamID(vars["team"])
	safe, err := gid.OwnsTeam(team)
	if err != nil {
		wasabee.Log.Notice(err)
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
	togid, err := wasabee.ToGid(to)
	if err != nil {
		wasabee.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	if err = team.Chown(togid); err != nil {
		wasabee.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(res, req, me, http.StatusPermanentRedirect)
}

func editTeamRoute(res http.ResponseWriter, req *http.Request) {
	gid, err := getAgentID(req)
	if err != nil {
		wasabee.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	team := wasabee.TeamID(vars["team"])
	safe, err := gid.OwnsTeam(team)
	if err != nil {
		wasabee.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	if !safe {
		http.Error(res, "Unauthorized", http.StatusUnauthorized)
		return
	}
	var teamList wasabee.TeamData
	if err = team.FetchTeam(&teamList, true); err != nil {
		wasabee.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	if err = templateExecute(res, req, "teamedit", teamList); err != nil {
		wasabee.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
	}
}

func addAgentToTeamRoute(res http.ResponseWriter, req *http.Request) {
	gid, err := getAgentID(req)
	if err != nil {
		wasabee.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	team := wasabee.TeamID(vars["team"])
	key := vars["key"]

	safe, err := gid.OwnsTeam(team)
	if err != nil {
		wasabee.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	if !safe {
		http.Error(res, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if key != "" { // prevents a bit of log spam
		togid, err := wasabee.ToGid(key)
		if err != nil {
			wasabee.Log.Notice(err)
			http.Error(res, err.Error(), http.StatusInternalServerError)
			return
		}
		if err = team.AddAgent(togid); err != nil {
			wasabee.Log.Notice(err)
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
		wasabee.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	team := wasabee.TeamID(vars["team"])
	togid, err := wasabee.ToGid(vars["key"])
	if err != nil {
		wasabee.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	safe, err := gid.OwnsTeam(team)
	if err != nil {
		wasabee.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	if !safe {
		http.Error(res, "Unauthorized", http.StatusUnauthorized)
		return
	}
	if err = team.RemoveAgent(togid); err != nil {
		wasabee.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	url := fmt.Sprintf("%s/team/%s/edit", apipath, team.String())
	http.Redirect(res, req, url, http.StatusPermanentRedirect)
}

func announceTeamRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Add("Content-Type", jsonType)
	gid, err := getAgentID(req)
	if err != nil {
		wasabee.Log.Notice(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	team := wasabee.TeamID(vars["team"])
	safe, err := gid.OwnsTeam(team)
	if err != nil {
		wasabee.Log.Notice(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	if !safe {
		err := fmt.Errorf("Unauthorized")
		http.Error(res, jsonError(err), http.StatusUnauthorized)
		return
	}

	message := req.FormValue("m")
	if message == "" {
		message = "This is a toast notification"
	}
	err = team.SendAnnounce(gid, message)
	if err != nil {
		wasabee.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	fmt.Fprintf(res, `{ "status": "ok" }`)
}

func setAgentTeamSquadRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Set("Content-Type", jsonType)

	gid, err := getAgentID(req)
	if err != nil {
		wasabee.Log.Notice(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	teamID := wasabee.TeamID(vars["team"])

	if owns, _ := gid.OwnsTeam(teamID); owns {
		inGid := wasabee.GoogleID(vars["gid"])
		squad := req.FormValue("squad")
		err := teamID.SetSquad(inGid, squad)
		if err != nil {
			wasabee.Log.Notice(err)
			http.Error(res, jsonError(err), http.StatusInternalServerError)
			return
		}
	} else {
		err = fmt.Errorf("only the team owner can set squads")
		wasabee.Log.Notice(err)
		http.Error(res, jsonError(err), http.StatusUnauthorized)
		return
	}
	fmt.Fprintf(res, `{ "status": "ok" }`)
}

func setAgentTeamDisplaynameRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Set("Content-Type", jsonType)

	gid, err := getAgentID(req)
	if err != nil {
		wasabee.Log.Notice(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	teamID := wasabee.TeamID(vars["team"])

	if owns, _ := gid.OwnsTeam(teamID); owns {
		inGid := wasabee.GoogleID(vars["gid"])
		displayname := req.FormValue("displayname")
		err := teamID.SetDisplaname(inGid, displayname)
		if err != nil {
			wasabee.Log.Notice(err)
			http.Error(res, jsonError(err), http.StatusInternalServerError)
			return
		}
	} else {
		err = fmt.Errorf("only the team owner can set display names")
		wasabee.Log.Notice(err)
		http.Error(res, jsonError(err), http.StatusUnauthorized)
		return
	}
	fmt.Fprintf(res, `{ "status": "ok" }`)
}
