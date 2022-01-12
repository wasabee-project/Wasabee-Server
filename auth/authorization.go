package auth

import (
	"fmt"
	"sync"

	"github.com/wasabee-project/Wasabee-Server/log"
	"github.com/wasabee-project/Wasabee-Server/model"
	"github.com/wasabee-project/Wasabee-Server/rocks"
	"github.com/wasabee-project/Wasabee-Server/v"
)

var logoutlist sync.Map

// Authorize is called to verify that an agent is permitted to use Wasabee.
// V and Rocks are updated (if configured).
// Accounts that are locked due to Google RISC are blocked.
// Accounts that have indicated they are RES in Intel are blocked.
// Returns true if the agent is authorized to continue, false if the agent is blacklisted or otherwise locked.
func Authorize(gid model.GoogleID) (bool, error) {
	// if the agent isn't known to this server, pre-populate everything
	if !gid.Valid() {
		if err := gid.FirstLogin(); err != nil {
			log.Error(err)
			return false, err
		}
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
	defer close(channel)
	go func() {
		channel <- v.Authorize(gid)
	}()
	go func() {
		channel <- rocks.Authorize(gid)
	}()

	// "true" means "not blocked", "false" means "blocked"
	e1, e2 := <-channel, <-channel
	if !e1 || !e2 {
		return false, fmt.Errorf("access denied")
	}

	return true, nil
}

// Logout adds a GoogleID to the list of logged out agents
func Logout(gid model.GoogleID, reason string) {
	logoutlist.Store(string(gid), true)
}

// isLoggedOut looks to see if the user is on the force logout list
func IsLoggedOut(gid model.GoogleID) bool {
	out, ok := logoutlist.Load(string(gid))
	if ok && out.(bool) {
		logoutlist.Delete(string(gid))
		return true
	}

	return false
}

// RevokeJWT adds a JWT ID to the revoked list
func RevokeJWT(tokenID string) {
	log.Infow("revoking JWT", "id", tokenID)
	logoutlist.Store(tokenID, true)
}

// IsRevokedJWT checks if a JWT ID is on the revoked list.
func IsRevokedJWT(tokenID string) bool {
	out, ok := logoutlist.Load(tokenID)
	if ok && out.(bool) {
		log.Infow("revoked JWT", "id", tokenID)
		return true
	}
	return false
}
