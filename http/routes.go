package wasabeehttps

import (
	"fmt"
	"net/http"
	// "net/http/httputil"
	"sync"

	"github.com/gorilla/mux"
	"github.com/wasabee-project/Wasabee-Server/config"
	"github.com/wasabee-project/Wasabee-Server/log"
)

var scannerMux sync.Mutex

func setupRouter() *mux.Router {
	// Main Router
	router := config.NewRouter()

	// apply to all
	router.Use(headersMW)
	router.Use(scannerMW)
	// router.Use(logRequestMW)
	// router.Use(debugMW)
	router.Use(c.unrolled.Handler)
	router.NotFoundHandler = http.HandlerFunc(notFoundRoute)
	router.MethodNotAllowedHandler = http.HandlerFunc(notFoundRoute)
	router.Methods("OPTIONS").HandlerFunc(optionsRoute)

	// Google Oauth2 stuff (constants defined in server.go)
	router.HandleFunc(login, googleRoute).Methods("GET")
	router.HandleFunc(callback, callbackRoute).Methods("GET")
	router.HandleFunc(aptoken, apTokenRoute).Methods("POST")
	router.HandleFunc(oneTimeToken, oneTimeTokenRoute).Methods("POST")

	// common files that live under /static
	router.Path("/favicon.ico").Handler(http.RedirectHandler("/static/favicon.ico", http.StatusFound))
	router.Path("/robots.txt").Handler(http.RedirectHandler("/static/robots.txt", http.StatusFound))
	router.Path("/sitemap.xml").Handler(http.RedirectHandler("/static/sitemap.xml", http.StatusFound))
	router.Path("/.well-known/security.txt").Handler(http.RedirectHandler("/static/.well-known/security.txt", http.StatusFound))
	// this cannot be a redirect -- sent it raw
	router.HandleFunc("/firebase-messaging-sw.js", fbmswRoute).Methods("GET")
	// do not make these static -- they should be translated via the templates system
	router.HandleFunc("/privacy", privacyRoute).Methods("GET")
	router.HandleFunc("/", frontRoute).Methods("GET")
	// v.enl.one posting when a team changes -- triggers a pull of all teams linked to the V team #
	router.HandleFunc("/v/{teamID}", vTeamRoute).Methods("POST")

	// /api/v1/... route
	api := config.Subrouter(apipath)
	api.Methods("OPTIONS").HandlerFunc(optionsRoute)
	setupAuthRoutes(api)
	api.Use(authMW)
	api.NotFoundHandler = http.HandlerFunc(notFoundJSONRoute)
	api.MethodNotAllowedHandler = http.HandlerFunc(notFoundJSONRoute)
	api.PathPrefix("/api").HandlerFunc(notFoundJSONRoute)

	// /me route
	meRouter := config.Subrouter(me)
	meRouter.Methods("OPTIONS").HandlerFunc(optionsRoute)
	meRouter.HandleFunc("", meRoute).Methods("GET", "POST", "HEAD")
	meRouter.Use(authMW)
	meRouter.NotFoundHandler = http.HandlerFunc(notFoundJSONRoute)
	meRouter.MethodNotAllowedHandler = http.HandlerFunc(notFoundJSONRoute)
	meRouter.PathPrefix("/me").HandlerFunc(notFoundJSONRoute)

	// /rocks route -- why a subrouter? for JSON error messages -- no longer necessary
	rocks := config.Subrouter("/rocks")
	rocks.HandleFunc("", rocksCommunityRoute).Methods("POST")
	rocks.NotFoundHandler = http.HandlerFunc(notFoundJSONRoute)
	rocks.MethodNotAllowedHandler = http.HandlerFunc(notFoundJSONRoute)
	rocks.PathPrefix("/rocks").HandlerFunc(notFoundJSONRoute)

	// /static files
	static := config.Subrouter("/static")
	static.PathPrefix("/").Handler(http.FileServer(http.Dir(c.FrontendPath)))
	// static.NotFoundHandler = http.HandlerFunc(notFoundRoute)

	// catch all others -- jacks up later subrouters (e.g. Telegram and GoogleRISC)
	// router.PathPrefix("/").HandlerFunc(notFoundRoute)
	router.NotFoundHandler = http.HandlerFunc(notFoundRoute)
	return router
}

// implied /api/v1
func setupAuthRoutes(r *mux.Router) {
	// This block requires authentication
	r.HandleFunc("/draw", drawUploadRoute).Methods("POST")
	r.HandleFunc("/draw/{document}", drawGetRoute).Methods("GET", "HEAD")
	r.HandleFunc("/draw/{document}", drawDeleteRoute).Methods("DELETE")
	r.HandleFunc("/draw/{document}", drawUpdateRoute).Methods("PUT")
	r.HandleFunc("/draw/{document}/delete", drawDeleteRoute).Methods("GET", "DELETE")
	r.HandleFunc("/draw/{document}/chown", drawChownRoute).Methods("GET").Queries("to", "{to}")
	// r.HandleFunc("/draw/{document}/stock", drawStockRoute).Methods("GET")
	r.HandleFunc("/draw/{document}/order", drawOrderRoute).Methods("POST")
	r.HandleFunc("/draw/{document}/info", drawInfoRoute).Methods("POST")
	r.HandleFunc("/draw/{document}/perms", drawPermsAddRoute).Methods("POST")
	r.HandleFunc("/draw/{document}/perms", drawPermsDeleteRoute).Methods("DELETE")
	r.HandleFunc("/draw/{document}/delperm", drawPermsDeleteRoute).Methods("GET") // .Queries("team", "{team}", "role", "{role}")
	r.HandleFunc("/draw/{document}/link/{link}", drawLinkFetch).Methods("GET")
	r.HandleFunc("/draw/{document}/link/{link}/assign", drawLinkAssignRoute).Methods("POST")
	r.HandleFunc("/draw/{document}/link/{link}/color", drawLinkColorRoute).Methods("POST")
	r.HandleFunc("/draw/{document}/link/{link}/desc", drawLinkDescRoute).Methods("POST")
	r.HandleFunc("/draw/{document}/link/{link}/complete", drawLinkCompleteRoute).Methods("GET")
	r.HandleFunc("/draw/{document}/link/{link}/incomplete", drawLinkIncompleteRoute).Methods("GET")
	r.HandleFunc("/draw/{document}/link/{link}/reject", drawLinkRejectRoute).Methods("POST")
	r.HandleFunc("/draw/{document}/link/{link}/claim", drawLinkClaimRoute).Methods("POST")
	r.HandleFunc("/draw/{document}/link/{link}/swap", drawLinkSwapRoute).Methods("GET")
	r.HandleFunc("/draw/{document}/link/{link}/zone", drawLinkZoneRoute).Methods("POST")
	r.HandleFunc("/draw/{document}/link/{link}/delta", drawLinkDeltaRoute).Methods("POST")
	r.HandleFunc("/draw/{document}/link/{link}/depend/{task}", drawLinkDependAddRoute).Methods("PUT")
	r.HandleFunc("/draw/{document}/link/{link}/depend/{task}", drawLinkDependDelRoute).Methods("DELETE")
	r.HandleFunc("/draw/{document}/marker/{marker}", drawMarkerFetch).Methods("GET")
	r.HandleFunc("/draw/{document}/marker/{marker}/assign", drawMarkerAssignRoute).Methods("POST")
	r.HandleFunc("/draw/{document}/marker/{marker}/comment", drawMarkerCommentRoute).Methods("POST")
	// agent acknowledge the assignment
	r.HandleFunc("/draw/{document}/marker/{marker}/acknowledge", drawMarkerAcknowledgeRoute).Methods("GET")
	// agent mark as complete
	r.HandleFunc("/draw/{document}/marker/{marker}/complete", drawMarkerCompleteRoute).Methods("GET")
	// agent undo complete mark
	r.HandleFunc("/draw/{document}/marker/{marker}/incomplete", drawMarkerIncompleteRoute).Methods("GET")
	// operator verify completing
	r.HandleFunc("/draw/{document}/marker/{marker}/reject", drawMarkerRejectRoute).Methods("GET")
	r.HandleFunc("/draw/{document}/marker/{marker}/claim", drawMarkerClaimRoute).Methods("POST")
	r.HandleFunc("/draw/{document}/marker/{marker}/zone", drawMarkerZoneRoute).Methods("POST")
	r.HandleFunc("/draw/{document}/marker/{marker}/delta", drawMarkerDeltaRoute).Methods("POST")
	r.HandleFunc("/draw/{document}/marker/{marker}/depend/{task}", drawMarkerDependAddRoute).Methods("PUT")
	r.HandleFunc("/draw/{document}/marker/{marker}/depend/{task}", drawMarkerDependDelRoute).Methods("DELETE")
	r.HandleFunc("/draw/{document}/portal/{portal}/comment", drawPortalCommentRoute).Methods("POST")
	r.HandleFunc("/draw/{document}/portal/{portal}/hardness", drawPortalHardnessRoute).Methods("POST")
	r.HandleFunc("/draw/{document}/portal/{portal}/keyonhand", drawPortalKeysRoute).Methods("POST")

	r.HandleFunc("/me", meSetAgentLocationRoute).Methods("GET").Queries("lat", "{lat}", "lon", "{lon}")
	r.HandleFunc("/me", meRoute).Methods("GET", "POST", "HEAD")
	r.HandleFunc("/me/delete", meDeleteRoute).Methods("GET") // purge all info for a agent
	r.HandleFunc("/me/{team}", meToggleTeamRoute).Methods("GET").Queries("state", "{state}")
	r.HandleFunc("/me/{team}", meRemoveTeamRoute).Methods("DELETE")
	r.HandleFunc("/me/{team}/delete", meRemoveTeamRoute).Methods("GET")
	r.HandleFunc("/me/{team}/wdshare", meToggleTeamWDShareRoute).Methods("GET").Queries("state", "{state}")
	r.HandleFunc("/me/{team}/wdload", meToggleTeamWDLoadRoute).Methods("GET").Queries("state", "{state}")
	r.HandleFunc("/me/logout", meLogoutRoute).Methods("GET")
	r.HandleFunc("/me/firebase", meFirebaseRoute).Methods("POST")      // post a token generated by google
	r.HandleFunc("/me/intelid", meIntelIDRoute).Methods("PUT", "POST") // get ID from intel (not trusted)
	r.HandleFunc("/me/VAPIkey", meVAPIkeyRoute).Methods("POST")        // send an V API key for team sync
	r.HandleFunc("/me/jwtrefresh", meJwtRefreshRoute).Methods("GET")

	// other agents
	// "profile" page, such as it is
	r.HandleFunc("/agent/{id}", agentProfileRoute).Methods("GET")
	r.HandleFunc("/agent/{id}/image", agentPictureRoute).Methods("GET")
	// send a message to a agent
	r.HandleFunc("/agent/{id}/message", agentMessageRoute).Methods("POST")
	r.HandleFunc("/agent/{id}/fbMessage", agentFBMessageRoute).Methods("POST")
	r.HandleFunc("/agent/{id}/target", agentTargetRoute).Methods("POST")

	// teams
	// create a new team
	r.HandleFunc("/team/new", newTeamRoute).Methods("POST", "GET").Queries("name", "{name}")
	r.HandleFunc("/team/vbulkimport", vBulkImportRoute).Methods("GET").Queries("mode", "{mode}")
	r.HandleFunc("/team/{team}", addAgentToTeamRoute).Methods("GET").Queries("key", "{key}")
	r.HandleFunc("/team/{team}", getTeamRoute).Methods("GET")
	r.HandleFunc("/team/{team}", deleteTeamRoute).Methods("DELETE")
	r.HandleFunc("/team/{team}/delete", deleteTeamRoute).Methods("GET", "DELETE")
	r.HandleFunc("/team/{team}/chown", chownTeamRoute).Methods("GET").Queries("to", "{to}")
	r.HandleFunc("/team/{team}/join/{key}", joinLinkRoute).Methods("GET")
	r.HandleFunc("/team/{team}/genJoinKey", genJoinKeyRoute).Methods("GET")
	r.HandleFunc("/team/{team}/delJoinKey", delJoinKeyRoute).Methods("GET")
	// (re)import the team from rocks
	r.HandleFunc("/team/{team}/rocks", rocksPullTeamRoute).Methods("GET")
	// configure team link to enl.rocks community
	r.HandleFunc("/team/{team}/rockscfg", rocksCfgTeamRoute).Methods("GET").Queries("rockscomm", "{rockscomm}", "rockskey", "{rockskey}")
	// V routes
	r.HandleFunc("/team/{team}/v", vPullTeamRoute).Methods("GET")
	r.HandleFunc("/team/{team}/v", vConfigureTeamRoute).Methods("POST")
	r.HandleFunc("/team/{team}/announce", announceTeamRoute).Methods("POST") // broadcast a message to the team
	r.HandleFunc("/team/{team}/rename", renameTeamRoute).Methods("PUT")
	r.HandleFunc("/team/{team}/{key}", addAgentToTeamRoute).Methods("GET", "POST")
	r.HandleFunc("/team/{team}/{gid}/squad", setAgentTeamCommentRoute).Methods("POST") // remove this
	r.HandleFunc("/team/{team}/{gid}/comment", setAgentTeamCommentRoute).Methods("POST")
	r.HandleFunc("/team/{team}/{key}/delete", delAgentFmTeamRoute).Methods("GET")
	r.HandleFunc("/team/{team}/{key}", delAgentFmTeamRoute).Methods("DELETE")

	// allow fetching specific teams in bulk - JSON list of teamIDs
	r.HandleFunc("/teams", bulkTeamFetchRoute).Methods("POST")

	r.HandleFunc("/d", getDefensiveKeys).Methods("GET")
	r.HandleFunc("/d", setDefensiveKey).Methods("POST")
	r.HandleFunc("/d/bulk", setDefensiveKeyBulk).Methods("POST")
	r.HandleFunc("/loc", getAgentsLocation).Methods("GET")

	r.NotFoundHandler = http.HandlerFunc(notFoundJSONRoute)
}

func optionsRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Add("Allow", "GET, PUT, POST, OPTIONS, HEAD, DELETE")
	res.WriteHeader(200)
}

// display the front page
func frontRoute(res http.ResponseWriter, req *http.Request) {
	err := templateExecute(res, req, "index", nil)
	if err != nil {
		log.Error(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
	}
}

// display the privacy policy
func privacyRoute(res http.ResponseWriter, req *http.Request) {
	err := templateExecute(res, req, "privacy", nil)
	if err != nil {
		log.Error(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
	}
}

// called when a resource/endpoint is not found
func notFoundRoute(res http.ResponseWriter, req *http.Request) {
	incrementScanner(req)
	// log.Debugf("404: %s", req.URL)
	http.Error(res, "404: no light here.", http.StatusNotFound)
}

// called when a resource/endpoint is not found
func notFoundJSONRoute(res http.ResponseWriter, req *http.Request) {
	err := fmt.Errorf("404 not found")
	// log.Debugw(err.Error(), "URL", req.URL)
	incrementScanner(req)
	http.Error(res, jsonError(err), http.StatusNotFound)
}

func incrementScanner(req *http.Request) {
	scannerMux.Lock()
	defer scannerMux.Unlock()
	i, ok := c.scanners[req.RemoteAddr]
	if ok {
		c.scanners[req.RemoteAddr] = i + 1
	} else {
		c.scanners[req.RemoteAddr] = 1
	}
}

func fbmswRoute(res http.ResponseWriter, req *http.Request) {
	prefix := http.Dir(c.FrontendPath)
	http.ServeFile(res, req, fmt.Sprintf("%s/static/firebase/firebase-messaging-sw.js", prefix))
}
