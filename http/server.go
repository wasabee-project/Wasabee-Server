package PhDevHTTP

import (
	"net/http"
	"net/http/httputil"
	"path/filepath"
	"strings"
	"os"

    "golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	"github.com/gorilla/mux"
    "github.com/gorilla/sessions"
	"github.com/cloudkucooland/PhDevBin"
/* http://www.gorillatoolkit.org/pkg/sessions */
)

type Configuration struct {
	ListenHTTPS   string
	FrontendPath  string
	Root          string
	path          string
	domain        string
	Hsts          string
	oauthStateString string
	CertDir		  string
}

/*
type User struct {
    Sub string `json:"sub"`
    Name string `json:"name"`
    GivenName string `json:"given_name"`
    FamilyName string `json:"family_name"`
    Profile string `json:"profile"`
    Picture string `json:"picture"`
    Email string `json:"email"`
    EmailVerified string `json:"email_verified"`
    Gender string `json:"gender"`
} */

var config Configuration
var googleOauthConfig *oauth2.Config
var store sessions.CookieStore

// initializeConfig will normalize the options and create the "config" object.
func initializeConfig(initialConfig Configuration) {
	config = initialConfig
	// Transform frontendPath to an absolute path
	frontendPath, err := filepath.Abs(config.FrontendPath)
	if err != nil {
		PhDevBin.Log.Critical("Frontend path couldn't be resolved.")
		panic(err)
	}
	config.FrontendPath = frontendPath

	config.Root = strings.TrimSuffix(config.Root, "/")

	// Extract "path" fron "root"
	rootParts := strings.SplitAfterN(config.Root, "/", 4) // https://example.org/[grab this part]
	config.path = ""
	if len(rootParts) > 3 { // Otherwise: application in root folder
		config.path = rootParts[3]
	}
	config.path = strings.TrimSuffix("/"+strings.TrimPrefix(config.path, "/"), "/")

	rootParts = strings.SplitN(strings.ToLower(config.Root), "://", 2)
	config.domain = strings.Split(rootParts[len(rootParts)-1], "/")[0]

	googleOauthConfig = &oauth2.Config{
		RedirectURL:  config.Root + "/callback",
		ClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		ClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		Scopes:       []string{"https://www.googleapis.com/auth/userinfo.email"},
		Endpoint:     google.Endpoint,
	}
    PhDevBin.Log.Noticef("ClientID: " + googleOauthConfig.ClientID)
    PhDevBin.Log.Noticef("ClientSecret: " + googleOauthConfig.ClientSecret)
	config.oauthStateString = PhDevBin.GenerateName()
    PhDevBin.Log.Noticef("StateString: " + config.oauthStateString)
    PhDevBin.Log.Noticef("Cookie Store: " + os.Getenv("SESSION_KEY"))
    // store = sessions.NewCookieStore(os.Getenv("SESSION_KEY"))

    config.CertDir = os.Getenv("CERTDIR")
	if config.CertDir == "" {
        config.CertDir = "certs/"
	}
    PhDevBin.Log.Noticef("Certificate Directory: " + config.CertDir)

}

// StartHTTP launches the HTTP server which is responsible for the frontend and the HTTP API.
func StartHTTP(initialConfig Configuration) {
	// Configure
	initializeConfig(initialConfig)

	// Route
	r := mux.NewRouter()
	setupRoutes(r)

	// Add important headers
	r.Use(headersMW)
	r.Use(debugMW)

	// Serve
	PhDevBin.Log.Noticef("HTTPS server starting on %s, you should be able to reach it at %s", config.ListenHTTPS, config.Root)
	err := http.ListenAndServeTLS(config.ListenHTTPS, config.CertDir + "/PhDevBin.fullchain.pem", config.CertDir + "/PhDevBin.key", r)
	if err != nil {
		PhDevBin.Log.Errorf("HTTPS server error: %s", err)
		panic(err)
	}
}

func headersMW(next http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		res.Header().Add("Server", "PhDevBin")
		res.Header().Add("X-Content-Type", "nosniff") // We're not a CDN.
		res.Header().Add("Access-Control-Allow-Origin", "https://intel.ingress.com")
		res.Header().Add("Access-Control-Allow-Methods", "POST, GET, PUT, OPTIONS, HEAD, DELETE")
		res.Header().Add("Access-Control-Allow-Credentials", "true")
		if config.Hsts != "" {
			res.Header().Add("Strict-Transport-Security", config.Hsts)
		}
		next.ServeHTTP(res, req)
	})
}

func debugMW(next http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		dump, _ := httputil.DumpRequest(req, false)
		PhDevBin.Log.Notice(string(dump))
		next.ServeHTTP(res, req)
	})
}


