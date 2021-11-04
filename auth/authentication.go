package auth

import (
	"database/sql"
	"fmt"
	"strings"
	"sync"

	"github.com/wasabee-project/Wasabee-Server/log"
	"github.com/wasabee-project/Wasabee-Server/model"
)

type logoutList struct {
	logoutlist map[string]bool
	mux        sync.Mutex
}

var ll logoutList

// init is bad magic; need a proper constructor
func init() {
	ll.logoutlist = make(map[GoogleID]bool)
}

// InitAgent is called from Oauth callback to set up a agent for the first time or revalidate them on subsequent logins.
// It also updates the local V and enl.rocks data, if configured.
// Returns true if the agent is authorized to continue, false if the agent is blacklisted or otherwise locked.
func InitAgent(gid string) (bool, error) {
	var authError bool
	var tmpName string
	var err error
	var vdata Vresult
	var rocks RocksAgent

	// query both rocks and V at the same time 
	channel := make(chan error, 2)
	go func() {
		channel <- VSearch(gid, &vdata)
	}()
	go func() {
		channel <- RocksSearch(gid, &rocks)
	}()
	defer close(channel)

	// would be better to start processing when either returned rather than waiting for both to be done, still better than serial calls
	e1, e2 := <-channel, <-channel
	if e1 != nil {
		log.Error(e1)
	}
	if e2 != nil {
		log.Error(e2)
	}

	// rocks agent names are less trustworthy than V, let V overwrite
	if rocks.Agent != "" {
		// if we got data, and the user already exists (not first login) update if necessary
		err = RocksUpdate(gid, &rocks)
		if err != nil {
			log.Info(err)
			return false, err
		}
		tmpName = rocks.Agent
		if rocks.Smurf {
			log.Warnw("access denied", "GID", gid, "reason", "listed as smurf at enl.rocks")
			authError = true
		}
	}

	if vdata.Data.Agent != "" {
		// if we got data, and the user already exists (not first login) update if necessary
		err = gid.VUpdate(&vdata)
		if err != nil {
			log.Error(err)
			return false, err
		}
		// overwrite what we got from rocks
		tmpName = vdata.Data.Agent
		if vdata.Data.Quarantine {
			log.Warnw("access denied", "GID", gid, "reason", "quarantined at V")
			authError = true
		}
		if vdata.Data.Flagged {
			log.Warnw("access denied", "GID", gid, "reason", "flagged at V")
			authError = true
		}
		if vdata.Data.Blacklisted || vdata.Data.Banned {
			log.Warnw("access denied", "GID", gid, "reason", "blacklisted at V")
			authError = true
		}
	}

	if authError {
		return false, fmt.Errorf("access denied")
	}

	// if the agent doesn't exist, prepopulate everything
	name, err := gid.IngressName()
	if err != nil && err == sql.ErrNoRows {
		log.Infow("first login", "GID", gid, "message", "first login for "+gid)

		if tmpName == "" {
			// triggered this in testing -- should never happen IRL
			length := 15
			if tmp := len(gid); tmp < length {
				length = tmp
			}
			tmpName = "UnverifiedAgent_" + gid[:length]
			log.Infow("using UnverifiedAgent name", "GID", gid, "name", tmpName)
		}

		ott, err := GenerateSafeName()
		if err != nil {
			log.Error(err)
			return false, err
		}

		ad := AgentData{
			GoogleID:      gid,
			IngressName:   tmpName,
			OneTimeToken:  OneTimeToken(ott),
			Level:         vdata.Data.Level,
			VVerified:     vdata.Data.Verified,
			VBlacklisted:  vdata.Data.Blacklisted,
			EnlID:         vdata.Data.EnlID,
			RocksVerified: rocks.Verified,
		}

		if err := ad.Save(); err != nil {
			log.Error(err)
			return false, err
		}
	} else if err != nil {
		log.Error(err)
		return false, err
	}

	if gid.RISC() {
		err := fmt.Errorf("account locked by Google RISC")
		log.Warnw(err.Error(), "GID", gid, "name", name)
		return false, err
	}

	if gid.IntelSmurf() {
		err := fmt.Errorf("intel account self-identified as RES")
		log.Warnw(err.Error(), "GID", gid, "name", name)
		return false, err
	}

	if tmpName != "" && strings.HasPrefix(name, "UnverifiedAgent_") {
		log.Infow("updating agent name", "GID", gid, "name", name, "new", tmpName)
		if err := gid.SetAgentName(tmpName); err != nil {
			log.Warnw(err.Error(), "GID", gid, "name", name, "new", tmpName)
			return true, nil
		}
	}

	return true, nil
}

// RevalidateEveryone -- if the schema changes or another reason causes us to need to pull data from V and rocks, this is a function which does that
// V had bulk API functions we should use instead. This is good enough, and I hope we don't need it again.
func RevalidateEveryone() error {
	channel := make(chan error, 2)
	defer close(channel)

	rows, err := db.Query("SELECT gid FROM agent")
	if err != nil {
		log.Error(err)
		return err
	}

	var gid string
	defer rows.Close()
	for rows.Next() {
		if err = rows.Scan(&gid); err != nil {
			log.Error(err)
			continue
		}

		var v Vresult
		var r RocksAgent

		go func() {
			channel <- VSearch(gid, &v)
		}()
		go func() {
			channel <- RocksSearch(gid, &r)
		}()
		if err = <-channel; err != nil {
			log.Error(err)
		}
		if err = <-channel; err != nil {
			log.Error(err)
		}

		if err = gid.VUpdate(&v); err != nil {
			log.Error(err)
		}

		if err = RocksUpdate(gid, &r); err != nil {
			log.Error(err)
		}
	}
	return nil
}

// Logout sets a temporary logout token - not stored in DB since logout cases are not critical
// and sessions are refreshed with google hourly
func Logout(gid string, reason string) {
	log.Infow("logout", "GID", gid, "reason", reason, "message", gid +" logout")
	ll.mux.Lock()
	defer ll.mux.Unlock()
	ll.logoutlist[gid] = true
}

// CheckLogout looks to see if the user is on the force logout list
func CheckLogout(gid string) bool {
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

