package wasabeehttps

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/wasabee-project/Wasabee-Server"
	"html"
	"net/http"
)

func getTeamRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Add("Content-Type", jsonType)
	var teamList wasabee.TeamData

	gid, err := getAgentID(req)
	if err != nil {
		wasabee.Log.Info(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	team := wasabee.TeamID(vars["team"])

	isowner, err := gid.OwnsTeam(team)
	if err != nil {
		wasabee.Log.Info(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	safe, err := gid.AgentInTeam(team, false)
	if err != nil {
		wasabee.Log.Info(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	if !safe {
		// XXX this should be a nice screen
		http.Error(res, "unauthorized: enable the team to access it", http.StatusUnauthorized)
		return
	}
	err = team.FetchTeam(&teamList, isowner) // send all to owner
	if err == sql.ErrNoRows {
		err = fmt.Errorf("team %s not found", team)
		wasabee.Log.Debug(err)
		http.Error(res, jsonError(err), http.StatusNotFound)
		return
	}
	if err != nil {
		wasabee.Log.Info(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	if !isowner {
		teamList.RocksComm = ""
		teamList.RocksKey = ""
		teamList.JoinLinkToken = ""
	}

	data, _ := json.Marshal(teamList)
	fmt.Fprint(res, string(data))
}

func newTeamRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Add("Content-Type", jsonType)
	gid, err := getAgentID(req)
	if err != nil {
		wasabee.Log.Info(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	name := html.EscapeString(vars["name"])

	_, err = gid.NewTeam(name)
	if err != nil {
		wasabee.Log.Info(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	fmt.Fprint(res, jsonStatusOK)
}

func deleteTeamRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Add("Content-Type", jsonType)
	gid, err := getAgentID(req)
	if err != nil {
		wasabee.Log.Info(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	team := wasabee.TeamID(vars["team"])
	safe, err := gid.OwnsTeam(team)
	if err != nil {
		wasabee.Log.Info(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	if !safe {
		http.Error(res, "Unauthorized", http.StatusUnauthorized)
		return
	}
	if err = team.Delete(); err != nil {
		wasabee.Log.Info(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	fmt.Fprint(res, jsonStatusOK)
}

func chownTeamRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Add("Content-Type", jsonType)
	gid, err := getAgentID(req)
	if err != nil {
		wasabee.Log.Info(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	team := wasabee.TeamID(vars["team"])
	safe, err := gid.OwnsTeam(team)
	if err != nil {
		wasabee.Log.Info(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	if !safe {
		http.Error(res, "Unauthorized", http.StatusUnauthorized)
		return
	}

	to, ok := vars["to"]
	if !ok { // this should not happen unless the router gets misconfigured
		err = fmt.Errorf("to unset")
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	togid, err := wasabee.ToGid(to)
	if err != nil {
		wasabee.Log.Info(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	if err = team.Chown(togid); err != nil {
		wasabee.Log.Info(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	fmt.Fprint(res, jsonStatusOK)
}

func addAgentToTeamRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Add("Content-Type", jsonType)
	gid, err := getAgentID(req)
	if err != nil {
		wasabee.Log.Info(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	team := wasabee.TeamID(vars["team"])
	key := vars["key"]

	safe, err := gid.OwnsTeam(team)
	if err != nil {
		wasabee.Log.Info(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	if !safe {
		http.Error(res, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if key != "" { // prevents a bit of log spam
		togid, err := wasabee.ToGid(key)
		if err != nil {
			wasabee.Log.Info(err)
			http.Error(res, jsonError(err), http.StatusInternalServerError)
			return
		}
		if err = team.AddAgent(togid); err != nil {
			wasabee.Log.Info(err)
			http.Error(res, jsonError(err), http.StatusInternalServerError)
			return
		}
	}
	fmt.Fprint(res, jsonStatusOK)
}

func delAgentFmTeamRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Add("Content-Type", jsonType)
	gid, err := getAgentID(req)
	if err != nil {
		wasabee.Log.Info(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	team := wasabee.TeamID(vars["team"])
	togid, err := wasabee.ToGid(vars["key"])
	if err != nil {
		wasabee.Log.Info(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	safe, err := gid.OwnsTeam(team)
	if err != nil {
		wasabee.Log.Info(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	if gid == togid {
		http.Error(res, "Cannot remove owner", http.StatusUnauthorized)
		return
	}
	if !safe {
		http.Error(res, "Unauthorized", http.StatusUnauthorized)
		return
	}
	if err = team.RemoveAgent(togid); err != nil {
		wasabee.Log.Info(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	fmt.Fprint(res, jsonStatusOK)
}

func announceTeamRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Add("Content-Type", jsonType)
	gid, err := getAgentID(req)
	if err != nil {
		wasabee.Log.Info(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	team := wasabee.TeamID(vars["team"])
	safe, err := gid.OwnsTeam(team)
	if err != nil {
		wasabee.Log.Info(err)
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
		wasabee.Log.Info(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	fmt.Fprint(res, jsonStatusOK)
}

func setAgentTeamSquadRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Set("Content-Type", jsonType)

	gid, err := getAgentID(req)
	if err != nil {
		wasabee.Log.Info(err)
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
			wasabee.Log.Info(err)
			http.Error(res, jsonError(err), http.StatusInternalServerError)
			return
		}
	} else {
		err = fmt.Errorf("only the team owner can set squads")
		wasabee.Log.Info(err)
		http.Error(res, jsonError(err), http.StatusUnauthorized)
		return
	}
	fmt.Fprint(res, jsonStatusOK)
}

func setAgentTeamDisplaynameRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Set("Content-Type", jsonType)

	gid, err := getAgentID(req)
	if err != nil {
		wasabee.Log.Info(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	teamID := wasabee.TeamID(vars["team"])

	if owns, _ := gid.OwnsTeam(teamID); owns {
		inGid := wasabee.GoogleID(vars["gid"])
		displayname := req.FormValue("displayname")
		err := teamID.SetDisplayname(inGid, displayname)
		if err != nil {
			wasabee.Log.Info(err)
			http.Error(res, jsonError(err), http.StatusInternalServerError)
			return
		}
	} else {
		err = fmt.Errorf("only the team owner can set display names")
		wasabee.Log.Info(err)
		http.Error(res, jsonError(err), http.StatusUnauthorized)
		return
	}
	fmt.Fprint(res, jsonStatusOK)
}

func renameTeamRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Set("Content-Type", jsonType)

	gid, err := getAgentID(req)
	if err != nil {
		wasabee.Log.Info(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	teamID := wasabee.TeamID(vars["team"])

	if owns, _ := gid.OwnsTeam(teamID); !owns {
		err = fmt.Errorf("only the team owner can rename a team")
		wasabee.Log.Info(err)
		http.Error(res, jsonError(err), http.StatusUnauthorized)
		return
	}

	teamname := req.FormValue("teamname")
	if teamname == "" {
		err = fmt.Errorf("empty team name")
		wasabee.Log.Info(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	if err := teamID.Rename(teamname); err != nil {
		wasabee.Log.Info(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	fmt.Fprint(res, jsonStatusOK)
}

func genJoinKeyRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Set("Content-Type", jsonType)
	gid, err := getAgentID(req)
	if err != nil {
		wasabee.Log.Info(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	teamID := wasabee.TeamID(vars["team"])

	var key string
	if owns, _ := gid.OwnsTeam(teamID); owns {
		key, err = teamID.GenerateJoinToken()
		if err != nil {
			wasabee.Log.Info(err)
			http.Error(res, jsonError(err), http.StatusInternalServerError)
			return
		}
	} else {
		err = fmt.Errorf("only the team owner can create join links")
		wasabee.Log.Info(err)
		http.Error(res, jsonError(err), http.StatusUnauthorized)
		return
	}

	type Out struct {
		Ok  string
		Key string
	}

	o := Out{
		Ok:  "OK",
		Key: key,
	}
	jo, _ := json.Marshal(o)

	fmt.Fprint(res, string(jo))
}

func delJoinKeyRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Set("Content-Type", jsonType)
	gid, err := getAgentID(req)
	if err != nil {
		wasabee.Log.Info(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	teamID := wasabee.TeamID(vars["team"])

	if owns, _ := gid.OwnsTeam(teamID); owns {
		err := teamID.DeleteJoinToken()
		if err != nil {
			wasabee.Log.Info(err)
			http.Error(res, jsonError(err), http.StatusInternalServerError)
			return
		}
	} else {
		err = fmt.Errorf("only the team owner can remove join links")
		wasabee.Log.Info(err)
		http.Error(res, jsonError(err), http.StatusUnauthorized)
		return
	}
	fmt.Fprint(res, jsonStatusOK)
}

func joinLinkRoute(res http.ResponseWriter, req *http.Request) {
	// redirects to the app interface for the user to manage the team
	gid, err := getAgentID(req)
	if err != nil {
		wasabee.Log.Info(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	teamID := wasabee.TeamID(vars["team"])
	key := vars["key"]

	err = teamID.JoinToken(gid, key)
	if err != nil {
		wasabee.Log.Info(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	http.Redirect(res, req, me, http.StatusFound)
}
