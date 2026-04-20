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

func linkRequires(res http.ResponseWriter, req *http.Request) (model.GoogleID, *model.Link, *model.Operation, error) {
	ctx := req.Context()
	op := model.Operation{}

	gid, err := getAgentID(req)
	if err != nil {
		http.Error(res, jsonError(err), http.StatusForbidden)
		return gid, &model.Link{}, &op, err
	}

	op.ID = model.OperationID(req.PathValue("opID"))
	if err = op.Populate(ctx, gid); err != nil {
		if err.Error() == model.ErrOpNotFound {
			http.Error(res, jsonError(err), http.StatusNotFound)
		} else {
			http.Error(res, jsonError(err), http.StatusNotAcceptable)
		}
		return gid, &model.Link{}, &op, err
	}

	// Double check for deleted status
	if op.ID.IsDeletedOp(ctx) {
		err := fmt.Errorf("requested deleted op")
		log.Warnw(err.Error(), "GID", gid, "resource", op.ID)
		http.Error(res, jsonError(err), http.StatusGone)
		return gid, &model.Link{}, &op, err
	}

	linkID := model.LinkID(req.PathValue("link"))
	link, err := op.GetLink(linkID)
	if err != nil {
		if err.Error() == model.ErrLinkNotFound {
			http.Error(res, jsonError(err), http.StatusNotFound)
		} else {
			http.Error(res, jsonError(err), http.StatusNotAcceptable)
		}
		return gid, link, &op, err
	}
	return gid, link, &op, nil
}

func drawLinkAssignRoute(res http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	gid, link, op, err := linkRequires(res, req)
	if err != nil {
		return
	}

	if !op.WriteAccess(ctx, gid) {
		err = fmt.Errorf("forbidden: write access required to assign agents")
		log.Warnw(err.Error(), "GID", gid, "resource", op.ID)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}

	agent := model.GoogleID(req.FormValue("agent"))
	if err = link.SetAssignments(ctx, []model.GoogleID{agent}, nil); err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	uid := linkAssignTouch(ctx, gid, link.ID, op)
	fmt.Fprint(res, jsonOKUpdateID(uid))
}

func drawLinkDescRoute(res http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	gid, link, op, err := linkRequires(res, req)
	if err != nil {
		return
	}

	if !op.WriteAccess(ctx, gid) {
		err = fmt.Errorf("write access required to set link descriptions")
		log.Warnw(err.Error(), "GID", gid, "resource", op.ID)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}

	desc := req.FormValue("desc")
	if err = link.SetComment(ctx, desc); err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	uid := linkStatusTouch(ctx, op, link.ID, "comment")
	fmt.Fprint(res, jsonOKUpdateID(uid))
}

func drawLinkColorRoute(res http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	gid, link, op, err := linkRequires(res, req)
	if err != nil {
		return
	}

	if !op.WriteAccess(ctx, gid) {
		err = fmt.Errorf("forbidden: write access required to set link color")
		log.Warnw(err.Error(), "GID", gid, "resource", op.ID)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}

	color := req.FormValue("color")
	if err = link.SetColor(ctx, color); err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	uid := linkStatusTouch(ctx, op, link.ID, "color")
	fmt.Fprint(res, jsonOKUpdateID(uid))
}

func drawLinkSwapRoute(res http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	gid, link, op, err := linkRequires(res, req)
	if err != nil {
		return
	}

	if !op.WriteAccess(ctx, gid) {
		err = fmt.Errorf("forbidden: write access required to swap link order")
		log.Warnw(err.Error(), "GID", gid, "resource", op.ID)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}

	if err = link.Swap(ctx); err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	uid := linkStatusTouch(ctx, op, link.ID, "swap")
	fmt.Fprint(res, jsonOKUpdateID(uid))
}

func drawLinkZoneRoute(res http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	gid, link, op, err := linkRequires(res, req)
	if err != nil {
		return
	}

	if !op.WriteAccess(ctx, gid) {
		err = fmt.Errorf("forbidden: write access required to set zone")
		log.Warnw(err.Error(), "GID", gid, "resource", op.ID)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}

	zone := model.ZoneFromString(req.FormValue("zone"))
	if err = link.SetZone(ctx, zone); err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	uid := linkStatusTouch(ctx, op, link.ID, "zone")
	fmt.Fprint(res, jsonOKUpdateID(uid))
}

func drawLinkDeltaRoute(res http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	gid, link, op, err := linkRequires(res, req)
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

	if err = link.SetDelta(ctx, int(delta)); err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	uid := linkStatusTouch(ctx, op, link.ID, "delta")
	fmt.Fprint(res, jsonOKUpdateID(uid))
}

func drawLinkCompleteRoute(res http.ResponseWriter, req *http.Request) {
	drawLinkCompRoute(res, req, true)
}

func drawLinkIncompleteRoute(res http.ResponseWriter, req *http.Request) {
	drawLinkCompRoute(res, req, false)
}

func drawLinkCompRoute(res http.ResponseWriter, req *http.Request, complete bool) {
	ctx := req.Context()
	gid, link, op, err := linkRequires(res, req)
	if err != nil {
		return
	}

	if !op.WriteAccess(ctx, gid) && !link.IsAssignedTo(req.Context(), gid) {
		err = fmt.Errorf("permission to mark link as complete denied")
		log.Warnw(err.Error(), "GID", gid, "resource", op.ID)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}

	if complete {
		if err = link.Complete(ctx); err != nil {
			log.Error(err)
			http.Error(res, jsonError(err), http.StatusInternalServerError)
			return
		}
	} else {
		if err = link.Incomplete(ctx); err != nil {
			log.Error(err)
			http.Error(res, jsonError(err), http.StatusInternalServerError)
			return
		}
	}

	uid := linkStatusTouch(ctx, op, link.ID, "complete")
	fmt.Fprint(res, jsonOKUpdateID(uid))
}

func drawLinkClaimRoute(res http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	gid, link, op, err := linkRequires(res, req)
	if err != nil {
		return
	}

	if r, _ := op.ReadAccess(ctx, gid); !r {
		err = fmt.Errorf("permission to claim link assignment denied")
		log.Warnw(err.Error(), "GID", gid, "resource", op.ID)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}

	if err = link.Claim(ctx, gid); err != nil {
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	uid := linkStatusTouch(ctx, op, link.ID, "assigned")
	fmt.Fprint(res, jsonOKUpdateID(uid))
}

func drawLinkRejectRoute(res http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	gid, link, op, err := linkRequires(res, req)
	if err != nil {
		return
	}

	if !link.IsAssignedTo(req.Context(), gid) {
		err = fmt.Errorf("permission to reject link assignment denied")
		log.Warnw(err.Error(), "GID", gid, "resource", op.ID)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}

	if err := link.Reject(ctx, gid); err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	uid := linkStatusTouch(ctx, op, link.ID, "pending")
	fmt.Fprint(res, jsonOKUpdateID(uid))
}

func drawLinkFetch(res http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	gid, link, op, err := linkRequires(res, req)
	if err != nil {
		return
	}

	if r, _ := op.ReadAccess(ctx, gid); !r && !op.AssignedOnlyAccess(ctx, gid) {
		err := fmt.Errorf("forbidden")
		log.Warnw(err.Error(), "GID", gid, "resource", op.ID)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}
	json.NewEncoder(res).Encode(link)
}

func linkAssignTouch(ctx context.Context, gid model.GoogleID, linkID model.LinkID, op *model.Operation) string {
	uid, err := op.Touch(ctx)
	if err != nil {
		log.Error(err)
	}

	_ = wfb.AssignLink(context.Background(), gid, model.TaskID(linkID), op.ID, uid)
	return uid
}

func linkStatusTouch(ctx context.Context, op *model.Operation, linkID model.LinkID, status string) string {
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
			_ = wfb.LinkStatus(bgCtx, model.TaskID(linkID), op.ID, ta, status, uid)
		}
	}()
	return uid
}
