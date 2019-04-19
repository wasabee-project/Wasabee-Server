package wasabi_test

import (
	"github.com/cloudkucooland/WASABI"
	"testing"
)

func TestConnect(t *testing.T) {
	// now taken care of in main
}

func TestQuery(t *testing.T) {
	// assumes a whole host of other things already work
	_, err := gid.InitAgent()
	if err != nil {
		t.Errorf(err.Error())
	}

	fgid, err := wasabi.SearchAgentName("deviousness")
	if err != nil {
		t.Errorf(err.Error())
	}
	if gid.String() != fgid.String() {
		t.Error("did not find the correct gid for deviousness")
	}
}
