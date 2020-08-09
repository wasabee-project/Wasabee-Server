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
