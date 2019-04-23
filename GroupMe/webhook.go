package wasabigm

import (
	// "errors"
	"encoding/json"
	"fmt"
	"github.com/cloudkucooland/WASABI"
	"io/ioutil"
	"net/http"
	"strings"
)

// GMWebHook is the http route for receiving GM updates
func GMWebHook(res http.ResponseWriter, req *http.Request) {
	var err error

	contentType := strings.Split(strings.Replace(strings.ToLower(req.Header.Get("Content-Type")), " ", "", -1), ";")[0]
	if contentType != "application/json" {
		err = fmt.Errorf("invalid request (needs to be application/json)")
		wasabi.Log.Error(err)
		http.Error(res, err.Error(), http.StatusNotAcceptable)
		return
	}
	jBlob, err := ioutil.ReadAll(req.Body)
	if err != nil {
		wasabi.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	if string(jBlob) == "" {
		err = fmt.Errorf("empty JSON")
		wasabi.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusNotAcceptable)
		return
	}
	jRaw := json.RawMessage(jBlob)
	// wasabi.Log.Debug(string(jRaw))
	config.upChan <- jRaw

	// XXX probably not needed
	// res.Header().Set("Content-Type", "application/json")
	// fmt.Fprint(res, "{Status: 'OK'}")
}
