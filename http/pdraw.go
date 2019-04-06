package WASABIhttps

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/cloudkucooland/WASABI"
	"github.com/gorilla/mux"
)

func pDrawUploadRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Set("Content-Type", "application/json")

	var gid WASABI.GoogleID
	gid = WASABI.GoogleID("118281765050946915735")

	contentType := strings.Split(strings.Replace(strings.ToLower(req.Header.Get("Content-Type")), " ", "", -1), ";")[0]
	if contentType != "application/json" {
		http.Error(res, "Invalid request (needs to be application/json)", http.StatusNotAcceptable)
		return
	}

	jBlob, err := ioutil.ReadAll(req.Body)
	if err != nil {
		WASABI.Log.Notice(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	if string(jBlob) == "" {
		WASABI.Log.Notice("empty JSON")
		http.Error(res, `{ "status": "error", "error": "Empty JSON" }`, http.StatusNotAcceptable)
		return
	}

	jRaw := json.RawMessage(jBlob)

	// WASABI.Log.Debug(string(jBlob))
	err = WASABI.PDrawInsert(jRaw, gid)
	if err != nil {
		WASABI.Log.Notice(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	fmt.Fprintf(res, `{ "status": "ok" }`)
}

func pDrawGetRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Set("Content-Type", "application/json")
	vars := mux.Vars(req)
	id := vars["document"]

	var gid WASABI.GoogleID
	gid = WASABI.GoogleID("118281765050946915735")

	var o WASABI.Operation
	o.ID = WASABI.OperationID(id)
	err := o.Populate(gid)
	if err != nil {
		WASABI.Log.Notice(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	s, err := json.MarshalIndent(o, "", "\t")
	if err != nil {
		WASABI.Log.Notice(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	fmt.Fprintf(res, string(s))
}

func jsonError(e error) string {
	s, _ := json.MarshalIndent(e, "", "\t")
	return string(s)
}
