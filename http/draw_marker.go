package wasabeehttps

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/wasabee-project/Wasabee-Server"
)

func drawMarkerAssignRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Set("Content-Type", jsonType)

	gid, err := getAgentID(req)
	if err != nil {
		wasabee.Log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	// only the ID needs to be set for this
	vars := mux.Vars(req)
	var op wasabee.Operation
	op.ID = wasabee.OperationID(vars["document"])

	if !op.WriteAccess(gid) {
		err = fmt.Errorf("write access required to assign targets")
		wasabee.Log.Warnw(err.Error(), "GID", gid, "resource", op.ID)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}

	marker := wasabee.MarkerID(vars["marker"])
	agent := wasabee.GoogleID(req.FormValue("agent"))
	if err = op.Populate(gid); err != nil {
		wasabee.Log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	uid, err := op.AssignMarker(marker, agent)
	if err != nil {
		wasabee.Log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	wasabee.Log.Infow("assigned marker", "GID", gid, "resource", op.ID, "marker", marker, "agent", agent, "message", "assigned marker")
	fmt.Fprint(res, jsonOKUpdateID(uid))
}

func drawMarkerClaimRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Set("Content-Type", jsonType)

	gid, err := getAgentID(req)
	if err != nil {
		wasabee.Log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	// only the ID needs to be set for this
	vars := mux.Vars(req)
	var op wasabee.Operation
	op.ID = wasabee.OperationID(vars["document"])

	marker := wasabee.MarkerID(vars["marker"])
	if err = op.Populate(gid); err != nil {
		wasabee.Log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	uid, err := op.ClaimMarker(marker, gid)
	if err != nil {
		wasabee.Log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	wasabee.Log.Infow("claimed marker", "GID", gid, "resource", op.ID, "marker", marker, "message", "claimed marker")
	fmt.Fprint(res, jsonOKUpdateID(uid))
}

func drawMarkerCommentRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Set("Content-Type", jsonType)

	gid, err := getAgentID(req)
	if err != nil {
		wasabee.Log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	// only the ID needs to be set for this
	vars := mux.Vars(req)
	var op wasabee.Operation
	op.ID = wasabee.OperationID(vars["document"])

	if !op.WriteAccess(gid) {
		err = fmt.Errorf("write access required to set marker comments")
		wasabee.Log.Warnw(err.Error(), "GID", gid, "resource", op.ID)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}

	marker := wasabee.MarkerID(vars["marker"])
	comment := req.FormValue("comment")
	uid, err := op.MarkerComment(marker, comment)
	if err != nil {
		wasabee.Log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	fmt.Fprint(res, jsonOKUpdateID(uid))
}

func drawMarkerZoneRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Set("Content-Type", jsonType)

	gid, err := getAgentID(req)
	if err != nil {
		wasabee.Log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	// only the ID needs to be set for this
	vars := mux.Vars(req)
	var op wasabee.Operation
	op.ID = wasabee.OperationID(vars["document"])

	if !op.WriteAccess(gid) {
		err = fmt.Errorf("write access required to set marker zone")
		wasabee.Log.Warnw(err.Error(), "GID", gid, "resource", op.ID)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}

	marker := wasabee.MarkerID(vars["marker"])
	zone := wasabee.ZoneFromString(req.FormValue("zone"))
	uid, err := marker.SetZone(&op, zone)
	if err != nil {
		wasabee.Log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	fmt.Fprint(res, jsonOKUpdateID(uid))
}

func drawMarkerDeltaRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Set("Content-Type", jsonType)

	gid, err := getAgentID(req)
	if err != nil {
		wasabee.Log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	// only the ID needs to be set for this
	vars := mux.Vars(req)
	var op wasabee.Operation
	op.ID = wasabee.OperationID(vars["document"])

	if !op.WriteAccess(gid) {
		err = fmt.Errorf("forbidden: write access required to set delta")
		wasabee.Log.Warnw(err.Error(), "GID", gid, "resource", op.ID)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}

	marker := wasabee.MarkerID(vars["marker"])
	delta, err := strconv.ParseInt(req.FormValue("delta"), 10, 32)
	if err != nil {
		wasabee.Log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	uid, err := marker.Delta(&op, int(delta))
	if err != nil {
		wasabee.Log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	fmt.Fprint(res, jsonOKUpdateID(uid))
}

func drawMarkerFetch(res http.ResponseWriter, req *http.Request) {
	res.Header().Set("Content-Type", jsonType)

	gid, err := getAgentID(req)
	if err != nil {
		wasabee.Log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	var op wasabee.Operation
	op.ID = wasabee.OperationID(vars["document"])

	// o.Populate determines all or assigned-only
	read, _ := op.ReadAccess(gid)
	if !read && !op.AssignedOnlyAccess(gid) {
		if op.ID.IsDeletedOp() {
			err := fmt.Errorf("requested deleted op")
			wasabee.Log.Warnw(err.Error(), "GID", gid, "resource", op.ID)
			http.Error(res, jsonError(err), http.StatusGone)
			return
		}

		err := fmt.Errorf("forbidden")
		wasabee.Log.Warnw(err.Error(), "GID", gid, "resource", op.ID)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}

	// populate the whole op, slow, but ensures we only get things we have access to see
	if err = op.Populate(gid); err != nil {
		wasabee.Log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	markerID := wasabee.MarkerID(vars["marker"])
	marker, err := op.GetMarker(markerID)
	if err != nil {
		wasabee.Log.Error(err)
		// not really a 404, but close enough, better than a 500 or 403
		http.Error(res, jsonError(err), http.StatusNotFound)
		return
	}
	j, err := json.Marshal(marker)
	if err != nil {
		wasabee.Log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	fmt.Fprint(res, string(j))
}

func drawMarkerCompleteRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Set("Content-Type", jsonType)
	gid, err := getAgentID(req)
	if err != nil {
		wasabee.Log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	var op wasabee.Operation
	op.ID = wasabee.OperationID(vars["document"])
	markerID := wasabee.MarkerID(vars["marker"])
	uid, err := markerID.Complete(&op, gid)
	if err != nil {
		wasabee.Log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	wasabee.Log.Infow("completed marker", "GID", gid, "resource", op.ID, "marker", markerID, "message", "completed marker")
	fmt.Fprint(res, jsonOKUpdateID(uid))
}

func drawMarkerIncompleteRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Set("Content-Type", jsonType)
	gid, err := getAgentID(req)
	if err != nil {
		wasabee.Log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	var op wasabee.Operation
	op.ID = wasabee.OperationID(vars["document"])
	markerID := wasabee.MarkerID(vars["marker"])
	uid, err := markerID.Incomplete(&op, gid)
	if err != nil {
		wasabee.Log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	wasabee.Log.Infow("incompleted marker", "GID", gid, "resource", op.ID, "marker", markerID, "message", "incompleted marker")
	fmt.Fprint(res, jsonOKUpdateID(uid))
}

func drawMarkerRejectRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Set("Content-Type", jsonType)
	gid, err := getAgentID(req)
	if err != nil {
		wasabee.Log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	var op wasabee.Operation
	op.ID = wasabee.OperationID(vars["document"])
	markerID := wasabee.MarkerID(vars["marker"])
	uid, err := markerID.Reject(&op, gid)
	if err != nil {
		wasabee.Log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	wasabee.Log.Infow("reject marker", "GID", gid, "resource", op.ID, "marker", markerID, "message", "rejected marker")
	fmt.Fprint(res, jsonOKUpdateID(uid))
}

func drawMarkerAcknowledgeRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Set("Content-Type", jsonType)
	gid, err := getAgentID(req)
	if err != nil {
		wasabee.Log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	var op wasabee.Operation
	op.ID = wasabee.OperationID(vars["document"])
	markerID := wasabee.MarkerID(vars["marker"])
	uid, err := markerID.Acknowledge(&op, gid)
	if err != nil {
		wasabee.Log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	wasabee.Log.Infow("acknowledged marker", "GID", gid, "resource", op.ID, "marker", markerID, "message", "acknowledged marker")
	fmt.Fprint(res, jsonOKUpdateID(uid))
}

func drawMarkerDependAddRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Set("Content-Type", jsonType)

	gid, err := getAgentID(req)
	if err != nil {
		wasabee.Log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	// only the ID needs to be set for this
	vars := mux.Vars(req)
	var op wasabee.Operation
	op.ID = wasabee.OperationID(vars["document"])

	if !op.WriteAccess(gid) {
		err = fmt.Errorf("forbidden: write access required to set dependency")
		wasabee.Log.Warnw(err.Error(), "GID", gid, "resource", op.ID)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}

	marker := wasabee.MarkerID(vars["marker"])
	if marker == "" {
		err = fmt.Errorf("empty marker ID on depend add")
		wasabee.Log.Warnw(err.Error(), "GID", gid, "resource", op.ID)
		http.Error(res, jsonError(err), http.StatusNotAcceptable)
		return
	}

	task := vars["task"]
	if task == "" {
		err = fmt.Errorf("empty task ID on depend add")
		wasabee.Log.Warnw(err.Error(), "GID", gid, "resource", op.ID)
		http.Error(res, jsonError(err), http.StatusNotAcceptable)
		return
	}

	uid, err := marker.AddDepend(&op, task)
	if err != nil {
		wasabee.Log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	fmt.Fprint(res, jsonOKUpdateID(uid))
}

func drawMarkerDependDelRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Set("Content-Type", jsonType)

	gid, err := getAgentID(req)
	if err != nil {
		wasabee.Log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	// only the ID needs to be set for this
	vars := mux.Vars(req)
	var op wasabee.Operation
	op.ID = wasabee.OperationID(vars["document"])

	if !op.WriteAccess(gid) {
		err = fmt.Errorf("forbidden: write access required to set dependency")
		wasabee.Log.Warnw(err.Error(), "GID", gid, "resource", op.ID)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}

	marker := wasabee.MarkerID(vars["marker"])
	if marker == "" {
		err = fmt.Errorf("empty marker ID on depend delete")
		wasabee.Log.Warnw(err.Error(), "GID", gid, "resource", op.ID)
		http.Error(res, jsonError(err), http.StatusNotAcceptable)
		return
	}

	task := vars["task"]
	if task == "" {
		err = fmt.Errorf("empty task on depend delete")
		wasabee.Log.Warnw(err.Error(), "GID", gid, "resource", op.ID)
		http.Error(res, jsonError(err), http.StatusNotAcceptable)
		return
	}


	uid, err := marker.DelDepend(&op, task)
	if err != nil {
		wasabee.Log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	fmt.Fprint(res, jsonOKUpdateID(uid))
}

