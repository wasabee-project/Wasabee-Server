package WASABIhttps

import (
	"encoding/json"
	"fmt"
	"github.com/cloudkucooland/WASABI"
	"github.com/gorilla/mux"
	"net/http"
)

func agentProfileRoute(res http.ResponseWriter, req *http.Request) {
	var agent WASABI.Agent

	_, err := getUserID(req)
	if err != nil {
		WASABI.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	id := vars["id"]

	err = WASABI.FetchAgent(id, &agent) // FetchAgent takes gid, lockey, eid ...
	// this won't be JSON, just here fore debugging
	data, _ := json.MarshalIndent(agent, "", "\t")
	s := string(data)
	res.Header().Add("Content-Type", "text/json")
	fmt.Fprintf(res, s)
	return
}
