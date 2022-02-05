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

const baseChangeRate = time.Second * 10

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
		log.Debug("no entry in map")
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
