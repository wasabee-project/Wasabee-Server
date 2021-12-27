package config

import (
	// "context"
	"encoding/json"
	"os"
	"path"
	"path/filepath"
	"sync"

	"golang.org/x/oauth2"

	"github.com/gorilla/mux"
	"github.com/lestrrat-go/jwx/jwk"

	"github.com/wasabee-project/Wasabee-Server/log"
)

// WasabeeConf is the primary config structure
type WasabeeConf struct {
	GoogleCreds       string // path to file.json
	GoogleProject     string // project name for firebase/profile/risc
	WordListFile      string // "eff-large-words.txt" filename
	DB                string // db connect string
	FrontendPath      string // path to directory
	Certs             string // path to director containing certs
	CertFile          string // name of file (relative to Certs)
	CertKey           string // name of file (relative to Certs)
	WebUIURL          string // URL of WebUI
	JKU               string // URL to well-known JKU (for 3rd parties to verify our JWT)
	DefaultPictureURL string // URL to a default image for agents
	JWKpriv           string // filename (relative to Certs)
	JWKpub            string // filename (relative to Certs)
	FirebaseKey       string // filename (relative to Certs)
	fbRunning         bool

	V        wv
	Rocks    wrocks
	Telegram wtg
	HTTP     whttp
	RISC     wrisc

	// loaded by LoadFile()
	jwSigningKeys jwk.Set
	jwParsingKeys jwk.Set
}

type wv struct {
	APIKey         string // get from V
	APIEndpoint    string // use default
	StatusEndpoint string // use default (unused)
	TeamEndpoint   string // use default
	running        bool
}

type wtg struct {
	APIKey   string // defined by Telegram
	HookPath string // use default
	name     string
	id       int
	running  bool
}

type wrisc struct {
	Cert      string // filename to cert.json
	Webhook   string // use default
	Discovery string // use default
}

type wrocks struct {
	APIKey            string // get from Rocks (gfl)
	StatusEndpoint    string // use default
	CommunityEndpoint string // use default
	running           bool
}

type whttp struct {
	Webroot          string // "https://xx.wasabee.rocks"
	ListenHTTPS      string // ":443" or "192.168.34.1:443"
	CookieSessionKey string // 32-char random
	Logfile          string // https logs
	SessionName      string // JWT aud / cookie name

	// defined by Google
	OauthClientID    string // required
	OauthSecret      string // required
	OauthUserInfoURL string // use defauilt
	OauthAuthURL     string // use default
	OauthTokenURL    string // use default

	// URLS
	APIPathURL      string // /api/v1
	ApTokenURL      string // post Google Oauth token, get JWT/Cookie
	MeURL           string // deprecated
	LoginURL        string // deprecated
	CallbackURL     string // deprecated
	OneTimeTokenURL string // probably deprecated

	oauthConfig *oauth2.Config
	router      *mux.Router
}

var once sync.Once
var c WasabeeConf

func LoadFile(f string) (*WasabeeConf, error) {
	raw, err := os.ReadFile(f)
	if err != nil {
		log.Panic(err)
	}

	in := defaults
	// overwrite the defaults with what is in the file
	if err := json.Unmarshal(raw, &in); err != nil {
		log.Panic(err)
	}

	// these env vars always win
	if o := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS"); o != "" {
		in.GoogleCreds = o
	}
	if o := os.Getenv("GCP_PROJECT"); o != "" {
		in.GoogleProject = o
	}

	// should probably do this for Frontend-Path too...
	if in.Certs == "" {
		log.Warn("Certificate directory unset: defaulting to 'certs'")
		c.Certs = "certs"
	}
	certdir, err := filepath.Abs(in.Certs)
	if err != nil {
		log.Fatal("certificate path could not be resolved.")
		// panic(err)
	}
	in.Certs = certdir
	log.Debugw("startup", "Certificate Directory", in.Certs)

	// make active
	c = *in

	// finish setup
	setupJWK(certdir, c.JWKpriv, c.JWKpub)

	c.HTTP.oauthConfig = &oauth2.Config{
		ClientID:     c.HTTP.OauthClientID,
		ClientSecret: c.HTTP.OauthSecret,
		Scopes:       []string{"profile email"},
		Endpoint: oauth2.Endpoint{
			AuthURL:   c.HTTP.OauthAuthURL,
			TokenURL:  c.HTTP.OauthTokenURL,
			AuthStyle: oauth2.AuthStyleInParams,
		},
	}

	// log.Debugw("running config", "c", c)
	return &c, nil
}

// Get the global configuration -- probably should not return a pointer so the callers can't overwrite
func Get() *WasabeeConf {
	return &c
}

// NewRouter creates the HTTPS router
func NewRouter() *mux.Router {
	once.Do(func() {
		log.Debugw("startup", "router", "main HTTPS router")
		c.HTTP.router = mux.NewRouter()
	})
	return c.HTTP.router
}

// Subrouter creates a Gorilla subroute with a prefix
func Subrouter(prefix string) *mux.Router {
	log.Debugw("startup", "router", prefix)
	if c.HTTP.router == nil {
		NewRouter()
	}

	return c.HTTP.router.PathPrefix(prefix).Subrouter()
}

// TGSetBot sets the bot name and ID for use in templates
func TGSetBot(name string, id int) {
	c.Telegram.name = name
	c.Telegram.id = id
	c.Telegram.running = true
}

// TGRunning reports if telegram is running; used for templates
func IsTelegramRunning() bool {
	return c.Telegram.running
}

// SetupJWK loads the keys used for the JWK signing and verification, set the file paths
func setupJWK(certdir, signers, parsers string) error {
	var err error
	c.jwSigningKeys, err = jwk.ReadFile(path.Join(certdir, signers))
	if err != nil {
		log.Error(err)
		return err
	}
	log.Debugw("loaded JWT signing keys", "count", c.jwSigningKeys.Len(), "path", signers)

	/* for iter := c.jwSigningKeys.Iterate(context.TODO()); iter.Next(context.TODO()); {
		x := iter.Pair()
		log.Debugw("jwk signer", "key", x)
	} */

	c.jwParsingKeys, err = jwk.ReadFile(path.Join(certdir, parsers))
	if err != nil {
		log.Error(err)
		return err
	}
	log.Debugw("loaded JWT parsing keys", "count", c.jwParsingKeys.Len(), "path", parsers)

	return nil
}

func SetVRunning(v bool) {
	c.V.running = v
}

func IsVRunning() bool {
	return c.V.running
}

func SetRocksRunning(r bool) {
	c.Rocks.running = r
}

func IsRocksRunning() bool {
	return c.Rocks.running
}

func JWParsingKeys() jwk.Set {
	return c.jwParsingKeys
}

func JWSigningKeys() jwk.Set {
	return c.jwSigningKeys
}

// GetWebroot is used by telegram templates
func GetWebroot() string {
	return c.HTTP.Webroot
}

// GetWebAPIPath is used by telegram templates
func GetWebAPIPath() string {
	return c.HTTP.APIPathURL
}

func GetOauthConfig() *oauth2.Config {
	return c.HTTP.oauthConfig
}

func IsFirebaseRunning() bool {
	return c.fbRunning
}

func SetFirebaseRunning(r bool) {
	c.fbRunning = r
}
