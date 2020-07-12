package wasabeehttps

import (
	"encoding/json"
	"fmt"
	"github.com/wasabee-project/Wasabee-Server"
	"net/http"
	"strconv"
)

func getDefensiveKeys(res http.ResponseWriter, req *http.Request) {
	res.Header().Add("Content-Type", jsonType)
	gid, err := getAgentID(req)
	if err != nil {
		wasabee.Log.Notice(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	dkl, err := gid.ListDefensiveKeys()
	if err != nil {
		wasabee.Log.Notice(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	data, _ := json.Marshal(dkl)
	fmt.Fprint(res, string(data))
}

func setDefensiveKey(res http.ResponseWriter, req *http.Request) {
	res.Header().Add("Content-Type", jsonType)
	gid, err := getAgentID(req)
	if err != nil {
		wasabee.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	portalID := wasabee.PortalID(req.FormValue("portalID"))
	capID := req.FormValue("capID")
	count, err := strconv.ParseInt(req.FormValue("count"), 10, 32)
	if err != nil {
		wasabee.Log.Notice(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	err = gid.InsertDefensiveKey(portalID, capID, int32(count))
	if err != nil {
		wasabee.Log.Notice(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	fmt.Fprint(res, jsonStatusOK)
}
