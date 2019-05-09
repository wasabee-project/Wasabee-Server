package wasabihttps

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	// "path"
	"time"

	"github.com/cloudkucooland/WASABI"
	"github.com/cloudkucooland/WASABI/GroupMe"
	"github.com/cloudkucooland/WASABI/RISC"
	"github.com/cloudkucooland/WASABI/Telegram"
	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
)

// generic things
func setupRoutes(r *mux.Router) {
	// r.Path("/favicon.ico").Handler(http.RedirectHandler("/static/favicon.ico", http.StatusFound))
}

// generic things, want logging
func setupNotauthed(r *mux.Router) {
	// XXX gorilla has CORSMethodMiddleware, should we use that instead? -- no, very limited functionality at this time
	r.Methods("OPTIONS").HandlerFunc(optionsRoute)

	// Google Oauth2 stuff
	r.HandleFunc("/login", googleRoute).Methods("GET")
	r.HandleFunc("/callback", callbackRoute).Methods("GET")
	r.HandleFunc("/GoogleRISC", risc.Webhook).Methods("POST")

	// For enl.rocks community -> WASABI team sync
	r.HandleFunc("/rocks", rocksCommunityRoute).Methods("POST")

	// raw files
	r.Path("/favicon.ico").Handler(http.RedirectHandler("/static/favicon.ico", http.StatusFound))
	r.Path("/robots.txt").Handler(http.RedirectHandler("/static/robots.txt", http.StatusFound))
	r.Path("/sitemap.xml").Handler(http.RedirectHandler("/static/sitemap.xml", http.StatusFound))
	r.Path("/.well-known/security.txt").Handler(http.RedirectHandler("/static/.well-known/security.txt", http.StatusFound))
	r.PathPrefix("/static/").Handler(http.FileServer(http.Dir(config.FrontendPath)))

	// Privacy Policy -- not static since we want to offer translations
	r.HandleFunc("/privacy", privacyRoute).Methods("GET")

	// index
	r.HandleFunc("/", frontRoute).Methods("GET")

	// 404 error page
	r.PathPrefix("/").HandlerFunc(notFoundRoute)
}

// implied /OwnTracks
func setupOwntracksRoute(r *mux.Router) {
	// OwnTracks URL -- basic auth is handled internally
	r.HandleFunc("", ownTracksRoute).Methods("POST")
}

// implied /simple
// do not log lest encryption key leaks
func setupSimpleRoutes(r *mux.Router) {
	// Simple -- the old-style, encrypted, unauthenticated/authorized documents
	r.HandleFunc("", uploadRoute).Methods("POST")
	r.HandleFunc("/{document}", getRoute).Methods("GET")
}

// implied /tg
func setupTelegramRoutes(r *mux.Router) {
	r.HandleFunc("/{hook}", wasabitelegram.TGWebHook).Methods("POST")
}

// implied /gm
func setupGMRoutes(r *mux.Router) {
	r.HandleFunc("/{hook}", wasabigm.GMWebHook).Methods("POST")
}

// implied /me
func setupMeRoutes(r *mux.Router) {
	// This block requires authentication
	// agent info (all HTML except /me which gives JSON for intel.ingrss.com
	r.HandleFunc("", meShowRoute).Methods("GET") // show my stats (agent name/teams)
}

// implied /api/v1
func setupAuthRoutes(r *mux.Router) {
	// This block requires authentication
	// Draw -- new-style, parsed, not-encrypted, authenticated, authorized, more-functional
	r.HandleFunc("/draw", pDrawUploadRoute).Methods("POST")
	r.HandleFunc("/draw/{document}", pDrawGetRoute).Methods("GET")
	r.HandleFunc("/draw/{document}", pDrawDeleteRoute).Methods("DELETE")
	r.HandleFunc("/draw/{document}/delete", pDrawDeleteRoute).Methods("GET")
	// r.HandleFunc("/draw/{document}/chown", pDrawChownRoute).Methods("GET").Queries("to", "{to}")
	r.HandleFunc("/draw/{document}", updateDrawRoute).Methods("PUT")

	r.HandleFunc("/me", meSetIngressNameRoute).Methods("GET").Queries("name", "{name}")                 // set my display name /me?name=deviousness
	r.HandleFunc("/me", meSetOwnTracksPWRoute).Methods("GET").Queries("otpw", "{otpw}")                 // set my OwnTracks Password (cleartext, yes, but SSL is required)
	r.HandleFunc("/me", meSetLocKeyRoute).Methods("GET").Queries("newlockey", "{y}")                    // request a new lockey
	r.HandleFunc("/me", meSetAgentLocationRoute).Methods("GET").Queries("lat", "{lat}", "lon", "{lon}") // manual location post
	r.HandleFunc("/me", meShowRoute).Methods("GET")                                                     // -- do not use, just here for safety
	r.HandleFunc("/me/delete", meDeleteRoute).Methods("GET")                                            // purge all info for a agent
	r.HandleFunc("/me/statuslocation", meStatusLocationRoute).Methods("GET").Queries("sl", "{sl}")      // toggle RAID/JEAH polling
	r.HandleFunc("/me/{team}", meToggleTeamRoute).Methods("GET").Queries("state", "{state}")            // /api/v1/me/wonky-team-1234?state={Off|On|Primary}
	r.HandleFunc("/me/{team}", meRemoveTeamRoute).Methods("DELETE")                                     // remove me from team
	r.HandleFunc("/me/{team}/delete", meRemoveTeamRoute).Methods("GET")                                 // remove me from team

	// other agents
	r.HandleFunc("/agent/{id}", agentProfileRoute).Methods("GET") // "profile" page, such as it is
	// r.HandleFunc("/agent/{id}/message", agentMessageRoute).Methods("POST")	// send a message to a agent

	// teams
	r.HandleFunc("/team/new", newTeamRoute).Methods("POST", "GET").Queries("name", "{name}")                                              // create a new team
	r.HandleFunc("/team/{team}", addAgentToTeamRoute).Methods("GET").Queries("key", "{key}")                                              // invite agent to team (owner)
	r.HandleFunc("/team/{team}", getTeamRoute).Methods("GET")                                                                             // return the location of every agent/target on team (team member/owner)
	r.HandleFunc("/team/{team}", deleteTeamRoute).Methods("DELETE")                                                                       // remove the team completely (owner)
	r.HandleFunc("/team/{team}/delete", deleteTeamRoute).Methods("GET")                                                                   // remove the team completely (owner)
	r.HandleFunc("/team/{team}/edit", editTeamRoute).Methods("GET")                                                                       // GUI to do basic edit (owner)
	r.HandleFunc("/team/{team}/rocks", rocksPullTeamRoute).Methods("GET")                                                                 // (re)import the team from rocks
	r.HandleFunc("/team/{team}/rockscfg", rocksCfgTeamRoute).Methods("GET").Queries("rockscomm", "{rockscomm}", "rockskey", "{rockskey}") // configure team link to enl.rocks community
	r.HandleFunc("/team/{team}/{key}", addAgentToTeamRoute).Methods("GET")                                                                // invite agent to team (owner)
	// r.HandleFunc("/team/{team}/{key}", setAgentTeamColorRoute).Methods("GET").Queries("color", "{color}") // set agent color on this team (owner)
	r.HandleFunc("/team/{team}/{key}/delete", delAgentFmTeamRoute).Methods("GET") // remove agent from team (owner)
	r.HandleFunc("/team/{team}/{key}", delAgentFmTeamRoute).Methods("DELETE")     // remove agent from team (owner)

	// waypoints
	r.HandleFunc("/waypoints/me", waypointsNearMeRoute).Methods("GET") // show OT waypoints & Operation markers near agent (html/json)

	// server control functions
	r.HandleFunc("/templates/refresh", templateUpdateRoute).Methods("GET") // trigger the server refresh of the template files
}

// probably useless now, but need to test before committing a removal
func optionsRoute(res http.ResponseWriter, req *http.Request) {
	// I think this is now taken care of in the middleware
	res.Header().Add("Allow", "GET, PUT, POST, OPTIONS, HEAD, DELETE")
	res.WriteHeader(200)
}

// display the front page
func frontRoute(res http.ResponseWriter, req *http.Request) {
	err := wasabiHTTPSTemplateExecute(res, req, "index", nil)
	if err != nil {
		wasabi.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
	}
}

// display the privacy policy
func privacyRoute(res http.ResponseWriter, req *http.Request) {
	err := wasabiHTTPSTemplateExecute(res, req, "privacy", nil)
	if err != nil {
		wasabi.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
	}
}

// this just reloads the templates on disk ; if someone makes a change we don't need to restart the server
func templateUpdateRoute(res http.ResponseWriter, req *http.Request) {
	err := wasabiHTTPSTemplateConfig()
	if err != nil {
		wasabi.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
	}
	res.Header().Add("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprintf(res, "Templates reloaded")
}

// called when a resource/endpoint is not found
func notFoundRoute(res http.ResponseWriter, req *http.Request) {
	i, ok := scanners[req.RemoteAddr]
	if ok {
		scanners[req.RemoteAddr] = i + 1
	} else {
		scanners[req.RemoteAddr] = 1
	}
	http.Error(res, "404: No light here.", http.StatusNotFound)
}

// final step of the oauth cycle
func callbackRoute(res http.ResponseWriter, req *http.Request) {
	type googleData struct {
		Gid   wasabi.GoogleID `json:"id"`
		Name  string          `json:"name"`
		Email string          `json:"email"`
	}

	content, tokenStr, err := getAgentInfo(req.FormValue("state"), req.FormValue("code"))
	if err != nil {
		wasabi.Log.Notice(err)
		return
	}

	var m googleData
	if err = json.Unmarshal(content, &m); err != nil {
		wasabi.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	// session cookie
	ses, err := config.store.Get(req, config.sessionName)
	if err != nil {
		// cookie is borked, maybe sessionName or key changed
		wasabi.Log.Notice("Cookie error: ", err)
		ses = sessions.NewSession(config.store, config.sessionName)
		ses.Options = &sessions.Options{
			Path:   "/",
			MaxAge: -1, // force delete
		}
		ses.Save(req, res)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	location := "/me?a=0"
	if ses.Values["loginReq"] != nil {
		rr := ses.Values["loginReq"].(string)
		// wasabi.Log.Debug("deep-link redirecting to", rr)
		if rr[:3] == "/me" || rr[:6] == "/login" { // leave /me check in place
			location = "/me?postlogin=1"
		} else {
			location = rr
		}
		delete(ses.Values, "loginReq")
	}

	authorized, err := m.Gid.InitAgent() // V & .rocks authorization takes place here
	if !authorized {
		http.Error(res, "Smurf go away!", http.StatusUnauthorized)
		return
	}
	if err != nil {
		wasabi.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	ses.Values["id"] = m.Gid.String()
	nonce, _ := calculateNonce(m.Gid)
	ses.Values["nonce"] = nonce
	ses.Values["google"] = tokenStr
	ses.Options = &sessions.Options{
		Path:   "/",
		MaxAge: 0,
	}
	ses.Save(req, res)
	http.Redirect(res, req, location, http.StatusFound)
}

// the secret value exchanged / verified each request
// not really a nonce, but it started life as one
func calculateNonce(gid wasabi.GoogleID) (string, string) {
	t := time.Now()
	now := t.Round(time.Hour).String()
	prev := t.Add(0 - time.Hour).Round(time.Hour).String()
	// something specific to the agent, something secret, something short-term
	current := sha256.Sum256([]byte(fmt.Sprintf("%s:%s:%s", gid, config.CookieSessionKey, now)))
	previous := sha256.Sum256([]byte(fmt.Sprintf("%s:%s:%s", gid, config.CookieSessionKey, prev)))
	return hex.EncodeToString(current[:]), hex.EncodeToString(previous[:])
}

// read the result from google at end of oauth session
// should we save the token in the session cookie for any reason?
func getAgentInfo(state string, code string) ([]byte, string, error) {
	if state != config.oauthStateString {
		return nil, "", fmt.Errorf("invalid oauth state")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	token, err := config.googleOauthConfig.Exchange(ctx, code)
	if err != nil {
		return nil, "", fmt.Errorf("code exchange failed: %s", err.Error())
	}
	tokenStr, _ := json.Marshal(token)
	response, err := http.Get("https://www.googleapis.com/oauth2/v2/userinfo?access_token=" + token.AccessToken)
	if err != nil {
		return nil, "", fmt.Errorf("failed getting agent info: %s", err.Error())
	}
	defer response.Body.Close()
	contents, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, "", fmt.Errorf("failed reading response body: %s", err.Error())
	}
	return contents, string(tokenStr), nil
}

// read the gid from the session cookie and return it
// this is the primary way to ensure a agent is authenticated
func getAgentID(req *http.Request) (wasabi.GoogleID, error) {
	ses, err := config.store.Get(req, config.sessionName)
	if err != nil {
		return "", err
	}

	// XXX I think this is impossible to trigger now
	if ses.Values["id"] == nil {
		err := errors.New("getAgentID called for unauthenticated agent")
		wasabi.Log.Critical(err)
		return "", err
	}

	var agentID = wasabi.GoogleID(ses.Values["id"].(string))
	return agentID, nil
}
