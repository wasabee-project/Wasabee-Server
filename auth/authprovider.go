package auth

import (
	// "github.com/wasabee-project/Wasabee-Server/log"
	"github.com/wasabee-project/Wasabee-Server/model"
)

var providers []AuthProvider

// AuthProvider is the interface type for authorizaiton services
type AuthProvider interface {
	Authorize(gid model.GoogleID) bool
}

// RegisterAuthProvider lets the authorization system know about a service that provides authorization (v/rocks)
func RegisterAuthProvider(a AuthProvider) {
	providers = append(providers, a)
}
