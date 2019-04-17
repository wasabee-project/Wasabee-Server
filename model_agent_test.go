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
	err := gid.SetIngressName("dEvIoUs")
	if err != nil {
		t.Errorf(err.Error())
	}

	// since populated from V/Rocks, rename is rejected
	g2, err := wasabi.SearchAgentName("deviousness")
	if err != nil {
		t.Errorf(err.Error())
	}
	if g2.String() != gid.String() {
		t.Errorf("gid mismatch after rename: %s %s", gid.String(), g2.String())
	}

	err = gid.SetIngressName("devioiusness")
	if err != nil {
		t.Errorf(err.Error())
	}
}
