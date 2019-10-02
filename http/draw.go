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
		http.Error(res, jsonStatusEmpty, http.StatusNotAcceptable)
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

	if !o.ReadAccess(gid) {
		err := fmt.Errorf("permission denied (%s: %s)", gid, o.ID)
		wasabee.Log.Notice(err)
		http.Error(res, jsonError(err), http.StatusUnauthorized)
		return
	}

	if err = o.Populate(gid); err != nil {
		wasabee.Log.Notice(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	lastModified, err := time.ParseInLocation("2006-01-02 15:04:05", o.Modified, time.UTC)
	if err != nil {
		wasabee.Log.Error(err)
	}
	res.Header().Set("Last-Modified", lastModified.Format(time.RFC1123))

	var skipsending bool
	ims := req.Header.Get("If-Modified-Since")
	if ims != "" && ims != "null" { // yes, the string "null", seen in the wild
		modifiedSince, err := time.ParseInLocation(time.RFC1123, ims, time.UTC)
		if err != nil {
			wasabee.Log.Error(err)
		} else {
			if lastModified.Before(modifiedSince) {
				// wasabee.Log.Debugf("skipsending: if-modified-since: %s / last-modified: %s", modifiedSince.In(time.UTC), lastModified.In(time.UTC))
				skipsending = true
			}
		}
	}

	// JSON if referer is intel.ingress.com or the mobile app
	if strings.Contains(req.Referer(), "intel.ingress.com") || strings.Contains(req.Header.Get("User-Agent"), appUserAgent) {
		if skipsending {
			// wasabee.Log.Debugf("sending 304 for %s", o.ID)
			res.Header().Set("Content-Type", "")
			http.Redirect(res, req, "", http.StatusNotModified)
			return
		}
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

	// pretty output for everyone else -- based on access... or expressed preference
	res.Header().Set("Cache-Control", "no-cache") // if the HTML version gets cached, IITC freaks
	friendly, err := pDrawFriendlyNames(&o, gid)
	if err != nil {
		wasabee.Log.Error(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
	}

	template := "opinfo"
	if o.WriteAccess(gid) {
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
		err := op.Delete(gid)
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

	fmt.Fprint(res, jsonStatusOK)
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
		http.Error(res, jsonStatusEmpty, http.StatusNotAcceptable)
		return
	}

	jRaw := json.RawMessage(jBlob)

	// wasabee.Log.Debug(string(jBlob))
	if err = wasabee.DrawUpdate(wasabee.OperationID(id), jRaw, gid); err != nil {
		wasabee.Log.Notice(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	fmt.Fprint(res, jsonStatusOK)
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
	fmt.Fprint(res, jsonStatusOK)
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

	count := 0
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
		count++
		if count > 60 {
			break
		}
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
	Completed      bool
	Color          string
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
	Order        int
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
	friendly.Name = op.Name
	friendly.Color = op.Color
	friendly.Modified = op.Modified
	friendly.Comment = op.Comment
	friendly.Gid = op.Gid

	friendly.Agent, err = op.Gid.IngressName()
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
		fl.Color = l.Color
		fl.Completed = l.Completed
		fl.ThrowOrder = int32(l.ThrowOrder)
		fl.From = portals[l.From].Name
		fl.FromID = l.From
		fl.To = portals[l.To].Name
		fl.ToID = l.To
		fl.AssignedTo, _ = l.AssignedTo.IngressNameOperation(op)
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
		fm.AssignedTo, _ = m.AssignedTo.IngressNameOperation(op)
		fm.Order = m.Order
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
					tmpfkoh.Agent, _ = km.Gid.IngressNameOperation(op)
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
			i, _ := agentID.IngressNameOperation(op)
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

	if op.WriteAccess(gid) {
		link := wasabee.LinkID(vars["link"])
		agent := wasabee.GoogleID(req.FormValue("agent"))
		err := op.AssignLink(link, agent)
		if err != nil {
			wasabee.Log.Notice(err)
			http.Error(res, jsonError(err), http.StatusInternalServerError)
			return
		}
	} else {
		err = fmt.Errorf("write access required to assign agents")
		wasabee.Log.Notice(err)
		http.Error(res, jsonError(err), http.StatusUnauthorized)
		return
	}
	fmt.Fprint(res, jsonStatusOK)
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

	if op.WriteAccess(gid) {
		link := wasabee.LinkID(vars["link"])
		desc := req.FormValue("desc")
		err := op.LinkDescription(link, desc)
		if err != nil {
			wasabee.Log.Notice(err)
			http.Error(res, jsonError(err), http.StatusInternalServerError)
			return
		}
	} else {
		err = fmt.Errorf("write access required to set link descriptions")
		wasabee.Log.Notice(err)
		http.Error(res, jsonError(err), http.StatusUnauthorized)
		return
	}
	fmt.Fprint(res, jsonStatusOK)
}

func pDrawLinkColorRoute(res http.ResponseWriter, req *http.Request) {
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

	if op.WriteAccess(gid) {
		link := wasabee.LinkID(vars["link"])
		color := req.FormValue("color")
		err := op.LinkColor(link, color)
		if err != nil {
			wasabee.Log.Notice(err)
			http.Error(res, jsonError(err), http.StatusInternalServerError)
			return
		}
	} else {
		err = fmt.Errorf("write access required to set link color")
		wasabee.Log.Notice(err)
		http.Error(res, jsonError(err), http.StatusUnauthorized)
		return
	}
	fmt.Fprint(res, jsonStatusOK)
}

func pDrawLinkSwapRoute(res http.ResponseWriter, req *http.Request) {
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

	if op.WriteAccess(gid) {
		link := wasabee.LinkID(vars["link"])
		err := op.LinkSwap(link)
		if err != nil {
			wasabee.Log.Notice(err)
			http.Error(res, jsonError(err), http.StatusInternalServerError)
			return
		}
	} else {
		err = fmt.Errorf("write access required swap link order")
		wasabee.Log.Notice(err)
		http.Error(res, jsonError(err), http.StatusUnauthorized)
		return
	}
	fmt.Fprint(res, jsonStatusOK)
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
	if !op.WriteAccess(gid) && !op.ID.AssignedTo(link, gid) {
		err = fmt.Errorf("permission to mark link as complete denied")
		http.Error(res, jsonError(err), http.StatusUnauthorized)
		return
	}

	err = op.LinkCompleted(link, complete)
	if err != nil {
		wasabee.Log.Notice(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	fmt.Fprint(res, jsonStatusOK)
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

	if op.WriteAccess(gid) {
		marker := wasabee.MarkerID(vars["marker"])
		agent := wasabee.GoogleID(req.FormValue("agent"))
		err := op.AssignMarker(marker, agent)
		if err != nil {
			wasabee.Log.Notice(err)
			http.Error(res, jsonError(err), http.StatusInternalServerError)
			return
		}
	} else {
		err = fmt.Errorf("write access required to assign targets")
		wasabee.Log.Notice(err)
		http.Error(res, jsonError(err), http.StatusUnauthorized)
		return
	}
	fmt.Fprint(res, jsonStatusOK)
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

	if op.WriteAccess(gid) {
		marker := wasabee.MarkerID(vars["marker"])
		comment := req.FormValue("comment")
		err := op.MarkerComment(marker, comment)
		if err != nil {
			wasabee.Log.Notice(err)
			http.Error(res, jsonError(err), http.StatusInternalServerError)
			return
		}
	} else {
		err = fmt.Errorf("write access required to set marker comments")
		wasabee.Log.Notice(err)
		http.Error(res, jsonError(err), http.StatusUnauthorized)
		return
	}
	fmt.Fprint(res, jsonStatusOK)
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

	if op.WriteAccess(gid) {
		portalID := wasabee.PortalID(vars["portal"])
		comment := req.FormValue("comment")
		err := op.PortalComment(portalID, comment)
		if err != nil {
			wasabee.Log.Notice(err)
			http.Error(res, jsonError(err), http.StatusInternalServerError)
			return
		}
	} else {
		err = fmt.Errorf("write access required to set portal comments")
		wasabee.Log.Notice(err)
		http.Error(res, jsonError(err), http.StatusUnauthorized)
		return
	}
	fmt.Fprint(res, jsonStatusOK)
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

	if op.WriteAccess(gid) {
		portalID := wasabee.PortalID(vars["portal"])
		hardness := req.FormValue("hardness")
		err := op.PortalHardness(portalID, hardness)
		if err != nil {
			wasabee.Log.Notice(err)
			http.Error(res, jsonError(err), http.StatusInternalServerError)
			return
		}
	} else {
		err = fmt.Errorf("write access required to set portal hardness")
		wasabee.Log.Notice(err)
		http.Error(res, jsonError(err), http.StatusUnauthorized)
		return
	}
	fmt.Fprint(res, jsonStatusOK)
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
	var op wasabee.Operation
	op.ID = wasabee.OperationID(vars["document"])

	if !op.ReadAccess(gid) {
		err = fmt.Errorf("read access required to view portal details")
		wasabee.Log.Notice(err)
		http.Error(res, jsonError(err), http.StatusUnauthorized)
		return
	}

	portalID := wasabee.PortalID(vars["portal"])
	portal, err := op.PortalDetails(portalID, gid)
	friendlyPortal.ID = portal.ID
	friendlyPortal.OpID = op.ID
	friendlyPortal.OpOwner = op.ID.IsOwner(gid)
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
	var op wasabee.Operation
	op.ID = wasabee.OperationID(vars["document"])

	if op.WriteAccess(gid) {
		order := req.FormValue("order")
		err = op.LinkOrder(order, gid)
		if err != nil {
			wasabee.Log.Notice(err)
			http.Error(res, jsonError(err), http.StatusInternalServerError)
			return
		}
	} else {
		err = fmt.Errorf("write access required to set link order")
		wasabee.Log.Notice(err)
		http.Error(res, jsonError(err), http.StatusUnauthorized)
		return
	}

	fmt.Fprint(res, jsonStatusOK)
}

func pDrawMarkerOrderRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Set("Content-Type", jsonType)
	gid, err := getAgentID(req)
	if err != nil {
		wasabee.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	var op wasabee.Operation
	op.ID = wasabee.OperationID(vars["document"])

	if op.WriteAccess(gid) {
		order := req.FormValue("order")
		err = op.MarkerOrder(order, gid)
		if err != nil {
			wasabee.Log.Notice(err)
			http.Error(res, jsonError(err), http.StatusInternalServerError)
			return
		}
	} else {
		err = fmt.Errorf("write access required to set marker order")
		wasabee.Log.Notice(err)
		http.Error(res, jsonError(err), http.StatusUnauthorized)
		return
	}

	fmt.Fprint(res, jsonStatusOK)
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
	var op wasabee.Operation
	op.ID = wasabee.OperationID(vars["document"])

	if op.WriteAccess(gid) {
		info := req.FormValue("info")
		err = op.SetInfo(info, gid)
		if err != nil {
			wasabee.Log.Notice(err)
			http.Error(res, jsonError(err), http.StatusInternalServerError)
			return
		}
	} else {
		err = fmt.Errorf("write access required to set operation info")
		wasabee.Log.Notice(err)
		http.Error(res, jsonError(err), http.StatusUnauthorized)
		return
	}

	fmt.Fprint(res, jsonStatusOK)
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
	var op wasabee.Operation
	op.ID = wasabee.OperationID(vars["document"])
	portalID := wasabee.PortalID(vars["portal"])
	onhand, err := strconv.Atoi(req.FormValue("onhand"))
	if err != nil { // user supplied non-numeric value
		onhand = 0
	}
	if onhand < 0 { // @Robely42 .... sigh
		onhand = 0
	}
	err = op.KeyOnHand(gid, portalID, int32(onhand))
	if err != nil {
		wasabee.Log.Notice(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	fmt.Fprint(res, jsonStatusOK)
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
	var op wasabee.Operation
	op.ID = wasabee.OperationID(vars["document"])
	markerID := wasabee.MarkerID(vars["marker"])
	err = markerID.Complete(op, gid)
	if err != nil {
		wasabee.Log.Notice(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	fmt.Fprint(res, jsonStatusOK)
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
	var op wasabee.Operation
	op.ID = wasabee.OperationID(vars["document"])
	markerID := wasabee.MarkerID(vars["marker"])
	err = markerID.Incomplete(op, gid)
	if err != nil {
		wasabee.Log.Notice(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	fmt.Fprint(res, jsonStatusOK)
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
	var op wasabee.Operation
	op.ID = wasabee.OperationID(vars["document"])
	markerID := wasabee.MarkerID(vars["marker"])
	err = markerID.Reject(&op, gid)
	if err != nil {
		wasabee.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	fmt.Fprint(res, jsonStatusOK)
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
	var op wasabee.Operation
	op.ID = wasabee.OperationID(vars["document"])
	markerID := wasabee.MarkerID(vars["marker"])
	err = markerID.Acknowledge(&op, gid)
	if err != nil {
		wasabee.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	fmt.Fprint(res, jsonStatusOK)
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
	var op wasabee.Operation
	op.ID = wasabee.OperationID(vars["document"])
	s, err := op.ID.Stat()
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
	var op wasabee.Operation
	op.ID = wasabee.OperationID(vars["document"])

	var a wasabee.Assignments
	err = gid.Assignments(op.ID, &a)
	if err != nil {
		wasabee.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	lls := "https://maps.google.com/maps/dir/?api=1"
	stops := len(a.Links) - 1

	if stops < 1 {
		res.Header().Set("Content-Type", jsonType)
		fmt.Fprint(res, `{ "status": "no assignments" }`)
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

type friendlyPerms struct {
	ID          wasabee.OperationID
	Gid         wasabee.GoogleID // needed for TeamMenu GUI
	Permissions []friendlyPerm
}

type friendlyPerm struct {
	TeamID   wasabee.TeamID
	Role     string
	TeamName string
}

func pDrawPermsRoute(res http.ResponseWriter, req *http.Request) {
	gid, err := getAgentID(req)
	if err != nil {
		wasabee.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	var op wasabee.Operation
	op.ID = wasabee.OperationID(vars["document"])

	if !op.ReadAccess(gid) {
		err = fmt.Errorf("permission to view permissions denied")
		wasabee.Log.Notice(err)
		http.Error(res, jsonError(err), http.StatusUnauthorized)
		return
	}

	op.PopulateTeams()
	var fp friendlyPerms
	fp.ID = op.ID
	fp.Gid = gid
	for _, v := range op.Teams {
		tmp, _ := v.TeamID.Name()
		tmpFp := friendlyPerm{
			TeamID:   v.TeamID,
			Role:     string(v.Role),
			TeamName: string(tmp),
		}
		fp.Permissions = append(fp.Permissions, tmpFp)
	}

	if err = templateExecute(res, req, "opperms", fp); err != nil {
		wasabee.Log.Error(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
	}
}

func pDrawPermsAddRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Set("Content-Type", jsonType)

	gid, err := getAgentID(req)
	if err != nil {
		wasabee.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	var op wasabee.Operation
	op.ID = wasabee.OperationID(vars["document"])

	if !op.ID.IsOwner(gid) {
		err = fmt.Errorf("permission to edit permissions permissions denied")
		wasabee.Log.Notice(err)
		http.Error(res, jsonError(err), http.StatusUnauthorized)
		return
	}

	teamID := wasabee.TeamID(req.FormValue("team"))
	role := req.FormValue("role")
	if teamID == "" || role == "" {
		err = fmt.Errorf("required value not set")
		wasabee.Log.Debug(err)
		http.Error(res, jsonError(err), http.StatusNotAcceptable)
		return
	}

	if err := op.AddPerm(gid, teamID, role); err != nil {
		wasabee.Log.Notice(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	if strings.Contains(req.Referer(), "intel.ingress.com") || strings.Contains(req.Header.Get("User-Agent"), appUserAgent) {
		fmt.Fprint(res, jsonStatusOK)
		return
	}
	url := fmt.Sprintf("%s/draw/%s/perms", apipath, op.ID)
	http.Redirect(res, req, url, http.StatusFound)
}

func pDrawPermsDeleteRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Set("Content-Type", jsonType)

	gid, err := getAgentID(req)
	if err != nil {
		wasabee.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(req)
	var op wasabee.Operation
	op.ID = wasabee.OperationID(vars["document"])

	if !op.ID.IsOwner(gid) {
		err = fmt.Errorf("permission to edit permissions permissions denied")
		wasabee.Log.Notice(err)
		http.Error(res, jsonError(err), http.StatusUnauthorized)
		return
	}

	teamID := wasabee.TeamID(req.FormValue("team"))
	role := req.FormValue("role")
	if teamID == "" || role == "" {
		err = fmt.Errorf("required value not set")
		wasabee.Log.Debug(err)
		http.Error(res, jsonError(err), http.StatusNotAcceptable)
		return
	}

	if err := op.DelPerm(gid, teamID, role); err != nil {
		wasabee.Log.Notice(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	if strings.Contains(req.Referer(), "intel.ingress.com") || strings.Contains(req.Header.Get("User-Agent"), appUserAgent) {
		fmt.Fprint(res, jsonStatusOK)
		return
	}
	url := fmt.Sprintf("%s/draw/%s/perms", apipath, op.ID)
	http.Redirect(res, req, url, http.StatusFound)
}
