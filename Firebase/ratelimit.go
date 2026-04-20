package wfb

import (
	"time"

	"github.com/wasabee-project/Wasabee-Server/log"
	"github.com/wasabee-project/Wasabee-Server/model"
	"github.com/wasabee-project/Wasabee-Server/util"
)

var rlAgent *util.Safemap
var rlOp *util.Safemap
var rlTeam *util.Safemap

// standard messaging rates apply
const baseChangeRate = 10 * time.Second

// global throttle modifiers
var agentLocationChangeRate = baseChangeRate
var mapChangeRate = baseChangeRate
var teamRate = baseChangeRate

func ratelimitinit() {
	rlAgent = util.NewSafemap()
	rlOp = util.NewSafemap()
	rlTeam = util.NewSafemap()
}

// checkLimit returns true if we ARE ALLOWED to send (i.e., not rate limited)
func checkLimit(sm *util.Safemap, key string, rate time.Duration) bool {
	now := time.Now()
	val, ok := sm.Get(key)
	if !ok {
		sm.Set(key, uint64(now.Unix()))
		return true
	}

	lastSent := time.Unix(int64(val), 0)
	// If now is before (lastSent + rate), we are sending too fast
	if now.Before(lastSent.Add(rate)) {
		return false
	}

	sm.Set(key, uint64(now.Unix()))
	return true
}

func ratelimitTeam(teamID model.TeamID) bool {
	return checkLimit(rlTeam, string(teamID), teamRate)
}

func ratelimitAgent(gid model.GoogleID) bool {
	return checkLimit(rlAgent, string(gid), agentLocationChangeRate)
}

func ratelimitOp(opID model.OperationID) bool {
	return checkLimit(rlOp, string(opID), mapChangeRate)
}

// slowdown exponentially increases the wait time between pushes
func slowdown() {
	agentLocationChangeRate *= 2
	mapChangeRate *= 2
	teamRate *= 2
	log.Infow("firebase rate limit increased (slowing down)", "mapChangeRate", mapChangeRate)
}

// ResetDefaultRateLimits brings us back to the 10s base rate
func ResetDefaultRateLimits() {
	if mapChangeRate == baseChangeRate {
		return
	}

	log.Debug("resetting firebase rate to defaults")
	agentLocationChangeRate = baseChangeRate
	mapChangeRate = baseChangeRate
	teamRate = baseChangeRate
}
