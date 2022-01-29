package config

import (
	// "context"
	"encoding/json"
	"net"
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
	GoogleCreds       string   // path to file.json
	GoogleProject     string   // project name for firebase/profile/risc
	DB                string   // db connect string
	WordListFile      string   // "eff-large-words.txt" filename
	FrontendPath      string   // path to directory continaing templates
	Certs             string   // path to director containing certs
	CertFile          string   // filename (relative to Certs)
	CertKey           string   // filename (relative to Certs)
	FirebaseKey       string   // filename (relative to Certs)
	JWKpriv           string   // filename (relative to Certs)
	JWKpub            string   // filename (relative to Certs)
	JKU               string   // URL to well-known JKU (for 3rd parties to verify our JWT)
	DefaultPictureURL string   // URL to a default image for agents
	WebUIURL          string   // URL of WebUI
	GRPCPort          uint16   // Port on which to send and receive gRPC messages
	Peers             []string // hostname/ip of servers to update
	GRPCDomain        string   // domain for grpc credentials

	// configuraiton for various subsystems
	V        wv
	Rocks    wrocks
	Telegram wtg
	HTTP     whttp
	RISC     wrisc

	// not configurable
	fbRunning bool
	peers     []net.IP

	// loaded by LoadFile()
	jwSigningKeys jwk.Set
	jwParsingKeys jwk.Set
}

// Configure v.enl.one
type wv struct {
	APIKey         string // get from V
	APIEndpoint    string // use default
	StatusEndpoint string // use default (unused)
	TeamEndpoint   string // use default
	running        bool
}

// Configuration for the Telegram Bot
type wtg struct {
	APIKey         string // defined by Telegram
	HookPath       string // use default
	CleanOnStartup bool   // almost certainly not needed
	name           string
	id             int
	running        bool
}

// Configure Google RISC
type wrisc struct {
	Cert      string // filename to cert.json
	Webhook   string // use default
	Discovery string // use default
}

// Configure enl.rocks
type wrocks struct {
	APIKey            string // get from Rocks (gfl)
	StatusEndpoint    string // use default
	CommunityEndpoint string // use default
	running           bool
}

// Configure the HTTPS REST interface
type whttp struct {
	Webroot          string // "https://xx.wasabee.rocks"
	ListenHTTPS      string // ":443" or "192.168.34.1:443"
	CookieSessionKey string // 32-char random (deprecated)
	Logfile          string // https logs
	SessionName      string // JWT aud name

	// defined by Google (deprecated)
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

	CORS []string // list of sites for which browsers will make API request

	oauthConfig *oauth2.Config
	router      *mux.Router
}

var once sync.Once
var c WasabeeConf

// LoadFile is the primary method for loading the Wasabee config file, setting the defaults
func LoadFile(filename string) (*WasabeeConf, error) {
	// #nosec
	raw, err := os.ReadFile(filename)
	if err != nil {
		log.Fatal(err)
	}

	in := defaults
	// overwrite the defaults with what is in the file
	if err := json.Unmarshal(raw, &in); err != nil {
		log.Fatal(err)
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
	}
	in.Certs = certdir
	log.Debugw("startup", "Certificate Directory", in.Certs)

	// make active
	c = *in

	// finish setup
	if err := setupJWK(certdir, c.JWKpriv, c.JWKpub); err != nil {
		log.Fatal(err)
	}

	if err := setupFederationPeers(); err != nil {
		log.Fatal(err)
	}

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

// Get returns the global configuration
// XXX it probably should not return a pointer so the callers can't overwrite the config
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
// currently there are no templates that use these values
func TGSetBot(name string, id int) {
	c.Telegram.name = name
	c.Telegram.id = id
	c.Telegram.running = true
}

// IsTelegramRunning reports if telegram is running; used for templates
func IsTelegramRunning() bool {
	return c.Telegram.running
}

// TelegramBotName returns the name of the running telegram bot
func TelegramBotName() string {
	return c.Telegram.name
}

// TelegramBotID returns the ID of the running telegram bot
func TelegramBotID() int {
	return c.Telegram.id
}

// setupJWK loads the keys used for the JWK signing and verification, set the file paths
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

// SetVRunning sets the current running state of V integration
func SetVRunning(v bool) {
	c.V.running = v
}

// IsVRunning reports the current running state of V integration
func IsVRunning() bool {
	return c.V.running
}

// SetRocksRunning sets the current running state of Rocks integration
func SetRocksRunning(r bool) {
	c.Rocks.running = r
}

// IsRocksRunning reports the current running state of Rocks integration
func IsRocksRunning() bool {
	return c.Rocks.running
}

// JWParsingKeys returns the public keys uses to verify the JWT
func JWParsingKeys() jwk.Set {
	return c.jwParsingKeys
}

// JWSigningKeys returns the private keys used to sign the JWT
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

// GetOauthConfig returns the Oauth config for Google
func GetOauthConfig() *oauth2.Config {
	return c.HTTP.oauthConfig
}

// IsFirebaseRunning repors the running state of the Firebase integration
func IsFirebaseRunning() bool {
	return c.fbRunning
}

// SetFirebaseRunning sets the running state of the Firebase integration
func SetFirebaseRunning(r bool) {
	c.fbRunning = r
}

// GetWebUI is used in templates
func GetWebUI() string {
	return c.WebUIURL
}

func setupFederationPeers() error {
	for _, peer := range c.Peers {
		log.Debugw("adding federation peer", "peer", peer)
		addrs, err := net.LookupIP(peer)
		if err != nil {
			log.Error(err)
			continue
		}

		// this might be too much -- multiple IPs per host?
		for _, a := range addrs {
			c.peers = append(c.peers, a)
		}
	}
	return nil
}

func GetFederationPeers() []net.IP {
	return c.peers
}
