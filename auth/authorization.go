package auth

import (
	"fmt"
	"sync"

	"github.com/wasabee-project/Wasabee-Server/log"
	"github.com/wasabee-project/Wasabee-Server/model"
	"github.com/wasabee-project/Wasabee-Server/rocks"
	"github.com/wasabee-project/Wasabee-Server/v"
)

type logoutList struct {
	logoutlist map[model.GoogleID]bool
	mux        sync.Mutex
}

var ll logoutList

func init() {
	ll.logoutlist = make(map[model.GoogleID]bool)
}

// Authorize is called from Oauth callback to set up a agent for the first time or revalidate them on subsequent logins.
// It also checks and updates the local V and enl.rocks data, if configured.
// Returns true if the agent is authorized to continue, false if the agent is blacklisted or otherwise locked.
func Authorize(gid model.GoogleID) (bool, error) {
	// if the agent doesn't exist, prepopulate everything
	if !gid.Valid() {
		gid.FirstLogin()
	}

	if gid.RISC() {
		err := fmt.Errorf("account locked by Google RISC")
		log.Warnw(err.Error(), "GID", gid)
		return false, err
	}

	if gid.IntelSmurf() {
		err := fmt.Errorf("intel account self-identified as RES")
		log.Warnw(err.Error(), "GID", gid)
		return false, err
	}

	// query both rocks and V at the same time -- probably not necessary now
	// *.Authorize checks cache in db, if too old, checks service and saves updates
	channel := make(chan bool, 2)
	go func() {
		channel <- v.Authorize(gid)
	}()
	go func() {
		channel <- rocks.Authorize(gid)
	}()
	defer close(channel)

	// "true" means "not blocked", "false" means "blocked"
	e1, e2 := <-channel, <-channel
	if !e1 || !e2 {
		return false, fmt.Errorf("access denied")
	}

	return true, nil
}

// The logout stuff goes away when we move to JWT

// Logout sets a temporary logout token - not stored in DB since logout cases are not critical
// and sessions are refreshed with google hourly
func Logout(gid model.GoogleID, reason string) {
	log.Infow("logout", "GID", gid, "reason", reason, "message", gid+" logout")
	ll.mux.Lock()
	defer ll.mux.Unlock()
	ll.logoutlist[gid] = true
}

// CheckLogout looks to see if the user is on the force logout list
func CheckLogout(gid model.GoogleID) bool {
	ll.mux.Lock()
	defer ll.mux.Unlock()
	logout, ok := ll.logoutlist[gid]
	if !ok { // not in the list
		return false
	}
	if logout {
		ll.logoutlist[gid] = false
		// log.Debugw("clearing from logoutlist", "GID", gid)
		delete(ll.logoutlist, gid)
	}
	return logout
}
