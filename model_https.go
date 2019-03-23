package PhDevBin

// phDevBinHTTPConfig stores values from the https server which are used in templates
// to allow URL creation in other services (e.g. Telegram)
var phDevBinHTTPConfig struct {
	webroot string
	apipath string
}

// SetWebroot is called at https startup
func SetWebroot(w string) {
	phDevBinHTTPConfig.webroot = w
}

// GetWebroot is called from templates
func GetWebroot() (string, error) {
	return phDevBinHTTPConfig.webroot, nil
}

// SetWebAPIPath is called at https startup
func SetWebAPIPath(a string) {
	phDevBinHTTPConfig.apipath = a
}

// GetWebAPIPath is called from templates
func GetWebAPIPath() (string, error) {
	return phDevBinHTTPConfig.apipath, nil
}
