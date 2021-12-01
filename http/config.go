package wasabeehttps

import (
	"sync"

	"github.com/gorilla/mux"
	"github.com/wasabee-project/Wasabee-Server/config"
	"github.com/wasabee-project/Wasabee-Server/log"
)

var once sync.Once

// NewRouter creates the HTTPS router
func NewRouter() *mux.Router {
	// http://marcio.io/2015/07/singleton-pattern-in-go/
	once.Do(func() {
		log.Debugw("startup", "router", "main HTTPS router")
		config.Get().HTTP.Router = mux.NewRouter()
	})
	return config.Get().HTTP.Router
}

// Subrouter creates a Gorilla subroute with a prefix
func Subrouter(prefix string) *mux.Router {
	log.Debugw("startup", "router", prefix)
	if c.router == nil {
		NewRouter()
	}

	sr := c.router.PathPrefix(prefix).Subrouter()
	return sr
}

// SetWebroot is called at https startup
func SetWebroot(w string) {
	config.Get().HTTP.Webroot = w
}

// GetWebroot is called from templates
func GetWebroot() string {
	return config.Get().HTTP.Webroot
}

// SetWebAPIPath is called at https startup
func SetWebAPIPath(a string) {
	config.Get().HTTP.APIpath = a
}

// GetWebAPIPath is called from templates
func GetWebAPIPath() string {
	return config.Get().HTTP.APIpath
}
