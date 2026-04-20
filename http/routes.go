package wasabeehttps

import (
	"fmt"
	"net"
	"net/http"

	"github.com/wasabee-project/Wasabee-Server/config"
	"github.com/wasabee-project/Wasabee-Server/log"
	"github.com/wasabee-project/Wasabee-Server/util"
)

var scanners *util.Safemap

func setupRouter() *http.ServeMux {
	// Main Mux
	mux := http.NewServeMux()
	c := config.Get().HTTP

	// static files
	frontendPath := config.Get().FrontendPath
	fs := http.FileServer(http.Dir(frontendPath))
	
	// Redirection handlers for common files
	mux.Handle("GET /favicon.ico", http.RedirectHandler("/static/favicon.ico", http.StatusFound))
	mux.Handle("GET /robots.txt", http.RedirectHandler("/static/robots.txt", http.StatusFound))
	mux.Handle("GET /sitemap.xml", http.RedirectHandler("/static/sitemap.xml", http.StatusFound))
	mux.Handle("GET /.well-known/security.txt", http.RedirectHandler("/static/.well-known/security.txt", http.StatusFound))

	// Firebase service worker
	mux.HandleFunc("GET /firebase-messaging-sw.js", fbmswRoute)
	mux.HandleFunc("GET /{$}", frontRoute)

	// Oauth2
	mux.HandleFunc("POST "+c.ApTokenURL, apTokenRoute)
	mux.HandleFunc("POST "+c.OneTimeTokenURL, oneTimeTokenRoute)

	// Static file serving
	mux.Handle("/static/", http.StripPrefix("/static/", fs))

	// API Sub-router logic
	// Note: Standard mux doesn't have "Subrouters" in the gorilla sense, 
	// but we just prefix the strings or use a middleware check
	setupAuthRoutes(mux, c.APIPathURL)

	// OPTIONS catch-all
	mux.HandleFunc("OPTIONS /", optionsRoute)

	return mux
}

func setupAuthRoutes(mux *http.ServeMux, prefix string) {
	// Wrapping the auth routes in the auth middleware
	// In standard library, we wrap the handler
	handle := func(pattern string, handler http.HandlerFunc) {
		mux.Handle(pattern, authMW(handler))
	}

	// Operations
	handle("POST "+prefix+"/draw", drawUploadRoute)
	handle("GET "+prefix+"/draw/{opID}", drawGetRoute)
	handle("HEAD "+prefix+"/draw/{opID}", drawGetRoute)
	handle("DELETE "+prefix+"/draw/{opID}", drawDeleteRoute)
	handle("PUT "+prefix+"/draw/{opID}", drawUpdateRoute)
	handle("POST "+prefix+"/draw/{opID}/order", drawOrderRoute)
	handle("POST "+prefix+"/draw/{opID}/info", drawInfoRoute)
	handle("POST "+prefix+"/draw/{opID}/perms", drawPermsAddRoute)
	handle("DELETE "+prefix+"/draw/{opID}/perms", drawPermsDeleteRoute)
	
	// Support both path value and query for chown
	handle("GET "+prefix+"/draw/{opID}/chown", drawChownRoute)

	// Links
	handle("GET "+prefix+"/draw/{opID}/link/{link}", drawLinkFetch)
	handle("POST "+prefix+"/draw/{opID}/link/{link}/color", drawLinkColorRoute)
	handle("GET "+prefix+"/draw/{opID}/link/{link}/swap", drawLinkSwapRoute)
	handle("POST "+prefix+"/draw/{opID}/link/{link}/assign", drawLinkAssignRoute)
	handle("POST "+prefix+"/draw/{opID}/link/{link}/desc", drawLinkDescRoute)
	handle("GET "+prefix+"/draw/{opID}/link/{link}/complete", drawLinkCompleteRoute)
	handle("GET "+prefix+"/draw/{opID}/link/{link}/incomplete", drawLinkIncompleteRoute)
	handle("POST "+prefix+"/draw/{opID}/link/{link}/reject", drawLinkRejectRoute)
	handle("POST "+prefix+"/draw/{opID}/link/{link}/claim", drawLinkClaimRoute)
	handle("POST "+prefix+"/draw/{opID}/link/{link}/zone", drawLinkZoneRoute)
	handle("POST "+prefix+"/draw/{opID}/link/{link}/delta", drawLinkDeltaRoute)

	// Markers
	handle("GET "+prefix+"/draw/{opID}/marker/{marker}", drawMarkerFetch)
	handle("POST "+prefix+"/draw/{opID}/marker/{marker}/assign", drawMarkerAssignRoute)
	handle("POST "+prefix+"/draw/{opID}/marker/{marker}/comment", drawMarkerCommentRoute)
	handle("GET "+prefix+"/draw/{opID}/marker/{marker}/acknowledge", drawMarkerAcknowledgeRoute)
	handle("GET "+prefix+"/draw/{opID}/marker/{marker}/complete", drawMarkerCompleteRoute)
	handle("GET "+prefix+"/draw/{opID}/marker/{marker}/incomplete", drawMarkerIncompleteRoute)
	handle("GET "+prefix+"/draw/{opID}/marker/{marker}/reject", drawMarkerRejectRoute)
	handle("POST "+prefix+"/draw/{opID}/marker/{marker}/claim", drawMarkerClaimRoute)
	handle("POST "+prefix+"/draw/{opID}/marker/{marker}/zone", drawMarkerZoneRoute)
	handle("POST "+prefix+"/draw/{opID}/marker/{marker}/delta", drawMarkerDeltaRoute)

	// Portals
	handle("POST "+prefix+"/draw/{opID}/portal/{portal}/comment", drawPortalCommentRoute)
	handle("PUT "+prefix+"/draw/{opID}/portal/{portal}/comment", drawPortalCommentRoute)
	handle("POST "+prefix+"/draw/{opID}/portal/{portal}/hardness", drawPortalHardnessRoute)
	handle("PUT "+prefix+"/draw/{opID}/portal/{portal}/hardness", drawPortalHardnessRoute)
	handle("POST "+prefix+"/draw/{opID}/portal/{portal}/keyonhand", drawPortalKeysRoute)
	handle("PUT "+prefix+"/draw/{opID}/portal/{portal}/keyonhand", drawPortalKeysRoute)

	// Tasks (Modern PUT-based API)
	handle("GET "+prefix+"/draw/{opID}/task/{taskID}", drawTaskFetch)
	handle("PUT "+prefix+"/draw/{opID}/task/{taskID}/order", drawTaskOrderRoute)
	handle("PUT "+prefix+"/draw/{opID}/task/{taskID}/assign", drawTaskAssignRoute)
	handle("DELETE "+prefix+"/draw/{opID}/task/{taskID}/assign", drawTaskAssignRoute)
	handle("PUT "+prefix+"/draw/{opID}/task/{taskID}/comment", drawTaskCommentRoute)
	handle("PUT "+prefix+"/draw/{opID}/task/{taskID}/complete", drawTaskCompleteRoute)
	handle("PUT "+prefix+"/draw/{opID}/task/{taskID}/acknowledge", drawTaskAcknowledgeRoute)
	handle("PUT "+prefix+"/draw/{opID}/task/{taskID}/incomplete", drawTaskIncompleteRoute)
	handle("PUT "+prefix+"/draw/{opID}/task/{taskID}/reject", drawTaskRejectRoute)
	handle("PUT "+prefix+"/draw/{opID}/task/{taskID}/claim", drawTaskClaimRoute)
	handle("PUT "+prefix+"/draw/{opID}/task/{taskID}/zone", drawTaskZoneRoute)
	handle("PUT "+prefix+"/draw/{opID}/task/{taskID}/delta", drawTaskDeltaRoute)
	handle("PUT "+prefix+"/draw/{opID}/task/{taskID}/depend/{dependsOn}", drawTaskDependAddRoute)
	handle("DELETE "+prefix+"/draw/{opID}/task/{taskID}/depend/{dependsOn}", drawTaskDependDelRoute)

	// Me
	handle("GET "+prefix+"/me", meRoute)
	handle("POST "+prefix+"/me", meRoute)
	handle("PUT "+prefix+"/me", meSetAgentLocationRoute) // using PathValue/Query logic inside handler
	handle("DELETE "+prefix+"/me/delete", meDeleteRoute)
	handle("PUT "+prefix+"/me/{team}", meToggleTeamRoute)
	handle("DELETE "+prefix+"/me/{team}", meRemoveTeamRoute)
	handle("PUT "+prefix+"/me/{team}/wdshare", meToggleTeamWDShareRoute)
	handle("PUT "+prefix+"/me/{team}/wdload", meToggleTeamWDLoadRoute)
	handle("GET "+prefix+"/me/logout", meLogoutRoute)
	handle("POST "+prefix+"/me/firebase", meFirebaseRoute)
	handle("PUT "+prefix+"/me/intelid", meIntelIDRoute)
	handle("POST "+prefix+"/me/intelid", meIntelIDRoute)
	handle("GET "+prefix+"/me/jwtrefresh", meJwtRefreshRoute)

	// Agents
	handle("GET "+prefix+"/agent/{id}", agentProfileRoute)
	handle("GET "+prefix+"/agent/{id}/image", agentPictureRoute)
	handle("POST "+prefix+"/agent/{id}/message", agentMessageRoute)
	handle("POST "+prefix+"/agent/{id}/target", agentTargetRoute)

	// Teams
	handle("POST "+prefix+"/team/new", newTeamRoute)
	handle("GET "+prefix+"/team/{team}", getTeamRoute)
	handle("DELETE "+prefix+"/team/{team}", deleteTeamRoute)
	handle("GET "+prefix+"/team/{team}/chown", chownTeamRoute)
	handle("GET "+prefix+"/team/{team}/join/{key}", joinLinkRoute)
	handle("GET "+prefix+"/team/{team}/genJoinKey", genJoinKeyRoute)
	handle("DELETE "+prefix+"/team/{team}/delJoinKey", delJoinKeyRoute)
	handle("GET "+prefix+"/team/{team}/rocks", rocksPullTeamRoute)
	handle("GET "+prefix+"/team/{team}/rockscfg", rocksCfgTeamRoute)
	handle("POST "+prefix+"/team/{team}/announce", announceTeamRoute)
	handle("PUT "+prefix+"/team/{team}/rename", renameTeamRoute)
	handle("POST "+prefix+"/team/{team}/{key}", addAgentToTeamRoute)
	handle("DELETE "+prefix+"/team/{team}/{key}", delAgentFmTeamRoute)
	handle("POST "+prefix+"/team/{team}/{gid}/comment", setAgentTeamCommentRoute)

	// Global / Bulk
	handle("POST "+prefix+"/teams", bulkTeamFetchRoute)
	handle("GET "+prefix+"/d", getDefensiveKeys)
	handle("POST "+prefix+"/d", setDefensiveKey)
	handle("POST "+prefix+"/d/bulk", setDefensiveKeyBulk)
	handle("GET "+prefix+"/loc", getAgentsLocation)
}

func optionsRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Add("Allow", "GET, PUT, POST, OPTIONS, HEAD, DELETE")
	res.WriteHeader(http.StatusOK)
}

func frontRoute(res http.ResponseWriter, req *http.Request) {
	c := config.Get()
	url := fmt.Sprintf("%s?server=%s", c.WebUIURL, c.HTTP.Webroot)
	http.Redirect(res, req, url, http.StatusMovedPermanently)
}

func notFoundRoute(res http.ResponseWriter, req *http.Request) {
	incrementScanner(req)
	http.Error(res, "404: file not found", http.StatusNotFound)
}

func notFoundJSONRoute(res http.ResponseWriter, req *http.Request) {
	incrementScanner(req)
	err := fmt.Errorf("file not found")
	http.Error(res, jsonError(err), http.StatusNotFound)
}

func incrementScanner(req *http.Request) {
	ip, _, _ := net.SplitHostPort(req.RemoteAddr)
	scanners.Increment(ip)
}

func isScanner(req *http.Request) bool {
	ip, _, _ := net.SplitHostPort(req.RemoteAddr)
	i, ok := scanners.Get(ip)
	return ok && i > 20
}

func fbmswRoute(res http.ResponseWriter, req *http.Request) {
	log.Info("old firebase service worker route")
	frontendPath := config.Get().FrontendPath
	fullPath := fmt.Sprintf("%s/static/firebase/firebase-messaging-sw.js", frontendPath)
	http.ServeFile(res, req, fullPath)
}
