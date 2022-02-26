package wasabeehttps

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/wasabee-project/Wasabee-Server/log"
	"github.com/wasabee-project/Wasabee-Server/model"
)

func getDefensiveKeys(res http.ResponseWriter, req *http.Request) {
	gid, err := getAgentID(req)
	if err != nil {
		http.Error(res, jsonError(err), http.StatusUnauthorized)
		return
	}

	dkl, err := gid.ListDefensiveKeys()
	if err != nil {
		log.Warn(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(res).Encode(dkl)
}

func setDefensiveKey(res http.ResponseWriter, req *http.Request) {
	var dk model.DefensiveKey

	gid, err := getAgentID(req)
	if err != nil {
		http.Error(res, err.Error(), http.StatusUnauthorized)
		return
	}

	if !contentTypeIs(req, jsonTypeShort) {
		err := fmt.Errorf("wasabee-IITC plugin version is too old, please update")
		log.Warnw(err.Error(), "GID", gid)
		http.Error(res, jsonError(err), http.StatusNotAcceptable)
		return
	}

	if err := json.NewDecoder(req.Body).Decode(&dk); err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	if dk.Name == "" {
		err := fmt.Errorf("empty portal name")
		log.Warnw(err.Error(), "GID", gid)
		http.Error(res, jsonStatusEmpty, http.StatusNotAcceptable)
		return
	}

	if dk.Lat == "" || dk.Lon == "" {
		err := fmt.Errorf("empty portal location")
		log.Warnw(err.Error(), "GID", gid)
		http.Error(res, jsonStatusEmpty, http.StatusNotAcceptable)
		return
	}

	err = gid.InsertDefensiveKey(dk)
	if err != nil {
		log.Warn(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	fmt.Fprint(res, jsonStatusOK)
}

func setDefensiveKeyBulk(res http.ResponseWriter, req *http.Request) {
	gid, err := getAgentID(req)
	if err != nil {
		http.Error(res, err.Error(), http.StatusUnauthorized)
		return
	}

	if !contentTypeIs(req, jsonTypeShort) {
		err := fmt.Errorf("JSON required")
		log.Warnw(err.Error(), "gid", gid)
		http.Error(res, jsonError(err), http.StatusNotAcceptable)
		return
	}

	var dkl []model.DefensiveKey
	if err := json.NewDecoder(req.Body).Decode(&dkl); err != nil {
		log.Errorw(err.Error(), "gid", gid)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	for _, dk := range dkl {
		if dk.Name == "" {
			err := fmt.Errorf("empty portal name")
			log.Warnw(err.Error(), "gid", gid)
			http.Error(res, jsonStatusEmpty, http.StatusNotAcceptable)
			return
		}

		if dk.Lat == "" || dk.Lon == "" {
			err := fmt.Errorf("empty portal location")
			log.Warnw(err.Error(), "gid", gid)
			http.Error(res, jsonStatusEmpty, http.StatusNotAcceptable)
			return
		}

		err = gid.InsertDefensiveKey(dk)
		if err != nil {
			http.Error(res, jsonError(err), http.StatusInternalServerError)
			return
		}
	}
	fmt.Fprint(res, jsonStatusOK)
}
