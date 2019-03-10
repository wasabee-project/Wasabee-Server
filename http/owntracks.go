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
		PhDevBin.Log.Notice("unset JSON")
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
	// PhDevBin.Log.Notice(string(s))

	switch t.Type {
	case "location":
		PhDevBin.OwnTracksUpdate(gid, string(jBlob), t.Lat, t.Lon)
		s, _ := PhDevBin.OwnTracksTeams(gid)
		fmt.Fprintf(res, string(s))
	case "waypoints":
		fmt.Fprintf(res, "{ }")
	default:
		PhDevBin.Log.Notice("unprocessed type: " + t.Type)
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
