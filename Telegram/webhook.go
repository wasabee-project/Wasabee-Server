package wtg

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/wasabee-project/Wasabee-Server/log"
)

const (
	jsonType = "application/json"
	ctHeader = "Content-Type"
)

// webhook is the http route for receiving Telegram updates
func webhook(res http.ResponseWriter, req *http.Request) {
	if hook == "" {
		http.Error(res, "Telegram API not configured", http.StatusInternalServerError)
		return
	}

	h := req.PathValue("hook")
	if h != hook {
		log.Warnw("unauthorized webhook access", "received", h)
		http.Error(res, "Unauthorized", http.StatusUnauthorized)
		return
	}

	ct := req.Header.Get(ctHeader)
	if !strings.HasPrefix(strings.ToLower(ct), jsonType) {
		log.Debugw("invalid content-type", ctHeader, ct)
		http.Error(res, "JSON expected", http.StatusNotAcceptable)
		return
	}

	var update tgbotapi.Update
	if err := json.NewDecoder(req.Body).Decode(&update); err != nil {
		log.Errorw("failed to decode telegram update", "error", err)
		http.Error(res, "Bad Request", http.StatusBadRequest)
		return
	}

	select {
	case upChan <- update:
		res.Header().Set(ctHeader, jsonType)
		res.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(res, `{"status":"ok"}`)
	case <-req.Context().Done():
		log.Warn("webhook request context cancelled before queueing")
	}
}
