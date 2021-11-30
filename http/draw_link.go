package wasabeehttps

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
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

	// only the ID needs to be set for this
	vars := mux.Vars(req)
	var op model.Operation
	op.ID = model.OperationID(vars["document"])

	if !op.WriteAccess(gid) {
		err = fmt.Errorf("forbidden: write access required to assign agents")
		log.Warnw(err.Error(), "GID", gid, "resource", op.ID)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}
	linkID := model.LinkID(vars["link"])
	agent := model.GoogleID(req.FormValue("agent"))

	err = op.Populate(gid)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	link, err := op.GetLink(linkID)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	err = link.Assign(agent)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	uid, err := op.Touch()
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
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

	// only the ID needs to be set for this
	vars := mux.Vars(req)
	var op model.Operation
	op.ID = model.OperationID(vars["document"])

	if !op.WriteAccess(gid) {
		err = fmt.Errorf("write access required to set link descriptions")
		log.Warnw(err.Error(), "GID", gid, "resource", op.ID)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}
	linkID := model.LinkID(vars["link"])
	desc := req.FormValue("desc")

	err = op.Populate(gid)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	link, err := op.GetLink(linkID)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	rr := link.Comment(desc)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	uid, err := op.Touch()
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
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
	linkID := model.LinkID(vars["link"])
	color := req.FormValue("color")

	err = op.Populate(gid)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	link, err := op.GetLink(linkID)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	err = link.SetColor(color)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	uid, err := op.Touch()
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

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

	// only the ID needs to be set for this
	vars := mux.Vars(req)
	var op model.Operation
	op.ID = model.OperationID(vars["document"])

	if !op.WriteAccess(gid) {
		err = fmt.Errorf("forbidden: write access required to swap link order")
		log.Warnw(err.Error(), "GID", gid, "resource", op.ID)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}

	linkID := model.LinkID(vars["link"])

	err = op.Populate(gid)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	link, err := op.GetLink(linkID)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	err = link.Swap()
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	uid, err := op.Touch()
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
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

	linkID := model.LinkID(vars["link"])
	zone := model.ZoneFromString(req.FormValue("zone"))

	err = op.Populate(gid)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	link, err := op.GetLink(linkID)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	err = link.SetZone(zone)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	uid, err := op.Touch()
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
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

	link := model.LinkID(vars["link"])
	delta, err := strconv.ParseInt(req.FormValue("delta"), 10, 32)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	uid, err := op.LinkDelta(link, int(delta))
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
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

	// write access OR asignee
	link := model.LinkID(vars["link"])
	if !op.WriteAccess(gid) && !op.ID.AssignedTo(link, gid) {
		err = fmt.Errorf("permission to mark link as complete denied")
		log.Warnw(err.Error(), "GID", gid, "resource", op.ID)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}

	uid, err := op.LinkCompleted(link, complete)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
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

	// only the ID needs to be set for this
	vars := mux.Vars(req)
	var op model.Operation
	op.ID = model.OperationID(vars["document"])

	link := model.LinkID(vars["link"])
	read, _ := op.ReadAccess(gid)
	if !read {
		err = fmt.Errorf("permission to claim link assignment denied")
		log.Warnw(err.Error(), "GID", gid, "resource", op.ID)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}

	uid, err := link.Claim(&op, gid)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
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

	// only the ID needs to be set for this
	vars := mux.Vars(req)
	var op model.Operation
	op.ID = model.OperationID(vars["document"])

	// asignee only
	link := model.LinkID(vars["link"])
	if !op.ID.AssignedTo(link, gid) {
		err = fmt.Errorf("permission to reject link assignment denied")
		log.Warnw(err.Error(), "GID", gid, "resource", op.ID)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}

	uid, err := link.Reject(&op, gid)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
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

	// populate the whole op, slow, but ensures we only get things we have access to see
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
