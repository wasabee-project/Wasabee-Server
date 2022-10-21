package wasabeehttps

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	wfb "github.com/wasabee-project/Wasabee-Server/Firebase"
	"github.com/wasabee-project/Wasabee-Server/auth"
	"github.com/wasabee-project/Wasabee-Server/config"
	"github.com/wasabee-project/Wasabee-Server/log"
	"github.com/wasabee-project/Wasabee-Server/model"

	"github.com/Timothylock/go-signin-with-apple/apple"
)

func appleRoute(res http.ResponseWriter, req *http.Request) {
	code, err := io.ReadAll(req.Body)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}
	if string(code) == "" {
		err = fmt.Errorf("empty send in apple route")
		log.Warn(err)
		http.Error(res, jsonStatusEmpty, http.StatusNotAcceptable)
		return
	}

	id, err := appleAuth(string(code))
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	// convert apple ID to GID
	gid, err := model.AppleIDtoGID(id)
	if err != nil {
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusInternalServerError)
		return
	}

	// will this ever be helpful? only if we map AppleIDs to valid GIDs
	authorized, err := auth.Authorize(gid) // V & .rocks authorization takes place here
	if !authorized {
		err = fmt.Errorf("access denied: %s", err.Error())
		log.Error(err)
		http.Error(res, jsonError(err), http.StatusForbidden)
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

	log.Infow("apple login",
		"gid", gid,
		"agent", name,
		"message", name+" login",
		"client", req.Header.Get("User-Agent"),
	)

	// notify other teams of agent login
	_ = wfb.AgentLogin(gid.TeamListEnabled(), gid)

	fmt.Fprint(res, string(data))
}

func appleAuth(code string) (string, error) {
	c := config.Get()

	// Generate the client secret used to authenticate with Apple's validation servers
	secret, err := apple.GenerateClientSecret(c.Apple.Secret, c.Apple.TeamID, c.Apple.ClientID, c.Apple.KeyID)
	if err != nil {
		log.Errorw(err.Error())
		return "", err
	}

	// Generate a new validation client
	client := apple.New()

	vReq := apple.AppValidationTokenRequest{
		ClientID:     c.Apple.ClientID,
		ClientSecret: secret,
		Code:         code,
	}

	var resp apple.ValidationResponse

	// Do the verification
	err = client.VerifyAppToken(context.Background(), vReq, &resp)
	if err != nil {
		log.Errorw(err.Error())
		return "", err
	}

	if resp.Error != "" {
		log.Errorw(resp.Error, resp.ErrorDescription)
		err := fmt.Errorf(resp.Error)
		return "", err
	}

	// Get the unique user ID
	unique, err := apple.GetUniqueID(resp.IDToken)
	if err != nil {
		log.Errorw(err.Error())
		return "", err
	}
	log.Infow(unique)

	// Get the email
	claim, err := apple.GetClaims(resp.IDToken)
	if err != nil {
		log.Errorw(err.Error())
		return "", err
	}

	log.Infow("got from apple:", claim)
	id := (*claim)["sub"].(string)
	return id, nil
}
