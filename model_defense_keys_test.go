package wasabee_test

import (
	"github.com/wasabee-project/Wasabee-Server"
	"testing"
)

func TestDefensiveKeys(t *testing.T) {
	err := gid.InsertDefensiveKey(wasabee.DefensiveKey{
		GID:      gid,
		PortalID: "bogus",
		CapID:    "mykeys",
		Count:    4,
		Name:     "Preston Trail Ascending Columns",
		Lat:      "-33.111",
		Lon:      "-33.333",
	})

	if err != nil {
		t.Error(err)
	}

	// get teams set up to get any data out
	dkl, err := gid.ListDefensiveKeys()
	if err != nil {
		t.Error(err)
	}
	wasabee.Log.Debugf("%+v", dkl.DefensiveKeys)

	/* count := len(dkl.DefensiveKeys)
	if count < 1 {
		t.Errorf("too few defensive keys")
	} */

	err = gid.InsertDefensiveKey(wasabee.DefensiveKey{
		GID:      gid,
		PortalID: "bogus",
		CapID:    "mykeys",
		Count:    0,
		Name:     "doesn't matter",
		Lat:      "-0",
		Lon:      "0/0",
	})
	if err != nil {
		t.Error(err)
	}

	err = gid.InsertDefensiveKey(wasabee.DefensiveKey{
		GID:      gid,
		PortalID: "bad",
		CapID:    "",
		Count:    -99999,
		Name:     "doesn't matter",
		Lat:      "this should default to 0",
		Lon:      "garbage becomes 0",
	})
	if err != nil {
		t.Error(err)
	}
}
