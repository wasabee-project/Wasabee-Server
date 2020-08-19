package wasabeehttps

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/wasabee-project/Wasabee-Server"
	"io/ioutil"
	"net/http"
	"strings"
)

func agentProfileRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Add("Content-Type", jsonType)
	var agent wasabee.Agent

	// must be authenticated
	gid, err := getAgentID(req)
	if err != nil {
		wasabee.Log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	id := vars["id"]

	togid, err := wasabee.ToGid(id)
	if err != nil {
		wasabee.Log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	err = wasabee.FetchAgent(togid, &agent)
	if err != nil {
		wasabee.Log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	agent.CanSendTo = gid.CanSendTo(togid)

	data, _ := json.Marshal(agent)
	fmt.Fprint(res, string(data))
}

func agentMessageRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Add("Content-Type", jsonType)
	gid, err := getAgentID(req)
	if err != nil {
		wasabee.Log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	id := vars["id"]
	togid, err := wasabee.ToGid(id)
	if err != nil {
		wasabee.Log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	message := req.FormValue("m")
	if message == "" {
		message = "This is a toast notification"
	}

	ok := gid.CanSendTo(togid)
	if !ok {
		err := fmt.Errorf("forbidden: only team owners can send to agents on the team")
		wasabee.Log.Warnw(err.Error(), "GID", gid, "resource", togid)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}
	ok, err = togid.SendMessage(message)
	if err != nil {
		wasabee.Log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	if !ok {
		err := fmt.Errorf("message did not send")
		wasabee.Log.Warnw(err.Error(), "from", gid, "to", togid, "contents", message)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	fmt.Fprint(res, jsonStatusOK)
}

func agentFBMessageRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Add("Content-Type", jsonType)
	gid, err := getAgentID(req)
	if err != nil {
		wasabee.Log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	id := vars["id"]
	togid, err := wasabee.ToGid(id)
	if err != nil {
		wasabee.Log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	if !contentTypeIs(req, jsonTypeShort) {
		err := fmt.Errorf("invalid request (needs to be application/json)")
		wasabee.Log.Warnw(err.Error(), "GID", gid, "resource", togid)
		http.Error(res, jsonError(err), http.StatusNotAcceptable)
		return
	}

	jBlob, err := ioutil.ReadAll(req.Body)
	if err != nil {
		wasabee.Log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	if string(jBlob) == "" {
		err := fmt.Errorf("empty JSON on firebase message")
		wasabee.Log.Warnw(err.Error(), "GID", gid, "resource", togid)
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
		wasabee.Log.Errorw(err.Error(), "GID", gid, "content", jRaw)
		return
	}

	if msg.Sender, err = gid.IngressName(); err != nil {
		wasabee.Log.Errorw("sender ingress name unknown", "GID", gid)
		return
	}

	toSend, err := json.Marshal(msg)
	if err != nil {
		wasabee.Log.Errorw(err.Error(), "GID", gid, "content", jRaw)
		return
	}

	// XXX for now anyone can send to anyone
	togid.FirebaseGenericMessage(string(toSend))
	if err != nil {
		wasabee.Log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	fmt.Fprint(res, jsonStatusOK)
}

func agentTargetRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Add("Content-Type", jsonType)
	gid, err := getAgentID(req)
	if err != nil {
		wasabee.Log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	if contentTypeIs(req, "multipart/form-data") {
		wasabee.Log.Infow("using old format for sending targets", "GID", gid)
		agentTargetRouteOld(res, req)
		return
	}

	if !contentTypeIs(req, jsonTypeShort) {
		err := fmt.Errorf("must use content-type: %s", jsonTypeShort)
		wasabee.Log.Errorw(err.Error(), "GID", gid)
		http.Error(res, jsonError(err), http.StatusNotAcceptable)
		return
	}

	vars := mux.Vars(req)
	id := vars["id"]
	togid, err := wasabee.ToGid(id)
	if err != nil {
		wasabee.Log.Error(err.Error())
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	jBlob, err := ioutil.ReadAll(req.Body)
	if err != nil {
		wasabee.Log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	if string(jBlob) == "" {
		wasabee.Log.Warnw("empty JSON", "GID", gid)
		http.Error(res, jsonStatusEmpty, http.StatusNotAcceptable)
		return
	}

	jRaw := json.RawMessage(jBlob)

	var target struct {
		Name string
		ID   wasabee.PortalID
		Lat  string
		Lng  string
	}
	err = json.Unmarshal(jRaw, &target)
	if err != nil {
		wasabee.Log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	if target.Name == "" {
		err := fmt.Errorf("portal not set")
		wasabee.Log.Warnw(err.Error(), "GID", gid)
		http.Error(res, jsonError(err), http.StatusNotAcceptable)
		return
	}

	if target.Lat == "" || target.Lng == "" {
		err := fmt.Errorf("lat/ng not set")
		wasabee.Log.Warnw(err.Error(), "GID", gid)
		http.Error(res, jsonError(err), http.StatusNotAcceptable)
		return
	}

	iname, err := gid.IngressName()
	if err != nil {
		wasabee.Log.Error(err)
	}

	templateData := struct {
		Name   string
		ID     wasabee.PortalID
		Lat    string
		Lon    string
		Type   string
		Sender string
	}{
		Name:   target.Name,
		ID:     target.ID,
		Lat:    target.Lat,
		Lon:    target.Lng,
		Type:   "ad-hoc target",
		Sender: iname,
	}

	msg, err := gid.ExecuteTemplate("target", templateData)
	if err != nil {
		wasabee.Log.Error(err)
		msg = fmt.Sprintf("template failed; ad-hoc target @ %s %s", target.Lat, target.Lng)
		// do not report send errors up the chain, just log
	}

	// XXX can send to anyone -- make "shared enabled team"
	ok, err := togid.SendMessage(msg)
	if err != nil {
		wasabee.Log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	if !ok {
		err := fmt.Errorf("message did not send")
		wasabee.Log.Warnw(err.Error(), "from", gid, "to", togid)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	out, err := json.Marshal(templateData)
	if err != nil {
		wasabee.Log.Warnw(err.Error(), "raw", templateData)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
	}
	togid.FirebaseTarget(string(out))
	fmt.Fprint(res, jsonStatusOK)
}

func agentTargetRouteOld(res http.ResponseWriter, req *http.Request) {
	res.Header().Add("Content-Type", jsonType)
	gid, err := getAgentID(req)
	if err != nil {
		wasabee.Log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	id := vars["id"]
	togid, err := wasabee.ToGid(id)
	if err != nil {
		wasabee.Log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	portal := req.FormValue("portal")
	if portal == "" {
		err := fmt.Errorf("portal net set")
		wasabee.Log.Warnw(err.Error(), "GID", gid)
		http.Error(res, jsonError(err), http.StatusNotAcceptable)
		return
	}

	ll := req.FormValue("ll")
	if ll == "" {
		err := fmt.Errorf("ll not set")
		wasabee.Log.Warnw(err.Error(), "GID", gid)
		http.Error(res, jsonError(err), http.StatusNotAcceptable)
		return
	}

	lls := strings.Split(ll, ",")
	lls = lls[:2] // make it be exactly 2 long

	iname, err := gid.IngressName()
	if err != nil {
		wasabee.Log.Error(err)
	}

	templateData := struct {
		Name   string
		Lat    string
		Lon    string
		Type   string
		Sender string
	}{
		Name:   portal,
		Lat:    lls[0],
		Lon:    lls[1],
		Type:   "ad-hoc target",
		Sender: iname,
	}

	msg, err := gid.ExecuteTemplate("target", templateData)
	if err != nil {
		wasabee.Log.Error(err)
		msg = fmt.Sprintf("template failed: ad-hoc target @ %s", ll)
		// do not report send errors up the chain, just log
	}

	ok, err := togid.SendMessage(msg)
	if err != nil {
		wasabee.Log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	if !ok {
		err := fmt.Errorf("message did not send")
		wasabee.Log.Warnw(err.Error(), "from", gid, "to", togid)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	out, err := json.Marshal(templateData)
	if err != nil {
		wasabee.Log.Warnw(err.Error(), "raw", templateData)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
	}
	togid.FirebaseTarget(string(out))
	fmt.Fprint(res, jsonStatusOK)
}

func agentPictureRoute(res http.ResponseWriter, req *http.Request) {
	_, err := getAgentID(req)
	if err != nil {
		wasabee.Log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	id := vars["id"]
	togid, err := wasabee.ToGid(id)
	if err != nil {
		wasabee.Log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	url := togid.GetPicture()
	http.Redirect(res, req, url, http.StatusPermanentRedirect)
}
