package wasabihttps

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/cloudkucooland/WASABI"
	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	"io/ioutil"
	"net/http"
	"strings"
	"time"
)

func setupRouter() *mux.Router {
	// Main Router
	router := wasabi.NewRouter()

	// apply to all
	router.Use(headersMW)
	router.Use(scannerMW)
	// router.Use(debugMW)
	router.Methods("OPTIONS").HandlerFunc(optionsRoute)

	// 404 error page
	router.NotFoundHandler = http.HandlerFunc(notFoundRoute)

	// establish subrouters -- these each have different middleware requirements
	// if we want to disable logging on /simple, these need to be on a subrouter
	notauthed := wasabi.Subrouter("")
	// Google Oauth2 stuff (constants defined in server.go)
	notauthed.HandleFunc(login, googleRoute).Methods("GET")
	notauthed.HandleFunc(callback, callbackRoute).Methods("GET")
	notauthed.HandleFunc(aptoken, apTokenRoute).Methods("POST")
	// common files that live under /static
	notauthed.Path("/favicon.ico").Handler(http.RedirectHandler("/static/favicon.ico", http.StatusFound))
	notauthed.Path("/robots.txt").Handler(http.RedirectHandler("/static/robots.txt", http.StatusFound))
	notauthed.Path("/sitemap.xml").Handler(http.RedirectHandler("/static/sitemap.xml", http.StatusFound))
	notauthed.Path("/.well-known/security.txt").Handler(http.RedirectHandler("/static/.well-known/security.txt", http.StatusFound))
	// do not make these static -- they should be translated via the templates system
	notauthed.HandleFunc("/privacy", privacyRoute).Methods("GET")
	notauthed.HandleFunc("/", frontRoute).Methods("GET")
	notauthed.Use(config.unrolled.Handler)
	notauthed.NotFoundHandler = http.HandlerFunc(notFoundRoute)

	// /api/v1/... route
	api := wasabi.Subrouter(apipath)
	api.Methods("OPTIONS").HandlerFunc(optionsRoute)
	setupAuthRoutes(api)
	api.Use(authMW)
	api.Use(config.unrolled.Handler)
	api.NotFoundHandler = http.HandlerFunc(notFoundJSONRoute)

	// /me route
	me := wasabi.Subrouter(me)
	me.Methods("OPTIONS").HandlerFunc(optionsRoute)
	me.HandleFunc("", meShowRoute).Methods("GET")
	me.Use(authMW)
	me.Use(config.unrolled.Handler)
	me.NotFoundHandler = http.HandlerFunc(notFoundRoute)

	// /OwnTracks route
	ot := wasabi.Subrouter("/OwnTracks")
	ot.HandleFunc("", ownTracksBasicRoute).Methods("POST")
	// does own auth
	// no need to log
	ot.NotFoundHandler = http.HandlerFunc(notFoundJSONRoute)

	// /rocks route
	rocks := wasabi.Subrouter("/rocks")
	rocks.HandleFunc("", rocksCommunityRoute).Methods("POST")
	// internal API-key based auth
	rocks.Use(config.unrolled.Handler)
	rocks.NotFoundHandler = http.HandlerFunc(notFoundJSONRoute)

	// /simple route
	simple := wasabi.Subrouter("/simple")
	setupSimpleRoutes(simple)
	// no auth
	// no log
	simple.NotFoundHandler = http.HandlerFunc(notFoundJSONRoute)

	// /static files
	static := wasabi.Subrouter("/static")
	static.PathPrefix("/").Handler(http.FileServer(http.Dir(config.FrontendPath)))
	// no auth
	static.Use(config.unrolled.Handler)
	static.NotFoundHandler = http.HandlerFunc(notFoundRoute)

	return router
}

// implied /simple
// do not log lest encryption key leaks
func setupSimpleRoutes(r *mux.Router) {
	// Simple -- the old-style, encrypted, unauthenticated/authorized documents
	r.HandleFunc("", uploadRoute).Methods("POST")
	r.HandleFunc("/{document}", getRoute).Methods("GET")
}

// implied /api/v1
func setupAuthRoutes(r *mux.Router) {
	// This block requires authentication
	r.HandleFunc("/draw", pDrawUploadRoute).Methods("POST")
	r.HandleFunc("/draw/{document}", pDrawGetRoute).Methods("GET", "HEAD")
	r.HandleFunc("/draw/{document}", pDrawDeleteRoute).Methods("DELETE")
	r.HandleFunc("/draw/{document}", pDrawUpdateRoute).Methods("PUT")
	r.HandleFunc("/draw/{document}/delete", pDrawDeleteRoute).Methods("GET", "DELETE")
	r.HandleFunc("/draw/{document}/chown", pDrawChownRoute).Methods("GET").Queries("to", "{to}")
	r.HandleFunc("/draw/{document}/chgrp", pDrawChgrpRoute).Methods("GET").Queries("team", "{team}")
	r.HandleFunc("/draw/{document}/stock", pDrawStockRoute).Methods("GET")
	r.HandleFunc("/draw/{document}/order", pDrawOrderRoute).Methods("POST")
	r.HandleFunc("/draw/{document}/info", pDrawInfoRoute).Methods("POST")
	r.HandleFunc("/draw/{document}/stat", pDrawStatRoute).Methods("GET")
	r.HandleFunc("/draw/{document}/myroute", pDrawMyRouteRoute).Methods("GET")
	r.HandleFunc("/draw/{document}/link/{link}/assign", pDrawLinkAssignRoute).Methods("POST")
	r.HandleFunc("/draw/{document}/link/{link}/desc", pDrawLinkDescRoute).Methods("POST")
	r.HandleFunc("/draw/{document}/marker/{marker}/assign", pDrawMarkerAssignRoute).Methods("POST")
	r.HandleFunc("/draw/{document}/marker/{marker}/comment", pDrawMarkerCommentRoute).Methods("POST")
	r.HandleFunc("/draw/{document}/marker/{marker}/complete", pDrawMarkerCompleteRoute).Methods("POST")
	r.HandleFunc("/draw/{document}/marker/{marker}/incomplete", pDrawMarkerIncompleteRoute).Methods("POST")
	r.HandleFunc("/draw/{document}/portal/{portal}/comment", pDrawPortalCommentRoute).Methods("POST")
	r.HandleFunc("/draw/{document}/portal/{portal}/hardness", pDrawPortalHardnessRoute).Methods("POST")
	r.HandleFunc("/draw/{document}/portal/{portal}/keyonhand", pDrawPortalKeysRoute).Methods("POST")
	r.HandleFunc("/draw/{document}/portal/{portal}", pDrawPortalRoute).Methods("GET")

	r.HandleFunc("/me", meSetIngressNameRoute).Methods("GET").Queries("name", "{name}")
	// set my OwnTracks Password (cleartext, yes, but SSL is required)
	r.HandleFunc("/me", meSetOwnTracksPWRoute).Methods("GET").Queries("otpw", "{otpw}")
	// request a new lockey
	r.HandleFunc("/me", meSetLocKeyRoute).Methods("GET").Queries("newlockey", "{y}")
	// manual location post
	r.HandleFunc("/me", meSetAgentLocationRoute).Methods("GET").Queries("lat", "{lat}", "lon", "{lon}")
	// -- do not use, just here for safety
	r.HandleFunc("/me", meShowRoute).Methods("GET")
	// r.HandleFunc("/me/delete", meDeleteRoute).Methods("GET") // purge all info for a agent
	// toggle RAID/JEAH polling
	r.HandleFunc("/me/settings", meSettingsRoute).Methods("GET")
	r.HandleFunc("/me/operations", meOperationsRoute).Methods("GET")
	r.HandleFunc("/me/statuslocation", meStatusLocationRoute).Methods("GET").Queries("sl", "{sl}")
	r.HandleFunc("/me/{team}", meToggleTeamRoute).Methods("GET").Queries("state", "{state}")
	r.HandleFunc("/me/{team}", meRemoveTeamRoute).Methods("DELETE")
	r.HandleFunc("/me/{team}/delete", meRemoveTeamRoute).Methods("GET")
	r.HandleFunc("/me/logout", meLogoutRoute).Methods("GET")
	r.HandleFunc("/me/ot", ownTracksWasabiRoute).Methods("POST")

	// other agents
	// "profile" page, such as it is
	r.HandleFunc("/agent/{id}", agentProfileRoute).Methods("GET")
	r.HandleFunc("/agent/{id}/image", agentPictureRoute).Methods("GET")
	// send a message to a agent
	r.HandleFunc("/agent/{id}/message", agentMessageRoute).Methods("POST")

	// teams
	// create a new team
	r.HandleFunc("/team/new", newTeamRoute).Methods("POST", "GET").Queries("name", "{name}")
	r.HandleFunc("/team/{team}", addAgentToTeamRoute).Methods("GET").Queries("key", "{key}")
	r.HandleFunc("/team/{team}", getTeamRoute).Methods("GET")
	r.HandleFunc("/team/{team}", deleteTeamRoute).Methods("DELETE")
	r.HandleFunc("/team/{team}/delete", deleteTeamRoute).Methods("GET", "DELETE")
	r.HandleFunc("/team/{team}/chown", chownTeamRoute).Methods("GET").Queries("to", "{to}")
	// GUI to do basic edit (owner)
	r.HandleFunc("/team/{team}/edit", editTeamRoute).Methods("GET")
	// (re)import the team from rocks
	r.HandleFunc("/team/{team}/rocks", rocksPullTeamRoute).Methods("GET")
	// configure team link to enl.rocks community
	r.HandleFunc("/team/{team}/rockscfg", rocksCfgTeamRoute).Methods("GET").Queries("rockscomm", "{rockscomm}", "rockskey", "{rockskey}")
	// broadcast a message to the team
	r.HandleFunc("/team/{team}/announce", announceTeamRoute).Methods("POST")
	r.HandleFunc("/team/{team}/{key}", addAgentToTeamRoute).Methods("GET", "POST")
	// r.HandleFunc("/team/{team}/{key}", setAgentTeamColorRoute).Methods("GET").Queries("color", "{color}")
	r.HandleFunc("/team/{team}/{key}/delete", delAgentFmTeamRoute).Methods("GET")
	r.HandleFunc("/team/{team}/{key}", delAgentFmTeamRoute).Methods("DELETE")

	// waypoints
	r.HandleFunc("/waypoints/me", waypointsNearMeRoute).Methods("GET")

	// server control functions
	// trigger the server refresh of the template files
	r.HandleFunc("/templates/refresh", templateUpdateRoute).Methods("GET")

	r.NotFoundHandler = http.HandlerFunc(notFoundJSONRoute)
}

// probably useless now, but need to test before committing a removal
func optionsRoute(res http.ResponseWriter, req *http.Request) {
	// I think this is now taken care of in the middleware
	res.Header().Add("Allow", "GET, PUT, POST, OPTIONS, HEAD, DELETE")
	res.WriteHeader(200)
}

// display the front page
func frontRoute(res http.ResponseWriter, req *http.Request) {
	err := templateExecute(res, req, "index", nil)
	if err != nil {
		wasabi.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
	}
}

// display the privacy policy
func privacyRoute(res http.ResponseWriter, req *http.Request) {
	err := templateExecute(res, req, "privacy", nil)
	if err != nil {
		wasabi.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
	}
}

// this just reloads the templates on disk ; if someone makes a change we don't need to restart the server
func templateUpdateRoute(res http.ResponseWriter, req *http.Request) {
	var err error
	config.TemplateSet, err = wasabi.TemplateConfig(config.FrontendPath) // XXX KLUDGE FOR NOW -- this does not update the other protocols
	if err != nil {
		wasabi.Log.Notice(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
	}
	res.Header().Add("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprintf(res, "Templates reloaded")
}

// called when a resource/endpoint is not found
func notFoundRoute(res http.ResponseWriter, req *http.Request) {
	i, ok := config.scanners[req.RemoteAddr]
	if ok {
		config.scanners[req.RemoteAddr] = i + 1
	} else {
		config.scanners[req.RemoteAddr] = 1
	}
	http.Error(res, "404: No light here.", http.StatusNotFound)
}

// called when a resource/endpoint is not found
func notFoundJSONRoute(res http.ResponseWriter, req *http.Request) {
	i, ok := config.scanners[req.RemoteAddr]
	if ok {
		config.scanners[req.RemoteAddr] = i + 1
	} else {
		config.scanners[req.RemoteAddr] = 1
	}
	http.Error(res, `{status: "Not Found"}`, http.StatusNotFound)
}

// final step of the oauth cycle
func callbackRoute(res http.ResponseWriter, req *http.Request) {
	type googleData struct {
		Gid   wasabi.GoogleID `json:"id"`
		Name  string          `json:"name"`
		Email string          `json:"email"`
		Pic   string          `json:"picture"`
	}

	content, tokenStr, err := getAgentInfo(req.Context(), req.FormValue("state"), req.FormValue("code"))
	if err != nil {
		wasabi.Log.Notice(err)
		return
	}
	// wasabi.Log.Debug(string(content))

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
		_ = ses.Save(req, res)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	location := me + "?a=0"
	if ses.Values["loginReq"] != nil {
		rr := ses.Values["loginReq"].(string)
		// wasabi.Log.Debug("deep-link redirecting to", rr)
		if rr[:len(me)] == me || rr[:len(login)] == login { // leave /me check in place
			location = me + "?postlogin=1"
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

	err = m.Gid.UpdatePicture(m.Pic)
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
	_ = ses.Save(req, res)
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
func getAgentInfo(rctx context.Context, state string, code string) ([]byte, string, error) {
	if state != config.oauthStateString {
		return nil, "", fmt.Errorf("invalid oauth state")
	}

	ctx, cancel := context.WithTimeout(rctx, 5*time.Second)
	defer cancel()
	token, err := config.googleOauthConfig.Exchange(ctx, code)
	if err != nil {
		return nil, "", fmt.Errorf("code exchange failed: %s", err.Error())
	}
	cancel()

	contents, err := getGoogleUserInfo(token.AccessToken)
	if err != nil {
		return nil, "", fmt.Errorf("failed getting agent info: %s", err.Error())
	}

	// XXX is this necessary?
	tokenStr, _ := json.Marshal(token)
	return contents, string(tokenStr), nil
}

// used in getAgentInfo and apTokenRoute -- takes a user's Oauth2 token from Google and requests their info from Google
// timeout enforced -- XXX url and timout should be consts in server.go...
func getGoogleUserInfo(accessToken string) ([]byte, error) {
	url := fmt.Sprintf("https://www.googleapis.com/oauth2/v2/userinfo?access_token=%s", accessToken)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		wasabi.Log.Error(err)
		return nil, err
	}
	client := &http.Client{
		Timeout: 3 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		wasabi.Log.Error(err)
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		wasabi.Log.Error(err)
		return nil, err
	}
	return body, nil
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

// apTokenRoute receives a Google Oauth2 token from the Android/iOS app and sets the authentication cookie
func apTokenRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Set("Content-Type", jsonType)
	// fetched from google
	type googleData struct {
		Gid   wasabi.GoogleID `json:"id"`
		Name  string          `json:"name"`
		Email string          `json:"email"`
		Pic   string          `json:"picture"`
	}
	var m googleData

	// passed in from Android/iOS app
	type token struct {
		AccessToken string `json:"accessToken"`
	}
	var t token

	contentType := strings.Split(strings.Replace(strings.ToLower(req.Header.Get("Content-Type")), " ", "", -1), ";")[0]
	if contentType != jsonTypeShort {
		err := fmt.Errorf("invalid request (needs to be application/json)")
		http.Error(res, jsonError(err), http.StatusNotAcceptable)
		wasabi.Log.Notice(err)
		return
	}

	jBlob, err := ioutil.ReadAll(req.Body)
	if err != nil {
		wasabi.Log.Notice(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	if string(jBlob) == "" {
		err = fmt.Errorf("empty JSON")
		wasabi.Log.Notice(err)
		http.Error(res, jsonError(err), http.StatusNotAcceptable)
		return
	}
	jRaw := json.RawMessage(jBlob)
	if err = json.Unmarshal(jRaw, &t); err != nil {
		wasabi.Log.Notice(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	contents, err := getGoogleUserInfo(t.AccessToken)
	if err != nil {
		err = fmt.Errorf("failed getting agent info: %s", err.Error())
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	if err = json.Unmarshal(contents, &m); err != nil {
		wasabi.Log.Notice(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
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
		_ = ses.Save(req, res)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	authorized, err := m.Gid.InitAgent() // V & .rocks authorization takes place here
	if !authorized {
		err = fmt.Errorf("access denied")
		http.Error(res, jsonError(err), http.StatusUnauthorized)
		return
	}
	if err != nil {
		wasabi.Log.Notice(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	ses.Values["id"] = m.Gid.String()
	nonce, _ := calculateNonce(m.Gid)
	ses.Values["nonce"] = nonce
	ses.Values["google"] = t.AccessToken
	ses.Options = &sessions.Options{
		Path:   "/",
		MaxAge: 0,
	}
	err = ses.Save(req, res)
	if err != nil {
		wasabi.Log.Notice(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	fmt.Fprintf(res, `{ "status": "OK"}`)
}
