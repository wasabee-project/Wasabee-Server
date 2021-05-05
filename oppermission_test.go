package wasabee_test

import (
	"github.com/wasabee-project/Wasabee-Server"
	"testing"
)

func TestOpPerm(t *testing.T) {
	bad := wasabee.OpPermRole("bad")
	good := wasabee.OpPermRole("read")

	if bad.Valid() != false {
		t.Errorf("bad perm showed as valid")
	}

	if !good.Valid() {
		t.Errorf("good perm showed as invalid")
	}
}
