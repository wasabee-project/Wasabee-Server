package wtg

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/gorilla/mux"

	"github.com/wasabee-project/Wasabee-Server/log"
)

const jsonType = "application/json"
const ctHeader = "Content-Type"

// webhook is the http route for receiving Telegram updates
func webhook(res http.ResponseWriter, req *http.Request) {
	if hook == "" {
		err := fmt.Errorf("the Telegram API is not configured")
		log.Error(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	h := vars["hook"]

	if h != hook {
		err := fmt.Errorf("%s is not a valid hook", h)
		log.Error(err)
		http.Error(res, err.Error(), http.StatusUnauthorized)
		return
	}

	contentType := strings.Split(strings.Replace(strings.ToLower(req.Header.Get(ctHeader)), " ", "", -1), ";")[0]
	if contentType != jsonType {
		err := fmt.Errorf("invalid request")
		log.Errorw(err.Error(), ctHeader, contentType)
		http.Error(res, err.Error(), http.StatusNotAcceptable)
		return
	}

	var update tgbotapi.Update
	if err := json.NewDecoder(req.Body).Decode(&update); err != nil {
		log.Error(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	// put the update into the subsystem update channel for processing by the bot logic
	upChan <- update

	res.Header().Set(ctHeader, jsonType)
	fmt.Fprint(res, `{"status":"ok"}`)
}
