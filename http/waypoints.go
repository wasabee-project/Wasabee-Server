package wasabeehttps

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/wasabee-project/Wasabee-Server"
	"net/http"
)

func waypointsNearMeRoute(res http.ResponseWriter, req *http.Request) {
	gid, err := getAgentID(req)
	if err != nil {
		wasabee.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	var data wasabee.TeamData
	err = gid.WaypointsNear(50, 10, &data)
	if err != nil {
		wasabee.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	format := vars["f"]

	if format == "json" {
		out, _ := json.MarshalIndent(data, "", "\t")
		res.Header().Add("Content-Type", jsonType)
		fmt.Fprint(res, string(out))
		return
	}

	// templateExecute outputs to res
	if err := templateExecute(res, req, "Waypoints", data); err != nil {
		wasabee.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
}
