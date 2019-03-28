package PhDevHTTP

import (
	"html/template"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"path/filepath"
	"strings"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	"github.com/cloudkucooland/PhDevBin"
	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
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
	// templateSet       *template.Template
}

var config Configuration

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
	PhDevBin.SetWebroot(config.Root)
	PhDevBin.SetWebAPIPath(config.apipath)

	if config.GoogleClientID == "" {
		PhDevBin.Log.Error("GOOGLE_CLIENT_ID unset: logins will fail")
	}
	if config.GoogleSecret == "" {
		PhDevBin.Log.Error("GOOGLE_SECRET unset: logins will fail")
	}

	config.googleOauthConfig = &oauth2.Config{
		RedirectURL:  config.Root + "/callback",
		ClientID:     config.GoogleClientID,
		ClientSecret: config.GoogleSecret,
		Scopes:       []string{"https://www.googleapis.com/auth/userinfo.email"},
		Endpoint:     google.Endpoint,
	}
	PhDevBin.Log.Debugf("ClientID: " + config.googleOauthConfig.ClientID)
	PhDevBin.Log.Debugf("ClientSecret: " + config.googleOauthConfig.ClientSecret)
	config.oauthStateString = PhDevBin.GenerateName()
	PhDevBin.Log.Debugf("oauthStateString: " + config.oauthStateString)

	if config.CookieSessionKey == "" {
		PhDevBin.Log.Error("SESSION_KEY unset: logins will fail")
	} else {
		key := config.CookieSessionKey
		PhDevBin.Log.Debugf("Session Key: " + key)
		config.store = sessions.NewCookieStore([]byte(key))
		config.sessionName = "PhDevBin"
	}

	// certificate directory cleanup
	if config.CertDir == "" {
		PhDevBin.Log.Error("CERDIR unset: defaulting to 'certs'")
		config.CertDir = "certs"
	}
	certdir, err := filepath.Abs(config.CertDir)
	config.CertDir = certdir
	if err != nil {
		PhDevBin.Log.Critical("Certificate path could not be resolved.")
		panic(err)
	}
	PhDevBin.Log.Debugf("Certificate Directory: " + config.CertDir)
	_ = phDevBinHTTPSTemplateConfig()
}

func phDevBinHTTPSTemplateConfig() error {
	// Transform frontendPath to an absolute path
	frontendPath, err := filepath.Abs(config.FrontendPath)
	if err != nil {
		PhDevBin.Log.Critical("Frontend path could not be resolved.")
		panic(err)
	}
	config.FrontendPath = frontendPath
	config.templateSet = make(map[string]*template.Template)

	PhDevBin.Log.Debugf("Loading Template function map")
	funcMap := template.FuncMap{
		"TGGetBotName": PhDevBin.TGGetBotName,
		"TGGetBotID":   PhDevBin.TGGetBotID,
		"TGRunning":    PhDevBin.TGRunning,
		"Webroot":      PhDevBin.GetWebroot,
		"WebAPIPath":   PhDevBin.GetWebAPIPath,
		"VEnlOne":      PhDevBin.GetvEnlOne,
		"EnlRocks":     PhDevBin.GetEnlRocks,
	}

	PhDevBin.Log.Notice("Including frontend templates from: ", config.FrontendPath)
	files, err := ioutil.ReadDir(config.FrontendPath)
	if err != nil {
		PhDevBin.Log.Error(err)
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
			PhDevBin.Log.Debugf("Templates for lang [%s] %s", lang, config.templateSet[lang].DefinedTemplates())
		}
	}
	return nil
}

// phDevBinHTTPSTemplateExecute outputs directly to the ResponseWriter
func phDevBinHTTPSTemplateExecute(res http.ResponseWriter, req *http.Request, name string, data interface{}) error {
	// get the lang from the request
	lang := "en"

	_, ok := config.templateSet[lang]
	if ok == false {
		lang = "en" // default to english if the map doesn't exist
	}

	if err := config.templateSet[lang].ExecuteTemplate(res, name, data); err != nil {
		PhDevBin.Log.Notice(err)
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

	s := r.PathPrefix("/").Subrouter()
	setupAuthRoutes(s)
	setupRoutes(r)

	r.Use(headersMW)
	// r.Use(debugMW)

	// s.Use(debugMW)
	s.Use(headersMW)
	s.Use(authMW)

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
		// untested
		// res.Header().Add("Access-Control-Allow-Headers", "Content-Type, Accept, X-Requested-With, remember-me")
		next.ServeHTTP(res, req)
	})
}

func debugMW(next http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		dump, _ := httputil.DumpRequest(req, false)
		PhDevBin.Log.Debug(string(dump))
		next.ServeHTTP(res, req)
	})
}

func authMW(next http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		ses, err := config.store.Get(req, config.sessionName)

		if err != nil {
			PhDevBin.Log.Debug(err)
			delete(ses.Values, "nonce")
			delete(ses.Values, "id")
			delete(ses.Values, "loginReq")
			ses.Save(req, res)
			http.Error(res, err.Error(), http.StatusInternalServerError)
			return
		}

		id, ok := ses.Values["id"]
		if ok == false || id == nil {
			// XXX cookie and returnto may be redundant, but cookie wasn't working in early tests
			ses.Values["loginReq"] = req.URL.String()
			ses.Save(req, res)
			http.Redirect(res, req, "/login?returnto="+req.URL.String(), http.StatusPermanentRedirect)
			return
		}

		gid := PhDevBin.GoogleID(id.(string))

		nonce, pNonce, _ := calculateNonce(gid)
		in, ok := ses.Values["nonce"]
		if ok != true || in == nil {
			PhDevBin.Log.Error("gid set, but nonce not")
			http.Redirect(res, req, "/login", http.StatusPermanentRedirect)
			return
		}
		inNonce := in.(string)

		if inNonce != nonce {
			if inNonce != pNonce {
				// PhDevBin.Log.Debug("Session timed out")
				ses.Values["nonce"] = "unset"
				ses.Save(req, res)
			} else {
				// PhDevBin.Log.Debug("Updating to new nonce")
				ses.Values["nonce"] = nonce
				ses.Save(req, res)
			}
		}

		if ses.Values["nonce"] == "unset" {
			http.Redirect(res, req, "/login", http.StatusPermanentRedirect)
			return
		}
		next.ServeHTTP(res, req)
	})
}

func googleRoute(res http.ResponseWriter, req *http.Request) {
	ret := req.FormValue("returnto")

	if ret != "" {
		ses, err := config.store.Get(req, config.sessionName)
		if err != nil {
			PhDevBin.Log.Debug(err)
			http.Error(res, err.Error(), http.StatusInternalServerError)
			return
		}
		ses.Values["loginReq"] = ret
		ses.Save(req, res)
	}

	url := config.googleOauthConfig.AuthCodeURL(config.oauthStateString)
	// res.Header().Add("Cache-Control", "no-cache")
	http.Redirect(res, req, url, http.StatusTemporaryRedirect)
}
