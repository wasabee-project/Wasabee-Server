package config

import (
	"golang.org/x/oauth2/google"
	"path"
)

const (
	// wasabee constants
	defPicURL = "https://cdn2.wasabee.rocks/android-chrome-512x512.png"
	jku       = "https://cdn2.wasabee.rocks/.well-known/jwks.json"

	// V constants
	vAPIEndpoint    = "https://v.enl.one/api/v1"
	vStatusEndpoint = "https://status.enl.one/api/location"
	vTeamEndpoint   = "https://v.enl.one/api/v2/teams"

	// Rocks constants
	rocksCommunityEndpoint = "https://enlightened.rocks/comm/api/membership"
	rocksStatusEndpoint    = "https://enlightened.rocks/api/user/status"

	// Google RISC
	riscHook           = "/GoogleRISC"
	googleDiscoveryURL = "https://accounts.google.com/.well-known/risc-configuration"

	// the directory the certificates are stored in
	certs = "certs"

	// JWK signing and verification keys
	jwkpriv = "jwkpriv.json"
	jwkpub  = "jwkpub.json"

	// the firebase key file
	fbkey = "firebase.json"

	oauthUserInfoURL = "https://www.googleapis.com/oauth2/v2/userinfo"
	// oauthAuthURL = google.Endpoint.AuthURL
	// oauthTokenURL =  google.Endpoint.TokenURL

	// Telegram
	tgHookPath = "/tg"

	// REST API URLs
	meURL           = "/me"
	loginURL        = "/login"
	callbackURL     = "/callback"
	aptokenURL      = "/aptok"
	apipathURL      = "/api/v1"
	oneTimeTokenURL = "/oneTimeToken"
)

var defaults *WasabeeConf = &WasabeeConf{
	WordListFile: "eff_large_wordlist.txt",
	FrontendPath: "Wasabee-Frontend",
	FirebaseKey:  fbkey,
	WebUIURL:     "https://wasabee-project.github.io/Wasabee-WebUI/",

	Certs:    certs,
	CertFile: "wasabee.fullchain.pem",
	CertKey:  "wasabee.key",
	DB:       "wasabee:test@unix(/var/www/var/run/mysql/mysql.sock)/wasabee",
	V: wv{
		APIEndpoint:    vAPIEndpoint,
		StatusEndpoint: vStatusEndpoint,
		TeamEndpoint:   vTeamEndpoint,
	},
	RISC: wrisc{
		Webhook:   riscHook,
		Discovery: googleDiscoveryURL,
	},
	Rocks: wrocks{
		CommunityEndpoint: rocksCommunityEndpoint,
		StatusEndpoint:    rocksStatusEndpoint,
	},
	HTTP: whttp{
		Webroot:          "https://locallhost/",
		ListenHTTPS:      ":443",
		CookieSessionKey: "soontobeunused",
		Logfile:          "logs/wasabee.log",
		SessionName:      "wasabee",
		MeURL:            meURL,
		LoginURL:         loginURL,
		CallbackURL:      callbackURL,
		APIPathURL:       apipathURL,
		ApTokenURL:       aptokenURL,
		OneTimeTokenURL:  oneTimeTokenURL,
		OauthUserInfoURL: oauthUserInfoURL,
		OauthAuthURL:     google.Endpoint.AuthURL,
		OauthTokenURL:    google.Endpoint.TokenURL,
	},
	Telegram: wtg{
		HookPath: tgHookPath,
	},
	JKU:               jku,
	DefaultPictureURL: defPicURL,
	JWKpriv:           path.Join(certs, jwkpriv),
	JWKpub:            path.Join(certs, jwkpub),
}
