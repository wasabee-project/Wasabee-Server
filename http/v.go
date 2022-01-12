package wasabeehttps

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/wasabee-project/Wasabee-Server/log"
	"github.com/wasabee-project/Wasabee-Server/model"
	"github.com/wasabee-project/Wasabee-Server/v"
)

func vPullTeamRoute(res http.ResponseWriter, req *http.Request) {
	gid, err := getAgentID(req)
	if err != nil {
		http.Error(res, jsonError(err), http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(req)
	team := model.TeamID(vars["team"])

	owns, err := gid.OwnsTeam(team)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}
	if !owns {
		err := fmt.Errorf("attempt to pull V for a team owned by someone else")
		log.Errorw(err.Error(), "GID", gid, "teamID", team)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}

	vkey, err := gid.GetVAPIkey()
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusNotAcceptable)
		return
	}
	if vkey == "" {
		err := fmt.Errorf("V API key not configured")
		log.Errorw(err.Error(), "GID", gid)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}

	if err := v.Sync(req.Context(), team, vkey); err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	fmt.Fprint(res, jsonStatusOK)
}

func vConfigureTeamRoute(res http.ResponseWriter, req *http.Request) {
	gid, err := getAgentID(req)
	if err != nil {
		http.Error(res, jsonError(err), http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(req)
	team := model.TeamID(vars["team"])

	owns, err := gid.OwnsTeam(team)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}
	if !owns {
		err := fmt.Errorf("attempt to configure V for a team owned by someone else")
		log.Errorw(err.Error(), "gid", gid, "teamID", team)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}

	vteam, err := strconv.ParseInt(req.FormValue("vteam"), 10, 64) // "0" to disable
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusNotAcceptable)
		return
	}

	r, err := strconv.ParseInt(req.FormValue("role"), 10, 8) // "0" for all roles
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusNotAcceptable)
		return
	}
	role := uint8(r)

	log.Infow("linking team to V", "GID", gid, "teamID", team, "vteam", vteam, "role", role)
	if err := team.VConfigure(vteam, role); err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	fmt.Fprint(res, jsonStatusOK)
}

func vBulkImportRoute(res http.ResponseWriter, req *http.Request) {
	gid, err := getAgentID(req)
	if err != nil {
		http.Error(res, jsonError(err), http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(req)
	mode := vars["mode"]

	if err = v.BulkImport(gid, mode); err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	fmt.Fprint(res, jsonStatusOK)
}
