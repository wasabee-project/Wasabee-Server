package wasabee_test

import (
	"github.com/wasabee-project/Wasabee-Server"
	"testing"
)

// nothing to do here, just run the code
func TestGM(t *testing.T) {
	wasabee.GMSetBot()
	b, err := wasabee.GMRunning()
	if err != nil {
		t.Errorf(err.Error())
	}
	if !b {
		t.Errorf(err.Error())
	}
}
