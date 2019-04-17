package wasabi_test

import (
	"encoding/json"
	"fmt"
	"github.com/cloudkucooland/WASABI"
	"io/ioutil"
	"testing"
)

func TestOperation(t *testing.T) {
	content, err := ioutil.ReadFile("testdata/test1.json")
	if err != nil {
		t.Error(err.Error())
	}

	j := json.RawMessage(content)

	err = wasabi.PDrawInsert(j, gid)
	if err != nil {
		t.Error(err.Error())
	}

	var op, in wasabi.Operation

	err = json.Unmarshal(j, &in)
	if err != nil {
		t.Error(err.Error())
	}

	op.ID = in.ID

	opp := &op
	opp.Populate(gid)
	out, err := json.MarshalIndent(&op, "", "  ")
	if err != nil {
		t.Error(err.Error())
	}

	if opp.IsOwner(gid) != true {
		t.Error("wrong owner (*Operation)")
	}
	if opp.ID.IsOwner(gid) != true {
		t.Error("wrong owner (OperationID)")
	}

	err = opp.Delete()
	if err != nil {
		t.Error(err.Error())
	}

	err = opp.TeamID.Delete()
	if err != nil {
		t.Error(err.Error())
	}

	fmt.Print(string(out))
	return
}
