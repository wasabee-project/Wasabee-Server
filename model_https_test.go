package wasabi_test

import (
	"github.com/cloudkucooland/WASABI"
	"testing"
)

// nothing to do here, just run the code
func testHttps(t *testing.T) {
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
