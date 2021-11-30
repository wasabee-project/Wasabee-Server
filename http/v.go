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

func vTeamRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Add("Content-Type", jsonType)

	// the POST is empty, all we have is the teamID from the URL
	vars := mux.Vars(req)
	id := vars["teamID"]
	if id == "" {
		err := fmt.Errorf("V hook called with empty team ID")
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	log.Infow("V requested team sync", "server", req.RemoteAddr, "team", id)

	vteam, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	teams, err := model.GetTeamsByVID(vteam)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	keys := make(map[model.GoogleID]string)

	for _, teamID := range teams {
		gid, err := teamID.Owner()
		if err != nil {
			log.Error(err)
			continue
		}

		key, ok := keys[gid]
		if !ok {
			key, err = gid.VAPIkey()
			if err != nil {
				log.Error(err)
				continue
			}
			if key == "" {
				log.Errorw("no VAPI key for team owner, skipping sync", "GID", gid, "teamID", teamID, "vteam", vteam)
				continue
			}
			keys[gid] = key
		}

		err = v.Sync(teamID, key)
		if err != nil {
			log.Error(err)
			http.Error(res, jsonError(err), http.StatusInternalServerError)
			return
		}
	}

	fmt.Fprint(res, jsonStatusOK)
}

func vPullTeamRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Add("Content-Type", jsonType)
	gid, err := getAgentID(req)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	team := model.TeamID(vars["team"])

	owns, err := gid.OwnsTeam(team)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	if !owns {
		err := fmt.Errorf("attempt to pull V for a team owned by someone else")
		log.Errorw(err.Error(), "GID", gid, "teamID", team)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}

	vkey, err := gid.VAPIkey()
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	if vkey == "" {
		err := fmt.Errorf("V API key not configured")
		log.Errorw(err.Error(), "GID", gid)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}

	if err := v.Sync(team, vkey); err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	fmt.Fprint(res, jsonStatusOK)
}

func vConfigureTeamRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Add("Content-Type", jsonType)
	gid, err := getAgentID(req)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	team := model.TeamID(vars["team"])

	owns, err := gid.OwnsTeam(team)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
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
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	r, err := strconv.ParseInt(req.FormValue("role"), 10, 8) // "0" for all roles
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
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
	res.Header().Add("Content-Type", jsonType)
	gid, err := getAgentID(req)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	key, err := gid.VAPIkey()
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	mode := vars["mode"]

	agent, err := gid.GetAgentData()
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	data, err := v.Teams(gid, key)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	var teamstomake []vTeamToMake
	switch mode {
	case "role":
		teamstomake, err = vProcessRoleTeams(data, agent.Teams, key)
	case "team":
		teamstomake, err = vProcessTeams(data, agent.Teams)
	default:
		id, err := strconv.ParseInt(mode, 10, 64)
		if err != nil {
			log.Error(err)
			return
		}
		for _, t := range data.Teams {
			if t.TeamID == id {
				teamstomake, err = vProcessRoleSingleTeam(t, agent.Teams, key)
				break
			}
		}
	}

	for _, t := range teamstomake {
		log.Debugw("Creating Wasabee team for V team", "v team", t.id, "role", t.role)
		teamID, err := gid.NewTeam(t.name)
		if err != nil {
			log.Error(err)
			http.Error(res, jsonError(err), http.StatusInternalServerError)
			return
		}
		err = teamID.VConfigure(t.id, t.role)
		if err != nil {
			log.Error(err)
			http.Error(res, jsonError(err), http.StatusInternalServerError)
			return
		}
		err = v.Sync(teamID, key)
		if err != nil {
			log.Error(err)
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

func vProcessRoleTeams(data v.AgentVTeams, teams []model.AdTeam, key string) ([]vTeamToMake, error) {
	var m []vTeamToMake

	raw := make(map[int64]map[uint8]bool)

	// for every team of which I am an admin
	for _, t := range v.Teams {
		if !t.Admin {
			// log.Debugw("not admin of v team, not creating w team", "v team", t.TeamID)
			continue
		}
		roles := make(map[uint8]bool)

		// load all agents
		vt, err := model.VGetTeam(t.TeamID, key)
		if err != nil {
			return m, err
		}

		// for every role of every agent
		for _, a := range vt.Agents {
			for _, r := range a.Roles {
				// don't make duplicates
				already := false
				for _, adt := range teams {
					if adt.VTeam == t.TeamID && adt.VTeamRole == r.ID {
						// log.Debugw("Wasabee team already exists for this V team/role", "v team", t.TeamID, "role", r.ID, "teamID", adt.ID)
						already = true
						break
					}
				}
				if already {
					continue
				}

				_, ok := roles[r.ID]
				if !ok { // first time we've seen this team/role
					m = append(m, vTeamToMake{
						id:   t.TeamID,
						role: r.ID,
						name: fmt.Sprintf("%s (%s)", t.Name, r.Name),
					})
					roles[r.ID] = true
				}
			}
		}
		raw[t.TeamID] = roles
	}

	return m, nil
}

func vProcessRoleSingleTeam(t v.AgentVTeam, teams []model.AdTeam, key string) ([]vTeamToMake, error) {
	var m []vTeamToMake

	if !t.Admin {
		// log.Debugw("not admin of v team, not creating w team", "v team", t.TeamID)
		return m, nil
	}
	roles := make(map[uint8]bool)

	vt, err := model.VGetTeam(t.TeamID, key)
	if err != nil {
		return m, err
	}

	// for every role of every agent -- this logic order is better, update the vProcessRoleTeams to use it...
	for _, a := range vt.Agents {
		for _, r := range a.Roles {
			_, ok := roles[r.ID]
			if !ok { // first time we've seen this team/role
				roles[r.ID] = true

				already := false
				for _, adt := range teams {
					if adt.VTeam == t.TeamID && adt.VTeamRole == r.ID {
						// log.Debugw("Wasabee team already exists for this V team/role", "v team", t.TeamID, "role", r.ID, "teamID", adt.ID)
						already = true
						break
					}
				}
				if already {
					continue
				}

				m = append(m, vTeamToMake{
					id:   t.TeamID,
					role: r.ID,
					name: fmt.Sprintf("%s (%s)", t.Name, r.Name),
				})
			}
		}
	}

	return m, nil
}

func vProcessTeams(vt v.AgentVTeams, teams []model.AdTeam) ([]vTeamToMake, error) {
	var m []vTeamToMake
	for _, t := range v.Teams {
		if !t.Admin {
			// log.Debugw("not admin of v team, not creating w team", "v team", t.TeamID)
			continue
		}

		// don't make duplicates
		already := false
		for _, adt := range teams {
			if adt.VTeam == t.TeamID && adt.VTeamRole == 0 {
				// log.Debugw("Wasabee team already exists for this V team", "v team", t.TeamID, "role", 0, "teamID", adt.ID)
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
