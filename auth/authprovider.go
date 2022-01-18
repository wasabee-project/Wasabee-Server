package auth

import (
	// "database/sql"

	"github.com/wasabee-project/Wasabee-Server/model"
)

var providers []AuthProvider

type AuthProvider interface {
	// ToDB(sql.Tx, interface{}) error
	// FromDB(sql.Tx), interface()
	Authorize(gid model.GoogleID) bool
}

func RegisterAuthProvider(a AuthProvider) {
	providers = append(providers, a)
}
