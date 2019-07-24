package wasabeehttps

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/wasabee-project/Wasabee-Server"
)

func pDrawUploadRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Set("Content-Type", jsonType)

	gid, err := getAgentID(req)
	if err != nil {
		wasabee.Log.Notice(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	contentType := strings.Split(strings.Replace(strings.ToLower(req.Header.Get("Content-Type")), " ", "", -1), ";")[0]
	if contentType != jsonTypeShort {
		http.Error(res, "Invalid request (needs to be application/json)", http.StatusNotAcceptable)
		return
	}

	// defer req.Body.Close()
	jBlob, err := ioutil.ReadAll(req.Body)
	if err != nil {
		wasabee.Log.Notice(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	if string(jBlob) == "" {
		wasabee.Log.Notice("empty JSON")
		http.Error(res, `{ "status": "error", "error": "Empty JSON" }`, http.StatusNotAcceptable)
		return
	}

	jRaw := json.RawMessage(jBlob)
	// wasabee.Log.Debugf("sent json: %s", string(jRaw))
	if err = wasabee.DrawInsert(jRaw, gid); err != nil {
		wasabee.Log.Notice(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	// the IITC plugin wants the full /me data on draw POST
	var ad wasabee.AgentData
	if err = gid.GetAgentData(&ad); err != nil {
		wasabee.Log.Notice(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	data, _ := json.Marshal(ad)
	fmt.Fprint(res, string(data))
}

func pDrawGetRoute(res http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	id := vars["document"]

	gid, err := getAgentID(req)
	if err != nil {
		wasabee.Log.Notice(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	var o wasabee.Operation
	o.ID = wasabee.OperationID(id)
	if err = o.Populate(gid); err != nil {
		wasabee.Log.Notice(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	var newer bool
	ims := req.Header.Get("If-Modified-Since")
	if ims != "" {
		// XXX use http.ParseTime?
		d, err := time.Parse(time.RFC1123, ims)
		if err != nil {
			wasabee.Log.Error(err)
		} else {
			wasabee.Log.Debug("if-modified-since: %s", d)
			m, err := time.Parse("2006-01-02 15:04:05", o.Modified)
			if err != nil {
				wasabee.Log.Error(err)
			} else if d.Before(m) {
				newer = true
			}
		}
	}

	method := req.Header.Get("Method")
	if newer && method == "HEAD" {
		wasabee.Log.Debug("HEAD with 302")
		res.Header().Set("Content-Type", "")          // disable the default output
		http.Redirect(res, req, "", http.StatusFound) // XXX redirect to nothing?
		return
	}

	// JSON if referer is intel.ingress.com
	if strings.Contains(req.Referer(), "intel.ingress.com") || strings.Contains(req.Header.Get("User-Agent"), appUserAgent) {
		res.Header().Set("Content-Type", jsonType)
		s, err := json.MarshalIndent(o, "", "\t")
		if err != nil {
			wasabee.Log.Notice(err)
			http.Error(res, jsonError(err), http.StatusInternalServerError)
			return
		}
		fmt.Fprint(res, string(s))
		return
	}

	// pretty output for everyone else
	friendly, err := pDrawFriendlyNames(&o, gid)
	if err != nil {
		wasabee.Log.Error(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
	}

	template := "opinfo"
	if gid == o.Gid {
		template = "opdata"
	}
	lite := req.FormValue("lite")
	if lite == "y" {
		template = "opinfo"
	}

	if err = templateExecute(res, req, template, friendly); err != nil {
		wasabee.Log.Error(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
	}
}

func pDrawDeleteRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Set("Content-Type", jsonType)

	gid, err := getAgentID(req)
	if err != nil {
		wasabee.Log.Notice(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)

	// only the ID needs to be set for this
	var op wasabee.Operation
	op.ID = wasabee.OperationID(vars["document"])

	// op.Delete checks ownership, do we need this check? -- yes for good status codes
	if op.ID.IsOwner(gid) {
		err = fmt.Errorf("deleting operation %s", op.ID)
		wasabee.Log.Notice(err)
		err := op.ID.Delete(gid, false)
		if err != nil {
			wasabee.Log.Notice(err)
			http.Error(res, jsonError(err), http.StatusInternalServerError)
			return
		}
	} else {
		err = fmt.Errorf("only the owner can delete an operation")
		wasabee.Log.Notice(err)
		http.Error(res, jsonError(err), http.StatusUnauthorized)
		return
	}

	fmt.Fprintf(res, `{ "status": "ok" }`)
}

func jsonError(e error) string {
	return fmt.Sprintf("{ \"status\": \"error\", \"error\": \"%s\" }", e.Error())
}

func pDrawUpdateRoute(res http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	id := vars["document"]

	res.Header().Set("Content-Type", jsonType)

	gid, err := getAgentID(req)
	if err != nil {
		wasabee.Log.Notice(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	contentType := strings.Split(strings.Replace(strings.ToLower(req.Header.Get("Content-Type")), " ", "", -1), ";")[0]
	if contentType != jsonTypeShort {
		http.Error(res, "Invalid request (needs to be application/json)", http.StatusNotAcceptable)
		return
	}

	// defer req.Body.Close()
	jBlob, err := ioutil.ReadAll(req.Body)
	if err != nil {
		wasabee.Log.Notice(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	if string(jBlob) == "" {
		wasabee.Log.Notice("empty JSON")
		http.Error(res, `{ "status": "error", "error": "Empty JSON" }`, http.StatusNotAcceptable)
		return
	}

	jRaw := json.RawMessage(jBlob)

	// wasabee.Log.Debug(string(jBlob))
	if err = wasabee.DrawUpdate(id, jRaw, gid); err != nil {
		wasabee.Log.Notice(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	fmt.Fprintf(res, `{ "status": "ok" }`)
}

func pDrawChownRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Set("Content-Type", jsonType)

	vars := mux.Vars(req)
	to := vars["to"]

	gid, err := getAgentID(req)
	if err != nil {
		wasabee.Log.Notice(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	// only the ID needs to be set for this
	var op wasabee.Operation
	op.ID = wasabee.OperationID(vars["document"])

	if op.ID.IsOwner(gid) {
		err = op.ID.Chown(gid, to)
		if err != nil {
			wasabee.Log.Notice(err)
			http.Error(res, jsonError(err), http.StatusInternalServerError)
			return
		}
	} else {
		err = fmt.Errorf("only the owner can set operation ownership ")
		wasabee.Log.Notice(err)
		http.Error(res, jsonError(err), http.StatusUnauthorized)
		return
	}
	fmt.Fprintf(res, `{ "status": "ok" }`)
}

func pDrawChgrpRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Set("Content-Type", jsonType)

	vars := mux.Vars(req)
	to := wasabee.TeamID(vars["team"])

	gid, err := getAgentID(req)
	if err != nil {
		wasabee.Log.Notice(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	// only the ID needs to be set for this
	var op wasabee.Operation
	op.ID = wasabee.OperationID(vars["document"])

	if op.ID.IsOwner(gid) {
		err = op.ID.Chgrp(gid, to)
		if err != nil {
			wasabee.Log.Notice(err)
			http.Error(res, jsonError(err), http.StatusInternalServerError)
			return
		}
	} else {
		err = fmt.Errorf("only the owner can set operation team")
		wasabee.Log.Notice(err)
		http.Error(res, jsonError(err), http.StatusUnauthorized)
		return
	}
	fmt.Fprintf(res, `{ "status": "ok" }`)
}

func pDrawStockRoute(res http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	id := vars["document"]

	gid, err := getAgentID(req)
	if err != nil {
		wasabee.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	var o wasabee.Operation
	o.ID = wasabee.OperationID(id)
	if err = o.Populate(gid); err != nil {
		wasabee.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	url := "https://intel.ingress.com/intel/?z=13&ll="

	type latlon struct {
		lat string
		lon string
	}

	var portals = make(map[wasabee.PortalID]latlon)

	for _, p := range o.OpPortals {
		var l latlon
		l.lat = p.Lat
		l.lon = p.Lon
		portals[p.ID] = l
	}

	var notfirst bool
	for _, l := range o.Links {
		x := portals[l.From]
		if notfirst {
			url += "_"
		} else {
			url += x.lat + "," + x.lon + "&pls="
			notfirst = true
		}
		url += x.lat + "," + x.lon + ","
		y := portals[l.To]
		url += y.lat + "," + y.lon
	}

	// wasabee.Log.Debugf("redirecting to :%s", url)
	http.Redirect(res, req, url, http.StatusFound)
}

type pdrawFriendly struct {
	ID       wasabee.OperationID
	Name     string
	Gid      wasabee.GoogleID
	Agent    string
	Color    string
	Modified string
	Comment  string
	TeamID   wasabee.TeamID
	Team     string
	Links    []friendlyLink
	Markers  []friendlyMarker
	Keys     []friendlyKeys
	Capsules []capsuleEntry
}

type friendlyLink struct {
	ID             wasabee.LinkID
	From           string
	FromID         wasabee.PortalID
	To             string
	ToID           wasabee.PortalID
	Desc           string
	AssignedTo     string
	AssignedToID   wasabee.GoogleID
	ThrowOrder     int32
	Distance       int32
	MinPortalLevel float64
}

type friendlyMarker struct {
	ID           wasabee.MarkerID
	Portal       string
	PortalID     wasabee.PortalID
	Type         wasabee.MarkerType
	Comment      string
	AssignedTo   string
	AssignedToID wasabee.GoogleID
	State        string
}

type friendlyKeys struct {
	ID         wasabee.PortalID
	Portal     string
	Required   int32
	Onhand     int32
	IHave      int32
	OnhandList []friendlyKeyOnHand
}

type friendlyKeyOnHand struct {
	ID     wasabee.PortalID
	Gid    wasabee.GoogleID
	Agent  string
	Onhand int32
}

type capsuleEntry struct {
	Gid      wasabee.GoogleID
	Agent    string
	ID       wasabee.PortalID
	Portal   string
	Required int32
}

// takes a populated op and returns a friendly named version
func pDrawFriendlyNames(op *wasabee.Operation, gid wasabee.GoogleID) (pdrawFriendly, error) {
	var err error
	var friendly pdrawFriendly
	friendly.ID = op.ID
	friendly.TeamID = op.TeamID
	friendly.Name = op.Name
	friendly.Color = op.Color
	friendly.Modified = op.Modified
	friendly.Comment = op.Comment
	friendly.Gid = op.Gid

	friendly.Agent, err = op.Gid.IngressName()
	if err != nil {
		return friendly, err
	}
	friendly.Team, err = op.TeamID.Name()
	if err != nil {
		return friendly, err
	}

	var portals = make(map[wasabee.PortalID]wasabee.Portal)

	for _, p := range op.OpPortals {
		portals[p.ID] = p
	}

	for _, l := range op.Links {
		var fl friendlyLink
		fl.ID = l.ID
		fl.Desc = l.Desc
		fl.ThrowOrder = int32(l.ThrowOrder)
		fl.From = portals[l.From].Name
		fl.FromID = l.From
		fl.To = portals[l.To].Name
		fl.ToID = l.To
		fl.AssignedTo, _ = l.AssignedTo.IngressName()
		fl.AssignedToID = l.AssignedTo
		tmp := wasabee.Distance(portals[l.From].Lat, portals[l.From].Lon, portals[l.To].Lat, portals[l.To].Lon)
		fl.Distance = int32(math.Round(tmp / 1000))
		fl.MinPortalLevel = wasabee.MinPortalLevel(tmp, 8, true)
		friendly.Links = append(friendly.Links, fl)
	}

	for _, m := range op.Markers {
		var fm friendlyMarker
		fm.ID = m.ID
		fm.State = m.State
		fm.AssignedToID = m.AssignedTo
		fm.Type = m.Type
		fm.Comment = m.Comment
		fm.PortalID = m.PortalID
		fm.Portal = portals[m.PortalID].Name
		fm.AssignedTo, _ = m.AssignedTo.IngressName()
		friendly.Markers = append(friendly.Markers, fm)
	}

	sort.Slice(friendly.Markers[:], func(i, j int) bool {
		return friendly.Markers[i].Portal < friendly.Markers[j].Portal
	})

	var keys = make(map[wasabee.PortalID]friendlyKeys)
	for _, l := range op.Links {
		_, ok := keys[l.To]
		if !ok {
			var onhandtmp, ihave int32
			var tmplist []friendlyKeyOnHand
			for _, km := range op.Keys {
				if km.ID == l.To {
					onhandtmp += km.Onhand
					var tmpfkoh friendlyKeyOnHand
					tmpfkoh.ID = km.ID
					tmpfkoh.Gid = km.Gid
					tmpfkoh.Onhand = km.Onhand
					tmpfkoh.Agent, _ = km.Gid.IngressName()
					tmplist = append(tmplist, tmpfkoh)
					if gid == km.Gid {
						ihave = km.Onhand
					}
				}
			}
			keys[l.To] = friendlyKeys{
				ID:         l.To,
				Portal:     portals[l.To].Name,
				Required:   1,
				Onhand:     onhandtmp,
				OnhandList: tmplist,
				IHave:      ihave,
			}
		} else {
			tmp := keys[l.To]
			tmp.Required++
			keys[l.To] = tmp
		}
	}
	for _, f := range keys {
		friendly.Keys = append(friendly.Keys, f)
	}
	sort.Slice(friendly.Keys[:], func(i, j int) bool {
		return friendly.Keys[i].Portal < friendly.Keys[j].Portal
	})

	var capsules = make(map[wasabee.GoogleID]map[wasabee.PortalID]capsuleEntry)
	for _, l := range op.Links {
		if l.AssignedTo != "" {
			if _, ok := capsules[l.AssignedTo]; ok {
				if _, ok := capsules[l.AssignedTo][l.To]; ok {
					tmp := capsules[l.AssignedTo][l.To]
					tmp.Required++
					capsules[l.AssignedTo][l.To] = tmp
				} else {
					capsules[l.AssignedTo][l.To] = capsuleEntry{
						Required: 1,
					}
				}
			} else {
				capsules[l.AssignedTo] = make(map[wasabee.PortalID]capsuleEntry)
				capsules[l.AssignedTo][l.To] = capsuleEntry{
					Required: 1,
				}
			}
		}
	}

	// now take capsules and do something with it
	var caps []capsuleEntry
	for agentID, entry := range capsules {
		for portalID, x := range entry {
			i, _ := agentID.IngressName()
			tmp := capsuleEntry{
				Gid:      agentID,
				Agent:    i,
				ID:       portalID,
				Portal:   portals[portalID].Name,
				Required: x.Required,
			}
			caps = append(caps, tmp)
		}
	}

	// XXX should also sort by portal name w/in each agent
	sort.Slice(caps[:], func(i, j int) bool {
		return caps[i].Agent < caps[j].Agent
	})

	friendly.Capsules = caps

	return friendly, nil
}

func pDrawLinkAssignRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Set("Content-Type", jsonType)

	gid, err := getAgentID(req)
	if err != nil {
		wasabee.Log.Notice(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	// only the ID needs to be set for this
	vars := mux.Vars(req)
	var op wasabee.Operation
	op.ID = wasabee.OperationID(vars["document"])

	if op.ID.IsOwner(gid) {
		link := wasabee.LinkID(vars["link"])
		agent := wasabee.GoogleID(req.FormValue("agent"))
		err := op.ID.AssignLink(link, agent)
		if err != nil {
			wasabee.Log.Notice(err)
			http.Error(res, jsonError(err), http.StatusInternalServerError)
			return
		}
	} else {
		err = fmt.Errorf("only the owner can assign agents")
		wasabee.Log.Notice(err)
		http.Error(res, jsonError(err), http.StatusUnauthorized)
		return
	}
	fmt.Fprintf(res, `{ "status": "ok" }`)
}

func pDrawLinkDescRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Set("Content-Type", jsonType)

	gid, err := getAgentID(req)
	if err != nil {
		wasabee.Log.Notice(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	// only the ID needs to be set for this
	vars := mux.Vars(req)
	var op wasabee.Operation
	op.ID = wasabee.OperationID(vars["document"])

	if op.ID.IsOwner(gid) {
		link := wasabee.LinkID(vars["link"])
		desc := req.FormValue("desc")
		err := op.ID.LinkDescription(link, desc)
		if err != nil {
			wasabee.Log.Notice(err)
			http.Error(res, jsonError(err), http.StatusInternalServerError)
			return
		}
	} else {
		err = fmt.Errorf("only the owner can set link descriptions")
		wasabee.Log.Notice(err)
		http.Error(res, jsonError(err), http.StatusUnauthorized)
		return
	}
	fmt.Fprintf(res, `{ "status": "ok" }`)
}

func pDrawLinkCompleteRoute(res http.ResponseWriter, req *http.Request) {
	pDrawLinkCompRoute(res, req, true)
}

func pDrawLinkIncompleteRoute(res http.ResponseWriter, req *http.Request) {
	pDrawLinkCompRoute(res, req, false)
}

func pDrawLinkCompRoute(res http.ResponseWriter, req *http.Request, complete bool) {
	res.Header().Set("Content-Type", jsonType)

	gid, err := getAgentID(req)
	if err != nil {
		wasabee.Log.Notice(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	// only the ID needs to be set for this
	vars := mux.Vars(req)
	var op wasabee.Operation
	op.ID = wasabee.OperationID(vars["document"])

	// operator / asignee
	link := wasabee.LinkID(vars["link"])
	if !op.ID.WriteAccess(gid) && !op.ID.AssignedTo(link, gid) {
		err = fmt.Errorf("permission to mark link as complete denied")
		http.Error(res, jsonError(err), http.StatusUnauthorized)
		return
	}

	err = op.ID.LinkCompleted(link, complete)
	if err != nil {
		wasabee.Log.Notice(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	fmt.Fprintf(res, `{ "status": "ok" }`)
}

func pDrawMarkerAssignRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Set("Content-Type", jsonType)

	gid, err := getAgentID(req)
	if err != nil {
		wasabee.Log.Notice(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	// only the ID needs to be set for this
	vars := mux.Vars(req)
	var op wasabee.Operation
	op.ID = wasabee.OperationID(vars["document"])

	if op.ID.IsOwner(gid) {
		marker := wasabee.MarkerID(vars["marker"])
		agent := wasabee.GoogleID(req.FormValue("agent"))
		err := op.ID.AssignMarker(marker, agent)
		if err != nil {
			wasabee.Log.Notice(err)
			http.Error(res, jsonError(err), http.StatusInternalServerError)
			return
		}
	} else {
		err = fmt.Errorf("only the owner can assign targets")
		wasabee.Log.Notice(err)
		http.Error(res, jsonError(err), http.StatusUnauthorized)
		return
	}
	fmt.Fprintf(res, `{ "status": "ok" }`)
}

func pDrawMarkerCommentRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Set("Content-Type", jsonType)

	gid, err := getAgentID(req)
	if err != nil {
		wasabee.Log.Notice(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	// only the ID needs to be set for this
	vars := mux.Vars(req)
	var op wasabee.Operation
	op.ID = wasabee.OperationID(vars["document"])

	if op.ID.IsOwner(gid) {
		marker := wasabee.MarkerID(vars["marker"])
		comment := req.FormValue("comment")
		err := op.ID.MarkerComment(marker, comment)
		if err != nil {
			wasabee.Log.Notice(err)
			http.Error(res, jsonError(err), http.StatusInternalServerError)
			return
		}
	} else {
		err = fmt.Errorf("only the owner set marker comments")
		wasabee.Log.Notice(err)
		http.Error(res, jsonError(err), http.StatusUnauthorized)
		return
	}
	fmt.Fprintf(res, `{ "status": "ok" }`)
}

func pDrawPortalCommentRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Set("Content-Type", jsonType)

	gid, err := getAgentID(req)
	if err != nil {
		wasabee.Log.Notice(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	// only the ID needs to be set for this
	vars := mux.Vars(req)
	var op wasabee.Operation
	op.ID = wasabee.OperationID(vars["document"])

	if op.ID.IsOwner(gid) {
		portalID := wasabee.PortalID(vars["portal"])
		comment := req.FormValue("comment")
		err := op.ID.PortalComment(portalID, comment)
		if err != nil {
			wasabee.Log.Notice(err)
			http.Error(res, jsonError(err), http.StatusInternalServerError)
			return
		}
	} else {
		err = fmt.Errorf("only the owner set portal comments")
		wasabee.Log.Notice(err)
		http.Error(res, jsonError(err), http.StatusUnauthorized)
		return
	}
	fmt.Fprintf(res, `{ "status": "ok" }`)
}

func pDrawPortalHardnessRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Set("Content-Type", jsonType)

	gid, err := getAgentID(req)
	if err != nil {
		wasabee.Log.Notice(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	// only the ID needs to be set for this
	vars := mux.Vars(req)
	var op wasabee.Operation
	op.ID = wasabee.OperationID(vars["document"])

	if op.ID.IsOwner(gid) {
		portalID := wasabee.PortalID(vars["portal"])
		hardness := req.FormValue("hardness")
		err := op.ID.PortalHardness(portalID, hardness)
		if err != nil {
			wasabee.Log.Notice(err)
			http.Error(res, jsonError(err), http.StatusInternalServerError)
			return
		}
	} else {
		err = fmt.Errorf("only the owner set portal hardness")
		wasabee.Log.Notice(err)
		http.Error(res, jsonError(err), http.StatusUnauthorized)
		return
	}
	fmt.Fprintf(res, `{ "status": "ok" }`)
}

func pDrawPortalRoute(res http.ResponseWriter, req *http.Request) {
	var friendlyPortal struct {
		ID       wasabee.PortalID
		OpID     wasabee.OperationID
		OpOwner  bool
		Name     string
		Lat      string
		Lon      string
		Comment  string
		Hardness string
	}

	gid, err := getAgentID(req)
	if err != nil {
		wasabee.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	opID := wasabee.OperationID(vars["document"])
	portalID := wasabee.PortalID(vars["portal"])
	portal, err := opID.PortalDetails(portalID, gid)
	friendlyPortal.ID = portal.ID
	friendlyPortal.OpID = opID
	friendlyPortal.OpOwner = opID.IsOwner(gid)
	friendlyPortal.Name = portal.Name
	friendlyPortal.Lat = portal.Lat
	friendlyPortal.Lon = portal.Lon
	friendlyPortal.Comment = portal.Comment
	friendlyPortal.Hardness = portal.Hardness

	if err != nil {
		wasabee.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	if err = templateExecute(res, req, "portaldata", friendlyPortal); err != nil {
		wasabee.Log.Error(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
	}
}

func pDrawOrderRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Set("Content-Type", jsonType)
	gid, err := getAgentID(req)
	if err != nil {
		wasabee.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	opID := wasabee.OperationID(vars["document"])
	if opID.IsOwner(gid) {
		order := req.FormValue("order")
		err = opID.LinkOrder(order, gid)
		if err != nil {
			wasabee.Log.Notice(err)
			http.Error(res, jsonError(err), http.StatusInternalServerError)
			return
		}
	} else {
		err = fmt.Errorf("only the owner set portal order")
		wasabee.Log.Notice(err)
		http.Error(res, jsonError(err), http.StatusUnauthorized)
		return
	}

	fmt.Fprintf(res, `{ "status": "ok" }`)
}

func pDrawInfoRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Set("Content-Type", jsonType)
	gid, err := getAgentID(req)
	if err != nil {
		wasabee.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	opID := wasabee.OperationID(vars["document"])
	if opID.IsOwner(gid) {
		info := req.FormValue("info")
		err = opID.SetInfo(info, gid)
		if err != nil {
			wasabee.Log.Notice(err)
			http.Error(res, jsonError(err), http.StatusInternalServerError)
			return
		}
	} else {
		err = fmt.Errorf("only the owner set operation info")
		wasabee.Log.Notice(err)
		http.Error(res, jsonError(err), http.StatusUnauthorized)
		return
	}

	fmt.Fprintf(res, `{ "status": "ok" }`)
}

func pDrawPortalKeysRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Set("Content-Type", jsonType)
	gid, err := getAgentID(req)
	if err != nil {
		wasabee.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	opID := wasabee.OperationID(vars["document"])
	portalID := wasabee.PortalID(vars["portal"])
	onhand, err := strconv.Atoi(req.FormValue("onhand"))
	if err != nil { // user supplied non-numeric value
		onhand = 0
	}
	if onhand < 0 { // @Robely42 .... sigh
		onhand = 0
	}
	err = opID.KeyOnHand(gid, portalID, int32(onhand))
	if err != nil {
		wasabee.Log.Notice(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	fmt.Fprintf(res, `{ "status": "ok" }`)
}

func pDrawMarkerCompleteRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Set("Content-Type", jsonType)
	gid, err := getAgentID(req)
	if err != nil {
		wasabee.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	opID := wasabee.OperationID(vars["document"])
	markerID := wasabee.MarkerID(vars["marker"])
	err = markerID.Complete(opID, gid)
	if err != nil {
		wasabee.Log.Notice(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	fmt.Fprintf(res, `{ "status": "ok" }`)
}

func pDrawMarkerIncompleteRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Set("Content-Type", jsonType)
	gid, err := getAgentID(req)
	if err != nil {
		wasabee.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	opID := wasabee.OperationID(vars["document"])
	markerID := wasabee.MarkerID(vars["marker"])
	err = markerID.Incomplete(opID, gid)
	if err != nil {
		wasabee.Log.Notice(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	fmt.Fprintf(res, `{ "status": "ok" }`)
}

func pDrawMarkerFinalizeRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Set("Content-Type", jsonType)
	gid, err := getAgentID(req)
	if err != nil {
		wasabee.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	opID := wasabee.OperationID(vars["document"])
	markerID := wasabee.MarkerID(vars["marker"])
	err = markerID.Finalize(opID, gid)
	if err != nil {
		wasabee.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	fmt.Fprintf(res, `{ "status": "ok" }`)
}

func pDrawMarkerRejectRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Set("Content-Type", jsonType)
	gid, err := getAgentID(req)
	if err != nil {
		wasabee.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	opID := wasabee.OperationID(vars["document"])
	markerID := wasabee.MarkerID(vars["marker"])
	err = markerID.Reject(opID, gid)
	if err != nil {
		wasabee.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	fmt.Fprintf(res, `{ "status": "ok" }`)
}

func pDrawMarkerAcknowledgeRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Set("Content-Type", jsonType)
	gid, err := getAgentID(req)
	if err != nil {
		wasabee.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	opID := wasabee.OperationID(vars["document"])
	markerID := wasabee.MarkerID(vars["marker"])
	err = markerID.Acknowledge(opID, gid)
	if err != nil {
		wasabee.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	fmt.Fprintf(res, `{ "status": "ok" }`)
}

func pDrawStatRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Set("Content-Type", jsonType)
	_, err := getAgentID(req)
	if err != nil {
		wasabee.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	opID := wasabee.OperationID(vars["document"])
	s, err := opID.Stat()
	if err != nil {
		wasabee.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	data, _ := json.Marshal(s)

	fmt.Fprint(res, string(data))
}

func pDrawMyRouteRoute(res http.ResponseWriter, req *http.Request) {
	gid, err := getAgentID(req)
	if err != nil {
		wasabee.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	opID := wasabee.OperationID(vars["document"])

	var a wasabee.Assignments
	err = gid.Assignments(opID, &a)
	if err != nil {
		wasabee.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	lls := "https://maps.google.com/maps/dir/?api=1"
	stops := len(a.Links) - 1

	if stops < 1 {
		res.Header().Set("Content-Type", jsonType)
		fmt.Fprintf(res, `{ "status": "no assignments" }`)
		return
	}

	for i, l := range a.Links {
		if i == 0 {
			lls = fmt.Sprintf("%s&origin=%s,%s&waypoints=", lls, a.Portals[l.From].Lat, a.Portals[l.From].Lon)
		}
		// Google only allows 9 waypoints
		// we could do something fancy and show every (n) link based on len(a.Links) / 8
		// this is good enough for now
		if i < 7 {
			lls = fmt.Sprintf("%s|%s,%s", lls, a.Portals[l.From].Lat, a.Portals[l.From].Lon)
		}
		if i == stops { // last one -- even if > 10 in list
			lls = fmt.Sprintf("%s&destination=%s,%s", lls, a.Portals[l.From].Lat, a.Portals[l.From].Lon)
		}
	}
	// wasabee.Log.Debug(lls)

	http.Redirect(res, req, lls, http.StatusFound)
}
