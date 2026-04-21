package wasabeehttps

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path"
	"strconv"
	"time"

	"github.com/wasabee-project/Wasabee-Server/Firebase"
	"github.com/wasabee-project/Wasabee-Server/config"
	"github.com/wasabee-project/Wasabee-Server/log"
	"github.com/wasabee-project/Wasabee-Server/messaging"
	"github.com/wasabee-project/Wasabee-Server/model"
)

func drawUploadRoute(res http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	gid, err := getAgentID(req)
	if err != nil {
		http.Error(res, jsonError(err), http.StatusUnauthorized)
		return
	}

	if !contentTypeIs(req, jsonTypeShort) {
		err := fmt.Errorf("invalid request (needs to be application/json)")
		log.Infow(err.Error(), "GID", gid, "resource", "new operation")
		http.Error(res, jsonError(err), http.StatusNotAcceptable)
		return
	}

	var o model.Operation
	d := json.NewDecoder(req.Body)
	if err := d.Decode(&o); err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusNotAcceptable)
		return
	}

	if err = model.DrawInsert(ctx, &o, gid); err != nil {
		log.Infow(err.Error(), "GID", gid)
		http.Error(res, jsonError(err), http.StatusNotAcceptable)
		return
	}

	agent, err := gid.GetAgent(ctx)
	if err != nil {
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	res.Header().Set("Cache-Control", "no-store")
	if err = json.NewEncoder(res).Encode(&agent); err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	c := config.Get()
	if c.StoreRevisions {
		fn := fmt.Sprintf("%s-POST.json", o.ID)
		p := path.Join(c.RevisionsDir, fn)
		log.Debugw("storing", "p", p)

		fh, err := os.OpenFile(p, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
		if err != nil {
			log.Error(err)
			return
		}

		if err = json.NewEncoder(fh).Encode(&o); err != nil {
			log.Error(err)
			fh.Close()
			return
		}
		fh.Close()

		var refetch model.Operation
		refetch.ID = o.ID
		_ = refetch.Populate(ctx, gid)

		fn = fmt.Sprintf("%s-POST-POPULATED.json", refetch.ID)
		p = path.Join(c.RevisionsDir, fn)
		log.Debugw("storing", "p", p)

		fhp, err := os.OpenFile(p, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
		if err != nil {
			log.Error(err)
			return
		}

		if err = json.NewEncoder(fhp).Encode(&refetch); err != nil {
			log.Error(err)
			fhp.Close()
			return
		}
		fhp.Close()
	}
}

func drawGetRoute(res http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	gid, err := getAgentID(req)
	if err != nil {
		http.Error(res, jsonError(err), http.StatusUnauthorized)
		return
	}

	var o model.Operation
	o.ID = model.OperationID(req.PathValue("opID"))

	if o.ID.IsDeletedOp(ctx) {
		err := fmt.Errorf("requested deleted op")
		log.Infow(err.Error(), "GID", gid, "resource", o.ID)
		http.Error(res, jsonError(err), http.StatusGone)
		return
	}

	read, _ := o.ReadAccess(ctx, gid)
	assignOnly := o.AssignedOnlyAccess(ctx, gid)
	if !read && !assignOnly {
		err := fmt.Errorf("forbidden")
		agent, _ := gid.IngressName(ctx)
		log.Warnw(err.Error(), "GID", gid, "resource", o.ID, "message", "no access to operation", "agent", agent)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}

	stat, err := o.ID.Stat(ctx)
	if err != nil {
		http.Error(res, jsonError(err), http.StatusNotFound)
		return
	}

	im := req.Header.Get("If-None-Match")
	if im != "" && im == stat.LastEditID {
		err := fmt.Errorf("local copy matches server copy")
		http.Error(res, jsonError(err), http.StatusNotModified)
		return
	}

	lastModified, err := time.ParseInLocation(time.RFC3339, stat.Modified, time.UTC)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusNotAcceptable)
		return
	}

	ims := req.Header.Get("If-Modified-Since")
	if ims != "" && ims != "null" {
		modifiedSince, err := time.ParseInLocation(time.RFC1123, ims, time.UTC)
		if err != nil {
			log.Error(err)
			http.Error(res, jsonError(err), http.StatusNotAcceptable)
			return
		}
		if !lastModified.After(modifiedSince) {
			res.Header().Set("Content-Type", "")
			http.Redirect(res, req, "", http.StatusNotModified)
			return
		}
	}

	if err = o.Populate(ctx, gid); err != nil {
		http.Error(res, jsonError(err), http.StatusNotAcceptable)
		return
	}

	res.Header().Set("Last-Modified", lastModified.Format(time.RFC1123))
	res.Header().Set("Cache-Control", "no-store")
	res.Header().Set("ETag", o.LastEditID)
	if err = json.NewEncoder(res).Encode(&o); err != nil {
		log.Errorw("unable to encode & send operation to client", "error", err.Error())
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
}

func drawDeleteRoute(res http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	gid, err := getAgentID(req)
	if err != nil {
		http.Error(res, jsonError(err), http.StatusUnauthorized)
		return
	}

	var op model.Operation
	op.ID = model.OperationID(req.PathValue("opID"))

	if !op.ID.IsOwner(ctx, gid) {
		err = fmt.Errorf("forbidden: only the owner can delete an operation")
		log.Warnw(err.Error(), "resource", op.ID, "GID", gid)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}

	if err := op.Delete(ctx, gid); err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	messaging.DeleteOperation(ctx, messaging.OperationID(op.ID))
	log.Infow("deleted operation", "resource", op.ID, "GID", gid, "message", "deleted operation")
	fmt.Fprint(res, jsonStatusOK)
}

func drawUpdateRoute(res http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	gid, err := getAgentID(req)
	if err != nil {
		http.Error(res, jsonError(err), http.StatusUnauthorized)
		return
	}

	opID := model.OperationID(req.PathValue("opID"))
	var op model.Operation
	op.ID = opID

	if !contentTypeIs(req, jsonTypeShort) {
		err := fmt.Errorf("invalid request (needs to be application/json)")
		log.Warnw(err.Error(), "GID", gid, "resource", op.ID)
		http.Error(res, jsonError(err), http.StatusNotAcceptable)
		return
	}

	if !op.WriteAccess(ctx, gid) {
		err = fmt.Errorf("forbidden: write access required to update an operation")
		log.Warnw(err.Error(), "GID", gid, "resource", op.ID)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}

	s, err := op.ID.Stat(ctx)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	im := req.Header.Get("If-Match")
	if im != "" && im != s.LastEditID {
		err := fmt.Errorf("local copy out-of-date")
		log.Debugw(err.Error(), "GID", gid, "resource", s.ID, "If-Match", im, "LastEditID", s.LastEditID)
		http.Error(res, jsonError(err), http.StatusPreconditionFailed)
		return
	}

	d := json.NewDecoder(req.Body)
	if err := d.Decode(&op); err != nil {
		log.Errorw("decoding incoming update", "error", err.Error(), "If-Match", im, "LastEditID", s.LastEditID, "Content-Length", req.Header.Get("Content-Length"))
		http.Error(res, jsonError(err), http.StatusNotAcceptable)
		return
	}

	if opID != op.ID {
		err := fmt.Errorf("incoming op.ID does not match the URL specified ID: refusing update")
		log.Errorw(err.Error(), "resource", opID, "mismatch", op.ID)
		http.Error(res, jsonError(err), http.StatusNotAcceptable)
		return
	}

	err = model.DrawUpdate(ctx, &op, gid)
	if err != nil {
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	uid := touch(ctx, op)
	fmt.Fprint(res, jsonOKUpdateID(uid))

	c := config.Get()
	if c.StoreRevisions {
		fn := fmt.Sprintf("%s-%s.json", opID, uid)
		p := path.Join(c.RevisionsDir, fn)
		log.Debugw("storing", "p", p)

		fh, err := os.OpenFile(p, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
		if err != nil {
			log.Error(err)
			return
		}

		if err = json.NewEncoder(fh).Encode(&op); err != nil {
			log.Error(err)
			fh.Close()
			return
		}
		fh.Close()

		var refetch model.Operation
		refetch.ID = op.ID
		_ = refetch.Populate(ctx, gid)

		fn = fmt.Sprintf("%s-%s-POPULATED.json", opID, uid)
		p = path.Join(c.RevisionsDir, fn)
		log.Debugw("storing", "p", p)

		fhp, err := os.OpenFile(p, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
		if err != nil {
			log.Error(err)
			return
		}

		if err = json.NewEncoder(fhp).Encode(&refetch); err != nil {
			log.Error(err)
			fhp.Close()
			return
		}
		fhp.Close()
	}
}

func drawChownRoute(res http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	gid, err := getAgentID(req)
	if err != nil {
		http.Error(res, jsonError(err), http.StatusUnauthorized)
		return
	}

	to := req.PathValue("to")
	var op model.Operation
	op.ID = model.OperationID(req.PathValue("opID"))

	if !op.ID.IsOwner(ctx, gid) {
		err = fmt.Errorf("forbidden: only the owner can set operation ownership ")
		log.Warnw(err.Error(), "GID", gid, "resource", op.ID)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}

	err = op.ID.Chown(ctx, gid, to)
	if err != nil {
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	fmt.Fprint(res, jsonStatusOK)
}

func drawPortalCommentRoute(res http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	gid, err := getAgentID(req)
	if err != nil {
		http.Error(res, jsonError(err), http.StatusUnauthorized)
		return
	}

	var op model.Operation
	op.ID = model.OperationID(req.PathValue("opID"))

	if !op.WriteAccess(ctx, gid) {
		err = fmt.Errorf("write access required to set portal comments")
		log.Warnw(err.Error(), "GID", gid, "resource", op.ID)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}

	portalID := model.PortalID(req.PathValue("portal"))
	comment := req.FormValue("comment")
	err = op.ID.PortalComment(ctx, portalID, comment)
	if err != nil {
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	uid := touch(ctx, op)
	fmt.Fprint(res, jsonOKUpdateID(uid))
}

func drawPortalHardnessRoute(res http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	gid, err := getAgentID(req)
	if err != nil {
		http.Error(res, jsonError(err), http.StatusUnauthorized)
		return
	}

	var op model.Operation
	op.ID = model.OperationID(req.PathValue("opID"))

	if !op.WriteAccess(ctx, gid) {
		err = fmt.Errorf("write access required to set portal hardness")
		log.Warnw(err.Error(), "GID", gid, "resource", op.ID)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}
	portalID := model.PortalID(req.PathValue("portal"))
	hardness := req.FormValue("hardness")
	err = op.ID.PortalHardness(ctx, portalID, hardness)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	uid := touch(ctx, op)
	fmt.Fprint(res, jsonOKUpdateID(uid))
}

func drawOrderRoute(res http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	gid, err := getAgentID(req)
	if err != nil {
		http.Error(res, jsonError(err), http.StatusUnauthorized)
		return
	}

	var op model.Operation
	op.ID = model.OperationID(req.PathValue("opID"))

	if !op.WriteAccess(ctx, gid) {
		err = fmt.Errorf("write access required to set operation order")
		log.Warnw(err.Error(), "GID", gid, "resource", op.ID)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}

	order := req.FormValue("order")
	err = op.LinkOrder(ctx, order)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusUnauthorized)
		return
	}
	err = op.MarkerOrder(ctx, order)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusUnauthorized)
		return
	}
	uid := touch(ctx, op)
	fmt.Fprint(res, jsonOKUpdateID(uid))
}

func drawInfoRoute(res http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	gid, err := getAgentID(req)
	if err != nil {
		http.Error(res, jsonError(err), http.StatusUnauthorized)
		return
	}

	var op model.Operation
	op.ID = model.OperationID(req.PathValue("opID"))

	if !op.WriteAccess(ctx, gid) {
		err = fmt.Errorf("write access required to set operation info")
		log.Warnw(err.Error(), "GID", gid, "resource", op.ID)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}
	info := req.FormValue("info")
	err = op.SetInfo(ctx, info)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	uid := touch(ctx, op)
	fmt.Fprint(res, jsonOKUpdateID(uid))
}

func drawPortalKeysRoute(res http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	gid, err := getAgentID(req)
	if err != nil {
		http.Error(res, jsonError(err), http.StatusUnauthorized)
		return
	}

	var op model.Operation
	op.ID = model.OperationID(req.PathValue("opID"))
	portalID := model.PortalID(req.PathValue("portal"))

	onhand, err := strconv.ParseInt(req.FormValue("count"), 10, 32)
	if err != nil {
		onhand = 0
	}
	if onhand < 0 {
		onhand = 0
	}
	if onhand > 3000 {
		onhand = 3000
	}
	capsule := req.FormValue("capsule")

	err = op.KeyOnHand(ctx, gid, portalID, int32(onhand), capsule)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusNotAcceptable)
		return
	}

	uid := touch(ctx, op)
	fmt.Fprint(res, jsonOKUpdateID(uid))
}

func drawPermsAddRoute(res http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	gid, err := getAgentID(req)
	if err != nil {
		http.Error(res, jsonError(err), http.StatusUnauthorized)
		return
	}

	var op model.Operation
	op.ID = model.OperationID(req.PathValue("opID"))

	if !op.ID.IsOwner(ctx, gid) {
		err = fmt.Errorf("permission to edit permissions denied")
		log.Warnw(err.Error(), "GID", gid, "resource", op.ID)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}

	teamID := model.TeamID(req.FormValue("team"))
	role := req.FormValue("role")
	if teamID == "" || role == "" {
		err = fmt.Errorf("required value not set to add permission to op")
		log.Warn(err)
		http.Error(res, jsonError(err), http.StatusNotAcceptable)
		return
	}
	zone := model.ZoneFromString(req.FormValue("zone"))

	err = op.ID.AddPerm(ctx, gid, teamID, role, zone)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	uid := touch(ctx, op)
	fmt.Fprint(res, jsonOKUpdateID(uid))
}

func drawPermsDeleteRoute(res http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	gid, err := getAgentID(req)
	if err != nil {
		http.Error(res, jsonError(err), http.StatusUnauthorized)
		return
	}

	var op model.Operation
	op.ID = model.OperationID(req.PathValue("opID"))

	if !op.ID.IsOwner(ctx, gid) {
		err = fmt.Errorf("permission to edit permissions denied")
		log.Warnw(err.Error(), "GID", gid, "resource", op.ID)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}

	teamID := model.TeamID(req.FormValue("team"))
	role := model.OpPermRole(req.FormValue("role"))
	zone := model.ZoneFromString(req.FormValue("zone"))
	if teamID == "" || role == "" {
		err = fmt.Errorf("required value not set to remove permission from op")
		log.Warnw(err.Error(), "GID", gid, "role", role, "zone", zone, "teamID", teamID, "resource", op.ID)
		http.Error(res, jsonError(err), http.StatusNotAcceptable)
		return
	}

	err = op.ID.DelPerm(ctx, gid, teamID, role, zone)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	uid := touch(ctx, op)
	fmt.Fprint(res, jsonOKUpdateID(uid))
}

func jsonOKUpdateID(uid string) string {
	return fmt.Sprintf("{\"status\":\"ok\", \"updateID\": \"%s\"}", uid)
}

func touch(ctx context.Context, op model.Operation) string {
	uid, err := op.Touch(ctx)
	if err != nil {
		return ""
	}

	go func() {
		// Use Background for the notification so it isn't killed if the request context ends
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
			_ = wfb.MapChange(bgCtx, ta, op.ID, uid)
		}
	}()
	return uid
}
