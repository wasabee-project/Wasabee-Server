package wasabeehttps

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/wasabee-project/Wasabee-Server/Firebase"
	"github.com/wasabee-project/Wasabee-Server/log"
	"github.com/wasabee-project/Wasabee-Server/model"
)

func markerRequires(res http.ResponseWriter, req *http.Request) (model.GoogleID, *model.Marker, *model.Operation, error) {
	ctx := req.Context()
	op := model.Operation{}

	gid, err := getAgentID(req)
	if err != nil {
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return gid, &model.Marker{}, &op, err
	}

	op.ID = model.OperationID(req.PathValue("opID"))
	if err = op.Populate(ctx, gid); err != nil {
		if op.ID.IsDeletedOp(ctx) {
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

	markerID := model.MarkerID(req.PathValue("marker"))
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
	ctx := req.Context()
	gid, marker, op, err := markerRequires(res, req)
	if err != nil {
		return
	}

	if !op.WriteAccess(ctx, gid) {
		err = fmt.Errorf("write access required to assign targets")
		log.Warnw(err.Error(), "GID", gid, "resource", op.ID)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}

	agent := model.GoogleID(req.FormValue("agent"))
	if err = marker.SetAssignments(ctx, []model.GoogleID{agent}, nil); err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	uid := markerAssignTouch(ctx, gid, marker.ID, op)
	fmt.Fprint(res, jsonOKUpdateID(uid))
}

func drawMarkerClaimRoute(res http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	gid, marker, op, err := markerRequires(res, req)
	if err != nil {
		return
	}

	if read, _ := op.ReadAccess(ctx, gid); !read {
		err = fmt.Errorf("read access required to claim targets")
		log.Warnw(err.Error(), "GID", gid, "resource", op.ID)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}

	if err = marker.Claim(ctx, gid); err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	uid := markerStatusTouch(ctx, op, marker.ID, "claimed")
	fmt.Fprint(res, jsonOKUpdateID(uid))
}

func drawMarkerCommentRoute(res http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	gid, marker, op, err := markerRequires(res, req)
	if err != nil {
		return
	}

	if !op.WriteAccess(ctx, gid) {
		err = fmt.Errorf("write access required to set marker comments")
		log.Warnw(err.Error(), "GID", gid, "resource", op.ID)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}

	comment := req.FormValue("comment")
	if err = marker.SetComment(ctx, comment); err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	uid := markerStatusTouch(ctx, op, marker.ID, "comment")
	fmt.Fprint(res, jsonOKUpdateID(uid))
}

func drawMarkerZoneRoute(res http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	gid, marker, op, err := markerRequires(res, req)
	if err != nil {
		return
	}

	if !op.WriteAccess(ctx, gid) {
		err = fmt.Errorf("write access required to set marker zone")
		log.Warnw(err.Error(), "GID", gid, "resource", op.ID)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}

	zone := model.ZoneFromString(req.FormValue("zone"))
	if err := marker.SetZone(ctx, zone); err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	uid := markerStatusTouch(ctx, op, marker.ID, "zone")
	fmt.Fprint(res, jsonOKUpdateID(uid))
}

func drawMarkerDeltaRoute(res http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	gid, marker, op, err := markerRequires(res, req)
	if err != nil {
		return
	}

	if !op.WriteAccess(ctx, gid) {
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

	if err = marker.SetDelta(ctx, int(delta)); err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	uid := markerStatusTouch(ctx, op, marker.ID, "delta")
	fmt.Fprint(res, jsonOKUpdateID(uid))
}

func drawMarkerFetch(res http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	gid, marker, op, err := markerRequires(res, req)
	if err != nil {
		return
	}

	read, _ := op.ReadAccess(ctx, gid)
	if !read && !op.AssignedOnlyAccess(ctx, gid) {
		if op.ID.IsDeletedOp(ctx) {
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
	json.NewEncoder(res).Encode(marker)
}

func drawMarkerCompleteRoute(res http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	gid, marker, op, err := markerRequires(res, req)
	if err != nil {
		return
	}

	if read, _ := op.ReadAccess(ctx, gid); !read && !op.AssignedOnlyAccess(ctx, gid) {
		err = fmt.Errorf("access required to claim targets")
		log.Warnw(err.Error(), "GID", gid, "resource", op.ID)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}

	if err := marker.Complete(ctx); err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	uid := markerStatusTouch(ctx, op, marker.ID, "completed")
	fmt.Fprint(res, jsonOKUpdateID(uid))
}

func drawMarkerIncompleteRoute(res http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	gid, marker, op, err := markerRequires(res, req)
	if err != nil {
		return
	}

	if read, _ := op.ReadAccess(ctx, gid); !read && !op.AssignedOnlyAccess(ctx, gid) {
		err = fmt.Errorf("access required to claim targets")
		log.Warnw(err.Error(), "GID", gid, "resource", op.ID)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}

	if err = marker.Incomplete(ctx); err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	uid := markerStatusTouch(ctx, op, marker.ID, "incomplete")
	fmt.Fprint(res, jsonOKUpdateID(uid))
}

func drawMarkerRejectRoute(res http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	gid, marker, op, err := markerRequires(res, req)
	if err != nil {
		return
	}

	if read, _ := op.ReadAccess(ctx, gid); !read && !op.AssignedOnlyAccess(ctx, gid) {
		err = fmt.Errorf("access required to claim targets")
		log.Warnw(err.Error(), "GID", gid, "resource", op.ID)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}

	if err = marker.Reject(ctx, gid); err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	uid := markerStatusTouch(ctx, op, marker.ID, "reject")
	fmt.Fprint(res, jsonOKUpdateID(uid))
}

func drawMarkerAcknowledgeRoute(res http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	gid, marker, op, err := markerRequires(res, req)
	if err != nil {
		return
	}

	if read, _ := op.ReadAccess(ctx, gid); !read && !op.AssignedOnlyAccess(ctx, gid) {
		err = fmt.Errorf("access required to claim targets")
		log.Warnw(err.Error(), "GID", gid, "resource", op.ID)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}

	if err = marker.Acknowledge(ctx); err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	uid := markerStatusTouch(ctx, op, marker.ID, "acknowledge")
	fmt.Fprint(res, jsonOKUpdateID(uid))
}

func markerAssignTouch(ctx context.Context, gid model.GoogleID, markerID model.MarkerID, op *model.Operation) string {
	uid, err := op.Touch(ctx)
	if err != nil {
		log.Error(err)
	}

	_ = wfb.AssignMarker(context.Background(), gid, model.TaskID(markerID), op.ID, uid)
	return uid
}

func markerStatusTouch(ctx context.Context, op *model.Operation, markerID model.MarkerID, status string) string {
	uid, err := op.Touch(ctx)
	if err != nil {
		return ""
	}

	go func() {
		bgCtx := context.Background()
		teams := make(map[model.TeamID]bool)
		for _, t := range op.Teams {
			teams[t.TeamID] = true
		}
		var ta []model.TeamID
		for t := range teams {
			ta = append(ta, t)
		}
		if len(ta) > 0 {
			_ = wfb.MarkerStatus(bgCtx, model.TaskID(markerID), op.ID, ta, status, uid)
		}
	}()
	return uid
}
