package wasabitelegram

import (
	// "errors"
	"encoding/json"
	"fmt"
	"github.com/cloudkucooland/WASABI"
	"github.com/go-telegram-bot-api/telegram-bot-api"
	"io/ioutil"
	"net/http"
	"strings"
)

// TGWebHook is the http route for recieving Telegram updates
func TGWebHook(res http.ResponseWriter, req *http.Request) {
	res.Header().Set("Content-Type", "application/json")
	contentType := strings.Split(strings.Replace(strings.ToLower(req.Header.Get("Content-Type")), " ", "", -1), ";")[0]
	if contentType != "application/json" {
		http.Error(res, "Invalid request (needs to be application/json)", http.StatusNotAcceptable)
		return
	}
	jBlob, err := ioutil.ReadAll(req.Body)
	if err != nil {
		wasabi.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	if string(jBlob) == "" {
		wasabi.Log.Notice("empty JSON")
		http.Error(res, "empty JSON", http.StatusInternalServerError)
		return
	}
	jRaw := json.RawMessage(jBlob)
	// wasabi.Log.Debug(string(jRaw))

	var update tgbotapi.Update
	err = json.Unmarshal(jRaw, &update)
	if err != nil {
		wasabi.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	config.upChan <- update
	fmt.Fprint(res, "{Status: 'OK'}")
}
