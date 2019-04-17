package wasabihttps

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"path"

	"golang.org/x/oauth2"

	"crypto/sha256"
	"errors"
	"github.com/cloudkucooland/WASABI"
	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	"time"
)

func setupRoutes(r *mux.Router) {
	r.Methods("OPTIONS").HandlerFunc(optionsRoute)

	// Google Oauth2 stuff
	r.HandleFunc("/login", googleRoute).Methods("GET")
	r.HandleFunc("/callback", callbackRoute).Methods("GET")

	// Simple -- the old-style, encrypted, unauthenticated/authorized documents
	r.HandleFunc("/simple", uploadRoute).Methods("POST")
	r.HandleFunc("/simple/{document}", getRoute).Methods("GET")

	// For enl.rocks community -> WASABI team sync
	r.HandleFunc("/rocks", rocksCommunityRoute).Methods("POST")

	// OwnTracks URL -- basic auth is handled internally
	r.HandleFunc("/OwnTracks", ownTracksRoute).Methods("POST")

	r.HandleFunc("/static/{doc}", staticRoute).Methods("GET")
	r.HandleFunc("/static/{dir}/{doc}", staticRoute).Methods("GET")

	// Privacy Policy
	r.HandleFunc("/privacy", privacyRoute).Methods("GET")

	// index
	r.HandleFunc("/", frontRoute).Methods("GET")

	// 404 error page
	r.PathPrefix("/").HandlerFunc(notFoundRoute)
}

func setupAuthRoutes(r *mux.Router) {
	// This block requires authentication
	// Draw -- new-style, parsed, not-encrypted, authenticated, authorized, more-functional
	r.HandleFunc("/api/v1/draw", pDrawUploadRoute).Methods("POST")
	r.HandleFunc("/api/v1/draw/{document}", pDrawGetRoute).Methods("GET")
	r.HandleFunc("/api/v1/draw/{document}", pDrawDeleteRoute).Methods("DELETE")
	r.HandleFunc("/api/v1/draw/{document}/delete", pDrawDeleteRoute).Methods("GET")
	// r.HandleFunc("/api/v1/draw/{document}/chown", pDrawChownRoute).Methods("GET").Queries("to", "{to}")
	r.HandleFunc("/api/v1/draw/{document}", updateDrawRoute).Methods("PUT")

	// agent info (all HTML except /me which gives JSON for intel.ingrss.com
	r.HandleFunc("/me", meShowRoute).Methods("GET") // show my stats (agent name/teams)

	r.HandleFunc("/api/v1/me", meSetIngressNameRoute).Methods("GET").Queries("name", "{name}")                 // set my display name /me?name=deviousness
	r.HandleFunc("/api/v1/me", meSetOwnTracksPWRoute).Methods("GET").Queries("otpw", "{otpw}")                 // set my OwnTracks Password (cleartext, yes, but SSL is required)
	r.HandleFunc("/api/v1/me", meSetLocKeyRoute).Methods("GET").Queries("newlockey", "{y}")                    // request a new lockey
	r.HandleFunc("/api/v1/me", meSetAgentLocationRoute).Methods("GET").Queries("lat", "{lat}", "lon", "{lon}") // manual location post
	r.HandleFunc("/api/v1/me", meShowRoute).Methods("GET")                                                     // -- do not use, just here for safety
	r.HandleFunc("/api/v1/me/delete", meDeleteRoute).Methods("GET")                                            // purge all info for a agent
	r.HandleFunc("/api/v1/me/statuslocation", meStatusLocationRoute).Methods("GET").Queries("sl", "{sl}")      // toggle RAID/JEAH polling
	r.HandleFunc("/api/v1/me/{team}", meToggleTeamRoute).Methods("GET").Queries("state", "{state}")            // /api/v1/me/wonky-team-1234?state={Off|On|Primary}
	r.HandleFunc("/api/v1/me/{team}", meRemoveTeamRoute).Methods("DELETE")                                     // remove me from team
	r.HandleFunc("/api/v1/me/{team}/delete", meRemoveTeamRoute).Methods("GET")                                 // remove me from team

	// other agents
	r.HandleFunc("/api/v1/agent/{id}", agentProfileRoute).Methods("GET") // "profile" page, such as it is
	// r.HandleFunc("/api/v1/agent/{id}/message", agentMessageRoute).Methods("POST")	// send a message to a agent

	// teams
	r.HandleFunc("/api/v1/team/new", newTeamRoute).Methods("POST", "GET").Queries("name", "{name}")                                              // create a new team
	r.HandleFunc("/api/v1/team/{team}", addAgentToTeamRoute).Methods("GET").Queries("key", "{key}")                                              // invite agent to team (owner)
	r.HandleFunc("/api/v1/team/{team}", getTeamRoute).Methods("GET")                                                                             // return the location of every agent/target on team (team member/owner)
	r.HandleFunc("/api/v1/team/{team}", deleteTeamRoute).Methods("DELETE")                                                                       // remove the team completely (owner)
	r.HandleFunc("/api/v1/team/{team}/delete", deleteTeamRoute).Methods("GET")                                                                   // remove the team completely (owner)
	r.HandleFunc("/api/v1/team/{team}/edit", editTeamRoute).Methods("GET")                                                                       // GUI to do basic edit (owner)
	r.HandleFunc("/api/v1/team/{team}/rocks", rocksPullTeamRoute).Methods("GET")                                                                 // (re)import the team from rocks
	r.HandleFunc("/api/v1/team/{team}/rockscfg", rocksCfgTeamRoute).Methods("GET").Queries("rockscomm", "{rockscomm}", "rockskey", "{rockskey}") // configure team link to enl.rocks community
	r.HandleFunc("/api/v1/team/{team}/{key}", addAgentToTeamRoute).Methods("GET")                                                                // invite agent to team (owner)
	// r.HandleFunc("/api/v1/team/{team}/{key}", setAgentTeamColorRoute).Methods("GET").Queries("color", "{color}") // set agent color on this team (owner)
	r.HandleFunc("/api/v1/team/{team}/{key}/delete", delAgentFmTeamRoute).Methods("GET") // remove agent from team (owner)
	r.HandleFunc("/api/v1/team/{team}/{key}", delAgentFmTeamRoute).Methods("DELETE")     // remove agent from team (owner)

	// waypoints
	r.HandleFunc("/api/v1/waypoints/me", waypointsNearMeRoute).Methods("GET") // show OT waypoints & Operation markers near agent (html/json)

	// server control functions
	r.HandleFunc("/api/v1/templates/refresh", templateUpdateRoute).Methods("GET") // trigger the server refresh of the template files
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
	http.Error(res, "404: No light here.", http.StatusNotFound)
}

// final step of the oauth cycle
func callbackRoute(res http.ResponseWriter, req *http.Request) {
	type googleData struct {
		Gid   wasabi.GoogleID `json:"id"`
		Name  string          `json:"name"`
		Email string          `json:"email"`
	}

	content, err := getAgentInfo(req.FormValue("state"), req.FormValue("code"))
	if err != nil {
		wasabi.Log.Notice(err)
		return
	}

	var m googleData
	err = json.Unmarshal(content, &m)
	if err != nil {
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
func getAgentInfo(state string, code string) ([]byte, error) {
	if state != config.oauthStateString {
		return nil, fmt.Errorf("invalid oauth state")
	}
	token, err := config.googleOauthConfig.Exchange(oauth2.NoContext, code)
	if err != nil {
		return nil, fmt.Errorf("code exchange failed: %s", err.Error())
	}
	response, err := http.Get("https://www.googleapis.com/oauth2/v2/userinfo?access_token=" + token.AccessToken)
	if err != nil {
		return nil, fmt.Errorf("failed getting agent info: %s", err.Error())
	}
	defer response.Body.Close()
	contents, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("failed reading response body: %s", err.Error())
	}
	return contents, nil
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

	var agentID wasabi.GoogleID = wasabi.GoogleID(ses.Values["id"].(string))
	return agentID, nil
}

func staticRoute(res http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	doc, ok := vars["doc"]

	// XXX I've never been able to trigger this, can probably remove
	if !ok {
		wasabi.Log.Debug("Marty, Doc is not OK")
		notFoundRoute(res, req)
		return
	}

	var cleandoc string
	dir, ok := vars["dir"]
	if ok {
		wasabi.Log.Debugf("static file requested: %s/%s", dir, doc)
		// XXX clean it first : .. is removed by ServeFile, but we should be more paranoid than that
		cleandoc = path.Join(config.FrontendPath, "static", dir, doc)
	} else {
		wasabi.Log.Debugf("static file requested: %s", doc)
		// XXX clean it first : .. is removed by ServeFile, but we should be more paranoid than that
		cleandoc = path.Join(config.FrontendPath, "static", doc)
	}
	wasabi.Log.Debugf("serving: %s", cleandoc)
	http.ServeFile(res, req, cleandoc)
}
