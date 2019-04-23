package wasabigm

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/cloudkucooland/WASABI"
	"io/ioutil"
	"net/http"
	"text/template"
	"time"
)

// InboundMessage is what we receive from GM
type InboundMessage struct {
	ID          string                   `json:"id"`
	AvatarURL   string                   `json:"avatar_url"`
	Name        string                   `json:"name"`
	SenderID    string                   `json:"sender_id"`
	SenderTypee string                   `json:"sender_type"`
	System      bool                     `json:"system"`
	Text        string                   `json:"text"`
	SourceGUID  string                   `json:"source_guid"`
	CreatedAt   int                      `json:"created_at"`
	UserID      string                   `json:"user_id"`
	GroupID     string                   `json:"group_id"`
	FavoritedBy []string                 `json:"favorited_by"`
	Attachments []map[string]interface{} `json:"attachments"`
}

// OutboundMessage is what we send
type OutboundMessage struct {
	ID   string `json:"bot_id"`
	Text string `json:"text"`
}

// GMConfiguration is the main configuration data for the GroupMe interface
// passed to main() pre-loaded with APIKey and FrontendPath set, the rest is built when the bot starts
type GMConfiguration struct {
	AccessToken  string
	BotID        string
	Name         string
	GroupID      string
	APIEndpoint  string
	FrontendPath string
	templateSet  map[string]*template.Template
	upChan       chan json.RawMessage
	hook         string
}

var config GMConfiguration

// GMbot is called from main() to start the bot.
func GMbot(init GMConfiguration) error {
	if init.AccessToken == "" {
		err := errors.New("access token not set")
		wasabi.Log.Info(err)
		return err
	}
	config.AccessToken = init.AccessToken

	if init.BotID == "" {
		err := errors.New("BotID not set")
		wasabi.Log.Info(err)
		return err
	}
	config.BotID = init.BotID

	if init.GroupID == "" {
		err := errors.New("GM GroupID not set")
		wasabi.Log.Info(err)
		return err
	}
	config.GroupID = init.GroupID

	if init.Name == "" {
		config.Name = "WASABI_bot"
	} else {
		config.Name = init.Name
	}

	config.APIEndpoint = "https://api.groupme.com/v3/bots"
	config.FrontendPath = init.FrontendPath
	if config.FrontendPath == "" {
		config.FrontendPath = "frontend"
	}
	_ = templates(config.templateSet)
	// let WASABI know we can process messages
	_ = wasabi.RegisterMessageBus("GroupMe", SendMessage)
	// Tell WASABI we are set up
	wasabi.GMSetBot()

	config.hook, _ = setWebHook()
	config.upChan = make(chan json.RawMessage, 1)

	for update := range config.upChan {
		err := runUpdate(update)
		if err != nil {
			wasabi.Log.Error(err)
			continue
		}
	}
	return nil
}

func runUpdate(update json.RawMessage) error {
	wasabi.Log.Debug(string(update))
	var i InboundMessage
	err := json.Unmarshal(update, &i)
	if err != nil {
		wasabi.Log.Error(err)
		return err
	}
	wasabi.Log.Debugf("Message %s from %s", i.Text, i.UserID)
	return nil
}

// SendMessage is registered with WASABI as a message bus to allow other modules to send messages via GroupMe
func SendMessage(gid wasabi.GoogleID, message string) (bool, error) {
	return false, nil
}

type gmBot struct {
	Name           string `json:"name"`
	GroupID        string `json:"group_id"`
	CallbackURL    string `json:"callback_url"`
	BotID          string `json:"bot_id"`
	GroupName      string `json:"group_name"`
	AvatarURL      string `json:"avatar_url"`
	DMnotification bool   `json:"dm_notification"`
}

type gmCmd struct {
	Bot gmBot `json:"bot"`
}

// {"meta":{"code":201},"response":{"bot":{"name":"WASABI_bot","bot_id":"2226d82f4f2c60e032241f85d7","group_id":"49978027","group_name":"Wasabi test","avatar_url":null,"callback_url":"https://qbin.phtiv.com:8443/gm/pushing-another-2bn2","dm_notification":false}}}
type gmResponse struct {
	Response struct {
		Bot gmBot `json:"bot"`
	} `json:"response"`
	Meta struct {
		Code   int64    `json:"code"`
		Errors []string `json:"errors"`
	} `json:"meta"`
}

func setWebHook() (string, error) {
	var cmd gmCmd

	// XXX using the proper function races and yeilds ""
	webroot := "https://qbin.phtiv.com:8443"
	hook := wasabi.GenerateName()
	t := fmt.Sprintf("%s/gm/%s", webroot, hook)
	wasabi.Log.Debugf("setting GM webroot to %s", t)
	cmd.Bot.CallbackURL = t

	cmd.Bot.Name = config.Name
	cmd.Bot.GroupID = config.GroupID

	jCmd, err := json.Marshal(cmd)
	if err != nil {
		wasabi.Log.Error(err)
		return hook, err
	}
	wasabi.Log.Debug(string(jCmd))
	b := bytes.NewBufferString(string(jCmd))

	url := fmt.Sprintf("%s?token=%s", config.APIEndpoint, config.AccessToken)
	wasabi.Log.Debug(url)

	req, err := http.NewRequest("POST", url, b)
	req.Header.Add("Content-Type", "application/json")

	if err != nil {
		wasabi.Log.Error(err)
		return hook, err
	}
	client := &http.Client{
		Timeout: 3 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		wasabi.Log.Error(err)
		return hook, err
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		wasabi.Log.Error(err)
		return hook, err
	}

	wasabi.Log.Debug(string(body))
	var gmRes gmResponse
	err = json.Unmarshal(json.RawMessage(body), &gmRes)
	if err != nil {
		wasabi.Log.Error(err)
		return hook, err
	}
	if gmRes.Meta.Code != 201 {
		err = fmt.Errorf("%d: %s", gmRes.Meta.Code, gmRes.Meta.Errors[0])
		wasabi.Log.Error(err)
		return hook, err
	}

	return hook, nil
}
