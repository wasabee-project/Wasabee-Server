package PhDevBin

var webroot string
var httpsrunning bool

// currently only used for template functions
// called at https startup
func SetWebroot(w string) {
	webroot = w
	tgrunning = true
}

// called from templates
func GetWebroot() (string, error) {
	if tgrunning == false {
		return "", nil
	}
	return webroot, nil
}
