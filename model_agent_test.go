package wasabi_test

import (
	"github.com/cloudkucooland/WASABI"
	"testing"
)

func TestInitAgent(t *testing.T) {
	_, err := gid.InitAgent()
	if err != nil {
		t.Errorf(err.Error())
	}

	err = gid.StatusLocationEnable()
	if err != nil {
		t.Errorf(err.Error())
	}
	err = gid.StatusLocationDisable()
	if err != nil {
		t.Errorf(err.Error())
	}

	var ad wasabi.AgentData
	err = gid.GetAgentData(&ad)
	if err != nil {
		t.Errorf(err.Error())
	}
	// xxx check a value or two in ad
}

func TestSetAgentName(t *testing.T) {
	name, err := gid.IngressName()
	if err != nil {
		t.Errorf(err.Error())
	}

	err = gid.SetIngressName("TEST_AGENT_RENAME")
	if err != nil {
		t.Errorf(err.Error())
	}

	// since populated from V/Rocks, rename is rejected
	g2, err := wasabi.SearchAgentName(name)
	if err != nil {
		t.Errorf(err.Error())
	}
	if g2.String() != gid.String() {
		t.Errorf("gid mismatch after rename: %s %s", gid.String(), g2.String())
	}

	err = gid.SetIngressName(name)
	if err != nil {
		t.Errorf(err.Error())
	}
}

func TestAgentLocation(t *testing.T) {
	err := gid.AgentLocation("33.148", "-96.787", "test.go")
	if err != nil {
		t.Errorf(err.Error())
	}
}

func TestAgentDelete(t *testing.T) {
	// special case google ID that is not really used
	ngid := wasabi.GoogleID("104743827901423568948")

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
