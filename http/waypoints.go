package PhDevHTTP

import (
	"encoding/json"
	"fmt"
	"github.com/cloudkucooland/PhDevBin"
	"github.com/gorilla/mux"
	"net/http"
)

func waypointsNearMeRoute(res http.ResponseWriter, req *http.Request) {
	gid, err := getUserID(req)
	if err != nil {
		PhDevBin.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	var data PhDevBin.TeamData
	err = gid.WaypointsNear(50, 10, &data)

	vars := mux.Vars(req)
	format := vars["f"]

	if format == "json" {
		out, _ := json.MarshalIndent(data, "", "\t")
		res.Header().Add("Content-Type", "text/json")
		fmt.Fprint(res, string(out))
		return
	}

	// phDevBinHTTPSTemplateExecute outputs to res
	if err := phDevBinHTTPSTemplateExecute(res, req, "Waypoints", data); err != nil {
		PhDevBin.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	return
}
