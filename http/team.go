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
		wasabee.Log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	team := wasabee.TeamID(vars["team"])

	isowner, err := gid.OwnsTeam(team)
	if err != nil {
		wasabee.Log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	onteam, err := gid.AgentInTeam(team)
	if err != nil {
		wasabee.Log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	if !isowner && !onteam {
		err := fmt.Errorf("not on team")
		wasabee.Log.Infow(err.Error(), "teamID", team, "GID", gid.String(), "message", err.Error())
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}
	err = team.FetchTeam(&teamList)
	if err == sql.ErrNoRows {
		err = fmt.Errorf("team not found while fetching member list")
		wasabee.Log.Warnw(err.Error(), "teamID", team, "GID", gid.String())
		http.Error(res, jsonError(err), http.StatusNotFound)
		return
	}
	if err != nil {
		wasabee.Log.Error(err)
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
		wasabee.Log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	name := html.EscapeString(vars["name"])

	if name == "" {
		err := fmt.Errorf("empty team name")
		wasabee.Log.Warnw(err.Error(), "GID", gid)
		http.Error(res, jsonError(err), http.StatusNotAcceptable)
		return
	}

	_, err = gid.NewTeam(name)
	if err != nil {
		wasabee.Log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	fmt.Fprint(res, jsonStatusOK)
}

func deleteTeamRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Add("Content-Type", jsonType)
	gid, err := getAgentID(req)
	if err != nil {
		wasabee.Log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	team := wasabee.TeamID(vars["team"])
	safe, err := gid.OwnsTeam(team)
	if err != nil {
		wasabee.Log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	if !safe {
		err := fmt.Errorf("forbidden")
		wasabee.Log.Warnw(err.Error(), "resource", team, "GID", gid)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}
	if err = team.Delete(); err != nil {
		wasabee.Log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	fmt.Fprint(res, jsonStatusOK)
}

func chownTeamRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Add("Content-Type", jsonType)
	gid, err := getAgentID(req)
	if err != nil {
		wasabee.Log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	team := wasabee.TeamID(vars["team"])
	safe, err := gid.OwnsTeam(team)
	if err != nil {
		wasabee.Log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	if !safe {
		err := fmt.Errorf("forbidden")
		wasabee.Log.Warnw(err.Error(), "resource", team, "GID", gid)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}

	to, ok := vars["to"]
	if !ok { // this should not happen unless the router gets misconfigured
		err = fmt.Errorf("team new owner unset")
		wasabee.Log.Warnw(err.Error(), "resource", team, "GID", gid)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	togid, err := wasabee.ToGid(to)
	if err != nil {
		wasabee.Log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	if err = team.Chown(togid); err != nil {
		wasabee.Log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	fmt.Fprint(res, jsonStatusOK)
}

func addAgentToTeamRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Add("Content-Type", jsonType)
	gid, err := getAgentID(req)
	if err != nil {
		wasabee.Log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	team := wasabee.TeamID(vars["team"])
	key := vars["key"]

	safe, err := gid.OwnsTeam(team)
	if err != nil {
		wasabee.Log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	if !safe {
		err := fmt.Errorf("forbidden")
		wasabee.Log.Warnw(err.Error(), "resource", team, "GID", gid)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}

	if key != "" { // prevents a bit of log spam
		togid, err := wasabee.ToGid(key)
		if err != nil && err.Error() == "unknown agent" {
			// no need to fill the logs with user typos
			http.Error(res, jsonError(err), http.StatusNotAcceptable)
			return
		} else if err != nil {
			wasabee.Log.Error(err)
			http.Error(res, jsonError(err), http.StatusInternalServerError)
			return
		}
		if err = team.AddAgent(togid); err != nil {
			wasabee.Log.Error(err)
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
		wasabee.Log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	team := wasabee.TeamID(vars["team"])
	togid, err := wasabee.ToGid(vars["key"])
	if err != nil {
		wasabee.Log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	safe, err := gid.OwnsTeam(team)
	if err != nil {
		wasabee.Log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	if gid == togid {
		err := fmt.Errorf("cannot remove owner")
		wasabee.Log.Warnw(err.Error(), "resource", team, "GID", gid)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}
	if !safe {
		err := fmt.Errorf("forbidden")
		wasabee.Log.Warnw(err.Error(), "resource", team, "GID", gid)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}
	if err = team.RemoveAgent(togid); err != nil {
		wasabee.Log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	fmt.Fprint(res, jsonStatusOK)
}

func announceTeamRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Add("Content-Type", jsonType)
	gid, err := getAgentID(req)
	if err != nil {
		wasabee.Log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	team := wasabee.TeamID(vars["team"])
	safe, err := gid.OwnsTeam(team)
	if err != nil {
		wasabee.Log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	if !safe {
		err := fmt.Errorf("forbidden: only team owners can send announcements")
		wasabee.Log.Warnw(err.Error(), "resource", team, "GID", gid)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}

	message := req.FormValue("m")
	if message == "" {
		message = "This is a toast notification"
	}
	err = team.SendAnnounce(gid, message)
	if err != nil {
		wasabee.Log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	fmt.Fprint(res, jsonStatusOK)
}

func setAgentTeamSquadRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Set("Content-Type", jsonType)

	gid, err := getAgentID(req)
	if err != nil {
		wasabee.Log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	teamID := wasabee.TeamID(vars["team"])

	if owns, _ := gid.OwnsTeam(teamID); !owns {
		err = fmt.Errorf("forbidden: only the team owner can set squads")
		wasabee.Log.Warnw(err.Error(), "resource", teamID, "GID", gid)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}

	inGid := wasabee.GoogleID(vars["gid"])
	squad := req.FormValue("squad")
	err = teamID.SetSquad(inGid, squad)
	if err != nil {
		wasabee.Log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	fmt.Fprint(res, jsonStatusOK)
}

func setAgentTeamDisplaynameRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Set("Content-Type", jsonType)

	gid, err := getAgentID(req)
	if err != nil {
		wasabee.Log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	teamID := wasabee.TeamID(vars["team"])

	if owns, _ := gid.OwnsTeam(teamID); !owns {
		err = fmt.Errorf("forbidden: only the team owner can set display names")
		wasabee.Log.Warnw(err.Error(), "resource", teamID, "GID", gid)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}

	inGid := wasabee.GoogleID(vars["gid"])
	displayname := req.FormValue("displayname")
	err = teamID.SetDisplayname(inGid, displayname)
	if err != nil {
		wasabee.Log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	fmt.Fprint(res, jsonStatusOK)
}

func renameTeamRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Set("Content-Type", jsonType)

	gid, err := getAgentID(req)
	if err != nil {
		wasabee.Log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	teamID := wasabee.TeamID(vars["team"])

	if owns, _ := gid.OwnsTeam(teamID); !owns {
		err = fmt.Errorf("only the team owner can rename a team")
		wasabee.Log.Warnw(err.Error(), "resource", teamID, "GID", gid)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}

	teamname := req.FormValue("teamname")
	if teamname == "" {
		err = fmt.Errorf("empty team name")
		wasabee.Log.Warnw(err.Error(), "resource", teamID, "GID", gid)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	if err := teamID.Rename(teamname); err != nil {
		wasabee.Log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	fmt.Fprint(res, jsonStatusOK)
}

func genJoinKeyRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Set("Content-Type", jsonType)
	gid, err := getAgentID(req)
	if err != nil {
		wasabee.Log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	teamID := wasabee.TeamID(vars["team"])

	var key string
	if owns, _ := gid.OwnsTeam(teamID); owns {
		key, err = teamID.GenerateJoinToken()
		if err != nil {
			wasabee.Log.Error(err)
			http.Error(res, jsonError(err), http.StatusInternalServerError)
			return
		}
	} else {
		err = fmt.Errorf("forbidden: only the team owner can create join links")
		wasabee.Log.Warnw(err.Error(), "resource", teamID, "GID", gid)
		http.Error(res, jsonError(err), http.StatusForbidden)
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
		wasabee.Log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	teamID := wasabee.TeamID(vars["team"])

	if owns, _ := gid.OwnsTeam(teamID); owns {
		err := teamID.DeleteJoinToken()
		if err != nil {
			wasabee.Log.Error(err)
			http.Error(res, jsonError(err), http.StatusInternalServerError)
			return
		}
	} else {
		err = fmt.Errorf("forbidden: only the team owner can remove join links")
		wasabee.Log.Warnw(err.Error(), "resource", teamID, "GID", gid)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}
	fmt.Fprint(res, jsonStatusOK)
}

func joinLinkRoute(res http.ResponseWriter, req *http.Request) {
	// redirects to the app interface for the user to manage the team
	gid, err := getAgentID(req)
	if err != nil {
		wasabee.Log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	teamID := wasabee.TeamID(vars["team"])
	key := vars["key"]

	err = teamID.JoinToken(gid, key)
	if err != nil {
		wasabee.Log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	http.Redirect(res, req, me, http.StatusFound)
}
