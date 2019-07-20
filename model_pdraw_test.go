package wasabee_test

import (
	"encoding/json"
	"fmt"
	"github.com/wasabee-project/Wasabee-Server"
	"io/ioutil"
	"testing"
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

	var op, in wasabee.Operation

	err = json.Unmarshal(j, &in)
	if err != nil {
		t.Error(err.Error())
	}

	op.ID = in.ID

	opp := &op
	if err := opp.Populate(gid); err != nil {
		t.Error(err.Error())
	}

	out, err := json.MarshalIndent(&op, "", "  ")
	if err != nil {
		t.Error(err.Error())
	}

	if opp.ID.IsOwner(gid) != true {
		t.Error("wrong owner (OperationID)")
	}

	if err := opp.ID.Delete(gid, false); err != nil {
		t.Error(err.Error())
	}
	fmt.Print(string(out))
}

func TestDamagedOp(t *testing.T) {
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

	if err = opp.ID.Delete(gid, false); err != nil {
		t.Error(err.Error())
	}
}
