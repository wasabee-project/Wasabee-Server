package wasabeehttps

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/wasabee-project/Wasabee-Server"
)

func vTeamRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Add("Content-Type", jsonType)
	// see what V sends us...

	fmt.Fprint(res, jsonStatusOK)
}

func vPullTeamRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Add("Content-Type", jsonType)
	gid, err := getAgentID(req)
	if err != nil {
		wasabee.Log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	team := wasabee.TeamID(vars["team"])

	owns, err := gid.OwnsTeam(team)
	if err != nil {
		wasabee.Log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	if !owns {
		err := fmt.Errorf("attempt to pull V for a team owned by someone else")
		wasabee.Log.Errorw(err.Error(), "GID", gid, "teamID", team)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}

	vkey, err := gid.VAPIkey()
	if err != nil {
		wasabee.Log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	if vkey == "" {
		err := fmt.Errorf("V API key not configured")
		wasabee.Log.Errorw(err.Error(), "GID", gid)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}

	if err := team.VSync(vkey); err != nil {
		wasabee.Log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	fmt.Fprint(res, jsonStatusOK)
}

func vConfigureTeamRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Add("Content-Type", jsonType)
	gid, err := getAgentID(req)
	if err != nil {
		wasabee.Log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	team := wasabee.TeamID(vars["team"])

	owns, err := gid.OwnsTeam(team)
	if err != nil {
		wasabee.Log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	if !owns {
		err := fmt.Errorf("attempt to configure V for a team owned by someone else")
		wasabee.Log.Errorw(err.Error(), "gid", gid, "teamID", team)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}

	vteam, err := strconv.ParseInt(vars["vteam"], 10, 64) // "0" to reset
	if err != nil {
		wasabee.Log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	role, err := strconv.ParseInt(vars["role"], 10, 64) // "0" for all roles
	if err != nil {
		wasabee.Log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	if err := team.VConfigure(vteam, role); err != nil {
		wasabee.Log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	fmt.Fprint(res, jsonStatusOK)
}
