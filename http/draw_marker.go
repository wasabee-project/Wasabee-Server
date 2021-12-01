package wasabeehttps

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/wasabee-project/Wasabee-Server/Firebase"
	"github.com/wasabee-project/Wasabee-Server/log"
	"github.com/wasabee-project/Wasabee-Server/model"
)

func drawMarkerAssignRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Set("Content-Type", jsonType)

	gid, err := getAgentID(req)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	var op model.Operation
	op.ID = model.OperationID(vars["document"])

	if !op.WriteAccess(gid) {
		err = fmt.Errorf("write access required to assign targets")
		log.Warnw(err.Error(), "GID", gid, "resource", op.ID)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}

	if err = op.Populate(gid); err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	markerID := model.MarkerID(vars["marker"])
	if err = op.Populate(gid); err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	marker, err := op.GetMarker(markerID)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusNotFound)
		return
	}

	agent := model.GoogleID(req.FormValue("agent"))
	if err = marker.Assign(agent); err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	uid := markerAssignTouch(gid, markerID, op)
	// log.Infow("assigned marker", "GID", gid, "resource", op.ID, "marker", marker, "agent", agent, "message", "assigned marker")
	fmt.Fprint(res, jsonOKUpdateID(uid))
}

func drawMarkerClaimRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Set("Content-Type", jsonType)

	gid, err := getAgentID(req)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	var op model.Operation
	op.ID = model.OperationID(vars["document"])

	if read, _ := op.ReadAccess(gid); !read {
		err = fmt.Errorf("read access required to claim targets")
		log.Warnw(err.Error(), "GID", gid, "resource", op.ID)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}

	if err = op.Populate(gid); err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	markerID := model.MarkerID(vars["marker"])
	marker, err := op.GetMarker(markerID)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusNotFound)
		return
	}

	if err = marker.Claim(gid); err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	// log.Infow("claimed marker", "GID", gid, "resource", op.ID, "marker", marker, "message", "claimed marker")
	uid := markerStatusTouch(op, markerID)
	fmt.Fprint(res, jsonOKUpdateID(uid))
}

func drawMarkerCommentRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Set("Content-Type", jsonType)

	gid, err := getAgentID(req)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	var op model.Operation
	op.ID = model.OperationID(vars["document"])

	if !op.WriteAccess(gid) {
		err = fmt.Errorf("write access required to set marker comments")
		log.Warnw(err.Error(), "GID", gid, "resource", op.ID)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}

	if err = op.Populate(gid); err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	markerID := model.MarkerID(vars["marker"])
	marker, err := op.GetMarker(markerID)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusNotFound)
		return
	}

	comment := req.FormValue("comment")
	if err = marker.SetComment(comment); err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	uid := markerStatusTouch(op, markerID)
	fmt.Fprint(res, jsonOKUpdateID(uid))
}

func drawMarkerZoneRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Set("Content-Type", jsonType)

	gid, err := getAgentID(req)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	var op model.Operation
	op.ID = model.OperationID(vars["document"])

	if !op.WriteAccess(gid) {
		err = fmt.Errorf("write access required to set marker zone")
		log.Warnw(err.Error(), "GID", gid, "resource", op.ID)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}

	if err = op.Populate(gid); err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	markerID := model.MarkerID(vars["marker"])
	marker, err := op.GetMarker(markerID)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusNotFound)
		return
	}

	zone := model.ZoneFromString(req.FormValue("zone"))
	if err := marker.SetZone(zone); err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	uid := markerStatusTouch(op, markerID)
	fmt.Fprint(res, jsonOKUpdateID(uid))
}

func drawMarkerDeltaRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Set("Content-Type", jsonType)

	gid, err := getAgentID(req)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	var op model.Operation
	op.ID = model.OperationID(vars["document"])

	if !op.WriteAccess(gid) {
		err = fmt.Errorf("forbidden: write access required to set delta")
		log.Warnw(err.Error(), "GID", gid, "resource", op.ID)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}

	delta, err := strconv.ParseInt(req.FormValue("delta"), 10, 32)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	if err = op.Populate(gid); err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	markerID := model.MarkerID(vars["marker"])
	marker, err := op.GetMarker(markerID)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusNotFound)
		return
	}

	if err = marker.Delta(int(delta)); err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	uid := markerStatusTouch(op, markerID)
	fmt.Fprint(res, jsonOKUpdateID(uid))
}

func drawMarkerFetch(res http.ResponseWriter, req *http.Request) {
	res.Header().Set("Content-Type", jsonType)

	gid, err := getAgentID(req)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	var op model.Operation
	op.ID = model.OperationID(vars["document"])

	// o.Populate determines all or assigned-only
	read, _ := op.ReadAccess(gid)
	if !read && !op.AssignedOnlyAccess(gid) {
		if op.ID.IsDeletedOp() {
			err := fmt.Errorf("requested deleted op")
			log.Warnw(err.Error(), "GID", gid, "resource", op.ID)
			http.Error(res, jsonError(err), http.StatusGone)
			return
		}

		err := fmt.Errorf("forbidden")
		log.Warnw(err.Error(), "GID", gid, "resource", op.ID)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}

	if err = op.Populate(gid); err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	markerID := model.MarkerID(vars["marker"])
	marker, err := op.GetMarker(markerID)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusNotFound)
		return
	}
	j, err := json.Marshal(marker)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	fmt.Fprint(res, string(j))
}

func drawMarkerCompleteRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Set("Content-Type", jsonType)
	gid, err := getAgentID(req)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	var op model.Operation
	op.ID = model.OperationID(vars["document"])

	if read, _ := op.ReadAccess(gid); !read && !op.AssignedOnlyAccess(gid) {
		err = fmt.Errorf("access required to claim targets")
		log.Warnw(err.Error(), "GID", gid, "resource", op.ID)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}

	if err = op.Populate(gid); err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	markerID := model.MarkerID(vars["marker"])
	marker, err := op.GetMarker(markerID)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusNotFound)
		return
	}

	if err := marker.Complete(gid); err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	uid := markerStatusTouch(op, markerID)
	// log.Infow("completed marker", "GID", gid, "resource", op.ID, "marker", markerID, "message", "completed marker")
	fmt.Fprint(res, jsonOKUpdateID(uid))
}

func drawMarkerIncompleteRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Set("Content-Type", jsonType)
	gid, err := getAgentID(req)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	var op model.Operation
	op.ID = model.OperationID(vars["document"])

	if read, _ := op.ReadAccess(gid); !read && !op.AssignedOnlyAccess(gid) {
		err = fmt.Errorf("access required to claim targets")
		log.Warnw(err.Error(), "GID", gid, "resource", op.ID)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}

	if err = op.Populate(gid); err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	markerID := model.MarkerID(vars["marker"])
	marker, err := op.GetMarker(markerID)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	if err = marker.Incomplete(gid); err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	uid := markerStatusTouch(op, markerID)
	log.Infow("incompleted marker", "GID", gid, "resource", op.ID, "marker", markerID, "message", "incompleted marker")
	fmt.Fprint(res, jsonOKUpdateID(uid))
}

func drawMarkerRejectRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Set("Content-Type", jsonType)
	gid, err := getAgentID(req)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	var op model.Operation
	op.ID = model.OperationID(vars["document"])

	if read, _ := op.ReadAccess(gid); !read && !op.AssignedOnlyAccess(gid) {
		err = fmt.Errorf("access required to claim targets")
		log.Warnw(err.Error(), "GID", gid, "resource", op.ID)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}

	if err = op.Populate(gid); err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	markerID := model.MarkerID(vars["marker"])
	marker, err := op.GetMarker(markerID)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	if err = marker.Reject(gid); err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	uid := markerStatusTouch(op, markerID)
	log.Infow("reject marker", "GID", gid, "resource", op.ID, "marker", markerID, "message", "rejected marker")
	fmt.Fprint(res, jsonOKUpdateID(uid))
}

func drawMarkerAcknowledgeRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Set("Content-Type", jsonType)

	gid, err := getAgentID(req)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	var op model.Operation
	op.ID = model.OperationID(vars["document"])

	if read, _ := op.ReadAccess(gid); !read && !op.AssignedOnlyAccess(gid) {
		err = fmt.Errorf("access required to claim targets")
		log.Warnw(err.Error(), "GID", gid, "resource", op.ID)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}

	if err = op.Populate(gid); err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	markerID := model.MarkerID(vars["marker"])
	marker, err := op.GetMarker(markerID)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	if err = marker.Acknowledge(gid); err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	uid := markerStatusTouch(op, markerID)
	log.Infow("acknowledged marker", "GID", gid, "resource", op.ID, "marker", markerID, "message", "acknowledged marker")
	fmt.Fprint(res, jsonOKUpdateID(uid))
}

// markerAssignTouch updates the updateID and notifies ONLY the agent to whom the assigment was made
func markerAssignTouch(gid model.GoogleID, markerID model.MarkerID, op model.Operation) string {
	uid, err := op.Touch()
	if err != nil {
		log.Error(err)
	}

	wfb.AssignMarker(wfb.GoogleID(gid), wfb.TaskID(markerID), wfb.OperationID(op.ID), uid)
	return uid
}

// linkStatusTouch updates the updateID and notifies all teams of the update
func markerStatusTouch(op model.Operation, markerID model.MarkerID) string {
	// update the timestamp and updateID
	uid, err := op.Touch()
	if err != nil {
		log.Error(err)
		return ""
	}

	// announce to all relevant teams
	var teams []model.TeamID
	for _, t := range op.Teams {
		teams = append(teams, t.TeamID)
	}
	if len(teams) == 0 {
		// not populated?
		teams, err := op.ID.Teams()
		if err != nil {
			log.Error(err)
			return uid
		}
	}

	for _, t := range teams {
		err := wfb.LinkStatus(wfb.TaskID(markerID), wfb.OperationID(op.ID), wfb.TeamID(t), uid)
		if err != nil {
			log.Error(err)
		}
	}
	return uid
}
