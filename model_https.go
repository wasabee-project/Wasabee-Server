package wasabi

import (
	"sync"

	"github.com/gorilla/mux"
)

var once sync.Once

// wasabiHTTPConfig stores values from the https server which are used in templates
// to allow URL creation in other services (e.g. Telegram)
var wasabiHTTPSConfig struct {
	webroot string
	apipath string
	router  *mux.Router
}

// NewRouter creates the HTTPS router
func NewRouter() *mux.Router {
	// http://marcio.io/2015/07/singleton-pattern-in-go/
	once.Do(func() {
		Log.Info("Establishing main HTTPS router")
		wasabiHTTPSConfig.router = mux.NewRouter()
	})
	return wasabiHTTPSConfig.router
}

// Subrouter creates a Gorilla subroute with a prefix
func Subrouter(prefix string) *mux.Router {
	Log.Debugf("Establishing HTTPS subrouter for %s", prefix)
	if wasabiHTTPSConfig.router == nil {
		NewRouter()
	}

	sr := wasabiHTTPSConfig.router.PathPrefix(prefix).Subrouter()
	return sr
}

// SetWebroot is called at https startup
func SetWebroot(w string) {
	wasabiHTTPSConfig.webroot = w
}

// GetWebroot is called from templates
func GetWebroot() (string, error) {
	return wasabiHTTPSConfig.webroot, nil
}

// SetWebAPIPath is called at https startup
func SetWebAPIPath(a string) {
	wasabiHTTPSConfig.apipath = a
}

// GetWebAPIPath is called from templates
func GetWebAPIPath() (string, error) {
	return wasabiHTTPSConfig.apipath, nil
}
