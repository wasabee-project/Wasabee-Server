package wasabi_test

import (
	"github.com/cloudkucooland/WASABI"
	"testing"
)

// nothing to do here, just run the code
func TestHttps(t *testing.T) {
	wasabi.SetWebroot("testing")
	b, err := wasabi.GetWebroot()
	if err != nil {
		t.Errorf(err.Error())
	}
	if b != "testing" {
		t.Errorf("set/get webroot mismatch")
	}
	wasabi.SetWebAPIPath("testing")
	b, err = wasabi.GetWebAPIPath()
	if err != nil {
		t.Errorf(err.Error())
	}
	if b != "testing" {
		t.Errorf("set/get web api path mismatch")
	}
}

func TestRouter(t *testing.T) {
	r := wasabi.NewRouter()
	s := wasabi.NewRouter()

	// there should only ever be one top router
	if r != s {
		t.Errorf("multiple top routers created")
	}

	x := wasabi.Subrouter("/X")
	y := wasabi.Subrouter("/Y")
	if x == y {
		t.Errorf("subrouters stepped on each other")
	}
}
