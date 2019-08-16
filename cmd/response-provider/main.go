package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

const port = "8445"

var tokens map[string]GoogleData
var gids map[string]GoogleData
var rocks map[string]RocksAgent
var vees map[string]vagent

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

type RocksAgent struct {
	Gid      string `json:"gid"`
	TGId     int64  `json:"tgid"`
	Agent    string `json:"agentid"`
	Verified bool   `json:"verified"`
	Smurf    bool   `json:"smurf"`
	Fullname string `json:"name"`
}

func main() {
	// XXX parse CLI options

	// token -> GoogleData
	tokens = make(map[string]GoogleData)
	// gid -> GoogleData
	gids = make(map[string]GoogleData)
	// gid -> RocksAgent
	rocks = make(map[string]RocksAgent)
	// gid -> vagent
	vees = make(map[string]vagent)

	// set up some default users to query against
	preload()

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
		fmt.Printf("generating new data for token %s", token)
		// generate new GoogleData with random values
		// XXX make this actually random
		gid := "12345"
		data = GoogleData{
			Gid:   gid,
			Name:  fmt.Sprintf("random %s inadequate", gid),
			Email: fmt.Sprintf("%s@example.com", gid),
			Pic:   fmt.Sprintf("http://example.com/%s.jpg", gid),
		}
		// stuff it into the maps for later usage
		tokens[token] = data
		gids[gid] = data
		rocks[gid] = RocksAgent{
			Gid:      gid,
			TGId:     0,
			Agent:    fmt.Sprintf("Barcode-%s", gid[1:4]),
			Verified: true,
			Smurf:    false,
			Fullname: "Rando Inadiquo",
		}
		vees[gid] = vagent{
			EnlID:       fmt.Sprintf("enl-%s", data.Gid),
			Vlevel:      1,
			Vpoints:     1,
			Agent:       fmt.Sprintf("Barcode-%s", data.Gid[1:4]),
			Level:       16,
			Quarantine:  false,
			Active:      true,
			Blacklisted: false,
			Verified:    true,
			Flagged:     false,
			Banned:      false,
			Cellid:      "AMS-GOLF-06",
		}
	}

	j, _ := json.Marshal(data)
	fmt.Printf("returning %s for token %s", string(j), token)
	res.Header().Set("Content-Type", "application/json; charset=UTF-8")
	fmt.Fprintf(res, string(j))
}

func vSearch(res http.ResponseWriter, req *http.Request) {
	// request is in this form: "%s/agent/%s/trust?apikey=%s"
	apikey := req.FormValue("apikey")
	fmt.Printf("V search request, apikey: %s ", apikey)

	chunks := strings.Split(req.URL.Path, "/")
	// fmt.Print(chunks)
	gid := chunks[1]
	fmt.Printf(" query: %s\n", gid)

	if gid == "" {
		fmt.Fprintf(res, "gid not set")
		return
	}
	var v Vresult
	vd, ok := vees[gid]
	if !ok {
		v = Vresult{
			Status:  "error",
			Message: "Agent not found",
		}
	} else {
		v = Vresult{
			Status: "ok",
		}
		v.Data = vd
	}
	j, _ := json.Marshal(v)
	res.Header().Set("Content-Type", "application/json; charset=UTF-8")
	fmt.Fprintf(res, string(j))
}

func rocksSearch(res http.ResponseWriter, req *http.Request) {
	// /api/user/status/{GID}?apikey=
	apikey := req.FormValue("apikey")
	fmt.Printf("Rocks search request, apikey: %s ", apikey)

	chunks := strings.Split(req.URL.Path, "/")
	// fmt.Print(chunks)
	gid := chunks[len(chunks)-1] // last element
	fmt.Printf(" query: %s\n", gid)

	if gid == "" {
		fmt.Fprintf(res, "gid not set")
		return
	}
	r, ok := rocks[gid]
	if !ok {
		// XXX get the error that Rocks actually uses
		res.Header().Set("Content-Type", "application/json; charset=UTF-8")
		fmt.Fprint(res, `{ status: "error" }`)
		return
	}
	j, _ := json.Marshal(r)
	res.Header().Set("Content-Type", "application/json; charset=UTF-8")
	fmt.Fprintf(res, string(j))
}

func preload() {
	// deviousness
	gids["118281765050946915735"] = GoogleData{
		Gid:   "118281765050946915735",
		Name:  "Scot Bontrager",
		Email: "scot@example.com",
		Pic:   "http://example.com/scot.jpg",
	}
	rocks["118281765050946915735"] = RocksAgent{
		Gid:      "118281765050946915735",
		TGId:     240908008,
		Agent:    "deviousness",
		Verified: true,
		Smurf:    false,
		Fullname: "Scot Bontrager",
	}
	vees["118281765050946915735"] = vagent{
		EnlID:       "23e27f48a04e55d6ae89188d3236d769f6629718",
		Vlevel:      3,
		Vpoints:     100,
		Agent:       "deviousness",
		Level:       16,
		Quarantine:  false,
		Active:      true,
		Blacklisted: false,
		Verified:    true,
		Flagged:     false,
		Banned:      false,
		Cellid:      "AMS-GOLF-06",
	}
	// bogus banned account
	gids["1111"] = GoogleData{
		Gid:   "1111",
		Name:  "Bogus Banned",
		Email: "banned@example.com",
		Pic:   "http://example.com/banned.jpg",
	}
	rocks["1111"] = RocksAgent{
		Gid:      "1111",
		TGId:     0,
		Agent:    "bannedbarcode",
		Verified: false,
		Smurf:    true,
		Fullname: "Bogus Banned",
	}
	vees["1111"] = vagent{
		EnlID:       "1111",
		Vlevel:      0,
		Vpoints:     0,
		Agent:       "bogusbanned",
		Level:       2,
		Quarantine:  false,
		Active:      false,
		Blacklisted: true,
		Verified:    false,
		Flagged:     false,
		Banned:      true,
		Cellid:      "AMS-GOLF-06",
	}
}
