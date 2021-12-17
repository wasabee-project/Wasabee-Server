package config

import (
	"context"
	// "encoding/json"
	"sync"

	"github.com/gorilla/mux"
	"github.com/lestrrat-go/jwx/jwk"
	"github.com/wasabee-project/Wasabee-Server/log"
)

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
	JWSigningKeys jwk.Set
	JWParsingKeys jwk.Set
}

var once sync.Once
var c WasabeeConf

func Get() *WasabeeConf {
	return &c
}

// NewRouter creates the HTTPS router
func NewRouter() *mux.Router {
	// http://marcio.io/2015/07/singleton-pattern-in-go/
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

func TGSetBot(name string, id int) {
	c.Telegram.Name = name
	c.Telegram.ID = id
}

func TGRunning() bool {
	return c.Telegram.ID != 0
}

func SetupJWK(signers, parsers string) error {
	var err error
	log.Debugw("Loading JWK signing keys", "path", signers)

	c.JWSigningKeys, err = jwk.ReadFile(signers)
	if err != nil {
		log.Error(err)
		return err
	}
	log.Debugw("loaded signer keys", "count", c.JWSigningKeys.Len())

	for iter := c.JWSigningKeys.Iterate(context.TODO()); iter.Next(context.TODO()); {
		x := iter.Pair()
		log.Debugw("jwk signer", "key", x)
	}

	log.Debugw("Loading JWK parsing keys", "path", parsers)
	c.JWParsingKeys, err = jwk.ReadFile(parsers)
	if err != nil {
		log.Error(err)
		return err
	}
	log.Debugw("loaded parsing keys", "count", c.JWParsingKeys.Len())

	for iter := c.JWParsingKeys.Iterate(context.TODO()); iter.Next(context.TODO()); {
		x := iter.Pair()
		log.Debugw("jwk parser", "key", x)
	}

	return nil
}
