package wfb

import (
	"fmt"
	"time"

	"github.com/wasabee-project/Wasabee-Server/model"
	"github.com/wasabee-project/Wasabee-Server/util"
)

var rlTeam *util.Safemap
var rlOp *util.Safemap

const agentLocationChangeRate = time.Second * 10
const mapChangeRate = time.Second * 10

func ratelimitinit() {
	rlTeam = util.NewSafemap()
	rlOp = util.NewSafemap()
}

// control how often teams are notified of agent location change
func ratelimitTeam(teamID model.TeamID) bool {
	now := time.Now()

	i, ok := rlTeam.Get(string(teamID))
	if !ok { // no entry for this team, must be OK
		return true
	}

	t := time.Unix(int64(i), 0)

	if t.After(now.Add(0 - agentLocationChangeRate)) {
		rlTeam.Set(string(teamID), uint64(now.Unix())) // update with this this time
		return true
	}

	return false
}

// control how often teams are notified of a map update
func ratelimitOp(teamID model.TeamID, opID model.OperationID) bool {
	now := time.Now()

	key := fmt.Sprint("%s-%s", string(opID), string(teamID))

	i, ok := rlOp.Get(key)
	if !ok {
		return true
	}

	t := time.Unix(int64(i), 0)

	if t.After(now.Add(0 - mapChangeRate)) {
		rlOp.Set(key, uint64(now.Unix()))
		return true
	}

	return false
}
