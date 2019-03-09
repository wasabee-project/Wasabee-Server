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

var testWaypoint string

func ownTracksRoute(res http.ResponseWriter, req *http.Request) {
	testWaypoint = `{ "_type": "cmd", "action": "setWaypoints", "waypoints": { "waypoints": [ { "desc": "Some place", "rad": 8867, "lon": 10.428771973, "lat": 46.935260881, "tst": 1437552714, "_type": "waypoint" } ], "_type": "waypoints" } }`

	if ownTracksAuthentication(req) == false {
		http.Error(res, "Unauthorized", http.StatusUnauthorized)
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

	PhDevBin.Log.Notice(string(jBlob))
	var t loc
	if err = json.Unmarshal(jBlob, &t); err != nil {
		PhDevBin.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	s, _ := json.Marshal(t)
	PhDevBin.Log.Notice(string(s))

	switch t.Type {
	case "location":
		fmt.Fprintf(res, testWaypoint)
	case "waypoints":
		fmt.Fprintf(res, "{ }")
	default:
		PhDevBin.Log.Notice("unprocessed type: " + t.Type)
		fmt.Fprintf(res, "{ }")
		// http.Error(res, "Invalid request (only _type location supported currently)", http.StatusNotAcceptable)
		// return
	}
}

func ownTrackWaypointPub(t loc) error {
	return nil
}

func ownTracksAuthentication(req *http.Request) bool {
	// user, pass, _ := req.BasicAuth()
	var res bool
	res = true
	return res
}
