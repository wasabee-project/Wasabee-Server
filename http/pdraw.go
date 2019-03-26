package PhDevHTTP

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/cloudkucooland/PhDevBin"
)

func PDrawUploadRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Set("Content-Type", "application/json")

	var gid PhDevBin.GoogleID
	gid = PhDevBin.GoogleID("118281765050946915735")

	contentType := strings.Split(strings.Replace(strings.ToLower(req.Header.Get("Content-Type")), " ", "", -1), ";")[0]
	if contentType != "application/json" {
		http.Error(res, "Invalid request (needs to be application/json)", http.StatusNotAcceptable)
		return
	}

	jBlob, err := ioutil.ReadAll(req.Body)
	if err != nil {
		PhDevBin.Log.Notice(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	if string(jBlob) == "" {
		PhDevBin.Log.Notice("empty JSON")
		http.Error(res, `{ "status": "error", "error": "Empty JSON" }`, http.StatusNotAcceptable)
		return
	}

	jRaw := json.RawMessage(jBlob)

	// PhDevBin.Log.Debug(string(jBlob))
	err = PhDevBin.PDrawInsert(jRaw, gid)
	if err != nil {
		PhDevBin.Log.Notice(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	fmt.Fprintf(res, `{ "status": "ok" }`)
}

func jsonError(e error) string {
	s, _ := json.MarshalIndent(e, "", "\t")
	return string(s)
}
