package wasabeehttps

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"io/ioutil"
	"net/http"

	"github.com/wasabee-project/Wasabee-Server"
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

	ok := messaging.CanSendTo(w.GoogleID(gid), w.GoogleID(togid))
	if !ok {
		err := fmt.Errorf("forbidden: only team owners can send to agents on the team")
		log.Warnw(err.Error(), "GID", gid, "resource", togid)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}
	ok, err = messaging.SendMessage(w.GoogleID(togid), message)
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

func agentFBMessageRoute(res http.ResponseWriter, req *http.Request) {
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

	if !contentTypeIs(req, jsonTypeShort) {
		err := fmt.Errorf("invalid request (needs to be application/json)")
		log.Warnw(err.Error(), "GID", gid, "resource", togid)
		http.Error(res, jsonError(err), http.StatusNotAcceptable)
		return
	}

	jBlob, err := ioutil.ReadAll(req.Body)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	if string(jBlob) == "" {
		err := fmt.Errorf("empty JSON on firebase message")
		log.Warnw(err.Error(), "GID", gid, "resource", togid)
		http.Error(res, jsonStatusEmpty, http.StatusNotAcceptable)
		return
	}

	jRaw := json.RawMessage(jBlob)

	var msg struct {
		Sender  string
		Message string
		Date    string
	}

	if err = json.Unmarshal(jRaw, &msg); err != nil {
		log.Errorw(err.Error(), "GID", gid, "content", jRaw)
		return
	}

	if msg.Sender, err = gid.IngressName(); err != nil {
		log.Errorw("sender ingress name unknown", "GID", gid)
		return
	}

	toSend, err := json.Marshal(msg)
	if err != nil {
		log.Errorw(err.Error(), "GID", gid, "content", jRaw)
		return
	}

	_, err = messaging.SendMessage(w.GoogleID(togid), string(toSend))
	if err != nil {
		log.Error(err)
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

	err = messaging.SendTarget(w.GoogleID(togid), target)
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
