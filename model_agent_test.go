package wasabi_test

import (
	// "fmt"
	"github.com/cloudkucooland/WASABI"
	"github.com/op/go-logging"
	"testing"
)

// TestMain is currently in model_venlone_test.go

func TestInitAgent(t *testing.T) {
	wasabi.SetLogLevel(logging.DEBUG)
	gid := wasabi.GoogleID("118281765050946915735")
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
	gid := wasabi.GoogleID("118281765050946915735")
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

// xxx move to model_team_test.go
func TestNewTeam(t *testing.T) {
	gid := wasabi.GoogleID("118281765050946915735")
	teamID, err := gid.NewTeam("Test Team")
	if err != nil {
		t.Error(err.Error())
	}
	// fmt.Printf("TeamID: %s", teamID.String())
	q, err := gid.AgentInTeam(teamID, false)
	if err != nil {
		t.Error(err.Error())
	}
	if q == false {
		t.Error("team creator not in team")
	}
	err = teamID.Rename("maeT tseT")
	if err != nil {
		t.Error(err.Error())
	}
	var td wasabi.TeamData
	err = teamID.FetchTeam(&td, true)
	if err != nil {
		t.Error(err.Error())
	}
	err = teamID.FetchTeam(&td, false)
	if err != nil {
		t.Error(err.Error())
	}

	err = gid.SetTeamState(teamID, "Off")
	if err != nil {
		t.Error(err.Error())
	}
	err = gid.SetTeamState(teamID, "On")
	if err != nil {
		t.Error(err.Error())
	}
	err = gid.SetTeamState(teamID, "Primary")
	if err != nil {
		t.Error(err.Error())
	}
	p, err := gid.PrimaryTeam()
	if err != nil {
		t.Error(err.Error())
	}
	if p != teamID.String() {
		t.Errorf("Primary team test fail: %s / %s", p, teamID.String())
	}

	err = gid.SetTeamState(teamID, "Wombat")
	if err != nil { // if err == nil, fail because this value is junk
		t.Error(err.Error())
	}

	err = teamID.Delete()
	if err != nil {
		t.Error(err.Error())
	}

	return
}
