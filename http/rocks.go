package wasabeehttps

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/wasabee-project/Wasabee-Server/log"
	"github.com/wasabee-project/Wasabee-Server/model"
	"github.com/wasabee-project/Wasabee-Server/rocks"
)

func rocksCommunityRoute(res http.ResponseWriter, req *http.Request) {
	if !contentTypeIs(req, jsonTypeShort) {
		http.Error(res, "Invalid request (needs to be application/json)", http.StatusNotAcceptable)
		return
	}

	// defer req.Body.Close()
	jBlob, err := ioutil.ReadAll(req.Body)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	if string(jBlob) == "" {
		err := fmt.Errorf("empty JSON on rocks community sync")
		log.Warnw(err.Error())
		http.Error(res, jsonStatusEmpty, http.StatusNotAcceptable)
		return
	}

	jRaw := json.RawMessage(jBlob)
	err = rocks.CommunitySync(jRaw)
	if err != nil {
		log.Errorw(err.Error(), "content", jRaw)
		http.Error(res, jsonError(err), http.StatusInternalServerError)

		// XXX get the team owner
		// XXX send a message to the team owner with relevant debug info

		return
	}

	fmt.Fprint(res, jsonStatusOK)
}

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
		log.Error(err)
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
		log.Error(err)
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
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	if !safe {
		err := fmt.Errorf("forbidden: only the team owner can configure the .rocks community")
		log.Warnw(err.Error(), "GID", gid.String(), "resource", team)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}
	err = team.SetRocks(rk, rc)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	fmt.Fprint(res, jsonStatusOK)
}
