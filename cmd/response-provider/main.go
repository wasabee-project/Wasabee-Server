package main

import (
	"encoding/json"
	"fmt"
	"net/http"
)

const port = "8445"

var tokens map[string]GoogleData
var gids map[string]GoogleData

// what is returned when pretending to be google
type GoogleData struct {
	Gid   string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
	Pic   string `json:"picture"`
}

// Vresult is set by the V API
type Vresult struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
	Data    vagent `json:"data"`
}

// vagent is set by the V API
type vagent struct {
	EnlID       string `json:"enlid"`
	Vlevel      int64  `json:"vlevel"`
	Vpoints     int64  `json:"vpoints"`
	Agent       string `json:"agent"`
	Level       int64  `json:"level"`
	Quarantine  bool   `json:"quarantine"`
	Active      bool   `json:"active"`
	Blacklisted bool   `json:"blacklisted"`
	Verified    bool   `json:"verified"`
	Flagged     bool   `json:"flagged"`
	Banned      bool   `json:"banned_by_nia"`
	Cellid      string `json:"cellid"`
}

func main() {
	// XXX parse CLI options

	// token -> GoogleData
	tokens = make(map[string]GoogleData)
	// gid -> GoogleData
	gids = make(map[string]GoogleData)

	// set up handlers for the URIs
	http.HandleFunc("/GoogleToken", googleToken)
	http.HandleFunc("/VSearch", vSearch)
	http.HandleFunc("/RocksSearch", rocksSearch)

	fmt.Printf("starting http listener on port %s\nuse control-c to stop\n\n", port)
	// start HTTP -- loop until something goes wrong (^C to kill)
	if err := http.ListenAndServe(fmt.Sprintf(":%s", port), nil); err != nil {
		panic(err)
	}
}

func googleToken(res http.ResponseWriter, req *http.Request) {
	token := req.FormValue("token")
	if token == "" {
		fmt.Fprintf(res, "token not set")
		return
	}

	fmt.Printf("looking for GoogleData for token %s", token)
	data, ok := tokens[token]
	if !ok {
		fmt.Printf("generating new GoogleData for token %s", token)
		// generate new GoogleData with random values
		// XXX make this actually random
		gid := "12345"
		data = GoogleData{
			Gid:   gid,
			Name:  "random",
			Email: "random@example.com",
			Pic:   "http://example.com/12345.jpg",
		}
		// stuff it into the maps for later usage
		tokens[token] = data
		gids[gid] = data
	}

	j, _ := json.Marshal(data)
	fmt.Printf("returning %s for token %s", string(j), token)
	res.Header().Set("Content-Type", "application/json; charset=UTF-8")
	fmt.Fprintf(res, string(j))
}

func vSearch(res http.ResponseWriter, req *http.Request) {
	gid := req.FormValue("gid")
	if gid == "" {
		fmt.Fprintf(res, "gid not set")
		return
	}
	data, ok := gids[gid]
	if !ok {
		fmt.Fprintf(res, "gid %s not known", gid)
		return
	}

	// XXX craft and send response
	v := Vresult{
		Status: "ok",
	}
	v.Data = vagent{
		EnlID:       fmt.Sprintf("enl-%s", data.Gid),
		Vlevel:      1,
		Vpoints:     1,
		Agent:       "Barcode",
		Level:       16,
		Quarantine:  false,
		Active:      true,
		Blacklisted: false,
		Verified:    true,
		Flagged:     false,
		Banned:      false,
		Cellid:      "AMS-GOLF-06",
	}
	j, _ := json.Marshal(v)
	res.Header().Set("Content-Type", "application/json; charset=UTF-8")
	fmt.Fprintf(res, string(j))
}

func rocksSearch(res http.ResponseWriter, req *http.Request) {

}
