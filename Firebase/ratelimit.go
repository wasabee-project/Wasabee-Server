package wfb

import (
	"time"

	"github.com/wasabee-project/Wasabee-Server/log"
	"github.com/wasabee-project/Wasabee-Server/model"
	"github.com/wasabee-project/Wasabee-Server/util"
)

// these three could be one map, since the keys will not(*) collide
// for discernable values of not
var rlAgent *util.Safemap
var rlOp *util.Safemap
var rlTeam *util.Safemap

// standard messaging rates apply
const baseChangeRate = time.Second * 10

// three values since these could be adjusted per-type, right now they move together
var agentLocationChangeRate = baseChangeRate
var mapChangeRate = baseChangeRate
var teamRate = baseChangeRate

func ratelimitinit() {
	rlAgent = util.NewSafemap()
	rlOp = util.NewSafemap()
	rlTeam = util.NewSafemap()
}

func ratelimitTeam(teamID model.TeamID) bool {
	now := time.Now()

	i, ok := rlTeam.Get(string(teamID))
	if !ok {
		rlTeam.Set(string(teamID), uint64(now.Unix()))
		return true
	}

	t := time.Unix(int64(i), 0)

	if t.After(now.Add(0 - agentLocationChangeRate)) {
		rlTeam.Set(string(teamID), uint64(now.Unix()))
		return true
	}

	return false
}

func ratelimitAgent(gid model.GoogleID) bool {
	now := time.Now()

	i, ok := rlAgent.Get(string(gid))
	if !ok {
		rlAgent.Set(string(gid), uint64(now.Unix()))
		return true
	}

	t := time.Unix(int64(i), 0)

	if t.After(now.Add(0 - agentLocationChangeRate)) {
		rlAgent.Set(string(gid), uint64(now.Unix()))
		return true
	}

	return false
}

func ratelimitOp(opID model.OperationID) bool {
	now := time.Now()

	i, ok := rlOp.Get(string(opID))
	if !ok {
		rlOp.Set(string(opID), uint64(now.Unix()))
		return true
	}

	t := time.Unix(int64(i), 0)

	if t.After(now.Add(0 - mapChangeRate)) {
		rlOp.Set(string(opID), uint64(now.Unix()))
		return true
	}

	return false
}

// break this in to per-type slowdowns....
func slowdown() {
	log.Debug("slowing firebase rate")

	agentLocationChangeRate = agentLocationChangeRate + agentLocationChangeRate
	mapChangeRate = mapChangeRate + mapChangeRate
	teamRate = teamRate + teamRate
	log.Infow("firebase rate limit slowing down", "mapChangeRate", mapChangeRate)
}

func ResetDefaultRateLimits() {
	if mapChangeRate == baseChangeRate {
		return
	}

	log.Debug("resetting firebase rate")
	agentLocationChangeRate = baseChangeRate
	mapChangeRate = baseChangeRate
	teamRate = baseChangeRate
}
