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

	vteam, err := strconv.ParseInt(req.FormValue("vteam"), 10, 64) // "0" to disable
	if err != nil {
		wasabee.Log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	r, err := strconv.ParseInt(req.FormValue("role"), 10, 8) // "0" for all roles
	if err != nil {
		wasabee.Log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	role := uint8(r)

	wasabee.Log.Infow("linking team to V", "GID", gid, "teamID", team, "vteam", vteam, "role", role)
	if err := team.VConfigure(vteam, role); err != nil {
		wasabee.Log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	fmt.Fprint(res, jsonStatusOK)
}

func vBulkImportRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Add("Content-Type", jsonType)
	gid, err := getAgentID(req)
	if err != nil {
		wasabee.Log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	mode := vars["mode"]

	var agent wasabee.AgentData
	err = gid.GetAgentData(&agent)
	if err != nil {
		wasabee.Log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	data, err := gid.VTeams()
	if err != nil {
		wasabee.Log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	var teamstomake []vTeamToMake
	if mode == "role" {
		teamstomake, err = vProcessRoleTeams(data, agent.Teams)
	} else {
		teamstomake, err = vProcessTeams(data, agent.Teams)
	}

	for _, t := range teamstomake {
		wasabee.Log.Debugw("Creating Wasabee team for V team", "v team", t.id, "role", 0)
		teamID, err := gid.NewTeam(t.name)
		if err != nil {
			wasabee.Log.Error(err)
			http.Error(res, jsonError(err), http.StatusInternalServerError)
			return
		}
		err = teamID.VConfigure(t.id, t.role)
		if err != nil {
			wasabee.Log.Error(err)
			http.Error(res, jsonError(err), http.StatusInternalServerError)
			return
		}
		key, err := gid.VAPIkey()
		if err != nil {
			wasabee.Log.Error(err)
			http.Error(res, jsonError(err), http.StatusInternalServerError)
			return
		}
		err = teamID.VSync(key)
		if err != nil {
			wasabee.Log.Error(err)
			http.Error(res, jsonError(err), http.StatusInternalServerError)
			return
		}
	}

	fmt.Fprint(res, jsonStatusOK)
}

type vTeamToMake struct {
	id   int64
	role uint8
	name string
}

func vProcessRoleTeams(v wasabee.AgentVTeams, teams []wasabee.AdTeam) ([]vTeamToMake, error) {
	var m []vTeamToMake
	// do the things

	return m, nil
}

func vProcessTeams(v wasabee.AgentVTeams, teams []wasabee.AdTeam) ([]vTeamToMake, error) {
	var m []vTeamToMake
	for _, t := range v.Teams {
		if !t.Admin {
			wasabee.Log.Debugw("not admin of v team, not creating w team", "v team", t.TeamID)
			continue
		}

		// don't make duplicates
		already := false
		for _, adt := range teams {
			if adt.VTeam == t.TeamID && adt.VTeamRole == 0 {
				wasabee.Log.Debugw("Wasabee team already exists for this V team", "v team", t.TeamID, "role", 0, "teamID", adt.ID)
				already = true
				break
			}
		}
		if already {
			continue
		}
		m = append(m, vTeamToMake{
			id:   t.TeamID,
			role: 0,
			name: fmt.Sprintf("%s (all)", t.Name),
		})
	}

	return m, nil
}
