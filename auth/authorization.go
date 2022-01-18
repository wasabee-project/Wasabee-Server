package auth

import (
	"context"
	"fmt"

	"github.com/wasabee-project/Wasabee-Server/log"
	"github.com/wasabee-project/Wasabee-Server/model"
	// "github.com/wasabee-project/Wasabee-Server/rocks"
	"github.com/wasabee-project/Wasabee-Server/util"
	// "github.com/wasabee-project/Wasabee-Server/v"
)

var logoutlist *util.Safemap
var revokedjwt *util.Safemap

// Start does initialization and loads/stores the revoked JWT token lists
func Start(ctx context.Context) {
	log.Infow("startup", "message", "setting up authorization")

	logoutlist = util.NewSafemap()
	revokedjwt = model.LoadRevokedJWT()

	<-ctx.Done()

	log.Infow("shutdown", "message", "shutting down authorization")
	model.StoreRevokedJWT(revokedjwt)
}

// Authorize is called to verify that an agent is permitted to use Wasabee.
// Accounts that are locked due to Google RISC are blocked.
// Accounts that have indicated they are RES in Intel are blocked.
// V and Rocks are checked (if configured).
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

	// sequentially loop through authorization providers
	for _, p := range providers {
		if !p.Authorize(gid) {
			return false, fmt.Errorf("access denied")
		}
	}

	return true, nil
}

// Logout adds a GoogleID to the list of logged out agents
func Logout(gid model.GoogleID, reason string) {
	logoutlist.SetBool(string(gid), true)
}

// IsLoggedOut looks to see if the user is on the force logout list
func IsLoggedOut(gid model.GoogleID) bool {
	out := logoutlist.GetBool(string(gid))
	if out {
		logoutlist.SetBool(string(gid), false)
		return true
	}

	return false
}

// RevokeJWT adds a JWT ID to the revoked list
func RevokeJWT(tokenID string) {
	log.Infow("revoking JWT", "id", tokenID)
	revokedjwt.SetBool(tokenID, true)
}

// IsRevokedJWT checks if a JWT ID is on the revoked list.
func IsRevokedJWT(tokenID string) bool {
	return revokedjwt.GetBool(tokenID)
}
