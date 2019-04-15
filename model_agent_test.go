package WASABI_test

import (
	"github.com/cloudkucooland/WASABI"
	"testing"
)

// TestMain is currently in model_venlone_test.go

func TestInitAgent(t *testing.T) {
	gid := WASABI.GoogleID("118281765050946915735")
	_, err := gid.InitAgent()
	if err != nil {
		t.Errorf(err.Error())
	}
}
