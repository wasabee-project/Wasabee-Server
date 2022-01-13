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

func markerRequires(res http.ResponseWriter, req *http.Request) (model.GoogleID, *model.Marker, *model.Operation, error) {
	op := model.Operation{}

	gid, err := getAgentID(req)
	if err != nil {
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return gid, &model.Marker{}, &op, err
	}

	vars := mux.Vars(req)
	op.ID = model.OperationID(vars["opID"])
	if err = op.Populate(gid); err != nil {
		if op.ID.IsDeletedOp() {
			err := fmt.Errorf("requested deleted op")
			log.Warnw(err.Error(), "GID", gid, "resource", op.ID)
			http.Error(res, jsonError(err), http.StatusGone)
			return gid, &model.Marker{}, &op, err
		}
		if err.Error() == model.ErrOpNotFound {
			http.Error(res, jsonError(err), http.StatusNotFound)
		} else {
			http.Error(res, jsonError(err), http.StatusNotAcceptable)
		}
		return gid, &model.Marker{}, &op, err
	}

	markerID := model.MarkerID(vars["marker"])
	if err = op.Populate(gid); err != nil {
		http.Error(res, jsonError(err), http.StatusNotAcceptable)
		return gid, &model.Marker{}, &op, err
	}

	marker, err := op.GetMarker(markerID)
	if err != nil {
		if err.Error() == model.ErrMarkerNotFound {
			http.Error(res, jsonError(err), http.StatusNotFound)
		} else {
			http.Error(res, jsonError(err), http.StatusNotAcceptable)
		}
		return gid, marker, &op, err
	}
	return gid, marker, &op, nil
}

func drawMarkerAssignRoute(res http.ResponseWriter, req *http.Request) {
	gid, marker, op, err := markerRequires(res, req)
	if err != nil {
		return
	}

	if !op.WriteAccess(gid) {
		err = fmt.Errorf("write access required to assign targets")
		log.Warnw(err.Error(), "GID", gid, "resource", op.ID)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}

	agent := model.GoogleID(req.FormValue("agent"))
	if err = marker.SetAssignments([]model.GoogleID{agent}, nil); err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	uid := markerAssignTouch(gid, marker.ID, op)
	fmt.Fprint(res, jsonOKUpdateID(uid))
}

func drawMarkerClaimRoute(res http.ResponseWriter, req *http.Request) {
	gid, marker, op, err := markerRequires(res, req)
	if err != nil {
		return
	}

	// markerRequires does Populate, which checks for read access ... this is redundant
	if read, _ := op.ReadAccess(gid); !read {
		err = fmt.Errorf("read access required to claim targets")
		log.Warnw(err.Error(), "GID", gid, "resource", op.ID)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}

	if err = marker.Claim(gid); err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	uid := markerStatusTouch(op, marker.ID, "claimed")
	fmt.Fprint(res, jsonOKUpdateID(uid))
}

func drawMarkerCommentRoute(res http.ResponseWriter, req *http.Request) {
	gid, marker, op, err := markerRequires(res, req)
	if err != nil {
		return
	}

	if !op.WriteAccess(gid) {
		err = fmt.Errorf("write access required to set marker comments")
		log.Warnw(err.Error(), "GID", gid, "resource", op.ID)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}

	comment := req.FormValue("comment")
	if err = marker.SetComment(comment); err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	uid := markerStatusTouch(op, marker.ID, "comment")
	fmt.Fprint(res, jsonOKUpdateID(uid))
}

func drawMarkerZoneRoute(res http.ResponseWriter, req *http.Request) {
	gid, marker, op, err := markerRequires(res, req)
	if err != nil {
		return
	}

	if !op.WriteAccess(gid) {
		err = fmt.Errorf("write access required to set marker zone")
		log.Warnw(err.Error(), "GID", gid, "resource", op.ID)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}

	zone := model.ZoneFromString(req.FormValue("zone"))
	if err := marker.SetZone(zone); err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	uid := markerStatusTouch(op, marker.ID, "zone")
	fmt.Fprint(res, jsonOKUpdateID(uid))
}

func drawMarkerDeltaRoute(res http.ResponseWriter, req *http.Request) {
	gid, marker, op, err := markerRequires(res, req)
	if err != nil {
		return
	}

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

	if err = marker.SetDelta(int(delta)); err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	uid := markerStatusTouch(op, marker.ID, "delta")
	fmt.Fprint(res, jsonOKUpdateID(uid))
}

func drawMarkerFetch(res http.ResponseWriter, req *http.Request) {
	gid, marker, op, err := markerRequires(res, req)
	if err != nil {
		return
	}

	// markerRequires does Populate, which checks for read access ... this is redundant
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

	j, err := json.Marshal(marker)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	fmt.Fprint(res, string(j))
}

func drawMarkerCompleteRoute(res http.ResponseWriter, req *http.Request) {
	gid, marker, op, err := markerRequires(res, req)
	if err != nil {
		return
	}

	// markerRequires does Populate, which checks for read access ... this is redundant
	if read, _ := op.ReadAccess(gid); !read && !op.AssignedOnlyAccess(gid) {
		err = fmt.Errorf("access required to claim targets")
		log.Warnw(err.Error(), "GID", gid, "resource", op.ID)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}

	if err := marker.Complete(); err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	uid := markerStatusTouch(op, marker.ID, "completed")
	fmt.Fprint(res, jsonOKUpdateID(uid))
}

func drawMarkerIncompleteRoute(res http.ResponseWriter, req *http.Request) {
	gid, marker, op, err := markerRequires(res, req)
	if err != nil {
		return
	}

	// markerRequires does Populate, which checks for read access ... this is redundant
	if read, _ := op.ReadAccess(gid); !read && !op.AssignedOnlyAccess(gid) {
		err = fmt.Errorf("access required to claim targets")
		log.Warnw(err.Error(), "GID", gid, "resource", op.ID)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}

	if err = marker.Incomplete(); err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	uid := markerStatusTouch(op, marker.ID, "incomplete")
	fmt.Fprint(res, jsonOKUpdateID(uid))
}

func drawMarkerRejectRoute(res http.ResponseWriter, req *http.Request) {
	gid, marker, op, err := markerRequires(res, req)
	if err != nil {
		return
	}

	// markerRequires does Populate, which checks for read access ... this is redundant
	if read, _ := op.ReadAccess(gid); !read && !op.AssignedOnlyAccess(gid) {
		err = fmt.Errorf("access required to claim targets")
		log.Warnw(err.Error(), "GID", gid, "resource", op.ID)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}

	if err = marker.Reject(gid); err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	uid := markerStatusTouch(op, marker.ID, "reject")
	fmt.Fprint(res, jsonOKUpdateID(uid))
}

func drawMarkerAcknowledgeRoute(res http.ResponseWriter, req *http.Request) {
	gid, marker, op, err := markerRequires(res, req)
	if err != nil {
		return
	}

	// markerRequires does Populate, which checks for read access ... this is redundant
	if read, _ := op.ReadAccess(gid); !read && !op.AssignedOnlyAccess(gid) {
		err = fmt.Errorf("access required to claim targets")
		log.Warnw(err.Error(), "GID", gid, "resource", op.ID)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}

	if err = marker.Acknowledge(); err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	uid := markerStatusTouch(op, marker.ID, "acknowledge")
	fmt.Fprint(res, jsonOKUpdateID(uid))
}

// markerAssignTouch updates the updateID and notifies ONLY the agent to whom the assigment was made
func markerAssignTouch(gid model.GoogleID, markerID model.MarkerID, op *model.Operation) string {
	uid, err := op.Touch()
	if err != nil {
		log.Error(err)
	}

	if err := wfb.AssignMarker(gid, model.TaskID(markerID), op.ID, uid); err != nil {
		log.Error(err)
	}
	return uid
}

// linkStatusTouch updates the updateID and notifies all teams of the update
func markerStatusTouch(op *model.Operation, markerID model.MarkerID, status string) string {
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
		teams, err = op.ID.Teams()
		if err != nil {
			log.Error(err)
			return uid
		}
	}

	for _, t := range teams {
		err := wfb.MarkerStatus(model.TaskID(markerID), op.ID, t, status, uid)
		if err != nil {
			log.Error(err)
		}
	}
	return uid
}
