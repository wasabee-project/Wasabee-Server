package PhDevBin

var HTTPConfig struct {
	webroot string
	apipath string
}

// called at https startup
func SetWebroot(w string) {
	HTTPConfig.webroot = w
}

// called from templates
func GetWebroot() (string, error) {
	return HTTPConfig.webroot, nil
}

// called at https startup
func SetWebAPIPath(a string) {
	HTTPConfig.apipath = a
}

// called from templates
func GetWebAPIPath() (string, error) {
	return HTTPConfig.apipath, nil
}
