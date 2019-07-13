package wasabee_test

import (
	"github.com/wasabee-project/Wasabee-Server"
	"testing"
)

func TestTG(t *testing.T) {
	tgID, err := gid.TelegramID()
	if err != nil {
		t.Errorf(err.Error())
	}
	ngid, v, err := tgID.GidV()
	if err != nil {
		t.Errorf(err.Error())
	}
	if !v {
		t.Error("gid should be verified")
	}
	if ngid != gid {
		t.Error("gid -> tgID -> gid did not roundtrip correctly")
	}
	_ = tgID.String()

	_, err = wasabee.TGGetBotID()
	if err != nil {
		t.Errorf(err.Error())
	}

	// init/verify/delete bogus acccount
}
