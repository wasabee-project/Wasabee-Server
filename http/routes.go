package PhDevHTTP

import (
//	"errors"
	"fmt"
	"net/http"
//	"net/http/httputil"
//	"strings"
	"io/ioutil"

	"golang.org/x/oauth2"
//	"golang.org/x/oauth2/google"

	"github.com/gorilla/mux"
	"github.com/cloudkucooland/PhDevBin"
)

func setupRoutes(r *mux.Router) {
	// Upload function
	r.HandleFunc("/", uploadRoute).Methods("POST")
	r.Methods("OPTIONS").HandlerFunc(optionsRoute)

	// Static aliased HTML files
	r.HandleFunc("/", advancedStaticRoute(config.FrontendPath, "/index.html", routeOptions{
		ignoreExceptions: true,
		modifySource: func(body *string) {
			replaceBlockVariable(body, "if_fork", false)
		},
	})).Methods("GET")

	// Static files
	PhDevBin.Log.Debugf("Including static files from: %s", config.FrontendPath)
	addStaticDirectory(config.FrontendPath, "/", r)

	r.HandleFunc("/me", meShowRoute).Methods("GET") // show my stats (color/tags)
	r.HandleFunc("/me", meSetColorRoute).Methods("GET").Queries("color", "{color}") // set my color /me?color=445566
	r.HandleFunc("/me/{tag}", meToggleTagRoute).Methods("GET").Queries("state", "{state}") // /me/wonky-tag-1234?state={Off|On}
	r.HandleFunc("/me/{tag}", meRemoveTagRoute).Methods("DELETE") // remove me from tag

    // Oauth2 stuff
	r.HandleFunc("/login", googleRoute).Methods("GET")
	r.HandleFunc("/callback", callbackRoute).Methods("GET")

	// Documents
	r.HandleFunc("/{document}", getRoute).Methods("GET")
	r.HandleFunc("/{document}", deleteRoute).Methods("DELETE")
	r.HandleFunc("/{document}", updateRoute).Methods("PUT")
	r.HandleFunc("/draw/{document}", getRoute).Methods("GET")
	r.HandleFunc("/draw/{document}", deleteRoute).Methods("DELETE")
	r.HandleFunc("/draw/{document}", updateRoute).Methods("PUT")

    // tags
    r.HandleFunc("/tag/{tag}", getTagRoute).Methods("GET") // return the location of every user if authorized
    r.HandleFunc("/tag/{tag}", deleteTagRoute).Methods("DELETE") // remove the tag completely
    r.HandleFunc("/tag/{tag}/{guid}", addUserToTagRoute).Methods("GET") // invite user to tag
    r.HandleFunc("/tag/{tag}/{guid}", delUserFmTagRoute).Methods("DELETE") // remove user from tag

	// 404 error page
	r.PathPrefix("/").HandlerFunc(notFoundRoute)
}

func optionsRoute(res http.ResponseWriter, req *http.Request) {
    // I think this is now taken care of in the middleware
    res.Header().Add("Allow", "GET, PUT, POST, OPTIONS, HEAD, DELETE")
	res.WriteHeader(200)
    return 
}

func getRoute(res http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	id := vars["document"]

	doc, err := PhDevBin.Request(id)
	if err != nil {
		notFoundRoute(res, req)
	}

	res.Header().Add("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprintf(res, "%s", doc.Content)
}

func deleteRoute(res http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	id := vars["document"]

	err := PhDevBin.Delete(id)
	if err != nil {
        PhDevBin.Log.Error(err)
	}

	res.Header().Add("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprint(res, "OK: document removed.\n")
}

func meRoute(res http.ResponseWriter, req *http.Request) {
    res.Header().Add("Content-Type", "text/plain")
	fmt.Fprint(res, "a screen full of data about me will be here.\n")
}

func internalErrorRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Add("Content-Type", "text/plain; charset=utf-8")
	res.WriteHeader(500)
	fmt.Fprint(res, "Oh no, the server is broken! ಠ_ಠ\nYou should try again in a few minutes, there's probably a desperate admin running around somewhere already trying to fix it.\n")
}

func notFoundRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Add("Content-Type", "text/plain; charset=utf-8")
	res.WriteHeader(404)
	fmt.Fprint(res, "404: Maybe the document is expired or has been removed.\n")
}

func googleRoute(w http.ResponseWriter, r *http.Request) {
	url := googleOauthConfig.AuthCodeURL(config.oauthStateString)
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

func callbackRoute(w http.ResponseWriter, r *http.Request) {
	content, err := getUserInfo(r.FormValue("state"), r.FormValue("code"))
	if err != nil {
		fmt.Println(err.Error())
		http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
		return
	}
	fmt.Fprintf(w, "Content: %s\n", content)
}

func getUserInfo(state string, code string) ([]byte, error) {
	if state != config.oauthStateString {
		return nil, fmt.Errorf("invalid oauth state")
	}
	token, err := googleOauthConfig.Exchange(oauth2.NoContext, code)
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

