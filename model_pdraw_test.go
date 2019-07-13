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

	err = wasabee.PDrawInsert(j, gid)
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
	err = opp.Populate(gid)
	if err != nil {
		t.Error(err.Error())
	}

	out, err := json.MarshalIndent(&op, "", "  ")
	if err != nil {
		t.Error(err.Error())
	}

	if opp.ID.IsOwner(gid) != true {
		t.Error("wrong owner (OperationID)")
	}

	err = opp.Delete(gid, false)
	if err != nil {
		t.Error(err.Error())
	}

	err = opp.TeamID.Delete()
	if err != nil {
		t.Error(err.Error())
	}

	fmt.Print(string(out))
}
