package WASABIhttps

import (
	"encoding/json"
	"fmt"
	"github.com/cloudkucooland/WASABI"
	"github.com/gorilla/mux"
	"net/http"
)

func waypointsNearMeRoute(res http.ResponseWriter, req *http.Request) {
	gid, err := getAgentID(req)
	if err != nil {
		WASABI.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	var data WASABI.TeamData
	err = gid.WaypointsNear(50, 10, &data)

	vars := mux.Vars(req)
	format := vars["f"]

	if format == "json" {
		out, _ := json.MarshalIndent(data, "", "\t")
		res.Header().Add("Content-Type", "text/json")
		fmt.Fprint(res, string(out))
		return
	}

	// wasabiHTTPSTemplateExecute outputs to res
	if err := wasabiHTTPSTemplateExecute(res, req, "Waypoints", data); err != nil {
		WASABI.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	return
}
