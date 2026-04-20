package rocks

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/wasabee-project/Wasabee-Server/log"
)

const jsonTypeShort = "application/json"
const jsonStatusOK = `{"status":"ok"}`

func rocksCommunityRoute(res http.ResponseWriter, req *http.Request) {
	// 1. Validate Content-Type
	if !contentTypeIs(req, jsonTypeShort) {
		http.Error(res, "Invalid request (needs to be application/json)", http.StatusNotAcceptable)
		return
	}

	// 2. Decode directly from the stream into a RawMessage
	// This satisfies the TODO and handles larger payloads more gracefully
	var jRaw json.RawMessage
	decoder := json.NewDecoder(req.Body)
	if err := decoder.Decode(&jRaw); err != nil {
		log.Errorw("failed to decode rocks community sync", "error", err)
		http.Error(res, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// 3. Pass the request context down
	// This ensures that if the HTTP connection drops, the DB operations in CommunitySync stop
	if err := CommunitySync(req.Context(), jRaw); err != nil {
		log.Errorw("rocks community sync failed", "error", err, "content", string(jRaw))
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	res.Header().Set("Content-Type", jsonTypeShort)
	res.WriteHeader(http.StatusOK)
	_, _ = res.Write([]byte(jsonStatusOK))
}

func contentTypeIs(req *http.Request, check string) bool {
	// Clean up the header string (remove charset, etc)
	header := req.Header.Get("Content-Type")
	contentType := strings.Split(strings.ReplaceAll(header, " ", ""), ";")[0]
	return strings.EqualFold(contentType, check)
}
