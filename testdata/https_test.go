package wasabee_test

import (
	"github.com/wasabee-project/Wasabee-Server"
	"testing"
)

// nothing to do here, just run the code
func TestHttps(t *testing.T) {
	wasabee.SetWebroot("testing")
	b, err := wasabee.GetWebroot()
	if err != nil {
		t.Errorf(err.Error())
	}
	if b != "testing" {
		t.Errorf("set/get webroot mismatch")
	}
	wasabee.SetWebAPIPath("testing")
	b, err = wasabee.GetWebAPIPath()
	if err != nil {
		t.Errorf(err.Error())
	}
	if b != "testing" {
		t.Errorf("set/get web api path mismatch")
	}
}

func TestRouter(t *testing.T) {
	r := wasabee.NewRouter()
	s := wasabee.NewRouter()

	// there should only ever be one top router
	if r != s {
		t.Errorf("multiple top routers created")
	}

	x := wasabee.Subrouter("/X")
	y := wasabee.Subrouter("/Y")
	if x == y {
		t.Errorf("subrouters stepped on each other")
	}
}
