package wasabeehttps

import (
	"encoding/json"
	"fmt"
	// "io"
	"net/http"
	"os"
	"path"
	"strconv"
	"time"

	"github.com/gorilla/mux"

	"github.com/wasabee-project/Wasabee-Server/Firebase"
	"github.com/wasabee-project/Wasabee-Server/config"
	"github.com/wasabee-project/Wasabee-Server/log"
	"github.com/wasabee-project/Wasabee-Server/messaging"
	"github.com/wasabee-project/Wasabee-Server/model"
)

func drawUploadRoute(res http.ResponseWriter, req *http.Request) {
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

	/* plan a
	jBlob, err := io.ReadAll(req.Body)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	var o model.Operation
	if err := json.Unmarshal(json.RawMessage(jBlob), &o); err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusNotAcceptable)
		return
	}
	plan a */

	/* plan b */
	var o model.Operation
	d := json.NewDecoder(req.Body)
	// d.DisallowUnknownFields()
	if err := d.Decode(&o); err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusNotAcceptable)
		return
	}
	/* plan b */

	if err = model.DrawInsert(&o, gid); err != nil {
		log.Infow(err.Error(), "GID", gid)
		http.Error(res, jsonError(err), http.StatusNotAcceptable)
		return
	}

	// the IITC plugin wants the full /me data on draw POST so it can update its list of ops
	agent, err := gid.GetAgent()
	if err != nil {
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	res.Header().Set("Cache-Control", "no-store")
	// data, _ := json.Marshal(agent)
	// fmt.Fprint(res, string(data))
	if err = json.NewEncoder(res).Encode(&agent); err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	// store backup revision -- used for testing
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
		defer fh.Close()

		if err = json.NewEncoder(fh).Encode(&o); err != nil {
			log.Error(err)
			return
		}
	}
}

func drawGetRoute(res http.ResponseWriter, req *http.Request) {
	gid, err := getAgentID(req)
	if err != nil {
		http.Error(res, jsonError(err), http.StatusUnauthorized)
		return
	}

	var o model.Operation
	vars := mux.Vars(req)
	o.ID = model.OperationID(vars["opID"])

	if o.ID.IsDeletedOp() {
		err := fmt.Errorf("requested deleted op")
		log.Infow(err.Error(), "GID", gid, "resource", o.ID)
		http.Error(res, jsonError(err), http.StatusGone)
		return
	}

	read, _ := o.ReadAccess(gid)
	assignOnly := o.AssignedOnlyAccess(gid)
	if !read && !assignOnly {
		err := fmt.Errorf("forbidden")
		agent, _ := gid.IngressName()
		log.Warnw(err.Error(), "GID", gid, "resource", o.ID, "message", "no access to operation", "agent", agent)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}

	// don't do full populate (slow) just yet
	stat, err := o.ID.Stat()
	if err != nil {
		http.Error(res, jsonError(err), http.StatusNotFound)
		return
	}

	// basically the same as If-Modified-Since
	im := req.Header.Get("If-None-Match")
	if im != "" && im == stat.LastEditID {
		err := fmt.Errorf("local copy matches server copy")
		// log.Debugw(err.Error(), "GID", gid, "resource", o.ID, "If-None-Match", im, "LastEditID", stat.LastEditID)
		http.Error(res, jsonError(err), http.StatusNotModified)
		return
	}

	lastModified, err := time.ParseInLocation("2006-01-02 15:04:05", stat.Modified, time.UTC)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusNotAcceptable)
		return
	}

	ims := req.Header.Get("If-Modified-Since")
	if ims != "" && ims != "null" { // yes, the string "null", seen in the wild
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

	// o.Populate determines all, zone, or assigned-only
	if err = o.Populate(gid); err != nil {
		http.Error(res, jsonError(err), http.StatusNotAcceptable)
		return
	}

	/* s, err := json.Marshal(o)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	} */
	res.Header().Set("Last-Modified", lastModified.Format(time.RFC1123))
	res.Header().Set("Cache-Control", "no-store")
	res.Header().Set("ETag", o.LastEditID)
	// fmt.Fprint(res, string(s))
	if err = json.NewEncoder(res).Encode(&o); err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
}

func drawDeleteRoute(res http.ResponseWriter, req *http.Request) {
	gid, err := getAgentID(req)
	if err != nil {
		http.Error(res, jsonError(err), http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(req)
	var op model.Operation
	op.ID = model.OperationID(vars["opID"])

	// op.Delete checks ownership, do we need this check? -- yes for good status codes
	if !op.ID.IsOwner(gid) {
		err = fmt.Errorf("forbidden: only the owner can delete an operation")
		log.Warnw(err.Error(), "resource", op.ID, "GID", gid)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}

	if err := op.Delete(gid); err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	messaging.DeleteOperation(messaging.OperationID(op.ID)) // announces to EVERYONE to delete it
	log.Infow("deleted operation", "resource", op.ID, "GID", gid, "message", "deleted operation")
	fmt.Fprint(res, jsonStatusOK)
}

func drawUpdateRoute(res http.ResponseWriter, req *http.Request) {
	gid, err := getAgentID(req)
	if err != nil {
		http.Error(res, jsonError(err), http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(req)
	var op model.Operation
	op.ID = model.OperationID(vars["opID"])
	opID := op.ID // used to verify that the URL used is right for the op data

	if !contentTypeIs(req, jsonTypeShort) {
		err := fmt.Errorf("invalid request (needs to be application/json)")
		log.Warnw(err.Error(), "GID", gid, "resource", op.ID)
		http.Error(res, jsonError(err), http.StatusNotAcceptable)
		return
	}

	if !op.WriteAccess(gid) {
		err = fmt.Errorf("forbidden: write access required to update an operation")
		log.Warnw(err.Error(), "GID", gid, "resource", op.ID)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}

	s, err := op.ID.Stat()
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

	/* plan a
	jBlob, err := io.ReadAll(req.Body)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	if err := json.Unmarshal(json.RawMessage(jBlob), &op); err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	} */

	/* plan b */
	d := json.NewDecoder(req.Body)
	// d.DisallowUnknownFields()
	if err := d.Decode(&op); err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusNotAcceptable)
		return
	}
	/* plan b */

	if opID != op.ID { // after unmarshal
		err := fmt.Errorf("incoming op.ID does not match the URL specified ID: refusing update")
		log.Errorw(err.Error(), "resource", opID, "mismatch", opID)
		http.Error(res, jsonError(err), http.StatusNotAcceptable)
		return
	}

	err = model.DrawUpdate(&op, gid)
	if err != nil {
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	uid := touch(op)
	fmt.Fprint(res, jsonOKUpdateID(uid))

	// store backup revision -- used for testing
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
		defer fh.Close()

		if err = json.NewEncoder(fh).Encode(&op); err != nil {
			log.Error(err)
			return
		}
	}
}

func drawChownRoute(res http.ResponseWriter, req *http.Request) {
	gid, err := getAgentID(req)
	if err != nil {
		http.Error(res, jsonError(err), http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(req)
	to := vars["to"]
	// only the ID needs to be set for this
	var op model.Operation
	op.ID = model.OperationID(vars["opID"])

	if !op.ID.IsOwner(gid) {
		err = fmt.Errorf("forbidden: only the owner can set operation ownership ")
		log.Warnw(err.Error(), "GID", gid, "resource", op.ID)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}

	err = op.ID.Chown(gid, to)
	if err != nil {
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	// no notification
	fmt.Fprint(res, jsonStatusOK)
}

func drawPortalCommentRoute(res http.ResponseWriter, req *http.Request) {
	gid, err := getAgentID(req)
	if err != nil {
		http.Error(res, jsonError(err), http.StatusUnauthorized)
		return
	}

	// only the ID needs to be set for this
	vars := mux.Vars(req)
	var op model.Operation
	op.ID = model.OperationID(vars["opID"])

	if !op.WriteAccess(gid) {
		err = fmt.Errorf("write access required to set portal comments")
		log.Warnw(err.Error(), "GID", gid, "resource", op.ID)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}

	portalID := model.PortalID(vars["portal"])
	comment := req.FormValue("comment")
	err = op.ID.PortalComment(portalID, comment)
	if err != nil {
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	uid := touch(op)
	fmt.Fprint(res, jsonOKUpdateID(uid))
}

func drawPortalHardnessRoute(res http.ResponseWriter, req *http.Request) {
	gid, err := getAgentID(req)
	if err != nil {
		http.Error(res, jsonError(err), http.StatusUnauthorized)
		return
	}

	// only the ID needs to be set for this
	vars := mux.Vars(req)
	var op model.Operation
	op.ID = model.OperationID(vars["opID"])

	if op.WriteAccess(gid) {
		err = fmt.Errorf("write access required to set portal hardness")
		log.Warnw(err.Error(), "GID", gid, "resource", op.ID)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}
	portalID := model.PortalID(vars["portal"])
	hardness := req.FormValue("hardness")
	err = op.ID.PortalHardness(portalID, hardness)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	uid := touch(op)
	fmt.Fprint(res, jsonOKUpdateID(uid))
}

func drawOrderRoute(res http.ResponseWriter, req *http.Request) {
	gid, err := getAgentID(req)
	if err != nil {
		http.Error(res, jsonError(err), http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(req)
	var op model.Operation
	op.ID = model.OperationID(vars["opID"])

	if !op.WriteAccess(gid) {
		err = fmt.Errorf("write access required to set operation order")
		log.Warnw(err.Error(), "GID", gid, "resource", op.ID)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}

	order := req.FormValue("order")
	err = op.LinkOrder(order)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusUnauthorized)
		return
	}
	err = op.MarkerOrder(order)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusUnauthorized)
		return
	}
	uid := touch(op)
	fmt.Fprint(res, jsonOKUpdateID(uid))
}

func drawInfoRoute(res http.ResponseWriter, req *http.Request) {
	gid, err := getAgentID(req)
	if err != nil {
		http.Error(res, jsonError(err), http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(req)
	var op model.Operation
	op.ID = model.OperationID(vars["opID"])

	if !op.WriteAccess(gid) {
		err = fmt.Errorf("write access required to set operation info")
		log.Warnw(err.Error(), "GID", gid, "resource", op.ID)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}
	info := req.FormValue("info")
	err = op.SetInfo(info, gid)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	uid := touch(op)
	fmt.Fprint(res, jsonOKUpdateID(uid))
}

func drawPortalKeysRoute(res http.ResponseWriter, req *http.Request) {
	gid, err := getAgentID(req)
	if err != nil {
		http.Error(res, jsonError(err), http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(req)
	var op model.Operation
	op.ID = model.OperationID(vars["opID"])
	portalID := model.PortalID(vars["portal"])

	onhand, err := strconv.ParseInt(req.FormValue("count"), 10, 32)
	if err != nil { // user supplied non-numeric value
		onhand = 0
	}
	if onhand < 0 { // @Robely42 .... sigh
		onhand = 0
	}
	// cap out at 3k, even though 2600 is the one-user absolute limit
	// because Niantic will Niantic
	if onhand > 3000 {
		onhand = 3000
	}
	capsule := req.FormValue("capsule")

	err = op.KeyOnHand(gid, portalID, int32(onhand), capsule)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusNotAcceptable)
		return
	}

	uid := touch(op)
	fmt.Fprint(res, jsonOKUpdateID(uid))
}

func drawPermsAddRoute(res http.ResponseWriter, req *http.Request) {
	gid, err := getAgentID(req)
	if err != nil {
		http.Error(res, jsonError(err), http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(req)
	var op model.Operation
	op.ID = model.OperationID(vars["opID"])

	if !op.ID.IsOwner(gid) {
		err = fmt.Errorf("permission to edit permissions denied")
		log.Warnw(err.Error(), "GID", gid, "resource", op.ID)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}

	teamID := model.TeamID(req.FormValue("team"))
	role := req.FormValue("role") // AddPerm verifies this is good
	if teamID == "" || role == "" {
		err = fmt.Errorf("required value not set to add permission to op")
		log.Warn(err)
		http.Error(res, jsonError(err), http.StatusNotAcceptable)
		return
	}
	// Pass in "Zeta" and get a zone back... defaults to "All"
	zone := model.ZoneFromString(req.FormValue("zone"))

	err = op.ID.AddPerm(gid, teamID, role, zone)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	uid := touch(op)
	fmt.Fprint(res, jsonOKUpdateID(uid))
}

func drawPermsDeleteRoute(res http.ResponseWriter, req *http.Request) {
	gid, err := getAgentID(req)
	if err != nil {
		http.Error(res, jsonError(err), http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(req)
	var op model.Operation
	op.ID = model.OperationID(vars["opID"])

	if !op.ID.IsOwner(gid) {
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

	err = op.ID.DelPerm(gid, teamID, role, zone)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	uid := touch(op)
	fmt.Fprint(res, jsonOKUpdateID(uid))
}

func jsonOKUpdateID(uid string) string {
	return fmt.Sprintf("{\"status\":\"ok\", \"updateID\": \"%s\"}", uid)
}

func touch(op model.Operation) string {
	// update the timestamp and updateID
	uid, err := op.Touch()
	if err != nil {
		return ""
	}

	// announce to all relevant teams
	go func() {
		teams := make(map[model.TeamID]bool)
		for _, t := range op.Teams {
			teams[t.TeamID] = true
		}
		var ta []model.TeamID
		for t := range teams {
			ta = append(ta, t)
		}
		if len(ta) > 0 {
			_ = wfb.MapChange(ta, op.ID, uid)
		}
	}()
	return uid
}
