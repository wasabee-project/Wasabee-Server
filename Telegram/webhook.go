package wasabeetelegram

import (
	"encoding/json"
	"fmt"
	"github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/gorilla/mux"
	"github.com/wasabee-project/Wasabee-Server"
	"io/ioutil"
	"net/http"
	"strings"
)

// TGWebHook is the http route for receiving Telegram updates
func TGWebHook(res http.ResponseWriter, req *http.Request) {
	var err error

	if config.APIKey == "" || config.hook == "" {
		err = fmt.Errorf("the Telegram API is not configured")
		wasabee.Log.Info(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	hook := vars["hook"]

	if hook != config.hook {
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
		wasabee.Log.Error(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	if string(jBlob) == "" {
		err = fmt.Errorf("empty JSON")
		wasabee.Log.Error(err)
		http.Error(res, err.Error(), http.StatusNotAcceptable)
		return
	}
	jRaw := json.RawMessage(jBlob)

	var update tgbotapi.Update
	err = json.Unmarshal(jRaw, &update)
	if err != nil {
		wasabee.Log.Error(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	// put the update into the subsystem update channel for processing by the bot logic
	config.upChan <- update

	res.Header().Set("Content-Type", "application/json")
	fmt.Fprint(res, "{Status: 'OK'}")
}
