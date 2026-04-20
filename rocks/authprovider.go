package rocks

import (
	"context"
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
func (r *Rocks) Authorize(ctx context.Context, gid model.GoogleID) bool {
	// Pass ctx to the DB call for tracing and timeout support
	a, fetched, err := model.RocksFromDB(ctx, gid)
	if err != nil {
		// do not block on db error, but log it
		log.Error(err)
		return true
	}

	// Logic check: if cache is empty or older than 1 hour, refresh from network
	if a.Agent == "" || fetched.Before(time.Now().Add(-1*time.Hour)) {
		// Assuming Search will be updated to accept context as well
		net, err := Search(ctx, string(gid))
		if err != nil {
			// do not block on network error unless already listed as a smurf in the cache
			return !a.Smurf
		}

		if net.Gid == "" {
			net.Gid = gid
		}

		// Update cache with new data
		if err := model.RocksToDB(ctx, net); err != nil {
			log.Error(err)
		}
		a = net
	}

	if a.Agent != "" && a.Smurf {
		log.Warnw("access denied", "GID", gid, "reason", "listed as smurf at enl.rocks")
		return false
	}

	return true
}
