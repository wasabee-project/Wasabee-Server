package wasabi_test

import (
	"fmt"
	"github.com/cloudkucooland/WASABI"
	"testing"
	"encoding/json"
	"io/ioutil"
)

// TestMain is currently in model_venlone_test.go

func TestOperation(t *testing.T) {
	// ID":"1aa847732063c58eaa956f365b6c030044c0f1aa","name":"April 2019 Lanes"
	gid := wasabi.GoogleID("118281765050946915735")
	content, err := ioutil.ReadFile("testdata/test1.json")
	if err != nil {
		t.Error(err.Error())
	}

	j := json.RawMessage(content)

	err = wasabi.PDrawInsert(j, gid)
	if err != nil {
		t.Error(err.Error())
	}

	var op wasabi.Operation
	op.ID = wasabi.OperationID("1aa847732063c58eaa956f365b6c030044c0f1aa")

	opp := &op
	opp.Populate(gid)
	out, err := json.MarshalIndent(&op, "", "  ")
	if err != nil {
		t.Error(err.Error())
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
