package wasabi_test

import (
	"github.com/cloudkucooland/WASABI"
	"testing"
)

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
	if td.Name != "maeT tseT" {
		t.Errorf("name change did not work: %s", td.Name)
	}
	if len(td.Agent) < 1 {
		t.Error("owner not in team")
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

	// err = gid.SetTeamState(teamID, "Wombat")
	//if err == nil {
	//	t.Error("SetTeamState did not return an error on a bad value")
	//}

	err = teamID.Delete()
	if err != nil {
		t.Error(err.Error())
	}

	return
}
