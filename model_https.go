package wasabi

// wasabiHTTPConfig stores values from the https server which are used in templates
// to allow URL creation in other services (e.g. Telegram)
var wasabiHTTPSConfig struct {
	webroot string
	apipath string
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
