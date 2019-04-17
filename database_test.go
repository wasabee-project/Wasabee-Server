package wasabi_test

import (
	"github.com/cloudkucooland/WASABI"
	"testing"
)

func TestConnect(t *testing.T) {
	// assumes a whole host of other things already work
	// but needed to pass TestQuery on a new install (e.g. Travis-CI)
	_, err := gid.InitAgent()
	if err != nil {
		t.Errorf(err.Error())
	}
}

func TestQuery(t *testing.T) {
	fgid, err := wasabi.SearchAgentName("deviousness")
	if err != nil {
		t.Errorf(err.Error())
	}
	if gid.String() != fgid.String() {
		t.Error("did not find the correct gid for deviousness")
	}
}
