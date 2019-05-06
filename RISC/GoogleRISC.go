package risc

import (
	"bytes"
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"golang.org/x/oauth2/google"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/cloudkucooland/WASABI"
	"github.com/lestrrat-go/jwx/jwa"
	"github.com/lestrrat-go/jwx/jws"
	"github.com/lestrrat-go/jwx/jwt"
)

type riscConfig struct {
	Issuer      string   `json:"issuer"`
	JWKURI      string   `json:"jwks_uri"`
	Methods     []string `json:"delivery_methods_supported"`
	AddEndpoint string   `json:"add_subject_endpoint"`
	RemEndpoint string   `json:"remove_subject_endpoint"`
}

type event struct {
	Type    string
	Reason  string
	Issuer  string
	Subject string
}

type keys struct {
	Keys []key `json:"keys"`
}

type key struct {
	KeyID   string `json:"kid"`
	E       string `json:"e"`
	KeyType string `json:"kty"`
	Alg     string `json:"alg"`
	N       string `json:"n"`
	Use     string `json:"use"`
}

type serviceCreds struct {
	Type            string `json:"type"`
	ProjectID       string `json:"project_id"`
	ProjectKeyID    string `json:"private_key_id"`
	PrivateKey      string `json:"private_key"`
	ClientEmail     string `json:"client_email"`
	ClientID        string `json:"client_id"`
	AuthURI         string `json:"auth_uri"`
	TokenURI        string `json:"token_uri"`
	ProviderCertURL string `json:"auth_provider_x509_cert_url"`
	ClientCertURL   string `json:"client_x509_cert_url"`
}

var riscchan chan event
var config riscConfig
var scred serviceCreds

func RISCinit(certdir string) error {
	// load config
	if err := googleRiscDiscovery(); err != nil {
		wasabi.Log.Error(err)
		return err
	}
	// wasabi.Log.Debug(config)

	if err := riscRegisterWebhook(certdir); err != nil {
		wasabi.Log.Error(err)
		return err
	}

	// listen for updates on a new thread
	riscchan = make(chan event, 2)
	go RISCloop()

	return nil
}

func riscRegisterWebhook(certdir string) error {
	// register the receiver with Google -- get creds for service account
	ctx := context.Background()
	data, err := ioutil.ReadFile("certs/risc.json") // XXX do not hardcoded, put it in certs/ with the others
	if err != nil {
		wasabi.Log.Error(err)
		return err
	}

	creds, err := google.CredentialsFromJSON(ctx, data, "https://www.googleapis.com/auth/bigquery")
	if err != nil {
		wasabi.Log.Fatal(err)
		return err
	}
	// wasabi.Log.Debug("Creds: ", string(creds.JSON))
	err = json.Unmarshal(creds.JSON, &scred)
	if err != nil {
		wasabi.Log.Error(err)
		return err
	}

	// establish webhook -- these last an hour then need to be updated
	wasabi.Log.Debug("setting up webhooks with google")
	updateWebhook(creds)
	ticker := time.NewTicker(time.Hour)
	select {
	case <-ticker.C:
		wasabi.Log.Debug("updating webhooks with google")
		updateWebhook(creds)
	}
	return nil
}

func updateWebhook(c *google.Credentials) error {
	t := time.Now()
	token := jwt.New()
	token.Set(jwt.AudienceKey, "https://risc.googleapis.com/google.identity.risc.v1beta.RiscManagementService")
	token.Set(jwt.SubjectKey, scred.ClientEmail)
	token.Set(jwt.IssuerKey, scred.ClientEmail)
	token.Set(jwt.IssuedAtKey, t.Format(time.UnixDate))
	token.Set(jwt.ExpirationKey, t.Add(time.Hour).Format(time.UnixDate))

	buf, err := json.MarshalIndent(token, "", "  ")
	// wasabi.Log.Debug(buf)
	if err != nil {
		wasabi.Log.Error(err)
		return err
	}

	pk, err := parseRsaPrivateKeyFromPemStr(scred.PrivateKey)
	// wasabi.Log.Debug(pk)
	if err != nil {
		wasabi.Log.Error(err)
		return err
	}

	// setting a header causes a segfault -- but this won't work without the header
	// var hdr jws.Headers
	// hdr.Set("kid", scred.ProjectKeyID)
	// jwsTok, err := jws.Sign(buf, jwa.RS256, pk, jws.WithHeaders(hdr))
	jwsTok, err := jws.Sign(buf, jwa.RS256, pk)
	if err != nil {
		wasabi.Log.Error(err)
		return err
	}
	wasabi.Log.Debug(string(jwsTok))

	apiurl := "https://risc.googleapis.com/v1beta/stream:update"
	jmsg := map[string]interface{} {
		"delivery": map[string]string {
			"delivery_method": "https://schemas.openid.net/secevent/risc/delivery-method/push",
			"url": "https://qbin.phtiv.com:8443/GoogleRISC",
		},
		"events_requested": []string {
			"https://schemas.openid.net/secevent/risc/event-type/account-credential-change-required",
			"https://schemas.openid.net/secevent/risc/event-type/account-disabled",
			},
		}
	raw, err := json.Marshal(jmsg)
	if err != nil {
		wasabi.Log.Error(err)
		return err
	}
	client := http.Client{}
	req, err := http.NewRequest("POST", apiurl, bytes.NewBuffer(raw))
	if err != nil {
		wasabi.Log.Error(err)
		return err
	}
	req.Header.Set("Bearer", string(jwsTok))
	req.Header.Set("Content-Type", "application/json; charset=UTF-8")

	response, err := client.Do(req)
	if err != nil {
		wasabi.Log.Error(err)
		return err
	}
	if response.StatusCode >= 300 {
		wasabi.Log.Debug(response)
	}

	return nil
}

func RISCloop() {
	for {
		e := <-riscchan
		wasabi.Log.Notice("Pulled from channel: ", e)
		// XXX process the message
	}
}

// This is called from the webhook
func RISCValidateToken(rawjwt []byte) error {
	token, err := jwt.ParseBytes(rawjwt)
	if err != nil {
		wasabi.Log.Error(err)
		return err
	}
	wasabi.Log.Debug(token)

	kid, ok := token.Get("kid")
	if !ok {
		err = fmt.Errorf("no Key ID in token")
		wasabi.Log.Error(err)
		return err
	}

	// XXX this gets a new list of keys from Google each time, do we need to do that?
	// Just start a thread to get them each hour and cache them locally
	key, err := googleCerts(kid.(string))
	if err != nil {
		wasabi.Log.Error(err)
		return err
	}

	err = token.Verify(jwt.WithAudience(scred.ClientEmail), jwt.WithAcceptableSkew(60), jwt.WithVerify(jwa.RS256, key))
	if err != nil {
		wasabi.Log.Error(err)
		return err
	}
	wasabi.Log.Debug("verified token")

	// this was written with the other jwt library, probably needs to be redone
	var e event
	tmp, ok := token.Get("events")
	if !ok {
		wasabi.Log.Error(err)
		return err
	}
	for k, v := range tmp.(map[string]interface{}) {
		// wasabi.Log.Debug("Event: ", k, v)
		e.Type = k
		x := v.(map[string]interface{})
		e.Reason = x["reason"].(string)
		y := x["subject"].(map[string]interface{})
		e.Issuer = y["iss"].(string)
		e.Subject = y["sub"].(string)

		wasabi.Log.Debug("Pushing into channel: ", e)

		riscchan <- e
	}
	return nil
}

func googleRiscDiscovery() error {
	discovery := "https://accounts.google.com/.well-known/risc-configuration"
	req, err := http.NewRequest("GET", discovery, nil)
	if err != nil {
		wasabi.Log.Error(err)
		return err
	}
	client := &http.Client{
		Timeout: 3 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		wasabi.Log.Error(err)
		return err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		wasabi.Log.Error(err)
		return err
	}
	//  wasabi.Log.Debug(string(body))
	err = json.Unmarshal(body, &config)
	if err != nil {
		wasabi.Log.Error(err)
		return err
	}
	return nil
}

func googleCerts(kid string) (key, error) {
	var x key // unset for returning w/ errors

	req, err := http.NewRequest("GET", config.JWKURI, nil)
	if err != nil {
		wasabi.Log.Error(err)
		return x, err
	}
	client := &http.Client{
		Timeout: 3 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		wasabi.Log.Error(err)
		return x, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		wasabi.Log.Error(err)
		return x, err
	}

	var k keys
	err = json.Unmarshal(body, &k)
	if err != nil {
		wasabi.Log.Error(err)
		return x, err
	}

	for _, v := range k.Keys {
		if v.KeyID == kid {
			return v, nil
		}
	}

	err = fmt.Errorf("no matching keys found")
	return x, err
}

func parseRsaPrivateKeyFromPemStr(privPEM string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(privPEM))
	if block == nil {
		err := fmt.Errorf("failed to parse PEM block containing the key")
		wasabi.Log.Error(err)
		return nil, err
	}

	priv, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		wasabi.Log.Error(err)
		return nil, err
	}

	return priv.(*rsa.PrivateKey), nil
}
