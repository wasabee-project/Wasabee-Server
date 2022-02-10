package v

import (
	"time"

	"github.com/wasabee-project/Wasabee-Server/log"
	"github.com/wasabee-project/Wasabee-Server/model"
)

// V is the interface to satisfy auth.AuthProvider
type V struct{}

// Authorize checks if an agent is permitted to use Wasabee based on V data
// data is cached per-agent for one hour
// if an agent is not known at V, they are implicitly permitted
// if an agent is banned, blacklisted, etc at V, they are prohibited
func (v *V) Authorize(gid model.GoogleID) bool {
	// log.Debugw("V authorize", "gid", gid)

	a, fetched, err := model.VFromDB(gid)
	if err != nil {
		log.Error(err)
		// do not block on db error
		return true
	}

	if a.Agent == "" || fetched.Before(time.Now().Add(0-time.Hour)) {
		net, err := trustCheck(gid)
		if err != nil {
			log.Error(err)
			// do not block on network error unless already listed as blacklisted in DB
			return !a.Blacklisted
		}
		// log.Debugw("v cache refreshed", "gid", gid, "data", net.Data)
		err = model.VToDB(&net.Data)
		if err != nil {
			log.Error(err)
		}
		a = &net.Data // use the network result now that it is saved
	}

	if a.Agent != "" {
		if a.Quarantine {
			log.Warnw("access denied", "GID", gid, "reason", "quarantined at V")
			return false
		}
		if a.Flagged {
			log.Warnw("access denied", "GID", gid, "reason", "flagged at V")
			return false
		}
		if a.Blacklisted || a.Banned {
			log.Warnw("access denied", "GID", gid, "reason", "blacklisted at V")
			return false
		}
	}

	return true
}
