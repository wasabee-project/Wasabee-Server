package v

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/wasabee-project/Wasabee-Server/log"
	"github.com/wasabee-project/Wasabee-Server/model"
)

const jsonStatusOK = `{"status":"ok"}`

func vTeamRoute(res http.ResponseWriter, req *http.Request) {
	// the POST is empty, all we have is the teamID from the URL
	vars := mux.Vars(req)
	id := vars["teamID"]
	if id == "" {
		err := fmt.Errorf("V hook called with empty team ID")
		log.Error(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	log.Infow("V requested team sync", "server", req.RemoteAddr, "team", id)

	vteam, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		log.Error(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	teams, err := model.GetTeamsByVID(vteam)
	if err != nil {
		log.Error(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	keys := make(map[model.GoogleID]string)

	for _, teamID := range teams {
		gid, err := teamID.Owner()
		if err != nil {
			log.Error(err)
			continue
		}

		key := keys[gid]
		if key != "" {
			key, err = gid.GetVAPIkey()
			if err != nil {
				log.Error(err)
				continue
			}
			if key == "" {
				log.Errorw("no VAPI key for team owner, skipping sync", "GID", gid, "teamID", teamID, "vteam", vteam)
				continue
			}
			keys[gid] = key
		}

		if err := Sync(req.Context(), teamID, key); err != nil {
			log.Error(err)
			http.Error(res, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	fmt.Fprint(res, jsonStatusOK)
}
