package wasabeehttps

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/wasabee-project/Wasabee-Server"
)

func rocksCommunityRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Set("Content-Type", jsonType)

	if !contentTypeIs(req, jsonTypeShort) {
		http.Error(res, "Invalid request (needs to be application/json)", http.StatusNotAcceptable)
		return
	}

	// defer req.Body.Close()
	jBlob, err := ioutil.ReadAll(req.Body)
	if err != nil {
		wasabee.Log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	if string(jBlob) == "" {
		err := fmt.Errorf("empty JSON on rocks community sync")
		wasabee.Log.Warnw(err.Error())
		http.Error(res, jsonStatusEmpty, http.StatusNotAcceptable)
		return
	}

	jRaw := json.RawMessage(jBlob)
	err = wasabee.RocksCommunitySync(jRaw)
	if err != nil {
		wasabee.Log.Errorw(err.Error(), "content", jRaw)
		http.Error(res, jsonError(err), http.StatusInternalServerError)

		// XXX get the team owner
		// XXX send a message to the team owner with relevant debug info

		return
	}

	fmt.Fprint(res, jsonStatusOK)
}

func rocksPullTeamRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Set("Content-Type", jsonType)
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
		err := fmt.Errorf("forbidden: only the team owner can pull the .rocks community")
		wasabee.Log.Warnw(err.Error(), "GID", gid.String(), "resource", team)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}

	err = team.RocksCommunityMemberPull()
	if err != nil {
		wasabee.Log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	fmt.Fprint(res, jsonStatusOK)
}

func rocksCfgTeamRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Set("Content-Type", jsonType)

	gid, err := getAgentID(req)
	if err != nil {
		wasabee.Log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	team := wasabee.TeamID(vars["team"])
	rc := vars["rockscomm"]
	rk := vars["rockskey"]

	safe, err := gid.OwnsTeam(team)
	if err != nil {
		wasabee.Log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	if !safe {
		err := fmt.Errorf("forbidden: only the team owner can configure the .rocks community")
		wasabee.Log.Warnw(err.Error(), "GID", gid.String(), "resource", team)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}
	err = team.SetRocks(rk, rc)
	if err != nil {
		wasabee.Log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	fmt.Fprint(res, jsonStatusOK)
}
