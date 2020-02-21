package wasabeehttps

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/wasabee-project/Wasabee-Server"
	"net/http"
)

func agentProfileRoute(res http.ResponseWriter, req *http.Request) {
	var agent wasabee.Agent

	// must be authenticated
	gid, err := getAgentID(req)
	if err != nil {
		wasabee.Log.Error(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	id := vars["id"]

	togid, err := wasabee.ToGid(id)
	if err != nil {
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	err = wasabee.FetchAgent(togid, &agent)
	if err != nil {
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	agent.CanSendTo = gid.CanSendTo(togid)

	// if the request comes from intel, just return JSON
	if wantsJSON(req) {
		data, _ := json.Marshal(agent)
		res.Header().Add("Content-Type", jsonType)
		fmt.Fprint(res, string(data))
		return
	}

	// TemplateExecute prints directly to the result writer
	if err := templateExecute(res, req, "agent", agent); err != nil {
		wasabee.Log.Error(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
	}
}

func agentMessageRoute(res http.ResponseWriter, req *http.Request) {
	gid, err := getAgentID(req)
	if err != nil {
		wasabee.Log.Error(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	id := vars["id"]
	togid, err := wasabee.ToGid(id)
	if err != nil {
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	message := req.FormValue("m")
	if message == "" {
		message = "This is a toast notification"
	}

	ok := gid.CanSendTo(togid)
	if !ok {
		http.Error(res, "Unauthorized", http.StatusUnauthorized)
		return
	}
	ok, err = togid.SendMessage(message)
	if err != nil {
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	if !ok {
		http.Error(res, "message did not send", http.StatusInternalServerError)
		return
	}
	res.Header().Add("Content-Type", jsonType)
	fmt.Fprintf(res, jsonStatusOK)
}

func agentTargetRoute(res http.ResponseWriter, req *http.Request) {
	_, err := getAgentID(req)
	if err != nil {
		wasabee.Log.Error(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	id := vars["id"]
	togid, err := wasabee.ToGid(id)
	if err != nil {
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	portal := req.FormValue("portal")
	if portal == "" {
		http.Error(res, "portal not set", http.StatusNotAcceptable)
		return
	}

	ll := req.FormValue("ll")
	if ll == "" {
		http.Error(res, "ll not set", http.StatusNotAcceptable)
		return
	}

        message := fmt.Sprintf(
	  "[%s](https://intel.ingress.com/intel?ll=%s&z=12&pll=%s): [Google Maps](http://maps.google.com/?q=%s) | [Apple Maps](http://maps.apple.com/?q=%s)",
	  portal, ll, ll, ll, ll);

	ok, err := togid.SendMessage(message)
	if err != nil {
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	if !ok {
		http.Error(res, "message did not send", http.StatusInternalServerError)
		return
	}
	res.Header().Add("Content-Type", jsonType)
	fmt.Fprintf(res, jsonStatusOK)
}

func agentPictureRoute(res http.ResponseWriter, req *http.Request) {
	_, err := getAgentID(req)
	if err != nil {
		wasabee.Log.Error(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	id := vars["id"]
	togid, err := wasabee.ToGid(id)
	if err != nil {
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	url := togid.GetPicture()
	http.Redirect(res, req, url, http.StatusPermanentRedirect)
}
