package wasabeehttps

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/wasabee-project/Wasabee-Server/log"
	"github.com/wasabee-project/Wasabee-Server/model"
	"html"
	"io/ioutil"
	"net/http"
	"strings"
)

func getTeamRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Add("Content-Type", jsonType)
	var teamList model.TeamData

	gid, err := getAgentID(req)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	team := model.TeamID(vars["team"])

	isowner, err := gid.OwnsTeam(team)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	onteam, err := gid.AgentInTeam(team)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	if !isowner && !onteam {
		err := fmt.Errorf("not on team")
		log.Infow(err.Error(), "teamID", team, "GID", gid.String(), "message", err.Error())
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}
	err = team.FetchTeam(&teamList)
	if err == sql.ErrNoRows {
		err = fmt.Errorf("team not found while fetching member list")
		log.Warnw(err.Error(), "teamID", team, "GID", gid.String())
		http.Error(res, jsonError(err), http.StatusNotFound)
		return
	}
	if err != nil {
		log.Error(err)
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
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	name := html.EscapeString(vars["name"])

	if name == "" {
		err := fmt.Errorf("empty team name")
		log.Warnw(err.Error(), "GID", gid)
		http.Error(res, jsonError(err), http.StatusNotAcceptable)
		return
	}

	_, err = gid.NewTeam(name)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	fmt.Fprint(res, jsonStatusOK)
}

func deleteTeamRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Add("Content-Type", jsonType)
	gid, err := getAgentID(req)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	team := model.TeamID(vars["team"])
	safe, err := gid.OwnsTeam(team)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	if !safe {
		err := fmt.Errorf("forbidden")
		log.Warnw(err.Error(), "resource", team, "GID", gid)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}
	if err = team.Delete(); err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	fmt.Fprint(res, jsonStatusOK)
}

func chownTeamRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Add("Content-Type", jsonType)
	gid, err := getAgentID(req)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	team := model.TeamID(vars["team"])
	safe, err := gid.OwnsTeam(team)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	if !safe {
		err := fmt.Errorf("forbidden")
		log.Warnw(err.Error(), "resource", team, "GID", gid)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}

	to, ok := vars["to"]
	if !ok { // this should not happen unless the router gets misconfigured
		err = fmt.Errorf("team new owner unset")
		log.Warnw(err.Error(), "resource", team, "GID", gid)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	togid, err := model.ToGid(to)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	if err = team.Chown(togid); err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	fmt.Fprint(res, jsonStatusOK)
}

func addAgentToTeamRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Add("Content-Type", jsonType)
	gid, err := getAgentID(req)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	team := model.TeamID(vars["team"])
	key := vars["key"]

	safe, err := gid.OwnsTeam(team)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	if !safe {
		err := fmt.Errorf("forbidden")
		log.Warnw(err.Error(), "resource", team, "GID", gid)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}

	if key != "" { // prevents a bit of log spam
		togid, err := model.ToGid(key)
		if err != nil && strings.Contains(err.Error(), "not registered with this wasabee server") {
			// no need to fill the logs with user typos
			http.Error(res, jsonError(err), http.StatusNotAcceptable)
			return
		} else if err != nil {
			log.Error(err)
			http.Error(res, jsonError(err), http.StatusInternalServerError)
			return
		}
		if err = team.AddAgent(togid); err != nil {
			log.Error(err)
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
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	team := model.TeamID(vars["team"])
	togid, err := model.ToGid(vars["key"])
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	safe, err := gid.OwnsTeam(team)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	if gid == togid {
		err := fmt.Errorf("cannot remove owner")
		log.Warnw(err.Error(), "resource", team, "GID", gid)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}
	if !safe {
		err := fmt.Errorf("forbidden")
		log.Warnw(err.Error(), "resource", team, "GID", gid)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}
	if err = team.RemoveAgent(togid); err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	fmt.Fprint(res, jsonStatusOK)
}

func announceTeamRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Add("Content-Type", jsonType)
	gid, err := getAgentID(req)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	team := model.TeamID(vars["team"])
	safe, err := gid.OwnsTeam(team)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	if !safe {
		err := fmt.Errorf("forbidden: only team owners can send announcements")
		log.Warnw(err.Error(), "resource", team, "GID", gid)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}

	message := req.FormValue("m")
	if message == "" {
		message = "This is a toast notification"
	}
	err = team.SendAnnounce(gid, message)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	fmt.Fprint(res, jsonStatusOK)
}

func setAgentTeamSquadRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Set("Content-Type", jsonType)

	gid, err := getAgentID(req)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	teamID := model.TeamID(vars["team"])

	if owns, _ := gid.OwnsTeam(teamID); !owns {
		err = fmt.Errorf("forbidden: only the team owner can set squads")
		log.Warnw(err.Error(), "resource", teamID, "GID", gid)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}

	inGid := model.GoogleID(vars["gid"])
	squad := req.FormValue("squad")
	err = teamID.SetSquad(inGid, squad)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	fmt.Fprint(res, jsonStatusOK)
}

func setAgentTeamDisplaynameRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Set("Content-Type", jsonType)

	gid, err := getAgentID(req)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	teamID := model.TeamID(vars["team"])

	if owns, _ := gid.OwnsTeam(teamID); !owns {
		err = fmt.Errorf("forbidden: only the team owner can set display names")
		log.Warnw(err.Error(), "resource", teamID, "GID", gid)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}

	inGid := model.GoogleID(vars["gid"])
	displayname := req.FormValue("displayname")
	err = teamID.SetDisplayname(inGid, displayname)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	fmt.Fprint(res, jsonStatusOK)
}

func renameTeamRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Set("Content-Type", jsonType)

	gid, err := getAgentID(req)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	teamID := model.TeamID(vars["team"])

	if owns, _ := gid.OwnsTeam(teamID); !owns {
		err = fmt.Errorf("only the team owner can rename a team")
		log.Warnw(err.Error(), "resource", teamID, "GID", gid)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}

	teamname := req.FormValue("teamname")
	if teamname == "" {
		err = fmt.Errorf("empty team name")
		log.Warnw(err.Error(), "resource", teamID, "GID", gid)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	if err := teamID.Rename(teamname); err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	fmt.Fprint(res, jsonStatusOK)
}

func genJoinKeyRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Set("Content-Type", jsonType)
	gid, err := getAgentID(req)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	teamID := model.TeamID(vars["team"])

	var key string
	if owns, _ := gid.OwnsTeam(teamID); owns {
		key, err = teamID.GenerateJoinToken()
		if err != nil {
			log.Error(err)
			http.Error(res, jsonError(err), http.StatusInternalServerError)
			return
		}
	} else {
		err = fmt.Errorf("forbidden: only the team owner can create join links")
		log.Warnw(err.Error(), "resource", teamID, "GID", gid)
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
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	teamID := model.TeamID(vars["team"])

	if owns, _ := gid.OwnsTeam(teamID); owns {
		err := teamID.DeleteJoinToken()
		if err != nil {
			log.Error(err)
			http.Error(res, jsonError(err), http.StatusInternalServerError)
			return
		}
	} else {
		err = fmt.Errorf("forbidden: only the team owner can remove join links")
		log.Warnw(err.Error(), "resource", teamID, "GID", gid)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}
	fmt.Fprint(res, jsonStatusOK)
}

func joinLinkRoute(res http.ResponseWriter, req *http.Request) {
	// redirects to the app interface for the user to manage the team
	gid, err := getAgentID(req)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	teamID := model.TeamID(vars["team"])
	key := vars["key"]

	if err = teamID.JoinToken(gid, key); err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	http.Redirect(res, req, me, http.StatusFound)
}

func getAgentsLocation(res http.ResponseWriter, req *http.Request) {
	res.Header().Set("Content-Type", jsonType)
	gid, err := getAgentID(req)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	list, err := gid.GetAgentLocations()
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	fmt.Fprint(res, list)
}

func bulkTeamFetchRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Set("Content-Type", jsonType)
	gid, err := getAgentID(req)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	if !contentTypeIs(req, jsonTypeShort) {
		err := fmt.Errorf("invalid request (needs to be application/json)")
		log.Warnw(err.Error(), "GID", gid, "resource", "bulk team request")
		http.Error(res, jsonError(err), http.StatusNotAcceptable)
		return
	}

	jBlob, err := ioutil.ReadAll(req.Body)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	if string(jBlob) == "" {
		err := fmt.Errorf("empty JSON on bulk team request")
		log.Warnw(err.Error(), "GID", gid, "resource", "new operation")
		http.Error(res, jsonStatusEmpty, http.StatusNotAcceptable)
		return
	}

	jRaw := json.RawMessage(jBlob)
	var requestedteams struct {
		TeamIDs []model.TeamID `json:"teamids"`
	}

	if err := json.Unmarshal(jRaw, &requestedteams); err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	var list []model.TeamData
	for _, team := range requestedteams.TeamIDs {
		var t model.TeamData
		isowner, err := gid.OwnsTeam(team)
		if err != nil {
			log.Error(err)
			continue
		}

		onteam, err := gid.AgentInTeam(team)
		if err != nil {
			log.Error(err)
			continue
		}
		if !isowner && !onteam {
			err := fmt.Errorf("not on team - in bulk pull; probably an op where agent can't see all teams")
			log.Debugw(err.Error(), "teamID", team, "GID", gid.String(), "message", err.Error())
			continue
		}
		err = team.FetchTeam(&t)
		if err == sql.ErrNoRows {
			err = fmt.Errorf("team not found while fetching member list - in bulk pull")
			log.Warnw(err.Error(), "teamID", team, "GID", gid.String())
			continue
		}
		if err != nil {
			log.Error(err)
			continue
		}

		if !isowner {
			t.RocksComm = ""
			t.RocksKey = ""
			t.JoinLinkToken = ""
		}

		list = append(list, t)
	}

	// no valid teams
	if list == nil || len(list) == 0 {
		fmt.Fprint(res, "[]")
		return
	}

	data, err := json.Marshal(list)
	if err != nil {
		log.Warn(err)
	}
	/* out := string(data) if out == "" { out = "[]" } */
	fmt.Fprint(res, string(data))
}
