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

func agentProfileRoute(res http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	gid, err := getAgentID(req)
	if err != nil {
		http.Error(res, jsonError(err), http.StatusUnauthorized)
		return
	}

	id := req.PathValue("id")

	togid, err := model.ToGid(ctx, id)
	if err != nil {
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	agent, err := model.FetchAgent(ctx, model.AgentID(togid), gid)
	if err != nil {
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	res.Header().Add("Cache-Control", "no-store") // location changes frequently
	json.NewEncoder(res).Encode(&agent)
}

func agentMessageRoute(res http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	gid, err := getAgentID(req)
	if err != nil {
		http.Error(res, jsonError(err), http.StatusUnauthorized)
		return
	}

	id := req.PathValue("id")
	togid, err := model.ToGid(ctx, id)
	if err != nil {
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	message := util.Sanitize(req.FormValue("m"))
	if message == "" {
		message = "This is a toast notification"
	}

	ok, err := messaging.SendMessage(ctx, messaging.GoogleID(togid), message)
	if err != nil {
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	if !ok {
		err := fmt.Errorf("message did not send")
		log.Warnw(err.Error(), "from", gid, "to", togid, "contents", message)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	fmt.Fprint(res, jsonStatusOK)
}

func agentTargetRoute(res http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	gid, err := getAgentID(req)
	if err != nil {
		http.Error(res, jsonError(err), http.StatusUnauthorized)
		return
	}

	if !contentTypeIs(req, jsonTypeShort) {
		err := fmt.Errorf("must use content-type: %s", jsonTypeShort)
		log.Errorw(err.Error(), "GID", gid)
		http.Error(res, jsonError(err), http.StatusNotAcceptable)
		return
	}

	id := req.PathValue("id")
	togid, err := model.ToGid(ctx, id)
	if err != nil {
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	var target messaging.Target
	if err := json.NewDecoder(req.Body).Decode(&target); err != nil {
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	target.Sender, err = gid.IngressName(ctx)
	if err != nil {
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	err = messaging.SendTarget(ctx, messaging.GoogleID(togid), target)
	if err != nil {
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	fmt.Fprint(res, jsonStatusOK)
}

func agentPictureRoute(res http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	if _, err := getAgentID(req); err != nil {
		http.Error(res, jsonError(err), http.StatusUnauthorized)
		return
	}

	togid, err := model.ToGid(ctx, req.PathValue("id"))
	if err != nil {
		http.Error(res, jsonError(err), http.StatusNotFound)
		return
	}

	url := togid.GetPicture(ctx)
	http.Redirect(res, req, url, http.StatusPermanentRedirect)
}
