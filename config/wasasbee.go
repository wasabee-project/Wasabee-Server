package config

import (
	// "context"
	// "encoding/json"
	"sync"

	"github.com/gorilla/mux"
	"github.com/lestrrat-go/jwx/jwk"

	"github.com/wasabee-project/Wasabee-Server/log"
)

const defPicUrl = "https://cdn2.wasabee.rocks/android-chrome-512x512.png"
const jku = "https://cdn2.wasabee.rocks/.well-known/jwks.json"

type WasabeeConf struct {
	V        bool
	Rocks    bool
	PubSub   bool
	Telegram struct {
		Name string
		ID   int
	}
	HTTP struct {
		Webroot  string
		APIpath  string
		WebUIurl string
		Router   *mux.Router
	}
	JWSigningKeys     jwk.Set
	JWParsingKeys     jwk.Set
	JKU               string
	DefaultPictureURL string
}

var once sync.Once
var c WasabeeConf

// Get the global configuration -- probably should not return a pointer so the callers can't overwrite
func Get() *WasabeeConf {
	return &c
}

// NewRouter creates the HTTPS router
func NewRouter() *mux.Router {
	once.Do(func() {
		log.Debugw("startup", "router", "main HTTPS router")
		c.HTTP.Router = mux.NewRouter()
	})
	return c.HTTP.Router
}

// Subrouter creates a Gorilla subroute with a prefix
func Subrouter(prefix string) *mux.Router {
	log.Debugw("startup", "router", prefix)
	if c.HTTP.Router == nil {
		NewRouter()
	}

	return c.HTTP.Router.PathPrefix(prefix).Subrouter()
}

// SetWebroot configures the root path for web requests
func SetWebroot(w string) {
	c.HTTP.Webroot = w
}

// GetWebroot is called from templates
func GetWebroot() string {
	return c.HTTP.Webroot
}

// SetWebUIurl is called at https startup
func SetWebUIurl(a string) {
	c.HTTP.WebUIurl = a
}

// SetWebAPIPath is called at https startup
func SetWebAPIPath(a string) {
	c.HTTP.APIpath = a
}

// GetWebAPIPath is called from templates
func GetWebAPIPath() string {
	return c.HTTP.APIpath
}

// TGSetBot sets the bot name and ID for use in templates
func TGSetBot(name string, id int) {
	c.Telegram.Name = name
	c.Telegram.ID = id
}

// TGRunning reports if telegram is running; used for templates
func TGRunning() bool {
	return c.Telegram.ID != 0
}

// SetupJWK loads the keys used for the JWK signing and verification, set the file paths
func SetupJWK(signers, parsers string) error {
	var err error
	c.JWSigningKeys, err = jwk.ReadFile(signers)
	if err != nil {
		log.Error(err)
		return err
	}
	log.Debugw("loaded JWT signing keys", "count", c.JWSigningKeys.Len(), "path", signers)

	/* for iter := c.JWSigningKeys.Iterate(context.TODO()); iter.Next(context.TODO()); {
		x := iter.Pair()
		log.Debugw("jwk signer", "key", x)
	} */

	c.JWParsingKeys, err = jwk.ReadFile(parsers)
	if err != nil {
		log.Error(err)
		return err
	}
	log.Debugw("loaded JWT parsing keys", "count", c.JWParsingKeys.Len(), "path", parsers)

	return nil
}

func PictureURL() string {
	if c.DefaultPictureURL == "" {
		return defPicUrl
	}
	return c.DefaultPictureURL
}

func JKU() string {
	if c.JKU != "" {
		return c.JKU
	}
	return jku
}
