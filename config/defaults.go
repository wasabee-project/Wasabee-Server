package config

import (
	"golang.org/x/oauth2/google"
)

var defaults *WasabeeConf = &WasabeeConf{
	DB:                "wasabee:test@unix(/var/www/var/run/mysql/mysql.sock)/wasabee",
	WordListFile:      "eff_large_wordlist.txt",
	FrontendPath:      "Wasabee-Frontend",
	WebUIURL:          "https://webui.wasabee.rocks/",
	JKU:               "https://cdn2.wasabee.rocks/.well-known/jwks.json",
	DefaultPictureURL: "https://cdn2.wasabee.rocks/android-chrome-512x512.png",

	Certs:       "certs",
	CertFile:    "wasabee.fullchain.pem",
	CertKey:     "wasabee.key",
	FirebaseKey: "firebase.json",
	JWKpriv:     "jwkpriv.json",
	JWKpub:      "jwkpub.json",

	StoreRevisions: false,
	RevisionsDir:   "ops",

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
		Logfile:          "logs/wasabee-https.log",
		SessionName:      "wasabee",
		APIPathURL:       "/api/v1",
		ApTokenURL:       "/aptok",
		OneTimeTokenURL:  "/oneTimeToken",
		OauthUserInfoURL: "https://www.googleapis.com/oauth2/v2/userinfo",
		OauthAuthURL:     google.Endpoint.AuthURL,
		OauthTokenURL:    google.Endpoint.TokenURL,
		CORS:             []string{"https://intel.ingress.com", "https://wasabee-project.github.io", "https://cdn2.wasabee.rocks", "https://webui.wasabee.rocks"},
	},
	Telegram: wtg{
		HookPath: "/tg",
	},
}
