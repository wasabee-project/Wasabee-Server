package wasabeehttps

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"net/http"

	"github.com/wasabee-project/Wasabee-Server/log"
	"github.com/wasabee-project/Wasabee-Server/messaging"
	"github.com/wasabee-project/Wasabee-Server/model"
	"github.com/wasabee-project/Wasabee-Server/util"
)

func agentProfileRoute(res http.ResponseWriter, req *http.Request) {
	gid, err := getAgentID(req)
	if err != nil {
		http.Error(res, jsonError(err), http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(req)
	id := vars["id"]

	togid, err := model.ToGid(id)
	if err != nil {
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	agent, err := model.FetchAgent(togid, gid)
	if err != nil {
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	res.Header().Add("Cache-Control", "no-store") // location changes frequently
	json.NewEncoder(res).Encode(&agent)
}

func agentMessageRoute(res http.ResponseWriter, req *http.Request) {
	gid, err := getAgentID(req)
	if err != nil {
		http.Error(res, jsonError(err), http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(req)
	id := vars["id"]
	togid, err := model.ToGid(id)
	if err != nil {
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	message := util.Sanitize(req.FormValue("m"))
	if message == "" {
		message = "This is a toast notification"
	}

	ok, err := messaging.SendMessage(messaging.GoogleID(togid), message)
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

	vars := mux.Vars(req)
	id := vars["id"]
	togid, err := model.ToGid(id)
	if err != nil {
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	var target messaging.Target
	if err := json.NewDecoder(req.Body).Decode(&target); err != nil {
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	target.Sender, err = gid.IngressName()
	if err != nil {
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	err = messaging.SendTarget(messaging.GoogleID(togid), target)
	if err != nil {
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	fmt.Fprint(res, jsonStatusOK)
}

func agentPictureRoute(res http.ResponseWriter, req *http.Request) {
	if _, err := getAgentID(req); err != nil {
		http.Error(res, jsonError(err), http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(req)
	togid, err := model.ToGid(vars["id"])
	if err != nil {
		http.Error(res, jsonError(err), http.StatusNotFound)
		return
	}

	url := togid.GetPicture()
	http.Redirect(res, req, url, http.StatusPermanentRedirect)
}
