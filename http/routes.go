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

	// XXX this is going away
	r.HandleFunc("/", uploadRoute).Methods("POST")

	// Oauth2 stuff
	r.HandleFunc("/login", googleRoute).Methods("GET")
	r.HandleFunc("/callback", callbackRoute).Methods("GET")

	// Simple -- the old-style, encrypted, unauthenticated/authorized documents
	r.HandleFunc("/simple", uploadRoute).Methods("POST")
	r.HandleFunc("/simple/{document}", getRoute).Methods("GET")
	r.HandleFunc("/simple/{document}", deleteRoute).Methods("DELETE")
	r.HandleFunc("/simple/{document}", updateRoute).Methods("PUT")

	// This block requires authentication
	// Draw -- new-style, (XXX TO BE) parsed, not-encrypted, authenticated, authorized, more-functional
	r.HandleFunc("/draw", uploadRoute).Methods("POST")
	r.HandleFunc("/draw/{document}", getRoute).Methods("GET")
	r.HandleFunc("/draw/{document}", deleteRoute).Methods("DELETE")
	r.HandleFunc("/draw/{document}", updateRoute).Methods("PUT")
	// user info
	r.HandleFunc("/me", meSetIngressNameRoute).Methods("GET").Queries("name", "{name}")                // set my display name /me?name=deviousness
	r.HandleFunc("/me", meSetOwnTracksPWRoute).Methods("GET").Queries("otpw", "{otpw}")                // set my OwnTracks Password (cleartext, yes, but SSL is required)
	r.HandleFunc("/me", meSetUserLocationRoute).Methods("GET").Queries("lat", "{lat}", "lon", "{lon}") // manual location post
	r.HandleFunc("/me", meShowRoute).Methods("GET")                                                    // show my stats (agen name/teams)
	r.HandleFunc("/me/{team}", meToggleTeamRoute).Methods("GET").Queries("state", "{state}")           // /me/wonky-team-1234?state={Off|On|Primary}
	r.HandleFunc("/me/{team}", meRemoveTeamRoute).Methods("DELETE")                                    // remove me from team
	r.HandleFunc("/me/{team}/delete", meRemoveTeamRoute).Methods("GET")                                // remove me from team
	// teams
	r.HandleFunc("/team/new", newTeamRoute).Methods("POST", "GET").Queries("name", "{name}") // create a new team
	r.HandleFunc("/team/{team}", addUserToTeamRoute).Methods("GET").Queries("key", "{key}")  // invite user to team
	r.HandleFunc("/team/{team}", getTeamRoute).Methods("GET")                                // return the location of every user/target on team (team member/owner)
	r.HandleFunc("/team/{team}", deleteTeamRoute).Methods("DELETE")                          // remove the team completely (owner)
	r.HandleFunc("/team/{team}/delete", deleteTeamRoute).Methods("GET")                      // remove the team completely (owner)
	r.HandleFunc("/team/{team}/edit", editTeamRoute).Methods("GET")                          // GUI to do basic edit (owner)
	r.HandleFunc("/team/{team}/{key}", addUserToTeamRoute).Methods("GET")                    // invite user to team (owner)
	// r.HandleFunc("/team/{team}/{key}", setUserTeamColorRoute).Methods("GET").Queries("color", "{color}") // set agent color on this team (owner)
	r.HandleFunc("/team/{team}/{key}/delete", delUserFmTeamRoute).Methods("GET") // remove user from team (owner)
	r.HandleFunc("/team/{team}/{key}", delUserFmTeamRoute).Methods("DELETE")     // remove user from team (owner)

	r.HandleFunc("/status", statusRoute).Methods("GET")

	// OwnTracks URL
	r.HandleFunc("/OwnTracks", ownTracksRoute).Methods("POST")

	r.HandleFunc("/{document}", getRoute).Methods("GET")       // XXX these are going away
	r.HandleFunc("/{document}", deleteRoute).Methods("DELETE") // XXX these are going away

	// index
	r.HandleFunc("/", frontRoute).Methods("GET")
	// 404 error page
	r.PathPrefix("/").HandlerFunc(notFoundRoute)
}

func optionsRoute(res http.ResponseWriter, req *http.Request) {
	// I think this is now taken care of in the middleware
	res.Header().Add("Allow", "GET, PUT, POST, OPTIONS, HEAD, DELETE")
	res.WriteHeader(200)
	return
}

func frontRoute(res http.ResponseWriter, req *http.Request) {
	err := config.templateSet.ExecuteTemplate(res, "index", nil)
	if err != nil {
		PhDevBin.Log.Notice(err.Error())
		http.Error(res, err.Error(), http.StatusInternalServerError)
	}
	return
}

func statusRoute(res http.ResponseWriter, req *http.Request) {
	// maybe show some interesting numbers, active agents, etc...
	err := config.templateSet.ExecuteTemplate(res, "status", nil)
	if err != nil {
		PhDevBin.Log.Notice(err.Error())
		http.Error(res, err.Error(), http.StatusInternalServerError)
	}
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
	me, err := GetUserID(req)
	if me == "" {
		PhDevBin.Log.Error("Not logged in, cannot delete document")
		http.Error(res, "Unauthorized", http.StatusUnauthorized)
		return
	}

	doc, err := PhDevBin.Request(id)
	if err != nil {
		PhDevBin.Log.Error(err)
		http.Error(res, err.Error(), http.StatusInternalServerError) // should be 404?
		return
	}
	if me != doc.UserID {
		PhDevBin.Log.Errorf("Attempt to delete document owned by someone else (%s, %s)", me, doc.UserID)
		http.Error(res, "Unauthorized", http.StatusUnauthorized)
		return
	}

	err = PhDevBin.Delete(id)
	if err != nil {
		PhDevBin.Log.Error(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	res.Header().Add("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprint(res, "OK: document removed.\n")
}

func notFoundRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Add("Cache-Control", "no-cache")
	// why not just: http.Error(res, "404: Maybe the document is expired or has been removed.", http.StatusFileNotFound)
	res.Header().Add("Content-Type", "text/plain; charset=utf-8")
	res.WriteHeader(404)
	fmt.Fprint(res, "404: Maybe the document is expired or has been removed.\n")
}

func googleRoute(res http.ResponseWriter, req *http.Request) {
	url := config.googleOauthConfig.AuthCodeURL(config.oauthStateString)
	res.Header().Add("Cache-Control", "no-cache")
	http.Redirect(res, req, url, http.StatusTemporaryRedirect)
}

func callbackRoute(res http.ResponseWriter, req *http.Request) {
	type PhDevUser struct {
		Id    string `json:"id"`
		Name  string `json:"name"`
		Email string `json:"email"`
	}

	content, err := getUserInfo(req.FormValue("state"), req.FormValue("code"))
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	var m PhDevUser
	err = json.Unmarshal(content, &m)
	if err != nil {
		PhDevBin.Log.Notice(err.Error())
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	// session cookie
	ses, err := config.store.Get(req, config.sessionName)
	if err != nil {
		PhDevBin.Log.Notice(err.Error())
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	// XXX need SOME smarts here; check to see if gid already exists...
	err = PhDevBin.InitUser(m.Id)
	if err != nil {
		PhDevBin.Log.Notice(err.Error())
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	// check and update V data on each login
	var v PhDevBin.Vresult
	err = PhDevBin.VSearchUser(m.Id, &v)
	if err != nil {
		PhDevBin.Log.Notice(err.Error())
		// Agent not found is not a 500 error
	}
	if v.Data.Agent != "" {
		ses.Values["Agent"] = v.Data.Agent
		err = PhDevBin.VUpdateUser(m.Id, &v)
		if err != nil {
			PhDevBin.Log.Notice(err.Error())
			http.Error(res, err.Error(), http.StatusInternalServerError)
			return
		}
		if v.Data.Blacklisted == true {
			http.Error(res, err.Error(), http.StatusUnauthorized)
			return
		}
	}

	ses.Values["id"] = m.Id
	// hash this -- verify it on each request
	nonce := fmt.Sprintf("%s:%s:%s", m.Id, "some secret", "time rounded to hour")
	ses.Values["nonce"] = nonce
	ses.Save(req, res)
	http.Redirect(res, req, "/me?postauth=1", http.StatusPermanentRedirect)
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

func GetUserID(req *http.Request) (string, error) {
	ses, err := config.store.Get(req, config.sessionName)
	if err != nil {
		return "", err
	}

	if ses.Values["id"] == nil {
		// PhDevBin.Log.Notice("GetUserID called for unauthenticated user")
		return "", nil
	}

	userID := ses.Values["id"].(string)
	return userID, nil
}
