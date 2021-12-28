package wasabeehttps

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	// "net/http/httputil"
	"os"
	"time"

	"github.com/gorilla/sessions"

	"github.com/lestrrat-go/jwx/jwa"
	"github.com/lestrrat-go/jwx/jws"
	"github.com/lestrrat-go/jwx/jwt"

	"github.com/wasabee-project/Wasabee-Server/Firebase"
	"github.com/wasabee-project/Wasabee-Server/auth"
	"github.com/wasabee-project/Wasabee-Server/config"
	"github.com/wasabee-project/Wasabee-Server/generatename"
	"github.com/wasabee-project/Wasabee-Server/log"
	"github.com/wasabee-project/Wasabee-Server/model"
)

// final step of the oauth cycle
func callbackRoute(res http.ResponseWriter, req *http.Request) {
	content, err := getAgentInfo(req.Context(), req.FormValue("state"), req.FormValue("code"))
	if err != nil {
		log.Error(err)
		return
	}

	var m struct {
		Gid model.GoogleID `json:"id"`
		Pic string         `json:"picture"`
	}
	if err = json.Unmarshal(content, &m); err != nil {
		log.Error(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	// session cookie
	c := config.Get().HTTP
	ses, err := store.Get(req, c.SessionName)
	if err != nil {
		// cookie is borked, maybe SessionName or key changed
		log.Error("Cookie error: ", err)
		ses = sessions.NewSession(store, c.SessionName)
		ses.Options = &sessions.Options{
			Path:     "/",
			MaxAge:   -1,
			SameSite: http.SameSiteNoneMode,
			Secure:   true,
		}
		// := creates a new err, not overwriting
		if err := ses.Save(req, res); err != nil {
			log.Error(err)
			http.Error(res, err.Error(), http.StatusInternalServerError)
			return
		}
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	authorized, err := auth.Authorize(m.Gid) // V & .rocks authorization takes place here
	if !authorized {
		log.Errorw("smurf detected", "GID", m.Gid)
		http.Error(res, "Internal Error", http.StatusForbidden)
		return
	}
	if err != nil {
		log.Error(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	if err = m.Gid.UpdatePicture(m.Pic); err != nil {
		log.Info(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	ses.Values["id"] = m.Gid.String()
	nonce, _ := calculateNonce(m.Gid)
	ses.Values["nonce"] = nonce
	ses.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   86400,
		SameSite: http.SameSiteNoneMode,
		Secure:   true,
	}

	_ = ses.Save(req, res)
	if !m.Gid.Valid() {
		log.Errorw("agent not valid at end of login?", "GID", m.Gid)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	// j, _ := mintjwt(m.Gid)
	// log.Debugw("minted jwt for testing", "jwt", j)

	for _, t := range m.Gid.TeamListEnabled() {
		if err := wfb.AgentLogin(t, m.Gid); err != nil {
			log.Error(err)
		}
	}

	name, err := m.Gid.IngressName()
	if err != nil {
		log.Errorw("no name at end of login?", "GID", m.Gid)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	// add random value to help curb login loops
	sha := sha256.Sum256([]byte(fmt.Sprintf("%s%s", m.Gid, time.Now().String())))
	h := hex.EncodeToString(sha[:])
	location := fmt.Sprintf("%s?r=%s", c.MeURL, h)
	log.Infow("legacy login", "GID", m.Gid, "name", name, "message", name+" legacy login")
	if ses.Values["loginReq"] != nil {
		rr := ses.Values["loginReq"].(string)
		if rr[:len(c.MeURL)] == c.MeURL || rr[:len(c.LoginURL)] == c.LoginURL {
			// -- need to invert this logic now
		} else {
			location = rr
		}
		delete(ses.Values, "loginReq")
	}
	http.Redirect(res, req, location, http.StatusFound) // http.StatusSeeOther
}

// the secret value exchanged / verified each request
// not really a nonce, but it started life as one
func calculateNonce(gid model.GoogleID) (string, string) {
	t := time.Now()
	y := t.Add(0 - 24*time.Hour)
	c := config.Get().HTTP

	now := fmt.Sprintf("%d-%02d-%02d", t.Year(), t.Month(), t.Day())  // t.Round(time.Hour).String()
	prev := fmt.Sprintf("%d-%02d-%02d", y.Year(), y.Month(), y.Day()) // t.Add(0 - time.Hour).Round(time.Hour).String()
	// something specific to the agent, something secret, something short-term
	current := sha256.Sum256([]byte(fmt.Sprintf("%s:%s:%s", gid, c.CookieSessionKey, now)))
	previous := sha256.Sum256([]byte(fmt.Sprintf("%s:%s:%s", gid, c.CookieSessionKey, prev)))
	return hex.EncodeToString(current[:]), hex.EncodeToString(previous[:])
}

// read the result from provider at end of oauth session
func getAgentInfo(rctx context.Context, state string, code string) ([]byte, error) {
	if state != oauthStateString {
		return nil, fmt.Errorf("invalid oauth state")
	}

	ctx, cancel := context.WithTimeout(rctx, (5 * time.Second))
	defer cancel()

	token, err := config.GetOauthConfig().Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("code exchange failed: %s", err.Error())
	}
	cancel()

	contents, err := getOauthUserInfo(token.AccessToken)
	if err != nil {
		return nil, fmt.Errorf("failed getting agent info: %s", err.Error())
	}

	return contents, nil
}

// used in getAgentInfo and apTokenRoute -- takes a user's Oauth2 token and requests their info
func getOauthUserInfo(accessToken string) ([]byte, error) {
	req, err := http.NewRequest("GET", config.Get().HTTP.OauthUserInfoURL, nil)
	if err != nil {
		log.Error(err)
		return nil, err
	}
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", accessToken))
	client := &http.Client{
		Timeout: (3 * time.Second),
	}
	resp, err := client.Do(req)
	if err != nil {
		log.Error(err)
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Error(err)
		return nil, err
	}
	return body, nil
}

func getAgentID(req *http.Request) (model.GoogleID, error) {
	// X-Wasabee-GID header set in authMW
	if x := req.Header.Get("X-Wasabee-GID"); x != "" {
		return model.GoogleID(x), nil
	}

	err := errors.New("getAgentID called for unauthenticated agent")
	log.Error(err)
	return "", err
}

// apTokenRoute receives a Google Oauth2 token from the Android/iOS app and sets the authentication cookie
func apTokenRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Set("Content-Type", jsonType)

	var m struct {
		Gid model.GoogleID `json:"id"`
		Pic string         `json:"picture"`
	}

	// passed in from the clients
	type token struct {
		AccessToken string `json:"accessToken"`
		BadAT       string `json:"access_token"` // some APIs use this name, have it here for logging
	}
	var t token

	if !contentTypeIs(req, jsonTypeShort) {
		err := fmt.Errorf("invalid aptok send (needs to be application/json)")
		http.Error(res, jsonError(err), http.StatusNotAcceptable)
		log.Warn(err)
		return
	}

	jBlob, err := ioutil.ReadAll(req.Body)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	if string(jBlob) == "" {
		err = fmt.Errorf("empty JSON in aptok route")
		log.Warn(err)
		http.Error(res, jsonStatusEmpty, http.StatusNotAcceptable)
		return
	}
	if err = json.Unmarshal(json.RawMessage(jBlob), &t); err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	contents, err := getOauthUserInfo(t.AccessToken)
	if err != nil {
		log.Info(err)
		err = fmt.Errorf("aptok failed getting agent info from Google")
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	if err = json.Unmarshal(contents, &m); err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	// yes, we've seen this with a bad accessToken
	if m.Gid == "" {
		log.Errorw("bad aptok", "from client", t, "from google", m)
		err = fmt.Errorf("no GoogleID set")
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	// session cookie
	c := config.Get().HTTP
	ses, err := store.Get(req, c.SessionName)
	if err != nil {
		// cookie is borked, maybe sessionName or key changed
		log.Errorw("aptok cookie error", "error", err.Error())
		ses = sessions.NewSession(store, c.SessionName)
		ses.Options = &sessions.Options{
			Path:     "/",
			MaxAge:   -1,
			SameSite: http.SameSiteNoneMode,
			Secure:   true,
		}
		_ = ses.Save(req, res)
		res.Header().Set("Connection", "close")
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	authorized, err := auth.Authorize(m.Gid) // V & .rocks authorization takes place here
	if !authorized {
		err = fmt.Errorf("access denied: %s", err.Error())
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}
	if err != nil { // XXX if !authorized err will be set ; if err is set !authorized ... this is redundant
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	ses.Values["id"] = m.Gid.String()
	nonce, _ := calculateNonce(m.Gid)
	ses.Values["nonce"] = nonce
	ses.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   86400,
		SameSite: http.SameSiteNoneMode,
		Secure:   true,
	}
	err = ses.Save(req, res)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	name, err := m.Gid.IngressName()
	if err != nil {
		log.Error(err)
	}

	agent, err := m.Gid.GetAgent()
	if err != nil {
		log.Error(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	agent.QueryToken = formValidationToken(req)
	agent.JWT, err = mintjwt(m.Gid)
	if err != nil {
		log.Error(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	data, err := json.Marshal(agent)
	if err != nil {
		log.Error(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	log.Infow("iitc/app login",
		"gid", m.Gid,
		"agent", name,
		"message", name+" login",
		"client", req.Header.Get("User-Agent"),
	)

	// notify other teams of agent login
	for _, t := range m.Gid.TeamListEnabled() {
		if err := wfb.AgentLogin(t, m.Gid); err != nil {
			log.Error(err)
		}
	}

	res.Header().Set("Connection", "close") // no keep-alives so cookies get processed, go makes this work in HTTP/2
	res.Header().Set("Cache-Control", "no-store")

	// update picture
	_ = m.Gid.UpdatePicture(m.Pic)

	fmt.Fprint(res, string(data))
}

func mintjwt(gid model.GoogleID) (string, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return "", err
	}

	// XXX use last, rather than first?
	key, ok := config.JWSigningKeys().Get(0)
	if !ok {
		return "", fmt.Errorf("encryption jwk not set")
	}

	// keyid, ok := key.Get("kid")
	// if ok { log.Debug("using kid: ", keyid.(string), " to sign this token") }

	jwts, err := jwt.NewBuilder().
		IssuedAt(time.Now()).
		Subject(string(gid)).
		Issuer(hostname).
		JwtID(generatename.GenerateID(16)).
		Audience([]string{"wasabee"}).
		Expiration(time.Now().Add(time.Hour * 24 * 7)).
		Build()
	if err != nil {
		return "", err
	}

	// let consumers know where to get the keys if they want to verify
	hdrs := jws.NewHeaders()
	_ = hdrs.Set(jws.JWKSetURLKey, config.Get().JKU)

	signed, err := jwt.Sign(jwts, jwa.RS256, key, jwt.WithHeaders(hdrs))
	if err != nil {
		return "", err
	}

	// log.Infow("jwt", "signed", string(signed[:]))
	return string(signed[:]), nil
}

// the user must first log in to the web interface to get this token
// which they use to log in via Wasabee-IITC or Wasabee-Mobile
// in the future can this bee the JWT value?
func oneTimeTokenRoute(res http.ResponseWriter, req *http.Request) {
	res.Header().Set("Content-Type", jsonType)

	if !contentTypeIs(req, "multipart/form-data") {
		err := fmt.Errorf("invalid content-type (needs to be multipart/form-data)")
		log.Warn(err)
		http.Error(res, jsonError(err), http.StatusNotAcceptable)
		return
	}

	token := model.OneTimeToken(req.FormValue("token"))
	if token == "" {
		err := fmt.Errorf("token not set")
		log.Warn(err)
		http.Error(res, jsonError(err), http.StatusNotAcceptable)
		return
	}

	gid, err := token.Increment()
	if err != nil {
		incrementScanner(req)
		err := fmt.Errorf("invalid one-time token")
		log.Warn(err)
		http.Error(res, jsonError(err), http.StatusNotAcceptable)
		return
	}

	// session cookie
	c := config.Get().HTTP
	ses, err := store.Get(req, config.Get().HTTP.SessionName)
	if err != nil {
		// cookie is borked, maybe sessionName or key changed
		log.Error("Cookie error: " + err.Error())
		ses = sessions.NewSession(store, c.SessionName)
		ses.Options = &sessions.Options{
			Path:     "/",
			MaxAge:   -1,
			SameSite: http.SameSiteNoneMode,
			Secure:   true,
		}
		_ = ses.Save(req, res)
		res.Header().Set("Connection", "close")
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	authorized, err := auth.Authorize(gid) // V & .rocks authorization takes place here
	if !authorized {
		err = fmt.Errorf("access denied: %s", err)
		http.Error(res, jsonError(err), http.StatusForbidden)
		return
	}
	if err != nil { // XXX if !authorized err will be set ; if err is set !authorized ... this is redundant
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	ses.Values["id"] = gid.String()
	nonce, _ := calculateNonce(gid)
	ses.Values["nonce"] = nonce
	ses.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   86400,
		SameSite: http.SameSiteNoneMode,
		Secure:   true,
	}
	err = ses.Save(req, res)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	name, err := gid.IngressName()
	if err != nil {
		log.Error(err)
	}

	agent, err := gid.GetAgent()
	if err != nil {
		log.Error(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	agent.QueryToken = formValidationToken(req)
	data, err := json.Marshal(agent)
	if err != nil {
		log.Error(err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	agent.JWT, _ = mintjwt(gid)

	log.Infow("oneTimeToken login",
		"GID", gid,
		"name", name,
		"message", name+" oneTimeToken login",
		"client", req.Header.Get("User-Agent"))

	for _, t := range gid.TeamListEnabled() {
		if err := wfb.AgentLogin(t, gid); err != nil {
			log.Error(err)
		}
	}

	res.Header().Set("Connection", "close") // no keep-alives so cookies get processed, go makes this work in HTTP/2
	res.Header().Set("Cache-Control", "no-store")

	fmt.Fprint(res, string(data))
}
