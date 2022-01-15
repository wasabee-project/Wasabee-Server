package wasabeehttps

import (
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/wasabee-project/Wasabee-Server/log"
	"github.com/wasabee-project/Wasabee-Server/model"
	"github.com/wasabee-project/Wasabee-Server/rocks"
)

func rocksPullTeamRoute(res http.ResponseWriter, req *http.Request) {
	gid, err := getAgentID(req)
	if err != nil {
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	teamID := model.TeamID(vars["team"])

	safe, err := gid.OwnsTeam(teamID)
	if err != nil {
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	if !safe {
		err := fmt.Errorf("forbidden: only the team owner can pull the .rocks community")
		log.Warnw(err.Error(), "GID", gid.String(), "resource", teamID)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}

	if err := rocks.CommunityMemberPull(teamID); err != nil {
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	fmt.Fprint(res, jsonStatusOK)
}

func rocksCfgTeamRoute(res http.ResponseWriter, req *http.Request) {
	gid, err := getAgentID(req)
	if err != nil {
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	team := model.TeamID(vars["team"])
	rc := vars["rockscomm"]
	rk := vars["rockskey"]

	safe, err := gid.OwnsTeam(team)
	if err != nil {
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	if !safe {
		err := fmt.Errorf("forbidden: only the team owner can configure the .rocks community")
		log.Warnw(err.Error(), "GID", gid.String(), "resource", team)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}

	if err = team.SetRocks(rk, rc); err != nil {
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	fmt.Fprint(res, jsonStatusOK)
}
