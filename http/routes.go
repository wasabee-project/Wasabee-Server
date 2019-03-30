package PhDevHTTP

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"golang.org/x/oauth2"

	"crypto/sha256"
	"errors"
	"github.com/cloudkucooland/PhDevBin"
	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	"time"
)

func setupRoutes(r *mux.Router) {
	r.Methods("OPTIONS").HandlerFunc(optionsRoute)

	// Oauth2 stuff
	r.HandleFunc("/login", googleRoute).Methods("GET")
	r.HandleFunc("/callback", callbackRoute).Methods("GET")

	// Simple -- the old-style, encrypted, unauthenticated/authorized documents
	r.HandleFunc("/simple", uploadRoute).Methods("POST")
	r.HandleFunc("/simple/{document}", getRoute).Methods("GET")

	// PDraw w/o auth for testing
	r.HandleFunc("/pd", pDrawUploadRoute).Methods("POST")
	r.HandleFunc("/pd/{document}", pDrawGetRoute).Methods("GET")

	// For enl.rocks community -> PhDevBin team sync
	r.HandleFunc("/rocks", rocksCommunityRoute).Methods("POST")

	// OwnTracks URL
	r.HandleFunc("/OwnTracks", ownTracksRoute).Methods("POST")

	// index
	r.HandleFunc("/", frontRoute).Methods("GET")

	// 404 error page
	// r.PathPrefix("/").HandlerFunc(notFoundRoute)
}

func setupAuthRoutes(r *mux.Router) {
	// This block requires authentication
	// Draw -- new-style, parsed, not-encrypted, authenticated, authorized, more-functional
	r.HandleFunc("/api/v1/draw", pDrawUploadRoute).Methods("POST")
	r.HandleFunc("/api/v1/draw/{document}", pDrawGetRoute).Methods("GET")
	r.HandleFunc("/api/v1/draw/{document}", deleteDrawRoute).Methods("DELETE")
	r.HandleFunc("/api/v1/draw/{document}", updateDrawRoute).Methods("PUT")
	// r.HandleFunc("/api/v1/draw/{document}/addlink/", updateDrawRoute).Methods("PUT")

	// user info (all HTML except /me which gives JSON for intel.ingrss.com
	r.HandleFunc("/me", meSetIngressNameRoute).Methods("GET").Queries("name", "{name}")                // set my display name /me?name=deviousness
	r.HandleFunc("/me", meSetOwnTracksPWRoute).Methods("GET").Queries("otpw", "{otpw}")                // set my OwnTracks Password (cleartext, yes, but SSL is required)
	r.HandleFunc("/me", meSetLocKeyRoute).Methods("GET").Queries("newlockey", "{y}")                   // request a new lockey
	r.HandleFunc("/me", meSetUserLocationRoute).Methods("GET").Queries("lat", "{lat}", "lon", "{lon}") // manual location post
	r.HandleFunc("/me", meShowRoute).Methods("GET")                                                    // show my stats (agen name/teams)
	r.HandleFunc("/me/{team}", meToggleTeamRoute).Methods("GET").Queries("state", "{state}")           // /me/wonky-team-1234?state={Off|On|Primary}
	r.HandleFunc("/me/{team}", meRemoveTeamRoute).Methods("DELETE")                                    // remove me from team
	r.HandleFunc("/me/{team}/delete", meRemoveTeamRoute).Methods("GET")                                // remove me from team

	// teams
	r.HandleFunc("/api/v1/team/new", newTeamRoute).Methods("POST", "GET").Queries("name", "{name}")                                              // create a new team
	r.HandleFunc("/api/v1/team/{team}", addUserToTeamRoute).Methods("GET").Queries("key", "{key}")                                               // invite user to team
	r.HandleFunc("/api/v1/team/{team}", getTeamRoute).Methods("GET")                                                                             // return the location of every user/target on team (team member/owner)
	r.HandleFunc("/api/v1/team/{team}", deleteTeamRoute).Methods("DELETE")                                                                       // remove the team completely (owner)
	r.HandleFunc("/api/v1/team/{team}/delete", deleteTeamRoute).Methods("GET")                                                                   // remove the team completely (owner)
	r.HandleFunc("/api/v1/team/{team}/edit", editTeamRoute).Methods("GET")                                                                       // GUI to do basic edit (owner)
	r.HandleFunc("/api/v1/team/{team}/rocks", rocksPullTeamRoute).Methods("GET")                                                                 // (re)import the team from rocks
	r.HandleFunc("/api/v1/team/{team}/rockscfg", rocksCfgTeamRoute).Methods("GET").Queries("rockscomm", "{rockscomm}", "rockskey", "{rockskey}") // configure team link to enl.rocks community
	r.HandleFunc("/api/v1/team/{team}/{key}", addUserToTeamRoute).Methods("GET")                                                                 // invite user to team (owner)
	// r.HandleFunc("/api/v1/team/{team}/{key}", setUserTeamColorRoute).Methods("GET").Queries("color", "{color}") // set agent color on this team (owner)
	r.HandleFunc("/api/v1/team/{team}/{key}/delete", delUserFmTeamRoute).Methods("GET") // remove user from team (owner)
	r.HandleFunc("/api/v1/team/{team}/{key}", delUserFmTeamRoute).Methods("DELETE")     // remove user from team (owner)

	// targets
	r.HandleFunc("/api/v1/targets/me", targetsNearMeRoute).Methods("GET")                 // show targets near user (html/json)
	r.HandleFunc("/api/v1/targets/{team}", targetsRoute).Methods("GET")                   // show targets for team
	r.HandleFunc("/api/v1/targets/{team}", targetsUploadRoute).Methods("POST")            // upload JSON target list
	r.HandleFunc("/api/v1/targets/{team}/{id}/delete", targetsDeleteRoute).Methods("GET") // upload JSON target list

	// doesn't need to be authenticated, but why not?
	r.HandleFunc("/status", statusRoute).Methods("GET")

	// server control functions
	r.HandleFunc("/api/v1/templates/refresh", templateUpdateRoute).Methods("GET") // trigger the server refresh of the template files
}

func optionsRoute(res http.ResponseWriter, req *http.Request) {
	// I think this is now taken care of in the middleware
	res.Header().Add("Allow", "GET, PUT, POST, OPTIONS, HEAD, DELETE")
	res.WriteHeader(200)
	return
}

func frontRoute(res http.ResponseWriter, req *http.Request) {
	err := phDevBinHTTPSTemplateExecute(res, req, "index", nil)
	if err != nil {
		PhDevBin.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
	}
	return
}

func statusRoute(res http.ResponseWriter, req *http.Request) {
	// maybe show some interesting numbers, active agents, etc...
	err := phDevBinHTTPSTemplateExecute(res, req, "status", nil)
	if err != nil {
		PhDevBin.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
	}
	return
}

func templateUpdateRoute(res http.ResponseWriter, req *http.Request) {
	// maybe show some interesting numbers, active agents, etc...
	err := phDevBinHTTPSTemplateConfig()
	if err != nil {
		PhDevBin.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
	}
	res.Header().Add("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprintf(res, "Templates reloaded")
	return
}

func notFoundRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Add("Cache-Control", "no-cache")
	// why not just: http.Error(res, "404: Maybe the document is expired or has been removed.", http.StatusFileNotFound)
	res.Header().Add("Content-Type", "text/plain; charset=utf-8")
	res.WriteHeader(404)
	fmt.Fprint(res, "404: Maybe the document is expired or has been removed.\n")
}

func callbackRoute(res http.ResponseWriter, req *http.Request) {
	type googleData struct {
		Gid   PhDevBin.GoogleID `json:"id"`
		Name  string            `json:"name"`
		Email string            `json:"email"`
	}

	content, err := getUserInfo(req.FormValue("state"), req.FormValue("code"))
	if err != nil {
		PhDevBin.Log.Notice(err)
		return
	}

	var m googleData
	err = json.Unmarshal(content, &m)
	if err != nil {
		PhDevBin.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	// session cookie
	ses, err := config.store.Get(req, config.sessionName)
	if err != nil {
		// cookie is borked, maybe sessionName or key changed
		PhDevBin.Log.Notice("Cookie error: ", err)
		ses = sessions.NewSession(config.store, config.sessionName)
		delete(ses.Values, "id")
		delete(ses.Values, "loginReq")
		delete(ses.Values, "nonce")
		ses.Options = &sessions.Options{
			Path:   "/",
			MaxAge: 3600,
		}
		ses.Save(req, res)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	authorized, err := m.Gid.InitUser() // V authorization takes place here now
	if err != nil {
		PhDevBin.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	if authorized == false {
		http.Error(res, err.Error(), http.StatusUnauthorized)
		return
	}

	location := "/me?a=1"
	if ses.Values["loginReq"] != nil {
		rr := ses.Values["loginReq"].(string)
		if rr == "/login" || rr == "/me" || rr == "" {
			location = "/me?a=2"
		} else {
			location = rr
		}
		delete(ses.Values, "loginReq")
		// ses.Values["loginReq"] = nil // to not save for future sessions
	}

	ses.Values["id"] = m.Gid.String()
	nonce, _, _ := calculateNonce(m.Gid)
	ses.Values["nonce"] = nonce
	ses.Options = &sessions.Options{
		Path:   "/",
		MaxAge: 3600,
	}
	ses.Save(req, res)
	http.Redirect(res, req, location, http.StatusPermanentRedirect)
}

func calculateNonce(gid PhDevBin.GoogleID) (string, string, error) {
	t := time.Now()
	now := t.Round(time.Hour).String()
	prev := t.Add(0 - time.Hour).Round(time.Hour).String()
	current := sha256.Sum256([]byte(fmt.Sprintf("%s:%s:%s", gid, config.CookieSessionKey, now)))
	previous := sha256.Sum256([]byte(fmt.Sprintf("%s:%s:%s", gid, config.CookieSessionKey, prev)))
	return hex.EncodeToString(current[:]), hex.EncodeToString(previous[:]), nil
}

func getUserInfo(state string, code string) ([]byte, error) {
	if state != config.oauthStateString {
		return nil, fmt.Errorf("invalid oauth state")
	}
	token, err := config.googleOauthConfig.Exchange(oauth2.NoContext, code)
	if err != nil {
		return nil, fmt.Errorf("code exchange failed: %s", err.Error())
	}
	response, err := http.Get("https://www.googleapis.com/oauth2/v2/userinfo?access_token=" + token.AccessToken)
	if err != nil {
		return nil, fmt.Errorf("failed getting user info: %s", err.Error())
	}
	defer response.Body.Close()
	contents, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("failed reading response body: %s", err.Error())
	}
	return contents, nil
}

func getUserID(req *http.Request) (PhDevBin.GoogleID, error) {
	ses, err := config.store.Get(req, config.sessionName)
	if err != nil {
		return "", err
	}

	if ses.Values["id"] == nil {
		err := errors.New("getUserID called for unauthenticated user")
		PhDevBin.Log.Notice(err)
		return "", err
	}

	var userID PhDevBin.GoogleID = PhDevBin.GoogleID(ses.Values["id"].(string))
	return userID, nil
}
