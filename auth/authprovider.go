package auth

import (
	// "database/sql"

	"github.com/wasabee-project/Wasabee-Server/model"
	"github.com/wasabee-project/Wasabee-Server/log"
)

var providers []AuthProvider

type AuthProvider interface {
	Authorize(gid model.GoogleID) bool
}

func RegisterAuthProvider(a AuthProvider) {
	log.Debugw("adding auth provider", "provider", a)
	providers = append(providers, a)
}
