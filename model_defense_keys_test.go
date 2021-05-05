package wasabee_test

import (
	"github.com/wasabee-project/Wasabee-Server"
	"testing"
)

func TestDefensiveKeys(t *testing.T) {
	dk := wasabee.DefensiveKey{
		GID:      gid,
		PortalID: "bogus",
		CapID:    "mykeys",
		Count:    4,
		Name:     "Preston Trail Ascending Columns",
		Lat:      "33.111",
		Lon:      "three point three three",
	}

	err := gid.InsertDefensiveKey(dk)
	if err != nil {
		t.Error(err)
	}

	dkl, err := gid.ListDefensiveKeys()
	if err != nil {
		t.Error(err)
	}

	if len(dkl.DefensiveKeys) < 1 {
		t.Errorf("not enough defensive keys : %+v", dkl)
	}

}
