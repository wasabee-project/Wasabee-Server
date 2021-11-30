package wasabeehttps

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"io/ioutil"
	"net/http"

	"github.com/wasabee-project/Wasabee-Server/Firebase"
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

	ok := messaging.CanSendTo(gid, togid)
	if !ok {
		err := fmt.Errorf("forbidden: only team owners can send to agents on the team")
		log.Warnw(err.Error(), "GID", gid, "resource", togid)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}
	ok, err = messaging.SendMessage(togid, message)
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

	ok, err := wfb.SendMessage(wfb.GoogleID(togid), string(toSend))
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

	var target struct {
		Name string
		ID   model.PortalID
		Lat  string
		Lng  string
		Type string
	}
	err = json.Unmarshal(jRaw, &target)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	if target.Name == "" {
		err := fmt.Errorf("portal not set")
		log.Warnw(err.Error(), "GID", gid)
		http.Error(res, jsonError(err), http.StatusNotAcceptable)
		return
	}

	if target.Lat == "" || target.Lng == "" {
		err := fmt.Errorf("lat/ng not set")
		log.Warnw(err.Error(), "GID", gid)
		http.Error(res, jsonError(err), http.StatusNotAcceptable)
		return
	}

	name, err := gid.IngressName()
	if err != nil {
		log.Error(err)
	}

	// Lng vs Lon ...
	templateData := struct {
		Name   string
		ID     model.PortalID
		Lat    string
		Lon    string
		Type   string
		Sender string
	}{
		Name:   target.Name,
		ID:     target.ID,
		Lat:    target.Lat,
		Lon:    target.Lng,
		Type:   target.Type,
		Sender: name,
	}

	msg, err := gid.ExecuteTemplate("target", templateData)
	if err != nil {
		log.Error(err)
		msg = fmt.Sprintf("template failed; target @ %s %s", target.Lat, target.Lng)
		// do not report send errors up the chain, just log
	}

	ok, err := messaging.SendMessage(togid, msg)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	if !ok {
		err := fmt.Errorf("message did not send")
		log.Infow(err.Error(), "from", gid, "to", togid, "msg", msg)
		// continue and send via firebase
	}

	out, err := json.Marshal(templateData)
	if err != nil {
		log.Warnw(err.Error(), "raw", templateData)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
	}
	wfb.SendTarget(wfb.GoogleID(togid), string(out))
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
