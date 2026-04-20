package auth

import (
	"context"

	"github.com/wasabee-project/Wasabee-Server/model"
)

var providers []Provider

// Provider is the interface type for authorization services
type Provider interface {
	// Authorize now accepts a context to allow for timeout propagation
	// during external API checks (e.g., calling Rocks or V).
	Authorize(ctx context.Context, gid model.GoogleID) bool
}

// RegisterAuthProvider lets the authorization system know about a service that provides authorization (v/rocks)
func RegisterAuthProvider(a Provider) {
	providers = append(providers, a)
}
