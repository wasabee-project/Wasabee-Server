package PhDevHTTP

import (
	//	"errors"
	"encoding/json"
	"fmt"
	"net/http"
	//	"net/http/httputil"
	//	"strings"
	"io/ioutil"

	"golang.org/x/oauth2"
	//	"golang.org/x/oauth2/google"

	"github.com/cloudkucooland/PhDevBin"
	"github.com/gorilla/mux"
)

func setupRoutes(r *mux.Router) {
	// Upload function
	r.Methods("OPTIONS").HandlerFunc(optionsRoute)
	r.HandleFunc("/", uploadRoute).Methods("POST")

	// Static aliased HTML files
	r.HandleFunc("/", advancedStaticRoute(config.FrontendPath, "/index.html", routeOptions{
		ignoreExceptions: true,
		modifySource: func(body *string) {
			replaceBlockVariable(body, "if_fork", false)
		},
	})).Methods("GET")

	// Static files
	PhDevBin.Log.Notice("Including static files from: %s", config.FrontendPath)
	addStaticDirectory(config.FrontendPath, "/", r)

	// Oauth2 stuff
	r.HandleFunc("/login", googleRoute).Methods("GET")
	r.HandleFunc("/callback", callbackRoute).Methods("GET")

	// Documents
	r.HandleFunc("/draw", uploadRoute).Methods("POST")
	r.HandleFunc("/draw/{document}", getRoute).Methods("GET")
	r.HandleFunc("/draw/{document}", deleteRoute).Methods("DELETE")
	r.HandleFunc("/draw/{document}", updateRoute).Methods("PUT")
	// user info
	r.HandleFunc("/me", meSetIngressNameRoute).Methods("GET").Queries("name", "{name}")    // set my display name /me?name=deviousness
	r.HandleFunc("/me", meShowRoute).Methods("GET")                                        // show my stats (agen name/tags)
	r.HandleFunc("/me/{tag}", meToggleTagRoute).Methods("GET").Queries("state", "{state}") // /me/wonky-tag-1234?state={Off|On}
	r.HandleFunc("/me/{tag}", meRemoveTagRoute).Methods("DELETE")                          // remove me from tag
	// tags
	r.HandleFunc("/tag/{tag}", getTagRoute).Methods("GET")                                          // return the location of every user if authorized
	r.HandleFunc("/tag/{tag}", deleteTagRoute).Methods("DELETE")                                    // remove the tag completely
	r.HandleFunc("/tag/{tag}/{guid}", addUserToTagRoute).Methods("GET")                             // invite user to tag
	r.HandleFunc("/tag/{tag}/{guid}", addUserToTagRoute).Methods("GET").Queries("color", "{color}") // set agent color on this tag
	r.HandleFunc("/tag/{tag}/{guid}", delUserFmTagRoute).Methods("DELETE")                          // remove user from tag

	r.HandleFunc("/{document}", getRoute).Methods("GET")
	r.HandleFunc("/{document}", deleteRoute).Methods("DELETE")
	r.HandleFunc("/{document}", updateRoute).Methods("PUT")

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
	type PhDevUser struct {
		Id    string `json:"id"`
		Name  string `json:"name"`
		Email string `json:"email"`
	}

	content, err := getUserInfo(r.FormValue("state"), r.FormValue("code"))
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	var m PhDevUser
	err = json.Unmarshal(content, &m)
	if err != nil {
		PhDevBin.Log.Notice(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	ses, err := store.Get(r, SessionName)
	if err != nil {
		PhDevBin.Log.Notice(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	ses.Values["id"] = m.Id
	ses.Values["name"] = m.Name
	ses.Save(r, w)

	err = PhDevBin.InsertOrUpdateUser(m.Id, m.Name)
	if err != nil {
		PhDevBin.Log.Notice(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	http.Redirect(w, r, "/me", http.StatusPermanentRedirect)
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

func GetUserID(req *http.Request) (string, error) {
	ses, err := store.Get(req, SessionName)
	if err != nil {
		return "", err
	}

    if ses.Values["id"] == nil {
		PhDevBin.Log.Notice("GetUserID called for unauthenticated user")
        return "", nil
	}

	userID := ses.Values["id"].(string)
	return userID, nil
}
