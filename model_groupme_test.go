package wasabi_test

import (
	"github.com/cloudkucooland/WASABI"
	"testing"
)

// nothing to do here, just run the code
func TestGM(t *testing.T) {
	wasabi.GMSetBot()
	b, err := wasabi.GMRunning()
	if err != nil {
		t.Errorf(err.Error())
	}
	if !b {
		t.Errorf(err.Error())
	}
}
