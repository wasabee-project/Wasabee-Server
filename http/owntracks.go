package wasabihttps

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
	res.Header().Set("Content-Type", "application/json; charset=UTF-8")

	gid, auth := ownTracksAuthentication(res, req)
	if !auth {
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
		wasabi.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	if string(jBlob) == "" {
		wasabi.Log.Notice("empty JSON: probably delete waypoint / person request")
		waypoints, _ := gid.OwnTracksWaypoints()
		fmt.Fprint(res, string(waypoints))
		return
	}

	jRaw := json.RawMessage(jBlob)

	// wasabi.Log.Debug(string(jBlob))
	var t loc
	if err = json.Unmarshal(jBlob, &t); err != nil {
		wasabi.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	switch t.Type {
	case "location":
		gid.OwnTracksUpdate(jRaw, t.Lat, t.Lon)
		s, err := gid.OwnTracksTeams()
		if err != nil {
			wasabi.Log.Notice(err)
			http.Error(res, err.Error(), http.StatusInternalServerError)
		}
		// wasabi.Log.Debug(string(s))
		fmt.Fprint(res, string(s))
	case "transition":
		s, err := gid.OwnTracksTransition(jRaw)
		if err != nil {
			wasabi.Log.Notice(err)
			http.Error(res, err.Error(), http.StatusInternalServerError)
		}
		fmt.Fprint(res, string(s))
	case "waypoints":
		s, _ := gid.OwnTracksSetWaypointList(jRaw)
		fmt.Fprint(res, string(s))
	case "waypoint":
		s, _ := gid.OwnTracksSetWaypoint(jRaw)
		fmt.Fprint(res, string(s))
	default: // seen "cmd" in the wild
		wasabi.Log.Noticef("unhandled owntracks t.Type: %s", t.Type)
		wasabi.Log.Debug(string(jRaw))
		fmt.Fprint(res, "{ }")
	}
}

func ownTracksAuthentication(res http.ResponseWriter, req *http.Request) (wasabi.GoogleID, bool) {
	l, otpw, ok := req.BasicAuth()
	lockey := wasabi.LocKey(l)
	if !ok {
		wasabi.Log.Notice("Unable to decode basic authentication")
		return "", false
	}

	gid, err := lockey.VerifyOwnTracksPW(otpw)
	if err != nil {
		wasabi.Log.Notice(err)
		return "", false
	}
	if gid == "" {
		wasabi.Log.Noticef("OwnTracks authentication failed for: %s", lockey)
		return "", false
	}

	return gid, true
}
