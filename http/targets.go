package PhDevHTTP

import (
	"encoding/json"
	"fmt"
	"github.com/cloudkucooland/PhDevBin"
	"github.com/gorilla/mux"
	"net/http"
)

func targetsUploadRoute(res http.ResponseWriter, req *http.Request) {
	gid, err := getUserID(req)
	if err != nil {
		PhDevBin.Log.Notice(err.Error())
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	PhDevBin.Log.Debug(gid)
	// process
	return
}

func targetsRoute(res http.ResponseWriter, req *http.Request) {
	gid, err := getUserID(req)
	if err != nil {
		PhDevBin.Log.Notice(err.Error())
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	PhDevBin.Log.Debug(gid)

	// vars := mux.Vars(req)
	// teamID := vars["team"]
	// process
	return
}

func targetsDeleteRoute(res http.ResponseWriter, req *http.Request) {
	gid, err := getUserID(req)
	if err != nil {
		PhDevBin.Log.Notice(err.Error())
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	PhDevBin.Log.Debug(gid)

	// process
	return
}

func targetsNearMeRoute(res http.ResponseWriter, req *http.Request) {
	gid, err := getUserID(req)
	if err != nil {
		PhDevBin.Log.Notice(err.Error())
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	var data PhDevBin.TeamData
	err = PhDevBin.TargetsNearGid(gid, 50, 10, &data)

	vars := mux.Vars(req)
	format := vars["f"]

	if format == "json" {
		out, _ := json.MarshalIndent(data, "", "\t")
		res.Header().Add("Content-Type", "text/json")
		fmt.Fprint(res, string(out))
		return
	}

	// phDevBinHTTPSTemplateExecute outputs to res
	if err := phDevBinHTTPSTemplateExecute(res, req, "Targets", data); err != nil {
		PhDevBin.Log.Notice(err.Error())
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	return
}
