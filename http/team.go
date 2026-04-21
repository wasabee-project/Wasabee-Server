package wasabeehttps

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/wasabee-project/Wasabee-Server/log"
	"github.com/wasabee-project/Wasabee-Server/messaging"
	"github.com/wasabee-project/Wasabee-Server/model"
	"github.com/wasabee-project/Wasabee-Server/util"
)

func getTeamRoute(res http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	gid, err := getAgentID(req)
	if err != nil {
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	team := model.TeamID(req.PathValue("teamID"))

	if !team.Valid(ctx) {
		err := fmt.Errorf("team not found")
		http.Error(res, jsonError(err), http.StatusNotFound)
		return
	}

	isowner, err := gid.OwnsTeam(ctx, team)
	if err != nil {
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	onteam, err := gid.AgentInTeam(ctx, team)
	if err != nil {
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	if !isowner && !onteam {
		err := fmt.Errorf("not on team")
		log.Infow(err.Error(), "teamID", team, "gid", gid, "message", err.Error())
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}

	teamList, err := team.FetchTeam(ctx)
	if err != nil {
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	if !isowner {
		teamList.RocksComm = ""
		teamList.RocksKey = ""
		teamList.JoinLinkToken = ""
	}
	json.NewEncoder(res).Encode(&teamList)
}

func newTeamRoute(res http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	gid, err := getAgentID(req)
	if err != nil {
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	name := util.Sanitize(req.PathValue("name"))
	if name == "" {
		err := fmt.Errorf("empty team name")
		log.Warnw(err.Error(), "gid", gid)
		http.Error(res, jsonError(err), http.StatusNotAcceptable)
		return
	}

	_, err = gid.NewTeam(ctx, name)
	if err != nil {
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	fmt.Fprint(res, jsonStatusOK)
}

func deleteTeamRoute(res http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	gid, err := getAgentID(req)
	if err != nil {
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	team := model.TeamID(req.PathValue("team"))
	safe, err := gid.OwnsTeam(ctx, team)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	if !safe {
		err := fmt.Errorf("forbidden")
		log.Warnw(err.Error(), "resource", team, "gid", gid)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}
	if err = team.Delete(ctx); err != nil {
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	fmt.Fprint(res, jsonStatusOK)
}

func chownTeamRoute(res http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	gid, err := getAgentID(req)
	if err != nil {
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	team := model.TeamID(req.PathValue("team"))
	safe, err := gid.OwnsTeam(ctx, team)
	if err != nil {
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	if !safe {
		err := fmt.Errorf("forbidden")
		log.Warnw(err.Error(), "resource", team, "gid", gid)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}

	togid, err := model.ToGid(ctx, req.PathValue("to"))
	if err != nil {
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	if err = team.Chown(ctx, togid); err != nil {
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	fmt.Fprint(res, jsonStatusOK)
}

func addAgentToTeamRoute(res http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	gid, err := getAgentID(req)
	if err != nil {
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	team := model.TeamID(req.PathValue("team"))
	key := req.PathValue("key")

	safe, err := gid.OwnsTeam(ctx, team)
	if err != nil {
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	if !safe {
		err := fmt.Errorf("forbidden")
		log.Warnw(err.Error(), "resource", team, "gid", gid)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}

	if key != "" {
		togid, err := model.ToGid(ctx, key)
		if err != nil && err.Error() == model.ErrAgentNotFound {
			http.Error(res, jsonError(err), http.StatusNotAcceptable)
			return
		} else if err != nil {
			http.Error(res, jsonError(err), http.StatusInternalServerError)
			return
		}
		if err = team.AddAgent(ctx, togid); err != nil {
			http.Error(res, jsonError(err), http.StatusInternalServerError)
			return
		}
	}
	fmt.Fprint(res, jsonStatusOK)
}

func delAgentFmTeamRoute(res http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	gid, err := getAgentID(req)
	if err != nil {
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	team := model.TeamID(req.PathValue("team"))
	togid, err := model.ToGid(ctx, req.PathValue("key"))
	if err != nil {
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	safe, err := gid.OwnsTeam(ctx, team)
	if err != nil {
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	if gid == togid {
		err := fmt.Errorf("cannot remove owner")
		log.Warnw(err.Error(), "resource", team, "gid", gid)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}
	if !safe {
		err := fmt.Errorf("forbidden")
		log.Warnw(err.Error(), "resource", team, "gid", gid)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}
	if err = team.RemoveAgent(ctx, togid); err != nil {
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	fmt.Fprint(res, jsonStatusOK)
}

func announceTeamRoute(res http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	gid, err := getAgentID(req)
	if err != nil {
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	team := model.TeamID(req.PathValue("team"))
	safe, err := gid.OwnsTeam(ctx, team)
	if err != nil {
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	if !safe {
		err := fmt.Errorf("forbidden: only team owners can send announcements")
		log.Warnw(err.Error(), "resource", team, "gid", gid)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}

	message := util.Sanitize(req.FormValue("m"))
	if message == "" {
		message = "This is a toast notification"
	}

	messaging.SendAnnounce(ctx, messaging.TeamID(team), messaging.Announce{
		Text:   message,
		Sender: messaging.GoogleID(gid),
		TeamID: messaging.TeamID(team),
	})
	fmt.Fprint(res, jsonStatusOK)
}

func setAgentTeamCommentRoute(res http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	gid, err := getAgentID(req)
	if err != nil {
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	teamID := model.TeamID(req.PathValue("team"))

	if owns, _ := gid.OwnsTeam(ctx, teamID); !owns {
		err = fmt.Errorf("forbidden: only the team owner can set comments")
		log.Warnw(err.Error(), "resource", teamID, "gid", gid)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}

	inGid := model.GoogleID(req.PathValue("gid"))
	squad := util.Sanitize(req.FormValue("squad"))
	if err = teamID.SetComment(ctx, inGid, squad); err != nil {
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	fmt.Fprint(res, jsonStatusOK)
}

func renameTeamRoute(res http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	gid, err := getAgentID(req)
	if err != nil {
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	teamID := model.TeamID(req.PathValue("team"))

	if owns, _ := gid.OwnsTeam(ctx, teamID); !owns {
		err = fmt.Errorf("only the team owner can rename a team")
		log.Warnw(err.Error(), "resource", teamID, "gid", gid)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}

	teamname := util.Sanitize(req.FormValue("teamname"))
	if teamname == "" {
		err = fmt.Errorf("empty team name")
		log.Warnw(err.Error(), "resource", teamID, "gid", gid)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	if err := teamID.Rename(ctx, teamname); err != nil {
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	fmt.Fprint(res, jsonStatusOK)
}

func genJoinKeyRoute(res http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	gid, err := getAgentID(req)
	if err != nil {
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	teamID := model.TeamID(req.PathValue("team"))

	var key string
	if owns, _ := gid.OwnsTeam(ctx, teamID); owns {
		key, err = teamID.GenerateJoinToken(ctx)
		if err != nil {
			http.Error(res, jsonError(err), http.StatusInternalServerError)
			return
		}
	} else {
		err = fmt.Errorf("forbidden: only the team owner can create join links")
		log.Warnw(err.Error(), "resource", teamID, "gid", gid)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}

	json.NewEncoder(res).Encode(struct {
		Status string `json:"status"`
		Key    string `json:"key"`
	}{
		Status: "ok",
		Key:    key,
	})
}

func delJoinKeyRoute(res http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	gid, err := getAgentID(req)
	if err != nil {
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	teamID := model.TeamID(req.PathValue("team"))

	if owns, _ := gid.OwnsTeam(ctx, teamID); !owns {
		err = fmt.Errorf("forbidden: only the team owner can remove join links")
		log.Warnw(err.Error(), "resource", teamID, "gid", gid)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}

	if err := teamID.DeleteJoinToken(ctx); err != nil {
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	fmt.Fprint(res, jsonStatusOK)
}

func joinLinkRoute(res http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	gid, err := getAgentID(req)
	if err != nil {
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	teamID := model.TeamID(req.PathValue("team"))
	key := req.PathValue("key")

	if err = teamID.JoinToken(ctx, gid, key); err != nil {
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	fmt.Fprint(res, jsonStatusOK)
}

func getAgentsLocation(res http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	gid, err := getAgentID(req)
	if err != nil {
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	list, err := gid.GetAgentLocations(ctx)
	if err != nil {
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(res).Encode(list)
}

func bulkTeamFetchRoute(res http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	gid, err := getAgentID(req)
	if err != nil {
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	if !contentTypeIs(req, jsonTypeShort) {
		err := fmt.Errorf("invalid request (needs to be application/json)")
		log.Warnw(err.Error(), "gid", gid, "resource", "bulk team request")
		http.Error(res, jsonError(err), http.StatusNotAcceptable)
		return
	}

	var requestedteams struct {
		TeamIDs []model.TeamID `json:"teamids"`
	}
	if err := json.NewDecoder(req.Body).Decode(&requestedteams); err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	list := make([]model.TeamData, 0)
	for _, team := range requestedteams.TeamIDs {
		isowner, err := gid.OwnsTeam(ctx, team)
		if err != nil {
			log.Error(err)
			continue
		}

		onteam, err := gid.AgentInTeam(ctx, team)
		if err != nil {
			continue
		}
		if !isowner && !onteam {
			continue
		}
		t, err := team.FetchTeam(ctx)
		if err != nil {
			log.Errorw(err.Error(), "teamID", team, "gid", gid)
			continue
		}

		if !isowner {
			t.RocksComm = ""
			t.RocksKey = ""
			t.JoinLinkToken = ""
		}

		list = append(list, *t)
	}

	json.NewEncoder(res).Encode(list)
}
