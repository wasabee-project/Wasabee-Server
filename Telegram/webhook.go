package wtg

import (
	"fmt"
	"net/http"

	"github.com/wasabee-project/Wasabee-Server/log"
)

const jsonType = "application/json"

// webhook is the http route for receiving Telegram updates
func webhook(res http.ResponseWriter, req *http.Request) {
	// If the bot isn't initialized or hook is missing, reject
	if bot == nil || hook == "" {
		log.Error("Telegram bot or hook not initialized")
		http.Error(res, "Service Unavailable", http.StatusServiceUnavailable)
		return
	}

	// Validate the rolling hook slug from the URL
	hh := req.PathValue("hook")
	if hh != hook {
		log.Warnw("unauthorized webhook attempt", "received", hh)
		http.Error(res, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Use v5's built-in helper to parse the update from the request body
	update, err := bot.HandleUpdate(req)
	if err != nil {
		log.Errorw("failed to parse telegram update", "error", err)
		http.Error(res, "Bad Request", http.StatusBadRequest)
		return
	}

	// Push to the update channel for the main loop in bot.go to pick up
	// Ensure the loop is context-aware so we don't block forever if shutting down
	select {
	case upChan <- *update:
		res.Header().Set("Content-Type", jsonType)
		res.WriteHeader(http.StatusOK)
		fmt.Fprint(res, `{"status":"ok"}`)
	default:
		log.Error("Telegram update channel full, dropping update")
		http.Error(res, "Service Unavailable", http.StatusServiceUnavailable)
	}
}
