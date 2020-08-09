package wasabee_test

import (
	"encoding/json"
	// "fmt"
	"io/ioutil"
	"testing"

	"github.com/wasabee-project/Wasabee-Server"
)

func TestOperation(t *testing.T) {
	content, err := ioutil.ReadFile("testdata/test1.json")
	if err != nil {
		t.Error(err.Error())
	}
	j := json.RawMessage(content)
	err = wasabee.DrawInsert(j, gid)
	if err != nil {
		t.Error(err.Error())
	}
	var op, opx, opy, in wasabee.Operation

	err = json.Unmarshal(j, &in)
	if err != nil {
		t.Error(err.Error())
	}

	op.ID = in.ID
	opx.ID = in.ID
	opy.ID = in.ID
	opp := &op
	if err := opp.Populate(gid); err != nil {
		t.Error(err.Error())
	}
	newj, err := json.MarshalIndent(&op, "", "  ")
	if err != nil {
		t.Error(err.Error())
	}
	// fmt.Print(string(newj))

	// make some changes
	opp.KeyOnHand(gid, wasabee.PortalID("83c4d2bee503409cbfc76db98af4d749.16"), 7, "")
	opp.KeyOnHand(gid, wasabee.PortalID("2aa9e865ab8a4bb9896fb371281dcb7b.16"), 99, "cap")
	opp.PortalHardness("2aa9e865ab8a4bb9896fb371281dcb7b.16", "booster required")
	opp.PortalHardness("83c4d2bee503409cbfc76db98af4d749.16", "BGAN only")
	opp.PortalComment("83c4d2bee503409cbfc76db98af4d749.16", "testing a comment")
	// pull again
	opp = &opx
	if err := opp.Populate(gid); err != nil {
		t.Error(err.Error())
	}
	newj, err = json.MarshalIndent(opp, "", "  ")
	if err != nil {
		t.Error(err.Error())
	}
	// fmt.Print(string(newj))

	// run an update
	if err := wasabee.DrawUpdate(op.ID, newj, gid); err != nil {
		t.Error(err.Error())
	}

	// pull again
	opp = &opy
	if err := opp.Populate(gid); err != nil {
		t.Error(err.Error())
	}
	newj, err = json.MarshalIndent(opp, "", "  ")
	if err != nil {
		t.Error(err.Error())
	}
	// fmt.Print(string(newj))

	// random test
	if opp.ID.IsOwner(gid) != true {
		t.Error("wrong owner (OperationID)")
	}

	if len(in.OpPortals) != len(opp.OpPortals) {
		t.Error("wrong portal count")
	}
	if len(in.Links) != len(opp.Links) {
		t.Error("wrong link count")
	}
	if len(in.Anchors) != len(opp.Anchors) {
		t.Error("wrong anchor count")
	}
	if len(in.Markers) != len(opp.Markers) {
		t.Error("wrong marker count")
	}

	// delete it - team should go too
	if err := opp.Delete(gid); err != nil {
		t.Error(err.Error())
	}

	var a wasabee.Assignments
	if err := gid.Assignments(opp.ID, &a); err != nil {
		t.Error(err.Error())
	}
	wasabee.Log.Infof("assignments: \n%v", a)

	wasabee.Log.Info("TestOperation completed")
}

func TestDamagedOperation(t *testing.T) {
	wasabee.Log.Info("starting TestDamageOperation")

	content, err := ioutil.ReadFile("testdata/test3.json")
	if err != nil {
		t.Error(err.Error())
	}
	j := json.RawMessage(content)

	// this should give an error in debug output
	if err := wasabee.DrawInsert(j, gid); err != nil {
		t.Error(err.Error())
	}
	var in wasabee.Operation

	if err = json.Unmarshal(j, &in); err != nil {
		t.Error(err.Error())
	}

	opp := &in
	// does not print error for invalid portals
	opp.KeyOnHand(gid, wasabee.PortalID("83c4d2bee503409cbfc76db98af4d749.xx"), 7, "")

	content, err = ioutil.ReadFile("testdata/test3-update.json")
	if err != nil {
		t.Error(err.Error())
	}

	j = json.RawMessage(content)

	if err := wasabee.DrawUpdate(opp.ID, j, gid); err != nil {
		t.Error(err.Error())
	}

	wasabee.Log.Info("testing damaged op")
	if err := wasabee.DrawUpdate("wrong.id", j, gid); err != nil {
		wasabee.Log.Info("properly ignored update to 'random'")
		// t.Error(err.Error())
	}

	if err = opp.Delete(gid); err != nil {
		t.Error(err.Error())
	}
	wasabee.Log.Info("TestDamageOperation completed")
}
