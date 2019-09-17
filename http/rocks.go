package wasabeehttps

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"github.com/wasabee-project/Wasabee-Server"
)

func rocksCommunityRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Set("Content-Type", jsonType)

	contentType := strings.Split(strings.Replace(strings.ToLower(req.Header.Get("Content-Type")), " ", "", -1), ";")[0]
	if contentType != jsonTypeShort {
		http.Error(res, "Invalid request (needs to be application/json)", http.StatusNotAcceptable)
		return
	}

	// defer req.Body.Close()
	jBlob, err := ioutil.ReadAll(req.Body)
	if err != nil {
		wasabee.Log.Notice(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	if string(jBlob) == "" {
		wasabee.Log.Notice("empty JSON")
		http.Error(res, `{ "status": "error", "error": "Empty JSON" }`, http.StatusNotAcceptable)
		return
	}

	jRaw := json.RawMessage(jBlob)

	wasabee.Log.Debug(string(jBlob))
	err = wasabee.RocksCommunitySync(jRaw)
	if err != nil {
		wasabee.Log.Notice(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	fmt.Fprintf(res, `{ "status": "ok" }`)
}

func rocksPullTeamRoute(res http.ResponseWriter, req *http.Request) {
	gid, err := getAgentID(req)
	if err != nil {
		wasabee.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	team := wasabee.TeamID(vars["team"])

	safe, err := gid.OwnsTeam(team)
	if err != nil {
		wasabee.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	if !safe {
		http.Error(res, "unauthorized", http.StatusUnauthorized)
		return
	}

	err = team.RocksCommunityMemberPull()
	if err != nil {
		wasabee.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	url := fmt.Sprintf("%s/team/%s/edit", apipath, team.String())
	http.Redirect(res, req, url, http.StatusSeeOther)
}

func rocksCfgTeamRoute(res http.ResponseWriter, req *http.Request) {
	gid, err := getAgentID(req)
	if err != nil {
		wasabee.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	team := wasabee.TeamID(vars["team"])
	rc := vars["rockscomm"]
	rk := vars["rockskey"]

	safe, err := gid.OwnsTeam(team)
	if err != nil {
		wasabee.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	if !safe {
		http.Error(res, "unauthorized", http.StatusUnauthorized)
		return
	}
	err = team.SetRocks(rk, rc)
	if err != nil {
		wasabee.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	url := fmt.Sprintf("%s/team/%s/edit", apipath, team.String())
	http.Redirect(res, req, url, http.StatusSeeOther)
}
