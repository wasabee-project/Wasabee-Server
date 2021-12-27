package config

import (
	"golang.org/x/oauth2/google"
)

var defaults *WasabeeConf = &WasabeeConf{
	DB:                "wasabee:test@unix(/var/www/var/run/mysql/mysql.sock)/wasabee",
	WordListFile:      "eff_large_wordlist.txt",
	FrontendPath:      "Wasabee-Frontend",
	WebUIURL:          "https://wasabee-project.github.io/Wasabee-WebUI/",
	JKU:               "https://cdn2.wasabee.rocks/.well-known/jwks.json",
	DefaultPictureURL: "https://cdn2.wasabee.rocks/android-chrome-512x512.png",

	Certs:       "certs",
	CertFile:    "wasabee.fullchain.pem",
	CertKey:     "wasabee.key",
	FirebaseKey: "firebase.json",
	JWKpriv:     "jwkpriv.json",
	JWKpub:      "jwkpub.json",

	V: wv{
		APIEndpoint:    "https://v.enl.one/api/v1",
		StatusEndpoint: "https://status.enl.one/api/location",
		TeamEndpoint:   "https://v.enl.one/api/v2/teams",
	},
	RISC: wrisc{
		Cert:      "risc.json",
		Webhook:   "/GoogleRISC",
		Discovery: "https://accounts.google.com/.well-known/risc-configuration",
	},
	Rocks: wrocks{
		CommunityEndpoint: "https://enlightened.rocks/comm/api/membership",
		StatusEndpoint:    "https://enlightened.rocks/api/user/status",
	},
	HTTP: whttp{
		Webroot:          "https://locallhost/",
		ListenHTTPS:      ":443",
		CookieSessionKey: "soontobeunused",
		Logfile:          "logs/wasabee.log",
		SessionName:      "wasabee",
		MeURL:            "/me",
		LoginURL:         "/login",
		CallbackURL:      "/callback",
		APIPathURL:       "/api/v1",
		ApTokenURL:       "/aptok",
		OneTimeTokenURL:  "/oneTimeToken",
		OauthUserInfoURL: "https://www.googleapis.com/oauth2/v2/userinfo",
		OauthAuthURL:     google.Endpoint.AuthURL,
		OauthTokenURL:    google.Endpoint.TokenURL,
	},
	Telegram: wtg{
		HookPath: "/tg",
	},
}
