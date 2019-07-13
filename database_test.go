package wasabee_test

import (
	"github.com/wasabee-project/Wasabee-Server"
	"testing"
)

func TestQuery(t *testing.T) {
	// assumes a whole host of other things already work
	_, err := gid.InitAgent()
	if err != nil {
		t.Errorf(err.Error())
	}

	fgid, err := wasabee.SearchAgentName("deviousness")
	if err != nil {
		t.Errorf(err.Error())
	}
	if gid.String() != fgid.String() {
		t.Error("did not find the correct gid for deviousness")
	}
}
