package wasabeehttps

import (
	"fmt"
	"net/http"

	"github.com/wasabee-project/Wasabee-Server/log"
	"github.com/wasabee-project/Wasabee-Server/model"
	"github.com/wasabee-project/Wasabee-Server/rocks"
)

func rocksPullTeamRoute(res http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	gid, err := getAgentID(req)
	if err != nil {
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	teamID := model.TeamID(req.PathValue("team"))

	safe, err := gid.OwnsTeam(ctx, teamID)
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

	if err := rocks.CommunityMemberPull(ctx, teamID); err != nil {
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	fmt.Fprint(res, jsonStatusOK)
}

func rocksCfgTeamRoute(res http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	gid, err := getAgentID(req)
	if err != nil {
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	team := model.TeamID(req.PathValue("team"))
	rc := req.PathValue("rockscomm")
	rk := req.PathValue("rockskey")

	safe, err := gid.OwnsTeam(ctx, team)
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

	if err = team.SetRocks(ctx, rk, rc); err != nil {
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	fmt.Fprint(res, jsonStatusOK)
}
