package wasabeetelegram

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/gorilla/mux"

	"github.com/wasabee-project/Wasabee-Server/log"
)

// TGWebHook is the http route for receiving Telegram updates
func TGWebHook(res http.ResponseWriter, req *http.Request) {
	var err error

	if c.APIKey == "" || c.hook == "" {
		err = fmt.Errorf("the Telegram API is not configured")
		log.Info(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	hook := vars["hook"]

	if hook != c.hook {
		err = fmt.Errorf("%s is not a valid hook", hook)
		log.Error(err)
		http.Error(res, err.Error(), http.StatusUnauthorized)
		return
	}

	contentType := strings.Split(strings.Replace(strings.ToLower(req.Header.Get("Content-Type")), " ", "", -1), ";")[0]
	if contentType != "application/json" {
		err = fmt.Errorf("invalid request (needs to be application/json)")
		log.Error(err)
		http.Error(res, err.Error(), http.StatusNotAcceptable)
		return
	}
	jBlob, err := ioutil.ReadAll(req.Body)
	if err != nil {
		log.Error(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	if string(jBlob) == "" {
		err = fmt.Errorf("empty JSON")
		log.Error(err)
		http.Error(res, err.Error(), http.StatusNotAcceptable)
		return
	}
	jRaw := json.RawMessage(jBlob)

	var update tgbotapi.Update
	err = json.Unmarshal(jRaw, &update)
	if err != nil {
		log.Error(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	// put the update into the subsystem update channel for processing by the bot logic
	c.upChan <- update

	res.Header().Set("Content-Type", "application/json")
	fmt.Fprint(res, `{"status":"ok"}`)
}
