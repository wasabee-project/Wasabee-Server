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

func linkRequires(res http.ResponseWriter, req *http.Request) (model.GoogleID, *model.Link, *model.Operation, error) {
	op := model.Operation{}
	res.Header().Set("Content-Type", jsonType)

	gid, err := getAgentID(req)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return gid, &model.Link{}, &op, err
	}

	vars := mux.Vars(req)
	op.ID = model.OperationID(vars["opID"])
	if err = op.Populate(gid); err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return gid, &model.Link{}, &op, err
	}

	if err = op.Populate(gid); err != nil {
		log.Error(err)
		if op.ID.IsDeletedOp() {
			err := fmt.Errorf("requested deleted op")
			log.Warnw(err.Error(), "GID", gid, "resource", op.ID)
			http.Error(res, jsonError(err), http.StatusGone)
			return gid, &model.Link{}, &op, err
		}
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return gid, &model.Link{}, &op, err
	}

	linkID := model.LinkID(vars["link"])
	link, err := op.GetLink(linkID)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusNotAcceptable)
		return gid, link, &op, err
	}
	return gid, link, &op, nil
}

func drawLinkAssignRoute(res http.ResponseWriter, req *http.Request) {
	gid, link, op, err := linkRequires(res, req)
	if err != nil {
		return
	}

	if !op.WriteAccess(gid) {
		err = fmt.Errorf("forbidden: write access required to assign agents")
		log.Warnw(err.Error(), "GID", gid, "resource", op.ID)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}

	agent := model.GoogleID(req.FormValue("agent"))
	link.Assignments = []model.GoogleID{agent}
	if err = link.Assign(nil); err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	uid := linkAssignTouch(gid, link.ID, op)
	fmt.Fprint(res, jsonOKUpdateID(uid))
}

func drawLinkDescRoute(res http.ResponseWriter, req *http.Request) {
	gid, link, op, err := linkRequires(res, req)
	if err != nil {
		return
	}

	if !op.WriteAccess(gid) {
		err = fmt.Errorf("write access required to set link descriptions")
		log.Warnw(err.Error(), "GID", gid, "resource", op.ID)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}

	desc := req.FormValue("desc")
	if err = link.SetComment(desc); err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	uid := linkStatusTouch(op, link.ID, "comment")
	fmt.Fprint(res, jsonOKUpdateID(uid))
}

func drawLinkColorRoute(res http.ResponseWriter, req *http.Request) {
	gid, link, op, err := linkRequires(res, req)
	if err != nil {
		return
	}

	if !op.WriteAccess(gid) {
		err = fmt.Errorf("forbidden: write access required to set link color")
		log.Warnw(err.Error(), "GID", gid, "resource", op.ID)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}

	color := req.FormValue("color")
	if err = link.SetColor(color); err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	uid := linkStatusTouch(op, link.ID, "color")
	fmt.Fprint(res, jsonOKUpdateID(uid))
}

func drawLinkSwapRoute(res http.ResponseWriter, req *http.Request) {
	gid, link, op, err := linkRequires(res, req)
	if err != nil {
		return
	}

	if !op.WriteAccess(gid) {
		err = fmt.Errorf("forbidden: write access required to swap link order")
		log.Warnw(err.Error(), "GID", gid, "resource", op.ID)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}

	if err = link.Swap(); err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	uid := linkStatusTouch(op, link.ID, "swap")
	fmt.Fprint(res, jsonOKUpdateID(uid))
}

func drawLinkZoneRoute(res http.ResponseWriter, req *http.Request) {
	gid, link, op, err := linkRequires(res, req)
	if err != nil {
		return
	}

	if !op.WriteAccess(gid) {
		err = fmt.Errorf("forbidden: write access required to set zone")
		log.Warnw(err.Error(), "GID", gid, "resource", op.ID)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}

	zone := model.ZoneFromString(req.FormValue("zone"))
	if err = link.SetZone(zone); err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	uid := linkStatusTouch(op, link.ID, "zone")
	fmt.Fprint(res, jsonOKUpdateID(uid))
}

func drawLinkDeltaRoute(res http.ResponseWriter, req *http.Request) {
	gid, link, op, err := linkRequires(res, req)
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

	if err = link.SetDelta(int(delta)); err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	uid := linkStatusTouch(op, link.ID, "delta")
	fmt.Fprint(res, jsonOKUpdateID(uid))
}

func drawLinkCompleteRoute(res http.ResponseWriter, req *http.Request) {
	drawLinkCompRoute(res, req, true)
}

func drawLinkIncompleteRoute(res http.ResponseWriter, req *http.Request) {
	drawLinkCompRoute(res, req, false)
}

func drawLinkCompRoute(res http.ResponseWriter, req *http.Request, complete bool) {
	gid, link, op, err := linkRequires(res, req)
	if err != nil {
		return
	}

	// write access OR asignee
	if !op.WriteAccess(gid) && !link.IsAssignedTo(gid) {
		err = fmt.Errorf("permission to mark link as complete denied")
		log.Warnw(err.Error(), "GID", gid, "resource", op.ID)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}

	if err = link.Complete(); err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	uid := linkStatusTouch(op, link.ID, "complete")
	fmt.Fprint(res, jsonOKUpdateID(uid))
}

func drawLinkClaimRoute(res http.ResponseWriter, req *http.Request) {
	gid, link, op, err := linkRequires(res, req)
	if err != nil {
		return
	}

	// linkRequires runs Populate, which checks ReadAccess... this is redundant
	if r, _ := op.ReadAccess(gid); !r {
		err = fmt.Errorf("permission to claim link assignment denied")
		log.Warnw(err.Error(), "GID", gid, "resource", op.ID)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}

	if err = link.Claim(gid); err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	uid := linkStatusTouch(op, link.ID, "assigned")
	fmt.Fprint(res, jsonOKUpdateID(uid))
}

func drawLinkRejectRoute(res http.ResponseWriter, req *http.Request) {
	gid, link, op, err := linkRequires(res, req)
	if err != nil {
		return
	}

	if !link.IsAssignedTo(gid) {
		err = fmt.Errorf("permission to reject link assignment denied")
		log.Warnw(err.Error(), "GID", gid, "resource", op.ID)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}

	if err := link.Reject(gid); err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	uid := linkStatusTouch(op, link.ID, "pending")
	fmt.Fprint(res, jsonOKUpdateID(uid))
}

func drawLinkFetch(res http.ResponseWriter, req *http.Request) {
	gid, link, op, err := linkRequires(res, req)
	if err != nil {
		return
	}

	// linkRequires runs Populate, which checks ReadAccess... this is redundant
	if r, _ := op.ReadAccess(gid); !r && !op.AssignedOnlyAccess(gid) {
		err := fmt.Errorf("forbidden")
		log.Warnw(err.Error(), "GID", gid, "resource", op.ID)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}

	j, err := json.Marshal(link)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	fmt.Fprint(res, string(j))
}

// linkAssignTouch updates the updateID and notifies ONLY the agent to whom the assigment was made
func linkAssignTouch(gid model.GoogleID, linkID model.LinkID, op *model.Operation) string {
	uid, err := op.Touch()
	if err != nil {
		log.Error(err)
	}

	if err := wfb.AssignLink(gid, model.TaskID(linkID), op.ID, uid); err != nil {
		log.Error(err)
	}
	return uid
}

// linkStatusTouch updates the updateID and notifies all teams of the update
func linkStatusTouch(op *model.Operation, linkID model.LinkID, status string) string {
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
		if err := wfb.LinkStatus(model.TaskID(linkID), op.ID, t, status, uid); err != nil {
			log.Error(err)
		}
	}
	return uid
}
