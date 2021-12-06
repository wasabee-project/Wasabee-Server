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

func drawLinkAssignRoute(res http.ResponseWriter, req *http.Request) {
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
		err = fmt.Errorf("forbidden: write access required to assign agents")
		log.Warnw(err.Error(), "GID", gid, "resource", op.ID)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}

	if err = op.Populate(gid); err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	linkID := model.LinkID(vars["link"])
	link, err := op.GetLink(linkID)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusNotFound)
		return
	}

	agent := model.GoogleID(req.FormValue("agent"))
	if err = link.Assign(agent); err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	uid := linkAssignTouch(gid, linkID, op)
	// log.Infow("assigned link", "GID", gid, "resource", op.ID, "link", linkID, "agent", agent, "message", "assigned link")
	fmt.Fprint(res, jsonOKUpdateID(uid))
}

func drawLinkDescRoute(res http.ResponseWriter, req *http.Request) {
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
		err = fmt.Errorf("write access required to set link descriptions")
		log.Warnw(err.Error(), "GID", gid, "resource", op.ID)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}

	if err = op.Populate(gid); err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	linkID := model.LinkID(vars["link"])
	link, err := op.GetLink(linkID)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusNotFound)
		return
	}

	desc := req.FormValue("desc")
	if err = link.Comment(desc); err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	uid := linkStatusTouch(op, linkID)
	fmt.Fprint(res, jsonOKUpdateID(uid))
}

func drawLinkColorRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Set("Content-Type", jsonType)

	gid, err := getAgentID(req)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	// only the ID needs to be set for this
	vars := mux.Vars(req)
	var op model.Operation
	op.ID = model.OperationID(vars["document"])

	if !op.WriteAccess(gid) {
		err = fmt.Errorf("forbidden: write access required to set link color")
		log.Warnw(err.Error(), "GID", gid, "resource", op.ID)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}

	if err = op.Populate(gid); err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	linkID := model.LinkID(vars["link"])
	link, err := op.GetLink(linkID)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusNotFound)
		return
	}

	color := req.FormValue("color")
	if err = link.SetColor(color); err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	uid := linkStatusTouch(op, linkID)
	fmt.Fprint(res, jsonOKUpdateID(uid))
}

func drawLinkSwapRoute(res http.ResponseWriter, req *http.Request) {
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
		err = fmt.Errorf("forbidden: write access required to swap link order")
		log.Warnw(err.Error(), "GID", gid, "resource", op.ID)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}

	if err = op.Populate(gid); err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	linkID := model.LinkID(vars["link"])
	link, err := op.GetLink(linkID)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusNotFound)
		return
	}

	if err = link.Swap(); err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	uid := linkStatusTouch(op, linkID)
	fmt.Fprint(res, jsonOKUpdateID(uid))
}

func drawLinkZoneRoute(res http.ResponseWriter, req *http.Request) {
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
		err = fmt.Errorf("forbidden: write access required to set zone")
		log.Warnw(err.Error(), "GID", gid, "resource", op.ID)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}

	if err = op.Populate(gid); err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	linkID := model.LinkID(vars["link"])
	link, err := op.GetLink(linkID)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusNotFound)
		return
	}

	zone := model.ZoneFromString(req.FormValue("zone"))
	if err = link.SetZone(zone); err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	uid := linkStatusTouch(op, linkID)
	fmt.Fprint(res, jsonOKUpdateID(uid))
}

func drawLinkDeltaRoute(res http.ResponseWriter, req *http.Request) {
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

	linkID := model.LinkID(vars["link"])
	link, err := op.GetLink(linkID)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusNotFound)
		return
	}

	if err = link.SetDelta(int(delta)); err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	uid := linkStatusTouch(op, linkID)
	fmt.Fprint(res, jsonOKUpdateID(uid))
}

func drawLinkCompleteRoute(res http.ResponseWriter, req *http.Request) {
	drawLinkCompRoute(res, req, true)
}

func drawLinkIncompleteRoute(res http.ResponseWriter, req *http.Request) {
	drawLinkCompRoute(res, req, false)
}

func drawLinkCompRoute(res http.ResponseWriter, req *http.Request, complete bool) {
	res.Header().Set("Content-Type", jsonType)

	gid, err := getAgentID(req)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	// only the ID needs to be set for this
	vars := mux.Vars(req)
	var op model.Operation
	op.ID = model.OperationID(vars["document"])

	if err = op.Populate(gid); err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	linkID := model.LinkID(vars["link"])
	link, err := op.GetLink(linkID)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusNotFound)
		return
	}

	// write access OR asignee
	// after op.Populate since we need the full link here
	if !op.WriteAccess(gid) && !link.IsAssignedTo(gid) {
		err = fmt.Errorf("permission to mark link as complete denied")
		log.Warnw(err.Error(), "GID", gid, "resource", op.ID)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}

	if err = link.Complete(complete); err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	uid := linkStatusTouch(op, linkID)
	fmt.Fprint(res, jsonOKUpdateID(uid))
}

func drawLinkClaimRoute(res http.ResponseWriter, req *http.Request) {
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

	read, _ := op.ReadAccess(gid)
	if !read {
		err = fmt.Errorf("permission to claim link assignment denied")
		log.Warnw(err.Error(), "GID", gid, "resource", op.ID)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}

	if err = op.Populate(gid); err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	linkID := model.LinkID(vars["link"])
	link, err := op.GetLink(linkID)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusNotFound)
		return
	}

	if err = link.Claim(gid); err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	uid := linkStatusTouch(op, linkID)
	fmt.Fprint(res, jsonOKUpdateID(uid))
}

func drawLinkRejectRoute(res http.ResponseWriter, req *http.Request) {
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

	if err = op.Populate(gid); err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	linkID := model.LinkID(vars["link"])
	link, err := op.GetLink(linkID)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusNotFound)
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

	uid := linkStatusTouch(op, linkID)
	fmt.Fprint(res, jsonOKUpdateID(uid))
}

func drawLinkFetch(res http.ResponseWriter, req *http.Request) {
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

	linkID := model.LinkID(vars["link"])
	link, err := op.GetLink(linkID)
	if err != nil {
		log.Error(err)
		// not really a 404, but close enough, better than a 500 or 403
		http.Error(res, jsonError(err), http.StatusNotFound)
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
func linkAssignTouch(gid model.GoogleID, linkID model.LinkID, op model.Operation) string {
	uid, err := op.Touch()
	if err != nil {
		log.Error(err)
	}

	wfb.AssignLink(gid, model.TaskID(linkID), op.ID, uid)
	return uid
}

// linkStatusTouch updates the updateID and notifies all teams of the update
func linkStatusTouch(op model.Operation, linkID model.LinkID) string {
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
		err := wfb.LinkStatus(model.TaskID(linkID), op.ID, t, uid)
		if err != nil {
			log.Error(err)
		}
	}
	return uid
}

func drawLinkDependAddRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Set("Content-Type", jsonType)

	gid, err := getAgentID(req)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	// only the ID needs to be set for this
	vars := mux.Vars(req)
	var op model.Operation
	op.ID = model.OperationID(vars["document"])

	if !op.WriteAccess(gid) {
		err = fmt.Errorf("forbidden: write access required to set dependency")
		log.Warnw(err.Error(), "GID", gid, "resource", op.ID)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}

	linkID := model.LinkID(vars["link"])
	if linkID == "" {
		err = fmt.Errorf("empty link ID on depend add")
		log.Warnw(err.Error(), "GID", gid, "resource", op.ID)
		http.Error(res, jsonError(err), http.StatusNotAcceptable)
		return
	}

	task := vars["task"]
	if task == "" {
		err = fmt.Errorf("empty task ID on depend add")
		log.Warnw(err.Error(), "GID", gid, "resource", op.ID)
		http.Error(res, jsonError(err), http.StatusNotAcceptable)
		return
	}

	link, err := op.GetLink(linkID)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusNotFound)
		return
	}

	err = link.AddDepend(task)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	uid := linkStatusTouch(op, linkID)
	fmt.Fprint(res, jsonOKUpdateID(uid))
}

func drawLinkDependDelRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Set("Content-Type", jsonType)

	gid, err := getAgentID(req)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	// only the ID needs to be set for this
	vars := mux.Vars(req)
	var op model.Operation
	op.ID = model.OperationID(vars["document"])

	if !op.WriteAccess(gid) {
		err = fmt.Errorf("forbidden: write access required to delete dependency")
		log.Warnw(err.Error(), "GID", gid, "resource", op.ID)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}

	linkID := model.LinkID(vars["link"])
	if linkID == "" {
		err = fmt.Errorf("empty link ID on depend delete")
		log.Warnw(err.Error(), "GID", gid, "resource", op.ID)
		http.Error(res, jsonError(err), http.StatusNotAcceptable)
		return
	}

	task := vars["task"]
	if task == "" {
		err = fmt.Errorf("empty task ID on depend delete")
		log.Warnw(err.Error(), "GID", gid, "resource", op.ID)
		http.Error(res, jsonError(err), http.StatusNotAcceptable)
		return
	}

	link, err := op.GetLink(linkID)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusNotFound)
		return
	}

	err = link.DelDepend(task)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	uid := linkStatusTouch(op, linkID)
	fmt.Fprint(res, jsonOKUpdateID(uid))
}
