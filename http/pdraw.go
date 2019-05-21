package wasabihttps

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/cloudkucooland/WASABI"
	"github.com/gorilla/mux"
)

func pDrawUploadRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Set("Content-Type", jsonType)

	gid, err := getAgentID(req)
	if err != nil {
		wasabi.Log.Notice(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	contentType := strings.Split(strings.Replace(strings.ToLower(req.Header.Get("Content-Type")), " ", "", -1), ";")[0]
	if contentType != jsonTypeShort {
		http.Error(res, "Invalid request (needs to be application/json)", http.StatusNotAcceptable)
		return
	}

	jBlob, err := ioutil.ReadAll(req.Body)
	if err != nil {
		wasabi.Log.Notice(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	if string(jBlob) == "" {
		wasabi.Log.Notice("empty JSON")
		http.Error(res, `{ "status": "error", "error": "Empty JSON" }`, http.StatusNotAcceptable)
		return
	}

	jRaw := json.RawMessage(jBlob)
	// wasabi.Log.Debugf("sent json:", string(jRaw))
	if err = wasabi.PDrawInsert(jRaw, gid); err != nil {
		wasabi.Log.Notice(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	fmt.Fprintf(res, `{ "status": "ok" }`)
}

func pDrawGetRoute(res http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	id := vars["document"]

	ims := req.Header.Get("If-Modified-Since")
	if ims != "" {
		d, err := time.Parse(time.RFC1123, ims)
		if err != nil {
			wasabi.Log.Error(err)
		} else {
			wasabi.Log.Debug("if-modified-since: %s", d)
			// XXX we need to store a last update value on the operation
			// return 200/302 if newer/not
		}
	}

	gid, err := getAgentID(req)
	if err != nil {
		wasabi.Log.Notice(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	// XXX if HEAD skip the rest and just return 200/302

	var o wasabi.Operation
	o.ID = wasabi.OperationID(id)
	if err = o.Populate(gid); err != nil {
		wasabi.Log.Notice(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	// JSON if referer is intel.ingress.com
	if strings.Contains(req.Referer(), "intel.ingress.com") {
		res.Header().Set("Content-Type", jsonType)
		s, err := json.MarshalIndent(o, "", "\t")
		if err != nil {
			wasabi.Log.Notice(err)
			http.Error(res, jsonError(err), http.StatusInternalServerError)
			return
		}
		fmt.Fprint(res, string(s))
		return
	}

	// pretty output for everyone else
	if err = wasabiHTTPSTemplateExecute(res, req, "opdata", o); err != nil {
		wasabi.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
	}
}

func pDrawDeleteRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Set("Content-Type", jsonType)

	gid, err := getAgentID(req)
	if err != nil {
		wasabi.Log.Notice(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)

	// only the ID needs to be set for this
	var op wasabi.Operation
	op.ID = wasabi.OperationID(vars["document"])

	if op.ID.IsOwner(gid) {
		err = fmt.Errorf("deleting operation %s", op.ID)
		wasabi.Log.Notice(err)
		op.Delete()
	} else {
		err = fmt.Errorf("only the owner can delete an operation")
		wasabi.Log.Notice(err)
		http.Error(res, jsonError(err), http.StatusUnauthorized)
		return
	}

	fmt.Fprintf(res, `{ "status": "ok" }`)
}

func jsonError(e error) string {
	s, _ := json.MarshalIndent(e, "", "\t")
	return string(s)
}

func updateDrawRoute(res http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	id := vars["document"]

	res.Header().Set("Content-Type", jsonType)

	gid, err := getAgentID(req)
	if err != nil {
		wasabi.Log.Notice(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	contentType := strings.Split(strings.Replace(strings.ToLower(req.Header.Get("Content-Type")), " ", "", -1), ";")[0]
	if contentType != jsonTypeShort {
		http.Error(res, "Invalid request (needs to be application/json)", http.StatusNotAcceptable)
		return
	}

	jBlob, err := ioutil.ReadAll(req.Body)
	if err != nil {
		wasabi.Log.Notice(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	if string(jBlob) == "" {
		wasabi.Log.Notice("empty JSON")
		http.Error(res, `{ "status": "error", "error": "Empty JSON" }`, http.StatusNotAcceptable)
		return
	}

	jRaw := json.RawMessage(jBlob)

	// wasabi.Log.Debug(string(jBlob))
	if err = wasabi.PDrawUpdate(id, jRaw, gid); err != nil {
		wasabi.Log.Notice(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	fmt.Fprintf(res, `{ "status": "ok" }`)
}

func pDrawChownRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Set("Content-Type", jsonType)

	vars := mux.Vars(req)
	to := vars["to"]

	gid, err := getAgentID(req)
	if err != nil {
		wasabi.Log.Notice(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	// only the ID needs to be set for this
	var op wasabi.Operation
	op.ID = wasabi.OperationID(vars["document"])

	if op.ID.IsOwner(gid) {
		err = op.ID.Chown(gid, to)
		if err != nil {
			wasabi.Log.Notice(err)
			http.Error(res, jsonError(err), http.StatusInternalServerError)
			return
		}
	} else {
		err = fmt.Errorf("only the owner can set operation ownership ")
		wasabi.Log.Notice(err)
		http.Error(res, jsonError(err), http.StatusUnauthorized)
		return
	}
	fmt.Fprintf(res, `{ "status": "ok" }`)
}

func pDrawChgrpRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Set("Content-Type", jsonType)

	vars := mux.Vars(req)
	to := wasabi.TeamID(vars["to"])

	gid, err := getAgentID(req)
	if err != nil {
		wasabi.Log.Notice(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	// only the ID needs to be set for this
	var op wasabi.Operation
	op.ID = wasabi.OperationID(vars["document"])

	if op.ID.IsOwner(gid) {
		err = op.ID.Chgrp(gid, to)
		if err != nil {
			wasabi.Log.Notice(err)
			http.Error(res, jsonError(err), http.StatusInternalServerError)
			return
		}
	} else {
		err = fmt.Errorf("only the owner can set operation team")
		wasabi.Log.Notice(err)
		http.Error(res, jsonError(err), http.StatusUnauthorized)
		return
	}
	fmt.Fprintf(res, `{ "status": "ok" }`)
}
