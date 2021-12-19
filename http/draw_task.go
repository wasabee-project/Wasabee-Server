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

func taskRequires(res http.ResponseWriter, req *http.Request) (model.GoogleID, *model.Operation, *model.Task, error) {
	res.Header().Set("Content-Type", jsonType)

	op := model.Operation{}

	gid, err := getAgentID(req)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return gid, &op, &model.Task{}, err
	}

	vars := mux.Vars(req)
	op.ID = model.OperationID(vars["opID"])
	if err = op.Populate(gid); err != nil {
		log.Error(err)
		if op.ID.IsDeletedOp() {
			err := fmt.Errorf("requested deleted op")
			log.Warnw(err.Error(), "GID", gid, "resource", op.ID)
			http.Error(res, jsonError(err), http.StatusGone)
			return gid, &op, &model.Task{}, err
		}
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return gid, &op, &model.Task{}, err
	}

	taskID := model.TaskID(vars["taskID"])
	task, err := op.GetTask(taskID)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusNotAcceptable)
		return gid, &op, task, err
	}
	return gid, &op, task, nil
}

func drawTaskAssignRoute(res http.ResponseWriter, req *http.Request) {
	gid, op, task, err := taskRequires(res, req)
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
	g := []model.GoogleID{agent}
	if err = task.Assign(g, nil); err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	uid := taskAssignTouch(gid, task.ID, op)
	fmt.Fprint(res, jsonOKUpdateID(uid))
}

func drawTaskClaimRoute(res http.ResponseWriter, req *http.Request) {
	gid, op, task, err := taskRequires(res, req)
	if err != nil {
		return
	}

	if read, _ := op.ReadAccess(gid); !read {
		err = fmt.Errorf("read access required to claim targets")
		log.Warnw(err.Error(), "GID", gid, "resource", op.ID)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}

	if err = task.Claim(gid); err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	uid := taskStatusTouch(op, task.ID, "claimed")
	fmt.Fprint(res, jsonOKUpdateID(uid))
}

func drawTaskCommentRoute(res http.ResponseWriter, req *http.Request) {
	gid, op, task, err := taskRequires(res, req)
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
	if err = task.SetComment(comment); err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	uid := taskStatusTouch(op, task.ID, "comment")
	fmt.Fprint(res, jsonOKUpdateID(uid))
}

func drawTaskZoneRoute(res http.ResponseWriter, req *http.Request) {
	gid, op, task, err := taskRequires(res, req)
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
	if err := task.SetZone(zone); err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	uid := taskStatusTouch(op, task.ID, "zone")
	fmt.Fprint(res, jsonOKUpdateID(uid))
}

func drawTaskDeltaRoute(res http.ResponseWriter, req *http.Request) {
	gid, op, task, err := taskRequires(res, req)
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

	if err = task.SetDelta(int(delta)); err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	uid := taskStatusTouch(op, task.ID, "delta")
	fmt.Fprint(res, jsonOKUpdateID(uid))
}

func drawTaskFetch(res http.ResponseWriter, req *http.Request) {
	gid, op, task, err := taskRequires(res, req)
	if err != nil {
		return
	}

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

	j, err := json.Marshal(task)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	fmt.Fprint(res, string(j))
}

func drawTaskCompleteRoute(res http.ResponseWriter, req *http.Request) {
	gid, op, task, err := taskRequires(res, req)
	if err != nil {
		return
	}

	if read, _ := op.ReadAccess(gid); !read && !op.AssignedOnlyAccess(gid) {
		err = fmt.Errorf("access required to claim targets")
		log.Warnw(err.Error(), "GID", gid, "resource", op.ID)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}

	if err := task.Complete(gid); err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	uid := taskStatusTouch(op, task.ID, "completed")
	fmt.Fprint(res, jsonOKUpdateID(uid))
}

func drawTaskIncompleteRoute(res http.ResponseWriter, req *http.Request) {
	gid, op, task, err := taskRequires(res, req)
	if err != nil {
		return
	}

	if read, _ := op.ReadAccess(gid); !read && !op.AssignedOnlyAccess(gid) {
		err = fmt.Errorf("access required to claim targets")
		log.Warnw(err.Error(), "GID", gid, "resource", op.ID)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}

	if err = task.Incomplete(gid); err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	uid := taskStatusTouch(op, task.ID, "incomplete")
	fmt.Fprint(res, jsonOKUpdateID(uid))
}

func drawTaskRejectRoute(res http.ResponseWriter, req *http.Request) {
	gid, op, task, err := taskRequires(res, req)
	if err != nil {
		return
	}

	if read, _ := op.ReadAccess(gid); !read && !op.AssignedOnlyAccess(gid) {
		err = fmt.Errorf("access required to claim targets")
		log.Warnw(err.Error(), "GID", gid, "resource", op.ID)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}

	if err = task.Reject(gid); err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	uid := taskStatusTouch(op, task.ID, "reject")
	fmt.Fprint(res, jsonOKUpdateID(uid))
}

func drawTaskAcknowledgeRoute(res http.ResponseWriter, req *http.Request) {
	gid, op, task, err := taskRequires(res, req)
	if err != nil {
		return
	}

	if read, _ := op.ReadAccess(gid); !read && !op.AssignedOnlyAccess(gid) {
		err = fmt.Errorf("access required to claim targets")
		log.Warnw(err.Error(), "GID", gid, "resource", op.ID)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}

	if err = task.Acknowledge(gid); err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	uid := taskStatusTouch(op, task.ID, "acknowledge")
	fmt.Fprint(res, jsonOKUpdateID(uid))
}

func taskAssignTouch(gid model.GoogleID, markerID model.TaskID, op *model.Operation) string {
	uid, err := op.Touch()
	if err != nil {
		log.Error(err)
	}

	wfb.AssignTask(gid, model.TaskID(markerID), op.ID, uid)
	return uid
}

func taskStatusTouch(op *model.Operation, taskID model.TaskID, status string) string {
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

	for _, t := range teams {
		err := wfb.TaskStatus(taskID, op.ID, t, status)
		if err != nil {
			log.Error(err)
		}
	}
	return uid
}

func drawTaskDependAddRoute(res http.ResponseWriter, req *http.Request) {
	gid, op, task, err := taskRequires(res, req)
	if err != nil {
		return
	}

	if !op.WriteAccess(gid) {
		err = fmt.Errorf("forbidden: write access required to set dependency")
		log.Warnw(err.Error(), "GID", gid, "resource", op.ID)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}

	vars := mux.Vars(req)

	dependsOn := vars["dependsOn"]
	if dependsOn == "" {
		err = fmt.Errorf("empty dependency ID on depend add")
		log.Warnw(err.Error(), "GID", gid, "resource", op.ID)
		http.Error(res, jsonError(err), http.StatusNotAcceptable)
		return
	}

	if err = task.AddDepend(model.TaskID(dependsOn)); err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	uid := taskStatusTouch(op, task.ID, "depends")
	fmt.Fprint(res, jsonOKUpdateID(uid))
}

func drawTaskDependDelRoute(res http.ResponseWriter, req *http.Request) {
	gid, op, task, err := taskRequires(res, req)
	if err != nil {
		return
	}

	if !op.WriteAccess(gid) {
		err = fmt.Errorf("forbidden: write access required to set dependency")
		log.Warnw(err.Error(), "GID", gid, "resource", op.ID)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}

	vars := mux.Vars(req)
	dependsOn := vars["dependsOn"]
	if dependsOn == "" {
		err = fmt.Errorf("empty dependency ID on depend delete")
		log.Warnw(err.Error(), "GID", gid, "resource", op.ID)
		http.Error(res, jsonError(err), http.StatusNotAcceptable)
		return
	}

	err = task.DelDepend(dependsOn)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	uid := taskStatusTouch(op, task.ID, "depends")
	fmt.Fprint(res, jsonOKUpdateID(uid))
}

func drawTaskOrderRoute(res http.ResponseWriter, req *http.Request) {
	gid, op, task, err := taskRequires(res, req)
	if err != nil {
		return
	}

	if !op.WriteAccess(gid) {
		err = fmt.Errorf("forbidden: write access required to set task order")
		log.Warnw(err.Error(), "GID", gid, "resource", op.ID)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}

	vars := mux.Vars(req)
	os := vars["order"]
	if os == "" {
		err = fmt.Errorf("empty order ID on order set")
		log.Warnw(err.Error(), "GID", gid, "resource", op.ID)
		http.Error(res, jsonError(err), http.StatusNotAcceptable)
		return
	}
	order, err := strconv.ParseInt(os, 10, 16)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	if err = task.SetOrder(int16(order)); err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	uid := taskStatusTouch(op, task.ID, "order")
	fmt.Fprint(res, jsonOKUpdateID(uid))
}
