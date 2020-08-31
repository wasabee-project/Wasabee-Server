package wasabee

import (
	"sync"

	"github.com/gorilla/mux"
)

var once sync.Once

// wasabeeHTTPConfig stores values from the https server which are used in templates
// to allow URL creation in other services (e.g. Telegram)
var wasabeeHTTPSConfig struct {
	webroot string
	apipath string
	router  *mux.Router
}

// NewRouter creates the HTTPS router
func NewRouter() *mux.Router {
	// http://marcio.io/2015/07/singleton-pattern-in-go/
	once.Do(func() {
		Log.Debugw("startup", "router", "main HTTPS router")
		wasabeeHTTPSConfig.router = mux.NewRouter()
	})
	return wasabeeHTTPSConfig.router
}

// Subrouter creates a Gorilla subroute with a prefix
func Subrouter(prefix string) *mux.Router {
	Log.Debugw("startup", "router", prefix)
	if wasabeeHTTPSConfig.router == nil {
		NewRouter()
	}

	sr := wasabeeHTTPSConfig.router.PathPrefix(prefix).Subrouter()
	return sr
}

// SetWebroot is called at https startup
func SetWebroot(w string) {
	wasabeeHTTPSConfig.webroot = w
}

// GetWebroot is called from templates
func GetWebroot() (string, error) {
	return wasabeeHTTPSConfig.webroot, nil
}

// SetWebAPIPath is called at https startup
func SetWebAPIPath(a string) {
	wasabeeHTTPSConfig.apipath = a
}

// GetWebAPIPath is called from templates
func GetWebAPIPath() (string, error) {
	return wasabeeHTTPSConfig.apipath, nil
}
