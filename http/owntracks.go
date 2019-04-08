package WASABIhttps

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/cloudkucooland/WASABI"
)

// or use the WASABI.Location struct
// this is minimal for what we need here
type loc struct {
	Lat  float64 `json:"lat"`
	Lon  float64 `json:"lon"`
	Type string  `json:"_type"`
}

func ownTracksRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Set("Content-Type", "application/json")

	gid, auth := ownTracksAuthentication(res, req)
	if auth == false {
		http.Error(res, "Error verifing authentication", http.StatusUnauthorized)
		return
	}

	contentType := strings.Split(strings.Replace(strings.ToLower(req.Header.Get("Content-Type")), " ", "", -1), ";")[0]
	if contentType != "application/json" {
		http.Error(res, "Invalid request (needs to be application/json)", http.StatusNotAcceptable)
		return
	}

	jBlob, err := ioutil.ReadAll(req.Body)
	if err != nil {
		WASABI.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	if string(jBlob) == "" {
		WASABI.Log.Notice("empty JSON: probably delete waypoint / person request")
		waypoints, _ := gid.OwnTracksWaypoints()
		fmt.Fprintf(res, string(waypoints))
		return
	}

	jRaw := json.RawMessage(jBlob)

	// WASABI.Log.Debug(string(jBlob))
	var t loc
	if err = json.Unmarshal(jBlob, &t); err != nil {
		WASABI.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	switch t.Type {
	case "location":
		gid.OwnTracksUpdate(jRaw, t.Lat, t.Lon)
		s, err := gid.OwnTracksTeams()
		if err != nil {
			WASABI.Log.Notice(err)
			http.Error(res, err.Error(), http.StatusInternalServerError)
		}
		// WASABI.Log.Debug(string(s))
		fmt.Fprintf(res, string(s))
	case "transition":
		s, err := gid.OwnTracksTransition(jRaw)
		if err != nil {
			WASABI.Log.Notice(err)
			http.Error(res, err.Error(), http.StatusInternalServerError)
		}
		fmt.Fprintf(res, string(s))
	case "waypoints":
		s, _ := gid.OwnTracksSetWaypointList(jRaw)
		fmt.Fprintf(res, string(s))
	case "waypoint":
		s, _ := gid.OwnTracksSetWaypoint(jRaw)
		fmt.Fprintf(res, string(s))
	default:
		WASABI.Log.Notice("unhandled type: " + t.Type)
		WASABI.Log.Debug(string(jRaw))
		fmt.Fprintf(res, "{ }")
	}
}

func ownTracksAuthentication(res http.ResponseWriter, req *http.Request) (WASABI.GoogleID, bool) {
	l, otpw, ok := req.BasicAuth()
	lockey := WASABI.LocKey(l)
	if ok == false {
		WASABI.Log.Notice("Unable to decode basic authentication")
		return "", false
	}

	gid, err := lockey.VerifyOwnTracksPW(otpw)
	if err != nil {
		WASABI.Log.Notice(err)
		return "", false
	}
	if gid == "" {
		WASABI.Log.Noticef("OwnTracks authentication failed for: %s", lockey)
		return "", false
	}

	return gid, true
}
