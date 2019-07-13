package wasabeegm

import (
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io/ioutil"
	"net/http"
	"time"

	wasabee "github.com/wasabee-project/Wasabee-Server"
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
// passed to main() pre-loaded with APIKey and TemplateSet, the rest is built when the bot starts
type GMConfiguration struct {
	AccessToken string
	APIEndpoint string
	TemplateSet map[string]*template.Template
	upChan      chan json.RawMessage
	bots        []gmBotcfg
}

type gmBotcfg struct {
	Name           string `json:"name"`
	GroupID        string `json:"group_id"`
	CallbackURL    string `json:"callback_url"`
	BotID          string `json:"bot_id"`
	GroupName      string `json:"group_name"`
	AvatarURL      string `json:"avatar_url"`
	DMnotification bool   `json:"dm_notification"`
}

var config GMConfiguration

// GMbot is called from main() to start the bot.
func GMbot(init GMConfiguration) {
	if init.AccessToken == "" {
		err := errors.New("access token not set")
		wasabee.Log.Info(err)
		return
	}
	config.AccessToken = init.AccessToken

	config.APIEndpoint = "https://api.groupme.com/v3"
	// XXX check to make sure the templates are good?

	// the webhook feeds this channel
	config.upChan = make(chan json.RawMessage, 1)

	gm := wasabee.Subrouter("/gm")
	gm.HandleFunc("/{hook}", GMWebHook).Methods("POST")

	// let Wasabee-server know we can process messages
	_ = wasabee.RegisterMessageBus("GroupMe", SendMessage)

	// Tell Wasabee-server we are set up
	wasabee.GMSetBot()

	// setup config.bots
	err := getBots()
	if err != nil {
		return
	}

	// loop and process updates on the channel
	for update := range config.upChan {
		err = runUpdate(update)
		if err != nil {
			wasabee.Log.Error(err)
			continue
		}
	}
}

func runUpdate(update json.RawMessage) error {
	// wasabee.Log.Debug(string(update))
	var i InboundMessage
	err := json.Unmarshal(update, &i)
	if err != nil {
		wasabee.Log.Error(err)
		return err
	}
	wasabee.Log.Debugf("Message %s from %s", i.Text, i.UserID)
	return nil
}

// SendMessage is registered with Wasabee-server as a message bus to allow other modules to send messages via GroupMe
func SendMessage(gid wasabee.GoogleID, message string) (bool, error) {
	return false, nil
}

type gmResponse struct {
	Bot  []gmBotcfg `json:"response"`
	Meta struct {
		Code   int64    `json:"code"`
		Errors []string `json:"errors"`
	} `json:"meta"`
}

func getBots() error {
	url := fmt.Sprintf("%s/bots?token=%s", config.APIEndpoint, config.AccessToken)
	wasabee.Log.Debugf("Getting list of GroupMe bots from: %s", url)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		wasabee.Log.Error(err)
		return err
	}
	client := &http.Client{
		Timeout: 3 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		wasabee.Log.Error(err)
		return err
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		wasabee.Log.Error(err)
		return err
	}

	// wasabee.Log.Debug(string(body))

	var gmRes gmResponse
	err = json.Unmarshal(json.RawMessage(body), &gmRes)
	if err != nil {
		wasabee.Log.Error(err)
		return err
	}
	if gmRes.Meta.Code != 200 {
		err = fmt.Errorf("%d: %s", gmRes.Meta.Code, gmRes.Meta.Errors[0])
		wasabee.Log.Error(err)
		return err
	}
	for _, v := range gmRes.Bot {
		config.bots = append(config.bots, v)
		wasabee.Log.Debugf("bot %s for group %s on callback %s", v.Name, v.GroupID, v.CallbackURL)
	}

	return nil
}
