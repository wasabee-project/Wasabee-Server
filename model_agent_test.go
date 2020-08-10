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
	// xxx check a value or two in ad
}

func TestAgentRISCLock(t *testing.T) {
	if gid.RISC() {
		t.Errorf("RISC true, should be false")
	}

	if err := gid.Lock("test locking"); err != nil {
		t.Errorf(err.Error())
	}

	if !gid.RISC() {
		t.Errorf("RISC false, should be true")
	}

	if err := gid.Unlock("test unlock"); err != nil {
		t.Errorf(err.Error())
	}

	if gid.RISC() {
		t.Errorf("RISC true, should be false")
	}

	if err := gid.Unlock("test double unlock"); err != nil {
		t.Errorf(err.Error())
	}

	if gid.RISC() {
		t.Errorf("RISC true, should be false")
	}

	if err := gid.Lock("test relock"); err != nil {
		t.Errorf(err.Error())
	}

	if !gid.RISC() {
		t.Errorf("RISC false, should be true")
	}

	_, err := gid.InitAgent()
	if err == nil {
		t.Errorf("InitAgent permitted a locked agent through...")
	}

	if err := gid.Unlock("clearing locks"); err != nil {
		t.Errorf(err.Error())
	}
}

func TestAgentLogout(t *testing.T) {
	lo := gid.CheckLogout()
	if lo {
		t.Errorf("should be logged in")
	}
	gid.Logout("testing")
	lo = gid.CheckLogout()
	if !lo {
		t.Errorf("should be logged out")
	}
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

func TestLocKey(t *testing.T) {
	var ad wasabee.AgentData
	err := gid.GetAgentData(&ad)
	if err != nil {
		t.Errorf(err.Error())
	}

	if ad.LocationKey == "" {
		// t.Errorf("location key unset")
		wasabee.Log.Info("location key unset")
		ad.LocationKey, _ = gid.NewLocKey()
	}

	ngid, err := ad.LocationKey.Gid()
	if err != nil {
		t.Errorf(err.Error())
	}
	if ngid != gid {
		t.Errorf("unable to round-trip gid->lockey->gid")
	}

	ngid, err = wasabee.OneTimeToken(ad.LocationKey)
	if err != nil {
		t.Errorf(err.Error())
	}
	if ngid != gid {
		t.Errorf("OneTimeToken did not round-trip")
	}

	if _, err := wasabee.LocKey("bogus").Gid(); err == nil {
		t.Errorf("wrongly returned result for bogus LocKey.Gid()")
	}
}
