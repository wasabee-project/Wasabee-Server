package wasabeegm

import (
	// "errors"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	wasabee "github.com/wasabee-project/Wasabee-Server"
	"github.com/gorilla/mux"
)

// GMWebHook is the http route for receiving GM updates
func GMWebHook(res http.ResponseWriter, req *http.Request) {
	var err error
	vars := mux.Vars(req)
	hook := vars["hook"]

	if config.AccessToken == "" {
		err = fmt.Errorf("the GroupMe API is not configured")
		wasabee.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	var found bool
	for _, v := range config.bots {
		tmp := strings.Split(v.CallbackURL, "/")
		botHook := tmp[len(tmp)-1]
		if hook == botHook {
			found = true
			break
		}
	}
	if !found {
		err = fmt.Errorf("%s is not a valid hook", hook)
		wasabee.Log.Error(err)
		http.Error(res, err.Error(), http.StatusUnauthorized)
		return
	}

	contentType := strings.Split(strings.Replace(strings.ToLower(req.Header.Get("Content-Type")), " ", "", -1), ";")[0]
	if contentType != "application/json" {
		err = fmt.Errorf("invalid request (needs to be application/json)")
		wasabee.Log.Error(err)
		http.Error(res, err.Error(), http.StatusNotAcceptable)
		return
	}
	jBlob, err := ioutil.ReadAll(req.Body)
	if err != nil {
		wasabee.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	if string(jBlob) == "" {
		err = fmt.Errorf("empty JSON")
		wasabee.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusNotAcceptable)
		return
	}
	jRaw := json.RawMessage(jBlob)
	// wasabee.Log.Debug(string(jRaw))
	config.upChan <- jRaw

	// XXX probably not needed
	res.Header().Set("Content-Type", "application/json")
	fmt.Fprint(res, `{"Status": "OK"}`)
}
