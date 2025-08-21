package wasabeehttps

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	// "net/http/httputil"
	"os"
	"time"

	"github.com/lestrrat-go/jwx/v3/jwa"
	"github.com/lestrrat-go/jwx/v3/jws"
	"github.com/lestrrat-go/jwx/v3/jwt"

	"github.com/wasabee-project/Wasabee-Server/Firebase"
	"github.com/wasabee-project/Wasabee-Server/auth"
	"github.com/wasabee-project/Wasabee-Server/config"
	"github.com/wasabee-project/Wasabee-Server/log"
	"github.com/wasabee-project/Wasabee-Server/model"
	"github.com/wasabee-project/Wasabee-Server/util"
)

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
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Error(err)
		return nil, err
	}
	return body, nil
}

func getAgentID(req *http.Request) (model.GoogleID, error) {
	if x := req.Context().Value("X-Wasabee-GID").(model.GoogleID); x != "" {
		return x, nil
	}

	err := errors.New("getAgentID called for unauthenticated agent")
	log.Error(err)
	return "", err
}

// apTokenRoute receives a Google Oauth2 token from the Android/iOS app and sets the JWT
func apTokenRoute(res http.ResponseWriter, req *http.Request) {
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

	jBlob, err := io.ReadAll(req.Body)
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
	_ = wfb.AgentLogin(m.Gid.TeamListEnabled(), m.Gid)

	// res.Header().Set("Connection", "close") // no keep-alives so cookies get processed, go makes this work in HTTP/2
	// res.Header().Set("Cache-Control", "no-store")

	// update picture
	_ = m.Gid.UpdatePicture(m.Pic)

	fmt.Fprint(res, string(data))
}

func mintjwt(gid model.GoogleID) (string, error) {
	sessionName := config.Get().HTTP.SessionName

	hostname, err := os.Hostname()
	if err != nil {
		return "", err
	}

	// XXX use last, rather than first?
	key, ok := config.JWSigningKeys().Key(0)
	if !ok {
		return "", fmt.Errorf("encryption jwk not set")
	}

	// keyid, ok := key.Get("kid")
	// if ok { log.Debug("using kid: ", keyid.(string), " to sign this token") }

	jwts, err := jwt.NewBuilder().
		IssuedAt(time.Now()).
		Subject(string(gid)).
		Issuer(hostname).
		JwtID(util.GenerateID(16)).
		Audience([]string{sessionName}).
		Expiration(time.Now().Add(time.Hour * 24 * 7)).
		Build()
	if err != nil {
		return "", err
	}

	// let consumers know where to get the keys if they want to verify
	hdrs := jws.NewHeaders()
	_ = hdrs.Set(jws.JWKSetURLKey, config.Get().JKU)

	signed, err := jwt.Sign(jwts, jwt.WithKey(jwa.RS256(), key, jws.WithProtectedHeaders(hdrs)))
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
		http.Error(res, jsonError(err), http.StatusForbidden)
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

	agent.JWT, err = mintjwt(gid)
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

	log.Infow("oneTimeToken login",
		"GID", gid,
		"name", name,
		"message", name+" oneTimeToken login",
		"client", req.Header.Get("User-Agent"))

	if err := wfb.AgentLogin(gid.TeamListEnabled(), gid); err != nil {
		log.Error(err)
	}

	// res.Header().Set("Connection", "close") // no keep-alives so cookies get processed, go makes this work in HTTP/2
	// res.Header().Set("Cache-Control", "no-store")

	fmt.Fprint(res, string(data))
}
