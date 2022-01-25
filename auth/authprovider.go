package auth

import (
	"github.com/wasabee-project/Wasabee-Server/model"
)

var providers []Provider

// Provider is the interface type for authorizaiton services
type Provider interface {
	Authorize(gid model.GoogleID) bool
}

// RegisterAuthProvider lets the authorization system know about a service that provides authorization (v/rocks)
func RegisterAuthProvider(a Provider) {
	providers = append(providers, a)
}
