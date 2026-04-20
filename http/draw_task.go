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

// setup common to all these calls
func taskRequires(res http.ResponseWriter, req *http.Request) (model.GoogleID, *model.Operation, *model.Task, error) {
	ctx := req.Context()
	op := model.Operation{}

	gid, err := getAgentID(req)
	if err != nil {
		http.Error(res, jsonError(err), http.StatusForbidden)
		return gid, &op, &model.Task{}, err
	}

	op.ID = model.OperationID(req.PathValue("opID"))
	if err = op.Populate(ctx, gid); err != nil {
		if op.ID.IsDeletedOp(ctx) {
			err := fmt.Errorf("requested deleted op")
			log.Warnw(err.Error(), "GID", gid, "resource", op.ID)
			http.Error(res, jsonError(err), http.StatusGone)
			return gid, &op, &model.Task{}, err
		}
		if err.Error() == model.ErrOpNotFound {
			http.Error(res, jsonError(err), http.StatusNotFound)
		} else {
			http.Error(res, jsonError(err), http.StatusNotAcceptable)
		}
		return gid, &op, &model.Task{}, err
	}

	taskID := model.TaskID(req.PathValue("taskID"))
	task, err := op.GetTask(taskID)
	if err != nil {
		if err.Error() == model.ErrTaskNotFound {
			http.Error(res, jsonError(err), http.StatusNotFound)
		} else {
			http.Error(res, jsonError(err), http.StatusNotAcceptable)
		}
		return gid, &op, task, err
	}
	return gid, &op, task, nil
}

// taskStatusAnnounce send the fb announce to all relevant teams
func taskStatusAnnounce(op *model.Operation, taskID model.TaskID, status string, updateID string) {
	// Use background context for fire-and-forget notifications
	ctx := context.Background()
	teams := make(map[model.TeamID]bool)
	for _, t := range op.Teams {
		teams[t.TeamID] = true
	}
	var ta []model.TeamID
	for t := range teams {
		ta = append(ta, t)
	}

	if len(ta) > 0 {
		_ = wfb.TaskStatus(ctx, taskID, op.ID, ta, status, updateID)
	}
}

func drawTaskAssignRoute(res http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	gid, op, task, err := taskRequires(res, req)
	if err != nil {
		return
	}

	if !op.WriteAccess(ctx, gid) {
		err = fmt.Errorf("write access required to assign targets")
		log.Warnw(err.Error(), "GID", gid, "resource", op.ID)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}

	assignments := []model.GoogleID{}

	if err := req.ParseMultipartForm(1024); err != nil {
		log.Errorw(err.Error(), "GID", gid, "resource", op.ID, "message", "failed to parse multipart form")
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	agents, ok := req.MultipartForm.Value["agent"]
	if ok {
		for _, v := range agents {
			assignments = append(assignments, model.GoogleID(v))
		}
	}

	if err = task.SetAssignments(ctx, assignments, nil); err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	uid, err := op.Touch(ctx)
	if err != nil {
		log.Error(err)
	}
	fmt.Fprint(res, jsonOKUpdateID(uid))

	go func() {
		bgCtx := context.Background()
		for _, agent := range assignments {
			_ = wfb.AssignTask(bgCtx, agent, task.ID, op.ID, uid)
		}
	}()
}

func drawTaskClaimRoute(res http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	gid, op, task, err := taskRequires(res, req)
	if err != nil {
		return
	}

	if read, _ := op.ReadAccess(ctx, gid); !read {
		err = fmt.Errorf("read access required to claim targets")
		log.Warnw(err.Error(), "GID", gid, "resource", op.ID)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}

	if err = task.Claim(ctx, gid); err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	uid, err := op.Touch(ctx)
	if err != nil {
		log.Error(err)
	}
	fmt.Fprint(res, jsonOKUpdateID(uid))
	go taskStatusAnnounce(op, task.ID, "claimed", uid)
}

func drawTaskCommentRoute(res http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	gid, op, task, err := taskRequires(res, req)
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
	if err = task.SetComment(ctx, comment); err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	uid, err := op.Touch(ctx)
	if err != nil {
		log.Error(err)
	}
	fmt.Fprint(res, jsonOKUpdateID(uid))
	go taskStatusAnnounce(op, task.ID, "comment", uid)
}

func drawTaskZoneRoute(res http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	gid, op, task, err := taskRequires(res, req)
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
	if err := task.SetZone(ctx, zone); err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	uid, err := op.Touch(ctx)
	if err != nil {
		log.Error(err)
	}
	fmt.Fprint(res, jsonOKUpdateID(uid))
	go taskStatusAnnounce(op, task.ID, "zone", uid)
}

func drawTaskDeltaRoute(res http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	gid, op, task, err := taskRequires(res, req)
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

	if err = task.SetDelta(ctx, int(delta)); err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	uid, err := op.Touch(ctx)
	if err != nil {
		log.Error(err)
	}
	fmt.Fprint(res, jsonOKUpdateID(uid))
	go taskStatusAnnounce(op, task.ID, "delta", uid)
}

func drawTaskFetch(res http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	gid, op, task, err := taskRequires(res, req)
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
	json.NewEncoder(res).Encode(task)
}

func drawTaskCompleteRoute(res http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	gid, op, task, err := taskRequires(res, req)
	if err != nil {
		return
	}

	if read, _ := op.ReadAccess(ctx, gid); !read && !op.AssignedOnlyAccess(ctx, gid) {
		err = fmt.Errorf("access required to claim targets")
		log.Warnw(err.Error(), "GID", gid, "resource", op.ID)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}

	if err := task.Complete(ctx); err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	uid, err := op.Touch(ctx)
	if err != nil {
		log.Error(err)
	}
	fmt.Fprint(res, jsonOKUpdateID(uid))
	go taskStatusAnnounce(op, task.ID, "completed", uid)
}

func drawTaskIncompleteRoute(res http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	gid, op, task, err := taskRequires(res, req)
	if err != nil {
		return
	}

	if read, _ := op.ReadAccess(ctx, gid); !read && !op.AssignedOnlyAccess(ctx, gid) {
		err = fmt.Errorf("access required to claim targets")
		log.Warnw(err.Error(), "GID", gid, "resource", op.ID)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}

	if err = task.Incomplete(ctx); err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	uid, err := op.Touch(ctx)
	if err != nil {
		log.Error(err)
	}
	fmt.Fprint(res, jsonOKUpdateID(uid))
	go taskStatusAnnounce(op, task.ID, "incomplete", uid)
}

func drawTaskRejectRoute(res http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	gid, op, task, err := taskRequires(res, req)
	if err != nil {
		return
	}

	if read, _ := op.ReadAccess(ctx, gid); !read && !op.AssignedOnlyAccess(ctx, gid) {
		err = fmt.Errorf("access required to claim targets")
		log.Warnw(err.Error(), "GID", gid, "resource", op.ID)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}

	if err = task.Reject(ctx, gid); err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	uid, err := op.Touch(ctx)
	if err != nil {
		log.Error(err)
	}
	fmt.Fprint(res, jsonOKUpdateID(uid))
	go taskStatusAnnounce(op, task.ID, "reject", uid)
}

func drawTaskAcknowledgeRoute(res http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	gid, op, task, err := taskRequires(res, req)
	if err != nil {
		return
	}

	if read, _ := op.ReadAccess(ctx, gid); !read && !op.AssignedOnlyAccess(ctx, gid) {
		err = fmt.Errorf("access required to claim targets")
		log.Warnw(err.Error(), "GID", gid, "resource", op.ID)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}

	if err = task.Acknowledge(ctx); err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	uid, err := op.Touch(ctx)
	if err != nil {
		log.Error(err)
	}
	fmt.Fprint(res, jsonOKUpdateID(uid))
	go taskStatusAnnounce(op, task.ID, "acknowledge", uid)
}

func drawTaskDependAddRoute(res http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	gid, op, task, err := taskRequires(res, req)
	if err != nil {
		return
	}

	if !op.WriteAccess(ctx, gid) {
		err = fmt.Errorf("forbidden: write access required to set dependency")
		log.Warnw(err.Error(), "GID", gid, "resource", op.ID)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}

	dependsOn := req.PathValue("dependsOn")
	if dependsOn == "" {
		err = fmt.Errorf("empty dependency ID on depend add")
		log.Warnw(err.Error(), "GID", gid, "resource", op.ID)
		http.Error(res, jsonError(err), http.StatusNotAcceptable)
		return
	}

	if err = task.AddDepend(ctx, model.TaskID(dependsOn)); err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	uid, err := op.Touch(ctx)
	if err != nil {
		log.Error(err)
	}
	fmt.Fprint(res, jsonOKUpdateID(uid))
	go taskStatusAnnounce(op, task.ID, "depends", uid)
}

func drawTaskDependDelRoute(res http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	gid, op, task, err := taskRequires(res, req)
	if err != nil {
		return
	}

	if !op.WriteAccess(ctx, gid) {
		err = fmt.Errorf("forbidden: write access required to set dependency")
		log.Warnw(err.Error(), "GID", gid, "resource", op.ID)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}

	dependsOn := model.TaskID(req.PathValue("dependsOn"))
	if dependsOn == "" {
		err = fmt.Errorf("empty dependency ID on depend delete")
		log.Warnw(err.Error(), "GID", gid, "resource", op.ID)
		http.Error(res, jsonError(err), http.StatusNotAcceptable)
		return
	}

	err = task.DelDepend(ctx, dependsOn)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	uid, err := op.Touch(ctx)
	if err != nil {
		log.Error(err)
	}
	fmt.Fprint(res, jsonOKUpdateID(uid))
	go taskStatusAnnounce(op, task.ID, "depends", uid)
}

func drawTaskOrderRoute(res http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	gid, op, task, err := taskRequires(res, req)
	if err != nil {
		return
	}

	if !op.WriteAccess(ctx, gid) {
		err = fmt.Errorf("forbidden: write access required to set task order")
		log.Warnw(err.Error(), "GID", gid, "resource", op.ID)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}

	os := req.PathValue("order")
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

	if err = task.SetOrder(ctx, int16(order)); err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	uid, err := op.Touch(ctx)
	if err != nil {
		log.Error(err)
	}
	fmt.Fprint(res, jsonOKUpdateID(uid))
	go taskStatusAnnounce(op, task.ID, "order", uid)
}
