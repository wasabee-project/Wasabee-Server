package PhDevHTTP

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/cloudkucooland/PhDevBin"
	// "github.com/gorilla/mux"
)

type loc struct {
	Lat      float64 `json:"lat"`
	Lon      float64 `json:"lon"`
	Type     string  `json:"_type"`
	Topic    string  `json:"topic"`
	Tid      string  `json:"tid"`
	T        string  `json:"t"`
	Conn     string  `json:"conn"`
	Altitude float64 `json:"alt"`
	Battery  float64 `json:"batt"`
	Accuracy float64 `json:"acc"`
	Vac      float64 `json:"vac"`
	Tst      float64 `json:"tst"`
	Vel      float64 `json:"vel"`
}

/*
type WaypointCommand struct {
	Type      string `json:"_type"`
	Action    string `json:"action"`
	Waypoints struct {
		Type      string     `json:"_type"`
		Waypoints []Waypoint `json:"waypoints"`
	} `json:"waypoints"`
}

type Waypoint struct {
	Type   string  `json:"_type"`
	Desc   string  `json:"desc"`
	Lat    float64 `json:"lat"`
	Lon    float64 `json:"lon"`
	Radius float64 `json:"rad"`
	ID     float64 `json:"tst"`
	UUID   string  `json:"uuid"`
	Major  string  `json:"major"`
	Minor  string  `json:"minor"`
} */

func ownTracksRoute(res http.ResponseWriter, req *http.Request) {
	gid, auth := ownTracksAuthentication(res, req)
	if auth == false {
		http.Error(res, "Error verifing authentication", http.StatusUnauthorized)
		// PhDevBin.Log.Debug("owntrack authentication failed")
		return
	}

	contentType := strings.Split(strings.Replace(strings.ToLower(req.Header.Get("Content-Type")), " ", "", -1), ";")[0]
	if contentType != "application/json" {
		http.Error(res, "Invalid request (needs to be application/json)", http.StatusNotAcceptable)
		return
	}

	jBlob, err := ioutil.ReadAll(req.Body)
	if err != nil {
		PhDevBin.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	if string(jBlob) == "" {
		PhDevBin.Log.Notice("empty JSON")
		fmt.Fprintf(res, "{ }")
		return
	}

	// PhDevBin.Log.Notice(string(jBlob))
	var t loc
	if err = json.Unmarshal(jBlob, &t); err != nil {
		PhDevBin.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	// s, _ := json.Marshal(t)

	switch t.Type {
	case "location":
		PhDevBin.OwnTracksUpdate(gid, string(jBlob), t.Lat, t.Lon)
		s, _ := PhDevBin.OwnTracksTeams(gid)
		fmt.Fprintf(res, string(s))
	case "waypoints":
		// since we don't know which team to add them to, we should probably just ignore them
		// {"_type":"waypoints","topic":"owntracks\/timid-outburst-qflh\/E2373EE8-538A-482C-99D6-F42E3B1373F3\/waypoints","waypoints":[{"_type":"waypoint","tst":1552259058,"lat":33.173594193279001,"lon":-96.815699161802996,"rad":30,"desc":"Center-1552259058"}]}
		fmt.Fprintf(res, "{ }")
	default:
		PhDevBin.Log.Notice("unprocessed type: " + t.Type)
		PhDevBin.Log.Debug(string(jBlob))
		fmt.Fprintf(res, "{ }")
	}
}

func ownTrackWaypointPub(t loc) error {
	return nil
}

func ownTracksAuthentication(res http.ResponseWriter, req *http.Request) (string, bool) {
	lockey, otpw, ok := req.BasicAuth()
	if ok == false {
		PhDevBin.Log.Notice("Unable to decode basic authentication")
		return "", false
	}

	gid, err := PhDevBin.VerifyOwnTracksPW(lockey, otpw)
	if err != nil {
		PhDevBin.Log.Notice(err)
		return "", false
	}
	if gid == "" {
		PhDevBin.Log.Noticef("OwnTracks authenticaion failed for: %s", lockey)
		return "", false
	}

	return gid, true
}
