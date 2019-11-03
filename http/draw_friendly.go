package wasabeehttps

import (
	"math"
	"sort"

	"github.com/wasabee-project/Wasabee-Server"
)

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
			if i == "" {
				i, _ = agentID.IngressName();
			}
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

/*
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
*/
