package wasabeehttps

import (
	"context"
	"crypto/tls"
	"fmt"
	"html/template"
	"net/http"
	"net/http/httputil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/oauth2"
	//"golang.org/x/oauth2/google"
	"github.com/lestrrat-go/jwx/jws"
	"github.com/lestrrat-go/jwx/jwt"

	"github.com/gorilla/sessions"
	"github.com/wasabee-project/Wasabee-Server/auth"
	"github.com/wasabee-project/Wasabee-Server/config"
	"github.com/wasabee-project/Wasabee-Server/generatename"
	"github.com/wasabee-project/Wasabee-Server/log"
	"github.com/wasabee-project/Wasabee-Server/model"

	// XXX gorilla has logging middleware, use that instead?
	"github.com/unrolled/logger"
)

// Configuration is the main configuration data for the https server
// an initial config is sent from main() and that is updated with defaults
// in the initializeConfig function
type Configuration struct {
	ListenHTTPS      string
	FrontendPath     string
	Root             string
	path             string
	oauthStateString string
	CertDir          string
	OauthConfig      *oauth2.Config
	OauthUserInfoURL string
	store            *sessions.CookieStore
	sessionName      string
	CookieSessionKey string
	TemplateSet      map[string]*template.Template // allow multiple translations
	Logfile          string
	srv              *http.Server
	logfileHandle    *os.File
	unrolled         *logger.Logger
	scanners         map[string]int64
}

var c Configuration

const jsonType = "application/json; charset=UTF-8"
const jsonTypeShort = "application/json"
const jsonStatusOK = `{"status":"ok"}`
const jsonStatusEmpty = `{"status":"error","error":"Empty JSON"}`
const me = "/me"
const login = "/login"
const callback = "/callback"
const aptoken = "/aptok"
const apipath = "/api/v1"
const oneTimeToken = "/oneTimeToken"

func initializeConfig(initialConfig Configuration) {
	c = initialConfig

	c.Root = strings.TrimSuffix(c.Root, "/")

	// Extract "path" fron "root"
	rootParts := strings.SplitAfterN(c.Root, "/", 4) // https://example.org/[grab this part]
	c.path = ""
	if len(rootParts) > 3 { // Otherwise: application in root folder
		c.path = rootParts[3]
	}
	c.path = strings.TrimSuffix("/"+strings.TrimPrefix(c.path, "/"), "/")

	// used for templates
	config.SetWebroot(c.Root)
	config.SetWebAPIPath(apipath)

	if c.OauthConfig.ClientID == "" {
		log.Fatal("OAUTH_CLIENT_ID unset: logins will fail")
	}
	if c.OauthConfig.ClientSecret == "" {
		log.Fatal("OAUTH_SECRET unset: logins will fail")
	}

	log.Debugw("startup", "ClientID", c.OauthConfig.ClientID)
	log.Debugw("startup", "ClientSecret", c.OauthConfig.ClientSecret)
	c.oauthStateString = generatename.GenerateName()
	log.Debugw("startup", "oauthStateString", c.oauthStateString)

	if c.CookieSessionKey == "" {
		log.Error("SESSION_KEY unset: logins will fail")
	} else {
		key := c.CookieSessionKey
		log.Debugw("startup", "Session Key", key)
		c.store = sessions.NewCookieStore([]byte(key))
		c.sessionName = "wasabee"
	}

	// certificate directory cleanup
	if c.CertDir == "" {
		log.Warn("CERTDIR unset: defaulting to 'certs'")
		c.CertDir = "certs"
	}
	certdir, err := filepath.Abs(c.CertDir)
	c.CertDir = certdir
	if err != nil {
		log.Fatal("certificate path could not be resolved.")
		// panic(err)
	}
	log.Debugw("startup", "Certificate Directory", c.CertDir)

	if c.Logfile == "" {
		log.Debug("https logfile unset: defaulting to 'wasabee-https.log'")
		c.Logfile = "wasabee-https.log"
	}
	log.Debugw("startup", "https logfile", c.Logfile)
	// #nosec
	c.logfileHandle, err = os.OpenFile(c.Logfile, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		log.Fatal(err)
	}
	c.unrolled = logger.New(logger.Options{
		Prefix: "wasabee",
		Out:    c.logfileHandle,
		IgnoredRequestURIs: []string{
			"/favicon.ico",
			"/apple-touch-icon-precomposed.png",
			"/apple-touch-icon-120x120-precomposed.png",
			"/apple-touch-icon-120x120.png",
			"/apple-touch-icon.png"},
	})
	c.scanners = make(map[string]int64)
}

// templateExecute outputs directly to the ResponseWriter
func templateExecute(res http.ResponseWriter, req *http.Request, name string, data interface{}) error {
	var lang string
	tmp := req.Header.Get("Accept-Language")
	if tmp == "" || len(tmp) < 2 {
		lang = "en"
	} else {
		lang = strings.ToLower(tmp)[:2]
	}
	_, ok := c.TemplateSet[lang]
	if !ok {
		lang = "en" // default to english if the map doesn't exist
	}

	if err := c.TemplateSet[lang].ExecuteTemplate(res, name, data); err != nil {
		log.Error(err)
		return err
	}
	return nil
}

// StartHTTP launches the HTTP server which is responsible for the frontend and the HTTP API.
func StartHTTP(initialConfig Configuration) {
	// take the incoming c, add defaults
	initializeConfig(initialConfig)

	// setup the main router an built-in subrouters
	router := setupRouter()

	// serve
	c.srv = &http.Server{
		Handler:           router,
		Addr:              c.ListenHTTPS,
		WriteTimeout:      (15 * time.Second),
		ReadTimeout:       (15 * time.Second),
		ReadHeaderTimeout: (2 * time.Second),
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

	log.Infow("startup", "port", c.ListenHTTPS, "url", c.Root, "message", "online at "+c.Root)
	if err := c.srv.ListenAndServeTLS(c.CertDir+"/wasabee.fullchain.pem", c.CertDir+"/wasabee.key"); err != nil {
		log.Fatal(err)
		// panic(err)
	}
}

// Shutdown forces a graceful shutdown of the https server
func Shutdown() error {
	log.Infow("shutdown", "message", "shutting down HTTPS server")
	if err := c.srv.Shutdown(context.Background()); err != nil {
		log.Error(err)
		return err
	}
	return nil
}

func headersMW(next http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		permitted := []string{"https://intel.ingress.com", "https://wasabee-project.github.io", "https://cdn2.wasabee.rocks"}

		ref := permitted[0]
		origin := req.Header.Get("Origin")
		for p, v := range permitted {
			if origin == v {
				ref = permitted[p]
			}
		}

		res.Header().Add("Server", "Wasabee-Server")
		res.Header().Add("Content-Security-Policy", fmt.Sprintf("frame-ancestors %s", ref))
		res.Header().Add("X-Frame-Options", fmt.Sprintf("allow-from %s", ref)) // deprecated
		res.Header().Add("Access-Control-Allow-Origin", ref)
		res.Header().Add("Access-Control-Allow-Methods", "POST, GET, PUT, OPTIONS, HEAD, DELETE")
		res.Header().Add("Access-Control-Allow-Credentials", "true")
		res.Header().Add("Access-Control-Allow-Headers", "Content-Type, Accept, If-Modified-Since, If-Match, If-None-Match")
		next.ServeHTTP(res, req)
	})
}

func scannerMW(next http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		i, ok := c.scanners[req.RemoteAddr]
		if ok && i > 30 {
			http.Error(res, "scanner detected", http.StatusForbidden)
			return
		}
		next.ServeHTTP(res, req)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (rec *statusRecorder) WriteHeader(code int) {
	rec.status = code
	rec.ResponseWriter.WriteHeader(code)
}

func logRequestMW(next http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		log.Debug("REQ", req.Method, req.RequestURI)
		rec := statusRecorder{res, 200}
		next.ServeHTTP(&rec, req)
		log.Debug("RESP", rec.status, req.RequestURI)
	})
}

func authMW(next http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		req.Header.Del("X-Wasabee-GID") // don't allow spoofing

		if h := req.Header.Get("Authorization"); h != "" {
			x := strings.TrimSpace(strings.TrimPrefix(h, "Bearer"))
			tst, err := jws.ParseString(x)
			log.Debugw("jws", "payload", string(tst.Payload()))

			token, err := jwt.ParseRequest(req, jwt.InferAlgorithmFromKey(true), jwt.UseDefaultKey(true))
			// jwt.WithKeySet(config.Get().JWParsingKeys))
			if err != nil {
				// log.Debugw("auth header", "value", req.Header.Get("Authorization"))
				log.Info(err)
				http.Error(res, err.Error(), http.StatusUnauthorized)
				return
			}
			// log.Infow("JWT", "value", token)

			if err := jwt.Validate(token, jwt.WithAudience("wasabee")); err != nil {
				log.Info(err)
				http.Error(res, err.Error(), http.StatusUnauthorized)
				return
			}

			// pass the GoogleID around so subsequent functions can easily access it
			req.Header.Set("X-Wasabee-GID", token.Subject())
			next.ServeHTTP(res, req)
			return
		}

		// no JWT, use legacy cookie
		ses, err := c.store.Get(req, c.sessionName)
		if err != nil {
			log.Error(err)
			delete(ses.Values, "nonce")
			delete(ses.Values, "id")
			delete(ses.Values, "loginReq")
			res.Header().Set("Connection", "close")
			_ = ses.Save(req, res)
			http.Error(res, err.Error(), http.StatusInternalServerError)
			return
		}
		ses.Options = &sessions.Options{
			Path:     "/",
			MaxAge:   86400, // 0,
			SameSite: http.SameSiteNoneMode,
			Secure:   true,
		}

		id, ok := ses.Values["id"]
		if !ok || id == nil {
			// XXX cookie and returnto may be redundant, but cookie wasn't working in early tests
			ses.Values["loginReq"] = req.URL.String()
			res.Header().Set("Connection", "close")
			_ = ses.Save(req, res)
			// log.Debug("not logged in")
			redirectOrError(res, req)
			return
		}

		gid := model.GoogleID(id.(string))
		if auth.CheckLogout(gid) {
			log.Debugw("honoring previously requested logout", "gid", gid)
			delete(ses.Values, "nonce")
			delete(ses.Values, "id")
			ses.Options = &sessions.Options{
				Path:     "/",
				MaxAge:   -1,
				SameSite: http.SameSiteNoneMode,
				Secure:   true,
			}
			http.Redirect(res, req, "/", http.StatusFound)
			return
		}

		in, ok := ses.Values["nonce"]
		if !ok || in == nil {
			log.Errorw("gid set, but no nonce", "GID", gid)
			redirectOrError(res, req)
			return
		}

		nonce, pNonce := calculateNonce(gid)
		if in.(string) != nonce {
			res.Header().Set("Connection", "close")
			if in.(string) != pNonce {
				// log.Debugw("session timed out", "gid", gid)
				ses.Values["nonce"] = "unset"
			} else {
				ses.Values["nonce"] = nonce
			}
			_ = ses.Save(req, res)
		}

		if ses.Values["nonce"] == "unset" {
			redirectOrError(res, req)
			return
		}

		req.Header.Set("X-Wasabee-GID", gid.String())
		next.ServeHTTP(res, req)
	})
}

func redirectOrError(res http.ResponseWriter, req *http.Request) {
	if strings.Contains(req.Referer(), "intel.ingress.com") {
		http.Error(res, "Unauthorized", http.StatusUnauthorized)
	} else {
		var redirectURL = login
		if req.URL.String()[:len(me)] != me {
			redirectURL = login + "?returnto=" + req.URL.String()
		}

		http.Redirect(res, req, redirectURL, http.StatusFound)
	}
}

func googleRoute(res http.ResponseWriter, req *http.Request) {
	ret := req.FormValue("returnto")

	ses, err := c.store.Get(req, c.sessionName)
	if err != nil {
		log.Error(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	if ret != "" {
		ses.Values["loginReq"] = ret
	} else {
		ses.Values["loginReq"] = me
	}
	ses.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   86400, // 0,
		SameSite: http.SameSiteNoneMode,
		Secure:   true,
	}
	_ = ses.Save(req, res)

	// the server may have several names/ports ; redirect back to the one the user called
	oc := c.OauthConfig
	oc.RedirectURL = fmt.Sprintf("https://%s%s", req.Host, callback)
	url := oc.AuthCodeURL(c.oauthStateString)
	http.Redirect(res, req, url, http.StatusSeeOther)
}

func jsonError(e error) string {
	return fmt.Sprintf(`{"status":"error","error":"%s"}`, e.Error())
}

func debugMW(next http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		dump, _ := httputil.DumpRequest(req, false)
		log.Debug(string(dump))
		next.ServeHTTP(res, req)
	})
}

func contentTypeIs(req *http.Request, check string) bool {
	contentType := strings.Split(strings.Replace(req.Header.Get("Content-Type"), " ", "", -1), ";")[0]
	return strings.EqualFold(contentType, check)
}
