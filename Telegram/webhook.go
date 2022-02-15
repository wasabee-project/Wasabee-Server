package wtg

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/gorilla/mux"

	"github.com/wasabee-project/Wasabee-Server/log"
)

// webhook is the http route for receiving Telegram updates
func webhook(res http.ResponseWriter, req *http.Request) {
	var err error

	if hook == "" {
		err := fmt.Errorf("the Telegram API is not configured")
		log.Info(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	h := vars["hook"]

	if h != hook {
		err = fmt.Errorf("%s is not a valid hook", h)
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
	jBlob, err := io.ReadAll(req.Body)
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
	upChan <- update

	res.Header().Set("Content-Type", "application/json")
	fmt.Fprint(res, `{"status":"ok"}`)
}
