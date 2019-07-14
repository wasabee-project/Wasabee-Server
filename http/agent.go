package wasabeehttps

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/wasabee-project/Wasabee-Server"
	"net/http"
	"strings"
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
	if strings.Contains(req.Referer(), "intel.ingress.com") || strings.Contains(req.Header.Get("User-Agent"), appUserAgent) {
		data, _ := json.MarshalIndent(agent, "", "\t")
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
	fmt.Fprintf(res, `{ "status": "ok" }`)
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
