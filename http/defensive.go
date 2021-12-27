package wasabeehttps

import (
	"encoding/json"
	"fmt"
	"net/http"
	// "strconv"
	"io/ioutil"

	"github.com/wasabee-project/Wasabee-Server/log"
	"github.com/wasabee-project/Wasabee-Server/model"
)

func getDefensiveKeys(res http.ResponseWriter, req *http.Request) {
	res.Header().Add("Content-Type", jsonType)
	gid, err := getAgentID(req)
	if err != nil {
		log.Warn(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	dkl, err := gid.ListDefensiveKeys()
	if err != nil {
		log.Warn(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	data, _ := json.Marshal(dkl)
	fmt.Fprint(res, string(data))
}

func setDefensiveKey(res http.ResponseWriter, req *http.Request) {
	var dk model.DefensiveKey

	res.Header().Add("Content-Type", jsonType)
	gid, err := getAgentID(req)
	if err != nil {
		log.Warn(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	if !contentTypeIs(req, jsonTypeShort) {
		err := fmt.Errorf("wasabee-IITC plugin version is too old, please update")
		log.Warnw(err.Error(), "GID", gid)
		http.Error(res, jsonError(err), http.StatusNotAcceptable)
		return
	}

	jBlob, err := ioutil.ReadAll(req.Body)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	if string(jBlob) == "" {
		err := fmt.Errorf("empty JSON for setDefensiveKey")
		log.Warnw(err.Error(), "GID", gid)
		http.Error(res, jsonStatusEmpty, http.StatusNotAcceptable)
		return
	}

	jRaw := json.RawMessage(jBlob)
	if err = json.Unmarshal(jRaw, &dk); err != nil {
		log.Errorw(err.Error(), "GID", gid, "content", jRaw)
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
	var dkl []model.DefensiveKey

	res.Header().Add("Content-Type", jsonType)
	gid, err := getAgentID(req)
	if err != nil {
		log.Warn(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	if !contentTypeIs(req, jsonTypeShort) {
		err := fmt.Errorf("JSON required")
		log.Warnw(err.Error(), "GID", gid)
		http.Error(res, jsonError(err), http.StatusNotAcceptable)
		return
	}

	jBlob, err := ioutil.ReadAll(req.Body)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	if string(jBlob) == "" {
		err := fmt.Errorf("empty JSON for setDefensiveKeyBulk")
		log.Warnw(err.Error(), "GID", gid)
		http.Error(res, jsonStatusEmpty, http.StatusNotAcceptable)
		return
	}

	jRaw := json.RawMessage(jBlob)
	if err = json.Unmarshal(jRaw, &dkl); err != nil {
		log.Errorw(err.Error(), "GID", gid, "content", jRaw)
		return
	}

	for _, dk := range dkl {
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
	}
	fmt.Fprint(res, jsonStatusOK)
}
