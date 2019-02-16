package PhDevHTTP

import (
	"errors"
	"fmt"
	"io/ioutil"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/cloudkucooland/PhDevBin"
	"github.com/gorilla/mux"
)

var staticFileExceptions = []string{
	"index.html",
	"output.html",
	"report.html",
	"guidelines.html",
	"Makefile",
	"README.md",
	"LICENSE",
	".styl",
}

type routeOptions struct {
	ignoreExceptions bool
	modifySource     func(*string)
	modifyResult     func(http.ResponseWriter, *http.Request, *string) error
}

// advancedStaticRoute adds a route to a file that can be manipulated on load from disk, and on each request.
func advancedStaticRoute(webroot string, path string, options routeOptions) func(http.ResponseWriter, *http.Request) {
	webroot = strings.TrimSuffix(webroot, "/") // .../ -> ...
	path = "/" + strings.TrimPrefix(path, "/") // ... -> /...

	// Load the file
	body, err := loadStaticFile(webroot+path, path, options.ignoreExceptions)
	if err == nil {
		// Manipulate on load from disk
		if options.modifySource != nil {
			options.modifySource(&body)
		}

		mimeType := mime.TypeByExtension(filepath.Ext(path))
		return func(res http.ResponseWriter, req *http.Request) {
			instanceBody := "" + body
			// Manipulate on each request
			if options.modifyResult != nil {
				err = options.modifyResult(res, req, &instanceBody)
				if err != nil {
					return
				}
			}

			res.Header().Add("Content-Type", mimeType+"; charset=utf-8")
			fmt.Fprintf(res, "%s", instanceBody)
		}
	}

	// We have an error. loadStaticFile already logged the error message.
	return internalErrorRoute
}

// staticRoute adds a route to a static file
func staticRoute(webroot string, path string, ignoreExceptions bool) func(http.ResponseWriter, *http.Request) {
	return advancedStaticRoute(webroot, path, routeOptions{ignoreExceptions: ignoreExceptions})
}

// addStaticDirectory traverses a directory and adds its files as static routes to a mux Router.
//
// /usr/share/www/example/
// ----webroot--- --path--
func addStaticDirectory(webroot string, path string, r *mux.Router) {
	webroot = strings.TrimSuffix(webroot, "/")                              // .../ -> ...
	path = "/" + strings.TrimPrefix(strings.TrimSuffix(path, "/")+"/", "/") // ... -> /.../

	filepath.Walk(webroot+path, func(localFilePath string, fileInfo os.FileInfo, err error) error {
		if err == nil {
			// /var/www/example/test.html -> /example/test.html
			webFilePath := strings.TrimPrefix(localFilePath, webroot)

			// Exclude directories, hidden files and files from hidden directories
			if !fileInfo.IsDir() && !strings.Contains(webFilePath, "/.") {
				// Read & serve file
				r.HandleFunc(webFilePath, staticRoute(webroot, webFilePath, false)).Methods("GET")
			}
		}
		return nil
	})
}

// loadStaticFile reads a single static file from disk and logs the process.
func loadStaticFile(localPath string, webPath string, ignoreExceptions bool) (string, error) {
	// Check if the file is exempt
	if !ignoreExceptions && isStaticFileExempt(filepath.Base(localPath)) {
		PhDevBin.Log.Debugf("Exempted static file: %s", webPath)
		return "", errors.New("file is exempt")
	}

	// Read the file
	content, err := ioutil.ReadFile(localPath)
	if err != nil {
		PhDevBin.Log.Errorf("Couldn't read static file '%s' for the following reason: %s", webPath, err)
		return "", err
	}

	// Log & return the file, with global variables already replaced
	PhDevBin.Log.Debugf("Adding static file: %s", webPath)
	body := string(content)
	replaceGlobal(&body)
	return body, nil
}

// isStaticFileExempt checks a filename against staticFileExceptions to determine if it should be exempted from a static directory.
func isStaticFileExempt(filename string) bool {
	for _, exemptFilename := range staticFileExceptions {
		// File extensions
		if strings.HasPrefix(exemptFilename, ".") && strings.HasSuffix(filename, exemptFilename) {
			return true
		}

		// Filenames
		if filename == exemptFilename {
			return true
		}
	}
	return false
}
