package rocks

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/wasabee-project/Wasabee-Server/log"
)

const jsonTypeShort = "application/json"
const jsonStatusOK = `{"status":"ok"}`

func rocksCommunityRoute(res http.ResponseWriter, req *http.Request) {
	if !contentTypeIs(req, jsonTypeShort) {
		http.Error(res, "Invalid request (needs to be application/json)", http.StatusNotAcceptable)
		return
	}

	// TODO: use json.Decode here, rewrite CommunitySync to take it already Decoded...
	jBlob, err := io.ReadAll(req.Body)
	if err != nil {
		log.Error(err)
		http.Error(res, err.Error(), http.StatusNotAcceptable)
		return
	}

	if string(jBlob) == "" {
		err := fmt.Errorf("empty JSON on rocks community sync")
		log.Warnw(err.Error())
		http.Error(res, err.Error(), http.StatusNotAcceptable)
		return
	}

	jRaw := json.RawMessage(jBlob)
	if err := CommunitySync(jRaw); err != nil {
		log.Errorw(err.Error(), "content", jRaw)
		http.Error(res, err.Error(), http.StatusNotAcceptable)
		return
	}

	fmt.Fprint(res, jsonStatusOK)
}

func contentTypeIs(req *http.Request, check string) bool {
	contentType := strings.Split(strings.Replace(req.Header.Get("Content-Type"), " ", "", -1), ";")[0]
	return strings.EqualFold(contentType, check)
}
