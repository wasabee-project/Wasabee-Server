package wasabee_test

import (
	"github.com/wasabee-project/Wasabee-Server"
	"testing"
)

func TestInitAgent(t *testing.T) {
	_, err := gid.InitAgent()
	if err != nil {
		t.Errorf(err.Error())
	}

	gid.SetAgentName("deviousness")

	// no one should use StatusLocation now...
	err = gid.StatusLocationEnable()
	if err != nil {
		t.Errorf(err.Error())
	}
	err = gid.StatusLocationDisable()
	if err != nil {
		t.Errorf(err.Error())
	}

	var ad wasabee.AgentData
	err = gid.GetAgentData(&ad)
	if err != nil {
		t.Errorf(err.Error())
	}
	wasabee.Log.Infof("%v", ad)

	if ad.LocationKey == "" {
		// t.Errorf("location key unset")
		wasabee.Log.Info("location key unset")

		// this does nothing yet
		lk, err := wasabee.GenerateSafeName()
		if err != nil {
			t.Error(err.Error())
		}
		ad.LocationKey = wasabee.LocKey(lk)
	} else {
		ngid, err := ad.LocationKey.Gid()
		if err != nil {
			t.Errorf(err.Error())
		}
		if ngid != gid {
			t.Errorf("unable to round-trip gid->lockey->gid")
		}
	}

	// xxx check a value or two in ad
}

func TestAgentLocation(t *testing.T) {
	err := gid.AgentLocation("33.148", "-96.787")
	if err != nil {
		t.Errorf(err.Error())
	}
}

func TestAgentDelete(t *testing.T) {
	// special case google ID that is not really used
	ngid := wasabee.GoogleID("104743827901423568948")

	_, err := ngid.InitAgent()
	if err != nil {
		t.Errorf(err.Error())
	}

	// add to a team, do some stuff, etc...

	err = ngid.Delete()
	if err != nil {
		t.Errorf(err.Error())
	}
}

func TestToGid(t *testing.T) {
	tgid, err := wasabee.ToGid("")
	if err == nil {
		t.Errorf("failed to catch empty ToGid()")
	}
	wasabee.Log.Info(tgid)

	tgid, err = wasabee.ToGid(gid.String())
	if err != nil {
		t.Errorf(err.Error())
	}
	wasabee.Log.Info(tgid)

	if tgid != gid {
		t.Errorf("failed to roundtrip gid")
	}
	wasabee.Log.Info(tgid)

	// not in database, but still a GID
	tgid, err = wasabee.ToGid("104743827901423568948")
	if err != nil {
		t.Errorf(err.Error())
	}
	wasabee.Log.Info(tgid)

	// normal handle
	tgid, err = wasabee.ToGid("deviousness")
	if err != nil {
		t.Errorf(err.Error())
	}
	wasabee.Log.Info(tgid)

	// telegram ID
	tgid, err = wasabee.ToGid("@deviousness")
	if err != nil {
		t.Errorf(err.Error())
	}
	wasabee.Log.Info(tgid)

	// enl id
	tgid, err = wasabee.ToGid("23e27f48a04e55d6ae89188d3236d769f6629718")
	if err != nil {
		t.Errorf(err.Error())
	}
	wasabee.Log.Info(tgid)
}
