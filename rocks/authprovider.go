package rocks

import (
	"time"

	"github.com/wasabee-project/Wasabee-Server/log"
	"github.com/wasabee-project/Wasabee-Server/model"
)

// Rocks is the interface to satisfy auth.AuthProvider
type Rocks struct{}

// Authorize checks Rocks to see if an agent is permitted to use Wasabee
// responses are cached for an hour
// unknown agents are permitted implicitly
// if an agent is marked as smurf at rocks, they are prohibited
func (r *Rocks) Authorize(gid model.GoogleID) bool {
	log.Debugw("rocks authorize", "gid", gid)

	a, fetched, err := model.RocksFromDB(gid)
	if err != nil {
		// do not block on db error
		return true
	}

	// log.Debugw("rocks from cache", "gid", gid, "data", a)
	if a.Agent == "" || fetched.Before(time.Now().Add(0-time.Hour)) {
		net, err := Search(string(gid))
		if err != nil {
			return !a.Smurf // do not block on network error unless already listed as a smurf in the cache
		}
		// log.Debugw("rocks cache refreshed", "gid", gid, "data", net)
		if net.Gid == "" {
			// log.Debugw("Rocks returned a result without a GID, adding it", "gid", gid, "result", net)
			net.Gid = gid
		}
		if err := model.RocksToDB(net); err != nil {
			log.Error(err)
		}
		a = net
	}

	if a.Agent != "" && a.Smurf {
		log.Warnw("access denied", "GID", gid, "reason", "listed as smurf at enl.rocks")
		return false
	}

	// not in rocks is not sufficient to block
	return true
}
