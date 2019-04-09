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
	// XXX temporary for testing, use getGid
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

func pDrawDeleteRoute(res http.ResponseWriter, req *http.Request) {
	var gid WASABI.GoogleID
	gid, err := getAgentID(req)
	if err != nil {
		WASABI.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)

	// only the ID needs to be set for this
	var op WASABI.Operation
	op.ID = WASABI.OperationID(vars["document"])

	if op.ID.IsOwner(gid) == true {
		err = fmt.Errorf("(not really) deleting operation %s", op.ID)
		WASABI.Log.Notice(err)
		// XXX op.Delete()
	} else {
		err = fmt.Errorf("Only the owner can delete an operation")
		WASABI.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusUnauthorized)
		return
	}
}

func jsonError(e error) string {
	s, _ := json.MarshalIndent(e, "", "\t")
	return string(s)
}
