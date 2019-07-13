package wasabeehttps

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/wasabee-project/Wasabee-Server"
)

// or use the wasabee.Location struct
// this is minimal for what we need here
type loc struct {
	Lat  float64 `json:"lat"`
	Lon  float64 `json:"lon"`
	Type string  `json:"_type"`
}

// basic auth for the OT app
func ownTracksBasicRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Set("Content-Type", jsonType)

	gid, auth := ownTracksAuthentication(res, req)
	if !auth {
		http.Error(res, "Error verifing authentication", http.StatusUnauthorized)
		return
	}
	ownTracksmain(res, req, gid)
}

// wasabee auth for our app
func ownTracksWasabeeRoute(res http.ResponseWriter, req *http.Request) {
	gid, err := getAgentID(req)
	if err != nil {
		http.Error(res, err.Error(), http.StatusUnauthorized)
	}
	ownTracksmain(res, req, gid)
}

func ownTracksmain(res http.ResponseWriter, req *http.Request, gid wasabee.GoogleID) {
	contentType := strings.Split(strings.Replace(strings.ToLower(req.Header.Get("Content-Type")), " ", "", -1), ";")[0]
	if contentType != jsonTypeShort {
		http.Error(res, "Invalid request (needs to be application/json)", http.StatusNotAcceptable)
		return
	}

	// defer req.Body.Close()
	jBlob, err := ioutil.ReadAll(req.Body)
	if err != nil {
		wasabee.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	if string(jBlob) == "" {
		wasabee.Log.Notice("empty JSON: probably delete waypoint / person request")
		waypoints, _ := gid.OwnTracksWaypoints()
		fmt.Fprint(res, string(waypoints))
		return
	}

	jRaw := json.RawMessage(jBlob)

	// wasabee.Log.Debug(string(jBlob))
	var t loc
	if err = json.Unmarshal(jBlob, &t); err != nil {
		wasabee.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	switch t.Type {
	case "location":
		err = gid.OwnTracksUpdate(jRaw, t.Lat, t.Lon)
		if err != nil {
			wasabee.Log.Notice(err)
			http.Error(res, err.Error(), http.StatusInternalServerError)
		}
		s, err := gid.OwnTracksTeams()
		if err != nil {
			wasabee.Log.Notice(err)
			http.Error(res, err.Error(), http.StatusInternalServerError)
		}
		// wasabee.Log.Debug(string(s))
		fmt.Fprint(res, string(s))
	case "transition":
		s, err := gid.OwnTracksTransition(jRaw)
		if err != nil {
			wasabee.Log.Notice(err)
			http.Error(res, err.Error(), http.StatusInternalServerError)
		}
		fmt.Fprint(res, string(s))
	/*
		case "waypoints":
			s, _ := gid.OwnTracksSetWaypointList(jRaw)
			fmt.Fprint(res, string(s))
		case "waypoint":
			s, _ := gid.OwnTracksSetWaypoint(jRaw)
			fmt.Fprint(res, string(s))
	*/
	default: // seen "cmd" in the wild
		wasabee.Log.Noticef("unhandled owntracks t.Type: %s", t.Type)
		wasabee.Log.Debug(string(jRaw))
		fmt.Fprint(res, "{ }")
	}
}

// convert this to gorilla middleware -- leave res in place even though unused
func ownTracksAuthentication(res http.ResponseWriter, req *http.Request) (wasabee.GoogleID, bool) {
	l, otpw, ok := req.BasicAuth()
	lockey := wasabee.LocKey(l)
	if !ok {
		wasabee.Log.Notice("Unable to decode basic authentication")
		return "", false
	}

	if lockey == "" {
		wasabee.Log.Noticef("OwnTracks username not set")
		return "", false
	}

	gid, err := lockey.VerifyOwnTracksPW(otpw)
	if err != nil {
		wasabee.Log.Notice(err)
		return "", false
	}
	if gid == "" {
		wasabee.Log.Noticef("OwnTracks authentication failed for: %s", lockey)
		return "", false
	}

	return gid, true
}
