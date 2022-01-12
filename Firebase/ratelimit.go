package wfb

import (
	"fmt"
	"sync"
	"time"

	"github.com/wasabee-project/Wasabee-Server/model"
)

// use sync.Map instead of map[] since we have multiple readers/writers
var rlTeam sync.Map
var rlOp sync.Map

const agentLocationChangeRate = time.Second * 10
const mapChangeRate = time.Second * 10

// control how often teams are notified of agent location change
func ratelimitTeam(teamID model.TeamID) bool {
	now := time.Now()

	i, ok := rlTeam.LoadOrStore(teamID, now)
	if !ok { // no entry for this team, must be OK
		return true
	}

	if i.(time.Time).After(now.Add(0 - agentLocationChangeRate)) {
		rlTeam.Store(teamID, now) // update with this this time
		return true
	}

	return false
}

// control how often teams are notified of a map update
func ratelimitOp(teamID model.TeamID, opID model.OperationID) bool {
	now := time.Now()

	key := fmt.Sprint("%s-%s", string(opID), string(teamID))

	i, ok := rlOp.LoadOrStore(key, now)
	if !ok {
		return true
	}

	if i.(time.Time).After(now.Add(0 - mapChangeRate)) {
		rlOp.Store(key, now)
		return true
	}

	return false
}
