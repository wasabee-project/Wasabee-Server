package WASABIhttps

import (
	"html/template"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	"github.com/cloudkucooland/WASABI"
	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	"github.com/unrolled/logger"
)

// Configuration is the main configuration data for the https server
// an initial config is sent from main() and that is updated with defaults
// in the initializeConfig function
type Configuration struct {
	ListenHTTPS       string
	FrontendPath      string
	Root              string
	path              string
	domain            string
	apipath           string
	oauthStateString  string
	CertDir           string
	GoogleClientID    string
	GoogleSecret      string
	googleOauthConfig *oauth2.Config
	store             *sessions.CookieStore
	sessionName       string
	CookieSessionKey  string
	templateSet       map[string]*template.Template // allow multiple translations
	Logfile           string
}

var config Configuration
var unrolled *logger.Logger
var logfile *os.File

// initializeConfig will normalize the options and create the "config" object.
func initializeConfig(initialConfig Configuration) {
	config = initialConfig

	config.Root = strings.TrimSuffix(config.Root, "/")

	// this can be hardcoded for now
	config.apipath = "api/v1"

	// Extract "path" fron "root"
	rootParts := strings.SplitAfterN(config.Root, "/", 4) // https://example.org/[grab this part]
	config.path = ""
	if len(rootParts) > 3 { // Otherwise: application in root folder
		config.path = rootParts[3]
	}
	config.path = strings.TrimSuffix("/"+strings.TrimPrefix(config.path, "/"), "/")

	rootParts = strings.SplitN(strings.ToLower(config.Root), "://", 2)
	config.domain = strings.Split(rootParts[len(rootParts)-1], "/")[0]

	// used for templates
	WASABI.SetWebroot(config.Root)
	WASABI.SetWebAPIPath(config.apipath)

	if config.GoogleClientID == "" {
		WASABI.Log.Error("GOOGLE_CLIENT_ID unset: logins will fail")
	}
	if config.GoogleSecret == "" {
		WASABI.Log.Error("GOOGLE_SECRET unset: logins will fail")
	}

	config.googleOauthConfig = &oauth2.Config{
		RedirectURL:  config.Root + "/callback",
		ClientID:     config.GoogleClientID,
		ClientSecret: config.GoogleSecret,
		Scopes:       []string{"https://www.googleapis.com/auth/userinfo.email"},
		Endpoint:     google.Endpoint,
	}
	WASABI.Log.Debugf("ClientID: " + config.googleOauthConfig.ClientID)
	WASABI.Log.Debugf("ClientSecret: " + config.googleOauthConfig.ClientSecret)
	config.oauthStateString = WASABI.GenerateName()
	WASABI.Log.Debugf("oauthStateString: " + config.oauthStateString)

	if config.CookieSessionKey == "" {
		WASABI.Log.Error("SESSION_KEY unset: logins will fail")
	} else {
		key := config.CookieSessionKey
		WASABI.Log.Debugf("Session Key: " + key)
		config.store = sessions.NewCookieStore([]byte(key))
		config.sessionName = "WASABI"
	}

	// certificate directory cleanup
	if config.CertDir == "" {
		WASABI.Log.Error("CERDIR unset: defaulting to 'certs'")
		config.CertDir = "certs"
	}
	certdir, err := filepath.Abs(config.CertDir)
	config.CertDir = certdir
	if err != nil {
		WASABI.Log.Critical("Certificate path could not be resolved.")
		panic(err)
	}
	WASABI.Log.Debugf("Certificate Directory: " + config.CertDir)
	_ = wasabiHTTPSTemplateConfig()

	if config.Logfile == "" {
		config.Logfile = "wasabi-https.log"
	}
	WASABI.Log.Infof("https logfile: %s", config.Logfile)
	logfile, err := os.OpenFile(config.Logfile, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		WASABI.Log.Fatal(err)
	}
	unrolled = logger.New(logger.Options{
		Prefix: "WASABI",
		Out:    logfile,
		IgnoredRequestURIs: []string{"/favicon.ico",
			"/OwnTracks",
			"/simple",
			"/apple-touch-icon-precomposed.png",
			"/apple-touch-icon-120x120-precomposed.png",
			"/apple-touch-icon-120x120.png",
			"/apple-touch-icon.png"},
	})
}

func wasabiHTTPSTemplateConfig() error {
	// Transform frontendPath to an absolute path
	frontendPath, err := filepath.Abs(config.FrontendPath)
	if err != nil {
		WASABI.Log.Critical("Frontend path could not be resolved.")
		panic(err)
	}
	config.FrontendPath = frontendPath
	config.templateSet = make(map[string]*template.Template)

	WASABI.Log.Debugf("Loading Template function map")
	funcMap := template.FuncMap{
		"TGGetBotName": WASABI.TGGetBotName,
		"TGGetBotID":   WASABI.TGGetBotID,
		"TGRunning":    WASABI.TGRunning,
		"Webroot":      WASABI.GetWebroot,
		"WebAPIPath":   WASABI.GetWebAPIPath,
		"VEnlOne":      WASABI.GetvEnlOne,
		"EnlRocks":     WASABI.GetEnlRocks,
	}

	WASABI.Log.Info("Including frontend templates from: ", config.FrontendPath)
	files, err := ioutil.ReadDir(config.FrontendPath)
	if err != nil {
		WASABI.Log.Error(err)
		return err
	}

	for _, f := range files {
		lang := f.Name()
		if f.IsDir() && len(lang) == 2 {
			config.templateSet[lang] = template.New("").Funcs(funcMap) // one funcMap for all languages
			// load the masters
			config.templateSet[lang].ParseGlob(config.FrontendPath + "/master/*.html")
			// overwrite with language specific
			config.templateSet[lang].ParseGlob(config.FrontendPath + "/" + lang + "/*.html")
			WASABI.Log.Debugf("Templates for lang [%s] %s", lang, config.templateSet[lang].DefinedTemplates())
		}
	}
	return nil
}

// wasabiHTTPSTemplateExecute outputs directly to the ResponseWriter
func wasabiHTTPSTemplateExecute(res http.ResponseWriter, req *http.Request, name string, data interface{}) error {
	// XXX get the lang from the request
	// XXX read and parse the request language header
	lang := "en"

	_, ok := config.templateSet[lang]
	if ok == false {
		lang = "en" // default to english if the map doesn't exist
	}

	if err := config.templateSet[lang].ExecuteTemplate(res, name, data); err != nil {
		WASABI.Log.Notice(err)
		return err
	}
	return nil
}

// errorScreen outputs directly to the response writer
// XXX this may not work since http.Error takes a string...
// XXX maybe replace http.Error calls with this?
func errorScreen(res http.ResponseWriter, req *http.Request, err error) {
	_ = wasabiHTTPSTemplateExecute(res, req, "error", err.Error())
}

// StartHTTP launches the HTTP server which is responsible for the frontend and the HTTP API.
func StartHTTP(initialConfig Configuration) {
	// Configure
	initializeConfig(initialConfig)

	// Route
	r := mux.NewRouter()

	s := r.PathPrefix("/").Subrouter()
	setupAuthRoutes(s)
	setupRoutes(r)

	// r.Use(debugMW)
	r.Use(headersMW)
	r.Use(unrolled.Handler)

	// s.Use(debugMW)
	// s.Use(headersMW) // seems to be redundant on s.
	// s.Use(unrolled.Handler) // seems to be redundant on s.
	s.Use(authMW)

	// Serve
	WASABI.Log.Noticef("HTTPS server starting on %s, you should be able to reach it at %s", config.ListenHTTPS, config.Root)
	err := http.ListenAndServeTLS(config.ListenHTTPS, config.CertDir+"/WASABI.fullchain.pem", config.CertDir+"/WASABI.key", r)
	if err != nil {
		WASABI.Log.Errorf("HTTPS server error: %s", err)
		panic(err)
	}
}

func headersMW(next http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		res.Header().Add("Server", "WASABI")
		res.Header().Add("X-Content-Type", "nosniff") // We're not a CDN.
		res.Header().Add("Access-Control-Allow-Origin", "https://intel.ingress.com")
		res.Header().Add("Access-Control-Allow-Methods", "POST, GET, PUT, OPTIONS, HEAD, DELETE")
		res.Header().Add("Access-Control-Allow-Credentials", "true")
		// untested
		// res.Header().Add("Access-Control-Allow-Headers", "Content-Type, Accept, X-Requested-With, remember-me")
		next.ServeHTTP(res, req)
	})
}

func debugMW(next http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		dump, _ := httputil.DumpRequest(req, false)
		WASABI.Log.Debug(string(dump))
		next.ServeHTTP(res, req)
	})
}

func authMW(next http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		ses, err := config.store.Get(req, config.sessionName)

		if err != nil {
			WASABI.Log.Debug(err)
			delete(ses.Values, "nonce")
			delete(ses.Values, "id")
			delete(ses.Values, "loginReq")
			ses.Save(req, res)
			http.Error(res, err.Error(), http.StatusInternalServerError)
			return
		}

		var redirectURL = "/login"
		if req.URL.String()[:3] != "/me" {
			redirectURL = "/login?returnto=" + req.URL.String()
		}

		id, ok := ses.Values["id"]
		if ok == false || id == nil {
			// XXX cookie and returnto may be redundant, but cookie wasn't working in early tests
			ses.Values["loginReq"] = req.URL.String()
			ses.Save(req, res)
			http.Redirect(res, req, redirectURL, http.StatusFound)
			return
		}

		gid := WASABI.GoogleID(id.(string))

		in, ok := ses.Values["nonce"]
		if ok != true || in == nil {
			WASABI.Log.Error("gid set, but nonce not")
			http.Redirect(res, req, redirectURL, http.StatusFound)
			return
		}
		inNonce := in.(string)
		nonce, pNonce := calculateNonce(gid)

		if inNonce != nonce {
			if inNonce != pNonce {
				WASABI.Log.Debug("Session timed out for", gid.String())
				ses.Values["nonce"] = "unset"
				ses.Save(req, res)
			} else {
				// WASABI.Log.Debug("Updating to new nonce")
				ses.Values["nonce"] = nonce
				ses.Save(req, res)
			}
		}

		if ses.Values["nonce"] == "unset" {
			http.Redirect(res, req, redirectURL, http.StatusFound)
			return
		}
		next.ServeHTTP(res, req)
	})
}

func googleRoute(res http.ResponseWriter, req *http.Request) {
	ret := req.FormValue("returnto")

	ses, err := config.store.Get(req, config.sessionName)
	if err != nil {
		WASABI.Log.Debug(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	if ret != "" {
		ses.Values["loginReq"] = ret
	} else {
		ses.Values["loginReq"] = "/me"
	}
	ses.Save(req, res)

	url := config.googleOauthConfig.AuthCodeURL(config.oauthStateString)
	// res.Header().Add("Cache-Control", "no-cache")
	// http.Redirect(res, req, url, http.StatusTemporaryRedirect)
	http.Redirect(res, req, url, http.StatusFound)
}
