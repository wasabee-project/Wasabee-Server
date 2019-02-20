package PhDevHTTP

import (
	"net/http"
	"net/http/httputil"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	"github.com/cloudkucooland/PhDevBin"
	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
)

type Configuration struct {
	ListenHTTPS      string
	FrontendPath     string
	Root             string
	path             string
	domain           string
	oauthStateString string
	CertDir          string
}

var config Configuration
var googleOauthConfig *oauth2.Config
var store *sessions.CookieStore
var SessionName string = "PhDevBin"

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
	key := os.Getenv("SESSION_KEY")
	PhDevBin.Log.Noticef("Cookie Store: " + key)
	store = sessions.NewCookieStore([]byte(key))

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

	// s := r.Subrouter()
	// setupAuthRoutes(s)

	// Add important headers
	r.Use(headersMW)
	// r.Use(debugMW)
	r.Use(authMW)

	// Serve
	PhDevBin.Log.Noticef("HTTPS server starting on %s, you should be able to reach it at %s", config.ListenHTTPS, config.Root)
	err := http.ListenAndServeTLS(config.ListenHTTPS, config.CertDir+"/PhDevBin.fullchain.pem", config.CertDir+"/PhDevBin.key", r)
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
		// res.Header().Add("Access-Control-Allow-Headers", "Content-Type, Accept, X-Requested-With, remember-me")
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

func authMW(next http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		ses, err := store.Get(req, SessionName)
		if err != nil {
			http.Error(res, err.Error(), http.StatusInternalServerError)
			return
		}

		// once this is used for requiring all URLs be authenticated, do this
		if ses.Values["id"] == nil {
			PhDevBin.Log.Notice("No Id")
			// http.Redirect(res, req, "/login", http.StatusPermanentRedirect)
			//return
		}

		next.ServeHTTP(res, req)
	})
}
