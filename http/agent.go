package wasabeehttps

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"io/ioutil"
	"net/http"

	"github.com/wasabee-project/Wasabee-Server/log"
	"github.com/wasabee-project/Wasabee-Server/messaging"
	"github.com/wasabee-project/Wasabee-Server/model"
)

func agentProfileRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Add("Content-Type", jsonType)
	res.Header().Add("Cache-Control", "no-store") // location changes frequently

	// must be authenticated
	gid, err := getAgentID(req)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	id := vars["id"]

	togid, err := model.ToGid(id)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	agent, err := model.FetchAgent(togid, gid)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	data, _ := json.Marshal(agent)
	fmt.Fprint(res, string(data))
}

func agentMessageRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Add("Content-Type", jsonType)
	gid, err := getAgentID(req)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	id := vars["id"]
	togid, err := model.ToGid(id)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	message := req.FormValue("m")
	if message == "" {
		message = "This is a toast notification"
	}

	ok, err := messaging.SendMessage(messaging.GoogleID(togid), message)
	if err != nil {
		log.Error(err)
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
	res.Header().Add("Content-Type", jsonType)
	gid, err := getAgentID(req)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
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
		log.Error(err.Error())
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	jBlob, err := ioutil.ReadAll(req.Body)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	if string(jBlob) == "" {
		log.Warnw("empty JSON", "GID", gid)
		http.Error(res, jsonStatusEmpty, http.StatusNotAcceptable)
		return
	}

	jRaw := json.RawMessage(jBlob)
	var target messaging.Target
	err = json.Unmarshal(jRaw, &target)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	target.Sender, err = gid.IngressName()
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	err = messaging.SendTarget(messaging.GoogleID(togid), target)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	fmt.Fprint(res, jsonStatusOK)
}

func agentPictureRoute(res http.ResponseWriter, req *http.Request) {
	_, err := getAgentID(req)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	id := vars["id"]
	togid, err := model.ToGid(id)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	url := togid.GetPicture()
	http.Redirect(res, req, url, http.StatusPermanentRedirect)
}
