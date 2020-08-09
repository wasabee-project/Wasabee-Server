package wasabee_test

import (
	"fmt"
	"github.com/wasabee-project/Wasabee-Server"
	"testing"
)

var tids []wasabee.TeamID

func TestNewTeam(t *testing.T) {
	teamID, err := gid.NewTeam("Test Team")
	if err != nil {
		t.Error(err.Error())
	}
	wasabee.Log.Infof("New teamID: %s", teamID.String())

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

	err = teamID.SetRocks("", "example.com")
	if err != nil {
		t.Error(err.Error())
	}
	t2, err := wasabee.RocksTeamID("example.com")
	if err != nil {
		t.Error(err.Error())
	}
	if t2.String() != teamID.String() {
		t.Error("rocks community mismatch")
	}

	var td wasabee.TeamData
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
	if td.RocksComm != "example.com" {
		t.Error("rocks community mismatch (&td)")
	}

	err = gid.SetTeamState(teamID, "Off")
	if err != nil {
		t.Error(err.Error())
	}
	err = gid.SetTeamState(teamID, "On")
	if err != nil {
		t.Error(err.Error())
	}

	/*
		err = gid.SetTeamState(teamID, "Wombat")
		if err == nil {
			t.Error("SetTeamState did not return an error on a bad value")
		}

		err = teamID.SendAnnounce(gid, "testing")
		if err != nil {
			t.Error(err.Error())
		} */

	err = teamID.Delete()
	if err != nil {
		t.Error(err.Error())
	}
}

func BenchmarkNewTeam(b *testing.B) {
	for i := 0; i < b.N; i++ {
		teamID, err := gid.NewTeam("Test Team")
		if err != nil {
			b.Error(err.Error())
		}
		tids = append(tids, teamID)
	}
}

func BenchmarkDeleteTeam(b *testing.B) {
	var err error
	for _, tid := range tids {
		err = tid.Delete()
		if err != nil {
			b.Error(err.Error())
		}
	}
}

func TestTeammatesNear(t *testing.T) {
	var td wasabee.TeamData

	err := gid.TeammatesNear(1000, 10, &td)
	if err != nil {
		t.Error(err.Error())
	}

	for _, v := range td.Agent {
		fmt.Printf("%s is %fkm away\n", v.Name, v.Distance)
	}
}
