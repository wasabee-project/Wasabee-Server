package wasabihttps

import (
	"crypto/tls"
	"html/template"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"os"
	"path/filepath"
	"strings"
	"time"

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
	wasabi.SetWebroot(config.Root)
	wasabi.SetWebAPIPath(config.apipath)

	if config.GoogleClientID == "" {
		wasabi.Log.Error("GOOGLE_CLIENT_ID unset: logins will fail")
	}
	if config.GoogleSecret == "" {
		wasabi.Log.Error("GOOGLE_SECRET unset: logins will fail")
	}

	config.googleOauthConfig = &oauth2.Config{
		RedirectURL:  config.Root + "/callback",
		ClientID:     config.GoogleClientID,
		ClientSecret: config.GoogleSecret,
		Scopes:       []string{"https://www.googleapis.com/auth/userinfo.email"},
		Endpoint:     google.Endpoint,
	}
	wasabi.Log.Debugf("ClientID: " + config.googleOauthConfig.ClientID)
	wasabi.Log.Debugf("ClientSecret: " + config.googleOauthConfig.ClientSecret)
	config.oauthStateString = wasabi.GenerateName()
	wasabi.Log.Debugf("oauthStateString: " + config.oauthStateString)

	if config.CookieSessionKey == "" {
		wasabi.Log.Error("SESSION_KEY unset: logins will fail")
	} else {
		key := config.CookieSessionKey
		wasabi.Log.Debugf("Session Key: " + key)
		config.store = sessions.NewCookieStore([]byte(key))
		config.sessionName = "WASABI"
	}

	// certificate directory cleanup
	if config.CertDir == "" {
		wasabi.Log.Error("CERTDIR unset: defaulting to 'certs'")
		config.CertDir = "certs"
	}
	certdir, err := filepath.Abs(config.CertDir)
	config.CertDir = certdir
	if err != nil {
		wasabi.Log.Critical("Certificate path could not be resolved.")
		panic(err)
	}
	wasabi.Log.Debugf("Certificate Directory: " + config.CertDir)
	_ = wasabiHTTPSTemplateConfig()

	if config.Logfile == "" {
		config.Logfile = "wasabi-https.log"
	}
	wasabi.Log.Infof("https logfile: %s", config.Logfile)
	logfile, err = os.OpenFile(config.Logfile, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		wasabi.Log.Fatal(err)
	}
	unrolled = logger.New(logger.Options{
		Prefix: "WASABI",
		Out:    logfile,
		IgnoredRequestURIs: []string{
			"/favicon.ico",
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
		wasabi.Log.Critical("Frontend path could not be resolved.")
		panic(err)
	}
	config.FrontendPath = frontendPath
	config.templateSet = make(map[string]*template.Template)

	wasabi.Log.Debugf("Loading Template function map")
	funcMap := template.FuncMap{
		"TGGetBotName": wasabi.TGGetBotName,
		"TGGetBotID":   wasabi.TGGetBotID,
		"TGRunning":    wasabi.TGRunning,
		"Webroot":      wasabi.GetWebroot,
		"WebAPIPath":   wasabi.GetWebAPIPath,
		"VEnlOne":      wasabi.GetvEnlOne,
		"EnlRocks":     wasabi.GetEnlRocks,
	}

	wasabi.Log.Info("Including frontend templates from: ", config.FrontendPath)
	files, err := ioutil.ReadDir(config.FrontendPath)
	if err != nil {
		wasabi.Log.Error(err)
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
			wasabi.Log.Debugf("Templates for lang [%s] %s", lang, config.templateSet[lang].DefinedTemplates())
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
	if !ok {
		lang = "en" // default to english if the map doesn't exist
	}

	if err := config.templateSet[lang].ExecuteTemplate(res, name, data); err != nil {
		wasabi.Log.Notice(err)
		return err
	}
	return nil
}

// StartHTTP launches the HTTP server which is responsible for the frontend and the HTTP API.
func StartHTTP(initialConfig Configuration) {
	// Configure
	initializeConfig(initialConfig)

	// Route
	r := mux.NewRouter()

	// establish subrouters -- these each have different middleware requirements
	api := r.PathPrefix("/" + config.apipath).Subrouter()
	tg := r.PathPrefix("/tg").Subrouter()
	me := r.PathPrefix("/me").Subrouter()
	simple := r.PathPrefix("/simple").Subrouter()
	notauthed := r.PathPrefix("").Subrouter()
	setupAuthRoutes(api)
	setupTelegramRoutes(tg)
	setupMeRoutes(me)
	setupSimpleRoutes(simple)
	setupRoutes(r)
	setupNotauthed(notauthed)

	// r. apply to all
	// r.Use(debugMW)
	r.Use(headersMW)
	// r.Use(unrolled.Handler)

	api.Use(authMW)
	api.Use(unrolled.Handler)
	me.Use(authMW)
	me.Use(unrolled.Handler)
	// tg.Use(debugMW)
	tg.Use(unrolled.Handler)
	notauthed.Use(unrolled.Handler)

	// Serve
	wasabi.Log.Noticef("HTTPS server starting on %s, you should be able to reach it at %s", config.ListenHTTPS, config.Root)
	srv := &http.Server{
		Handler:           r,
		Addr:              config.ListenHTTPS,
		WriteTimeout:      15 * time.Second,
		ReadTimeout:       15 * time.Second,
		ReadHeaderTimeout: 2 * time.Second,
		TLSConfig: &tls.Config{
			MinVersion:               tls.VersionTLS12,
			CurvePreferences:         []tls.CurveID{tls.CurveP521, tls.CurveP384, tls.CurveP256},
			PreferServerCipherSuites: true,
			CipherSuites: []uint16{
				tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
				tls.TLS_RSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_RSA_WITH_AES_256_CBC_SHA,
			},
		},
	}
	err := srv.ListenAndServeTLS(config.CertDir+"/WASABI.fullchain.pem", config.CertDir+"/WASABI.key")
	if err != nil {
		wasabi.Log.Errorf("HTTPS server error: %s", err)
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
		wasabi.Log.Debug(string(dump))
		next.ServeHTTP(res, req)
	})
}

func authMW(next http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		ses, err := config.store.Get(req, config.sessionName)

		if err != nil {
			wasabi.Log.Debug(err)
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
		if !ok || id == nil {
			// XXX cookie and returnto may be redundant, but cookie wasn't working in early tests
			ses.Values["loginReq"] = req.URL.String()
			ses.Save(req, res)
			http.Redirect(res, req, redirectURL, http.StatusFound)
			return
		}

		gid := wasabi.GoogleID(id.(string))

		in, ok := ses.Values["nonce"]
		if !ok || in == nil {
			wasabi.Log.Error("gid set, but nonce not")
			http.Redirect(res, req, redirectURL, http.StatusFound)
			return
		}
		inNonce := in.(string)
		nonce, pNonce := calculateNonce(gid)

		if inNonce != nonce {
			if inNonce != pNonce {
				// wasabi.Log.Debug("Session timed out for", gid.String())
				ses.Values["nonce"] = "unset"
				ses.Save(req, res)
			} else {
				// wasabi.Log.Debug("Updating to new nonce")
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
		wasabi.Log.Debug(err)
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
