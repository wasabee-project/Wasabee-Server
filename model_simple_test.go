package wasabi_test

import (
	"github.com/cloudkucooland/WASABI"
	"testing"
	"time"
)

func TestSimple(t *testing.T) {
	teststring := [...]struct {
		fail bool // true if should fail
		val  string
	}{
		{false, "This is some random content to be added to the database"},
		{true, "This is some random \x00 content which should fail"},
		// {true, ""}, // block empty strings?
		// {true, "  "}, // block empty strings?
	}
	var sd wasabi.SimpleDocument
	sd.Expiration = time.Unix(0, 0) // mark it as volitile so it self-destructs on first read

	for _, v := range teststring {
		sd.Content = v.val

		err := sd.Store()
		if !v.fail && err != nil {
			t.Errorf(err.Error())
		}
		if v.fail && err == nil {
			t.Error("a test which should have failed did not: [" + v.val + "]")
		}

		if !v.fail {
			rd, err := wasabi.Request(sd.ID)
			if err != nil {
				t.Errorf(err.Error())
			}
			if rd.Content != sd.Content {
				t.Errorf("SimpleDocument round trip failed: [" + v.val + "]")
			}
		}
	}
}
