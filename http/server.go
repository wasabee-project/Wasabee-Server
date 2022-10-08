package wasabeehttps

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"net/http/httputil"
	"os"
	"path"
	"strings"
	"time"

	// "golang.org/x/oauth2"
	//"golang.org/x/oauth2/google"
	// "github.com/lestrrat-go/jwx/v2/jws"
	"github.com/lestrrat-go/jwx/v2/jws"
	"github.com/lestrrat-go/jwx/v2/jwt"

	// "github.com/gorilla/sessions"
	"github.com/wasabee-project/Wasabee-Server/auth"
	"github.com/wasabee-project/Wasabee-Server/config"
	"github.com/wasabee-project/Wasabee-Server/log"
	"github.com/wasabee-project/Wasabee-Server/model"
	"github.com/wasabee-project/Wasabee-Server/util"

	// XXX gorilla has logging middleware, use that instead?
	"github.com/unrolled/logger"
)

var srv *http.Server

var unrolled *logger.Logger
var oauthStateString string

const jsonType = "application/json; charset=UTF-8"
const jsonTypeShort = "application/json"
const jsonStatusOK = `{"status":"ok"}`
const jsonStatusEmpty = `{"status":"error","error":"Empty JSON"}`

// Start launches the HTTP server which is responsible for the frontend and the HTTP API.
func Start() {
	c := config.Get()
	c.HTTP.Webroot = strings.TrimSuffix(c.HTTP.Webroot, "/")

	// set up the scanners list
	scanners = util.NewSafemap()

	oc := config.GetOauthConfig()
	if oc.ClientID == "" || oc.ClientSecret == "" {
		log.Errorw("startup", "Oauth ClientID", oc.ClientID, "Oauth ClientSecret", oc.ClientSecret)
		log.Fatal("Oauth Client not configured: logins will fail")
	}

	oauthStateString = util.GenerateName()
	// log.Debugw("startup", "oauthStateString", oauthStateString)

	log.Debugw("startup", "https logfile", c.HTTP.Logfile)
	// #nosec
	logfileHandle, err := os.OpenFile(c.HTTP.Logfile, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		log.Fatal(err)
	}
	unrolled = logger.New(logger.Options{
		Prefix: "wasabee",
		Out:    logfileHandle,
		IgnoredRequestURIs: []string{
			"/favicon.ico",
			"/apple-touch-icon-precomposed.png",
			"/apple-touch-icon-120x120-precomposed.png",
			"/apple-touch-icon-120x120.png",
			"/apple-touch-icon.png"},
	})

	// setup the main router an built-in subrouters
	router := setupRouter()

	// serve
	srv = &http.Server{
		Handler:           router,
		Addr:              c.HTTP.ListenHTTPS,
		WriteTimeout:      (30 * time.Second),
		ReadTimeout:       (30 * time.Second),
		ReadHeaderTimeout: (2 * time.Second),
		TLSConfig: &tls.Config{
			MinVersion:               tls.VersionTLS12,
			CurvePreferences:         []tls.CurveID{tls.CurveP521, tls.CurveP384, tls.CurveP256},
			PreferServerCipherSuites: true,
		},
	}

	fc := path.Join(c.Certs, c.CertFile)
	k := path.Join(c.Certs, c.CertKey)
	log.Infow("startup", "port", c.HTTP.ListenHTTPS, "url", c.HTTP.Webroot, "message", "online at "+c.HTTP.Webroot)
	if err := srv.ListenAndServeTLS(fc, k); err != http.ErrServerClosed {
		log.Error(err)
	}
}

// Shutdown forces a graceful shutdown of the https server
func Shutdown() error {
	log.Infow("shutdown", "message", "shutting down HTTPS server")
	if err := srv.Shutdown(context.Background()); err != nil {
		log.Error(err)
		return err
	}
	return nil
}

func headersMW(next http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		if isScanner(req) {
			log.Warnw("scanner detected", "ip", req.RemoteAddr)
			http.Error(res, "permission denied", http.StatusForbidden)
			return
		}

		permitted := config.Get().HTTP.CORS
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
		res.Header().Add("Access-Control-Allow-Methods", "POST, GET, PUT, OPTIONS, HEAD, DELETE, PATCH")
		res.Header().Add("Access-Control-Allow-Credentials", "true")
		res.Header().Add("Access-Control-Allow-Headers", "Content-Type, Accept, If-Modified-Since, If-Match, If-None-Match, Authorization")
		res.Header().Add("Content-Type", jsonType)
		next.ServeHTTP(res, req)
	})
}

/*
func logRequestMW(next http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		log.Debug("REQ", req.Method, req.RequestURI)
		rec := statusRecorder{res, 200}
		next.ServeHTTP(&rec, req)
		log.Debug("RESP", rec.status, req.RequestURI)
	})
}
*/

func authMW(next http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		sessionName := config.Get().HTTP.SessionName

		h := req.Header.Get("Authorization")
		if h == "" {
			log.Infow("JWT missing")
			http.Error(res, "JWT missing", http.StatusUnauthorized)
			return
		}

		token, err := jwt.ParseRequest(req,
			jwt.WithKeySet(config.JWParsingKeys(), jws.WithInferAlgorithmFromKey(true), jws.WithUseDefault(true)),
			jwt.WithValidate(true),
			jwt.WithAudience(sessionName),
			// jwt.WithIssuer("https://accounts.google.com"),
			jwt.WithAcceptableSkew(20*time.Second),
		)
		if err != nil {
			log.Info(err)
			http.Error(res, err.Error(), http.StatusUnauthorized)
			return
		}

		// expiration validation is implicit -- redundant with above now
		if err := jwt.Validate(token, jwt.WithAudience(sessionName)); err != nil {
			log.Infow("JWT validate failed", "error", err, "sub", token.Subject())
			http.Error(res, err.Error(), http.StatusUnauthorized)
			return
		}

		if auth.IsRevokedJWT(token.JwtID()) {
			log.Infow("JWT revoked", "sub", token.Subject(), "token ID", token.JwtID())
			http.Error(res, err.Error(), http.StatusUnauthorized)
			return
		}

		gid := model.GoogleID(token.Subject())
		// too db intensive? -- cache it?
		if !gid.Valid() {
			// token minted on another server, never logged in to this server
			if err := gid.FirstLogin(); err != nil {
				log.Info(err)
				http.Error(res, err.Error(), http.StatusUnauthorized)
				return
			}
		}

		// pass the GoogleID around so subsequent functions can easily access it
		ctx := context.WithValue(req.Context(), "X-Wasabee-GID", gid)
		req = req.WithContext(ctx)
		next.ServeHTTP(res, req)
	})
}

func redirectOrError(res http.ResponseWriter, req *http.Request) {
	c := config.Get().HTTP

	if strings.Contains(req.Referer(), "intel.ingress.com") {
		http.Error(res, "Unauthorized", http.StatusUnauthorized)
	} else {
		var redirectURL = c.LoginURL
		if req.URL.String()[:len(c.MeURL)] != c.MeURL {
			redirectURL = c.LoginURL + "?returnto=" + req.URL.String()
		}

		http.Redirect(res, req, redirectURL, http.StatusFound)
	}
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
