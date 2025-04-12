package wasabeehttps

import (
	"fmt"
	"net"
	"net/http"
	// "net/http/httputil"
	// "strings"

	"github.com/gorilla/mux"
	"github.com/wasabee-project/Wasabee-Server/config"
	"github.com/wasabee-project/Wasabee-Server/log"
	"github.com/wasabee-project/Wasabee-Server/util"
)

var scanners *util.Safemap

func setupRouter() *mux.Router {
	// Main Router
	router := config.NewRouter()
	c := config.Get().HTTP

	// apply to all
	router.Use(headersMW)
	// router.Use(debugMW)
	router.Use(unrolled.Handler)
	router.NotFoundHandler = http.HandlerFunc(notFoundRoute)
	router.MethodNotAllowedHandler = http.HandlerFunc(notFoundRoute)
	router.Methods("OPTIONS").HandlerFunc(optionsRoute)

	// Google Oauth2 stuff (constants defined in server.go)
	router.HandleFunc(c.ApTokenURL, apTokenRoute).Methods("POST")           // all clients should use this
	router.HandleFunc(c.OneTimeTokenURL, oneTimeTokenRoute).Methods("POST") // provided for cases where aptok does not work

	// Apple Authentication routes
	// router.HandleFunc("/apple", appleRoute) // need more details, good enough for now

	// common files that live under /static
	router.Path("/favicon.ico").Handler(http.RedirectHandler("/static/favicon.ico", http.StatusFound))
	router.Path("/robots.txt").Handler(http.RedirectHandler("/static/robots.txt", http.StatusFound))
	router.Path("/sitemap.xml").Handler(http.RedirectHandler("/static/sitemap.xml", http.StatusFound))
	router.Path("/.well-known/security.txt").Handler(http.RedirectHandler("/static/.well-known/security.txt", http.StatusFound))

	// this cannot be a redirect -- sent it raw
	router.HandleFunc("/firebase-messaging-sw.js", fbmswRoute).Methods("GET")
	router.HandleFunc("/", frontRoute).Methods("GET")

	// /api/v1/... route
	api := config.Subrouter(c.APIPathURL)
	api.Methods("OPTIONS").HandlerFunc(optionsRoute)
	setupAuthRoutes(api)
	api.Use(authMW)
	api.NotFoundHandler = http.HandlerFunc(notFoundJSONRoute)
	api.MethodNotAllowedHandler = http.HandlerFunc(notFoundJSONRoute)
	api.PathPrefix("/api").HandlerFunc(notFoundJSONRoute)

	// /static files
	static := config.Subrouter("/static")
	static.PathPrefix("/").Handler(http.FileServer(http.Dir(config.Get().FrontendPath)))
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
	r.HandleFunc("/draw/{opID}", drawGetRoute).Methods("GET", "HEAD")
	r.HandleFunc("/draw/{opID}", drawDeleteRoute).Methods("DELETE")
	r.HandleFunc("/draw/{opID}", drawUpdateRoute).Methods("PUT")
	r.HandleFunc("/draw/{opID}/delete", drawDeleteRoute).Methods("GET", "DELETE")
	r.HandleFunc("/draw/{opID}/chown", drawChownRoute).Methods("GET").Queries("to", "{to}")
	r.HandleFunc("/draw/{opID}/order", drawOrderRoute).Methods("POST")
	r.HandleFunc("/draw/{opID}/info", drawInfoRoute).Methods("POST")
	r.HandleFunc("/draw/{opID}/perms", drawPermsAddRoute).Methods("POST")
	r.HandleFunc("/draw/{opID}/perms", drawPermsDeleteRoute).Methods("DELETE")
	r.HandleFunc("/draw/{opID}/delperm", drawPermsDeleteRoute).Methods("GET") // .Queries("team", "{team}", "role", "{role}")

	// links
	r.HandleFunc("/draw/{opID}/link/{link}", drawLinkFetch).Methods("GET")
	r.HandleFunc("/draw/{opID}/link/{link}/color", drawLinkColorRoute).Methods("POST")
	r.HandleFunc("/draw/{opID}/link/{link}/swap", drawLinkSwapRoute).Methods("GET")
	r.HandleFunc("/draw/{opID}/link/{link}/assign", drawLinkAssignRoute).Methods("POST")        // deprecated, use task
	r.HandleFunc("/draw/{opID}/link/{link}/desc", drawLinkDescRoute).Methods("POST")            // deprecated, use task
	r.HandleFunc("/draw/{opID}/link/{link}/complete", drawLinkCompleteRoute).Methods("GET")     // deprecated, use task
	r.HandleFunc("/draw/{opID}/link/{link}/incomplete", drawLinkIncompleteRoute).Methods("GET") // deprecated, use task
	r.HandleFunc("/draw/{opID}/link/{link}/reject", drawLinkRejectRoute).Methods("POST")        // deprecated, use task
	r.HandleFunc("/draw/{opID}/link/{link}/claim", drawLinkClaimRoute).Methods("POST")          // deprecated, use task
	r.HandleFunc("/draw/{opID}/link/{link}/zone", drawLinkZoneRoute).Methods("POST")            // deprecated, use task
	r.HandleFunc("/draw/{opID}/link/{link}/delta", drawLinkDeltaRoute).Methods("POST")          // deprecated, use task

	// markers
	r.HandleFunc("/draw/{opID}/marker/{marker}", drawMarkerFetch).Methods("GET")
	r.HandleFunc("/draw/{opID}/marker/{marker}/assign", drawMarkerAssignRoute).Methods("POST")          // deprecated, use task
	r.HandleFunc("/draw/{opID}/marker/{marker}/comment", drawMarkerCommentRoute).Methods("POST")        // deprecated, use task
	r.HandleFunc("/draw/{opID}/marker/{marker}/acknowledge", drawMarkerAcknowledgeRoute).Methods("GET") // deprecated, use task
	r.HandleFunc("/draw/{opID}/marker/{marker}/complete", drawMarkerCompleteRoute).Methods("GET")       // deprecated, use task
	r.HandleFunc("/draw/{opID}/marker/{marker}/incomplete", drawMarkerIncompleteRoute).Methods("GET")   // deprecated, use task
	r.HandleFunc("/draw/{opID}/marker/{marker}/reject", drawMarkerRejectRoute).Methods("GET")           // deprecated, use task
	r.HandleFunc("/draw/{opID}/marker/{marker}/claim", drawMarkerClaimRoute).Methods("POST")            // deprecated, use task
	r.HandleFunc("/draw/{opID}/marker/{marker}/zone", drawMarkerZoneRoute).Methods("POST")              // deprecated, use task
	r.HandleFunc("/draw/{opID}/marker/{marker}/delta", drawMarkerDeltaRoute).Methods("POST")            // deprecated, use task

	// portals
	r.HandleFunc("/draw/{opID}/portal/{portal}/comment", drawPortalCommentRoute).Methods("POST", "PUT")   // prefer PUT
	r.HandleFunc("/draw/{opID}/portal/{portal}/hardness", drawPortalHardnessRoute).Methods("POST", "PUT") // prefer PUT
	r.HandleFunc("/draw/{opID}/portal/{portal}/keyonhand", drawPortalKeysRoute).Methods("POST", "PUT")    // prefer PUT

	// tasks -- TODO unify between markers, links and generic tasks -- note changes from POST/GET to PUT
	r.HandleFunc("/draw/{opID}/task/{taskID}", drawTaskFetch).Methods("GET")                                // none
	r.HandleFunc("/draw/{opID}/task/{taskID}/order", drawTaskOrderRoute).Methods("PUT")                     // order int16
	r.HandleFunc("/draw/{opID}/task/{taskID}/assign", drawTaskAssignRoute).Methods("PUT")                   // assign []GoogleID
	r.HandleFunc("/draw/{opID}/task/{taskID}/assign", drawTaskAssignRoute).Methods("DELETE")                // none
	r.HandleFunc("/draw/{opID}/task/{taskID}/comment", drawTaskCommentRoute).Methods("PUT")                 // none
	r.HandleFunc("/draw/{opID}/task/{taskID}/complete", drawTaskCompleteRoute).Methods("PUT")               // none
	r.HandleFunc("/draw/{opID}/task/{taskID}/acknowledge", drawTaskAcknowledgeRoute).Methods("PUT")         // none
	r.HandleFunc("/draw/{opID}/task/{taskID}/incomplete", drawTaskIncompleteRoute).Methods("PUT")           // none
	r.HandleFunc("/draw/{opID}/task/{taskID}/reject", drawTaskRejectRoute).Methods("PUT")                   // none
	r.HandleFunc("/draw/{opID}/task/{taskID}/claim", drawTaskClaimRoute).Methods("PUT")                     // none
	r.HandleFunc("/draw/{opID}/task/{taskID}/zone", drawTaskZoneRoute).Methods("PUT")                       // zone uint8
	r.HandleFunc("/draw/{opID}/task/{taskID}/delta", drawTaskDeltaRoute).Methods("PUT")                     // delta int64
	r.HandleFunc("/draw/{opID}/task/{taskID}/depend/{dependsOn}", drawTaskDependAddRoute).Methods("PUT")    // none
	r.HandleFunc("/draw/{opID}/task/{taskID}/depend/{dependsOn}", drawTaskDependDelRoute).Methods("DELETE") // none

	r.HandleFunc("/me", meSetAgentLocationRoute).Methods("GET", "PUT").Queries("lat", "{lat}", "lon", "{lon}") // prefer PUT
	r.HandleFunc("/me", meRoute).Methods("GET", "POST", "HEAD")
	r.HandleFunc("/me/delete", meDeleteRoute).Methods("DELETE")                                     // purge all info for a agent, requires query token
	r.HandleFunc("/me/{team}", meToggleTeamRoute).Methods("GET", "PUT").Queries("state", "{state}") // prefer PUT
	r.HandleFunc("/me/{team}", meRemoveTeamRoute).Methods("DELETE")
	r.HandleFunc("/me/{team}/delete", meRemoveTeamRoute).Methods("GET")                                            // deprecated, use DELETE /me/{team}
	r.HandleFunc("/me/{team}/wdshare", meToggleTeamWDShareRoute).Methods("GET", "PUT").Queries("state", "{state}") // prefer PUT
	r.HandleFunc("/me/{team}/wdload", meToggleTeamWDLoadRoute).Methods("GET", "PUT").Queries("state", "{state}")   // prefer PUT
	r.HandleFunc("/me/logout", meLogoutRoute).Methods("GET")                                                       // deprecated, no need with JWT
	r.HandleFunc("/me/firebase", meFirebaseRoute).Methods("POST")                                                  // post a firebase token generated by google
	r.HandleFunc("/me/intelid", meIntelIDRoute).Methods("PUT", "POST")                                             // get ID from intel (not trusted)
	r.HandleFunc("/me/VAPIkey", meVAPIkeyRoute).Methods("POST")                                                    // send an V API key for team sync
	r.HandleFunc("/me/jwtrefresh", meJwtRefreshRoute).Methods("GET")                                               // returns a new JWT with the current token ID
	// r.HandleFunc("/me/commproof", meCommProofRoute).Methods("GET").Queries("name", "{name}")                       // generate a JWT to post on niantic's community to prove identity
	// r.HandleFunc("/me/commverify", meCommVerifyRoute).Methods("GET").Queries("name", "{name}")                     // fetch and verify the JWT posted on niantic's community
	// r.HandleFunc("/me/commverify", meCommClearRoute).Methods("DELETE")                                             // clear it

	// other agents
	// "profile" page, such as it is
	r.HandleFunc("/agent/{id}", agentProfileRoute).Methods("GET")
	r.HandleFunc("/agent/{id}/image", agentPictureRoute).Methods("GET")
	// send a message to a agent
	r.HandleFunc("/agent/{id}/message", agentMessageRoute).Methods("POST")
	// r.HandleFunc("/agent/{id}/fbMessage", agentFBMessageRoute).Methods("POST") // deprecated, /agent/x/message will send it via firebase
	r.HandleFunc("/agent/{id}/target", agentTargetRoute).Methods("POST") // send a target-formatted message

	// teams
	// create a new team
	r.HandleFunc("/team/new", newTeamRoute).Methods("POST", "GET").Queries("name", "{name}")     // create new team
	// r.HandleFunc("/team/{team}", addAgentToTeamRoute).Methods("GET").Queries("key", "{key}") // deprecated
	r.HandleFunc("/team/{team}", getTeamRoute).Methods("GET")       // get team membership/data
	r.HandleFunc("/team/{team}", deleteTeamRoute).Methods("DELETE") // delete team
	// r.HandleFunc("/team/{team}/delete", deleteTeamRoute).Methods("GET", "DELETE") // deprecated
	r.HandleFunc("/team/{team}/chown", chownTeamRoute).Methods("GET").Queries("to", "{to}")                                               // change team owner
	r.HandleFunc("/team/{team}/join/{key}", joinLinkRoute).Methods("GET")                                                                 // join via join-link-token
	r.HandleFunc("/team/{team}/genJoinKey", genJoinKeyRoute).Methods("GET")                                                               // generate join-link-token
	r.HandleFunc("/team/{team}/delJoinKey", delJoinKeyRoute).Methods("GET", "DELETE")                                                     // remove join-link-token
	r.HandleFunc("/team/{team}/rocks", rocksPullTeamRoute).Methods("GET")                                                                 // (re)import the team from rocks
	r.HandleFunc("/team/{team}/rockscfg", rocksCfgTeamRoute).Methods("GET").Queries("rockscomm", "{rockscomm}", "rockskey", "{rockskey}") // configure team link to enl.rocks community
	r.HandleFunc("/team/{team}/announce", announceTeamRoute).Methods("POST")             // broadcast a message to the team (form-data: m)
	r.HandleFunc("/team/{team}/rename", renameTeamRoute).Methods("PUT")                  // rename the team, (form-data: teamname)
	r.HandleFunc("/team/{team}/{key}", addAgentToTeamRoute).Methods("GET", "POST")       // key can be gid/name/enlid
	r.HandleFunc("/team/{team}/{key}", delAgentFmTeamRoute).Methods("DELETE")            // remove agent from team
	r.HandleFunc("/team/{team}/{key}/delete", delAgentFmTeamRoute).Methods("GET")        // deprecated
	r.HandleFunc("/team/{team}/{gid}/comment", setAgentTeamCommentRoute).Methods("POST") // set agent comment

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
	c := config.Get()

	url := fmt.Sprintf("%s?server=%s", c.WebUIURL, c.HTTP.Webroot)
	http.Redirect(res, req, url, http.StatusMovedPermanently)
}

// called when a resource/endpoint is not found
func notFoundRoute(res http.ResponseWriter, req *http.Request) {
	incrementScanner(req)
	// log.Debugw("404", "req", req.URL)
	http.Error(res, "404: file not found", http.StatusNotFound)
}

// called when a resource/endpoint is not found
func notFoundJSONRoute(res http.ResponseWriter, req *http.Request) {
	incrementScanner(req)
	err := fmt.Errorf("file not found")
	// log.Debugw(err.Error(), "URL", req.URL)
	http.Error(res, jsonError(err), http.StatusNotFound)
}

func incrementScanner(req *http.Request) {
	ip, _, _ := net.SplitHostPort(req.RemoteAddr)
	scanners.Increment(ip)
}

// true == block, false == permit
func isScanner(req *http.Request) bool {
	ip, _, _ := net.SplitHostPort(req.RemoteAddr)

	i, ok := scanners.Get(ip)
	if ok && i > 20 {
		return true
	}
	return false
}

func fbmswRoute(res http.ResponseWriter, req *http.Request) {
	log.Info("old firebase service worker route")
	http.ServeFile(res, req, fmt.Sprintf("%s/static/firebase/firebase-messaging-sw.js", http.Dir(config.Get().FrontendPath)))
}
