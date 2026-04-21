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

	"github.com/lestrrat-go/jwx/v3/jws"
	"github.com/lestrrat-go/jwx/v3/jwt"

	"github.com/wasabee-project/Wasabee-Server/auth"
	"github.com/wasabee-project/Wasabee-Server/config"
	"github.com/wasabee-project/Wasabee-Server/log"
	"github.com/wasabee-project/Wasabee-Server/model"
	"github.com/wasabee-project/Wasabee-Server/util"

	"github.com/unrolled/logger"
)

var srv *http.Server
var unrolled *logger.Logger

// private type for context keys to avoid collisions
type wasabeeCtxKey string

const gidKey wasabeeCtxKey = "X-Wasabee-GID"

const (
	jsonType        = "application/json; charset=UTF-8"
	jsonTypeShort   = "application/json"
	jsonStatusOK    = `{"status":"ok"}`
	jsonStatusEmpty = `{"status":"error","error":"Empty JSON"}`
)

// Start launches the HTTP server
func Start() {
	c := config.Get()
	c.HTTP.Webroot = strings.TrimSuffix(c.HTTP.Webroot, "/")

	scanners = util.NewSafemap()

	oc := config.GetOauthConfig()
	if oc.ClientID == "" || oc.ClientSecret == "" {
		log.Fatal("Oauth Client not configured: logins will fail")
	}

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

	// setupRouter now returns *http.ServeMux
	mux := setupRouter()

	// Wrap the mux in global middleware
	handler := headersMW(unrolled.Handler(mux))

	srv = &http.Server{
		Handler:           handler,
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

// Shutdown forces a graceful shutdown
func Shutdown() error {
	log.Infow("shutdown", "message", "shutting down HTTPS server")
	return srv.Shutdown(context.Background())
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
		for _, v := range permitted {
			if origin == v {
				ref = v
				break
			}
		}

		res.Header().Add("Server", "Wasabee-Server")
		res.Header().Add("Content-Security-Policy", fmt.Sprintf("frame-ancestors %s", ref))
		res.Header().Add("X-Frame-Options", fmt.Sprintf("allow-from %s", ref))
		res.Header().Add("Access-Control-Allow-Origin", ref)
		res.Header().Add("Access-Control-Allow-Methods", "POST, GET, PUT, OPTIONS, HEAD, DELETE, PATCH")
		res.Header().Add("Access-Control-Allow-Credentials", "true")
		res.Header().Add("Access-Control-Allow-Headers", "Content-Type, Accept, If-Modified-Since, If-Match, If-None-Match, Authorization")
		res.Header().Add("Content-Type", jsonType)

		if req.Method == "OPTIONS" {
			res.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(res, req)
	})
}

func authMW(next http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		ctx := req.Context()
		sessionName := config.Get().HTTP.SessionName

		h := req.Header.Get("Authorization")
		if h == "" {
			http.Error(res, jsonError(fmt.Errorf("JWT missing")), http.StatusUnauthorized)
			return
		}

		token, err := jwt.ParseRequest(req,
			jwt.WithKeySet(config.JWParsingKeys(), jws.WithInferAlgorithmFromKey(true), jws.WithUseDefault(true)),
			jwt.WithValidate(true),
			jwt.WithAudience(sessionName),
			jwt.WithAcceptableSkew(20*time.Second),
		)
		if err != nil {
			http.Error(res, jsonError(err), http.StatusUnauthorized)
			return
		}

		subject, ok := token.Subject()
		if !ok {
			http.Error(res, jsonError(fmt.Errorf("JWT missing subject")), http.StatusUnauthorized)
			return
		}

		id, ok := token.JwtID()
		if !ok || auth.IsRevokedJWT(id) {
			log.Infow("JWT revoked", "sub", subject, "token ID", id)
			http.Error(res, jsonError(fmt.Errorf("JWT revoked")), http.StatusUnauthorized)
			return
		}

		gid := model.GoogleID(subject)
		if !gid.Valid(ctx) {
			if err := gid.FirstLogin(ctx); err != nil {
				log.Info(err)
				http.Error(res, jsonError(err), http.StatusUnauthorized)
				return
			}
		}

		// Update context with the GID
		ctx = context.WithValue(ctx, gidKey, gid)
		next.ServeHTTP(res, req.WithContext(ctx))
	})
}

func getAgentID(req *http.Request) (model.GoogleID, error) {
	gid, ok := req.Context().Value(gidKey).(model.GoogleID)
	if !ok {
		return "", fmt.Errorf("unable to identify agent")
	}
	return gid, nil
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
