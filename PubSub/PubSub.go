package wasabeepubsub

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"cloud.google.com/go/pubsub"
	"github.com/wasabee-project/Wasabee-Server"
	// "google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

var topic *pubsub.Topic
var cancel context.CancelFunc

type Configuration struct {
	Cert         string
	Topic        string
	Project      string
	subscription string
	responder    bool
}

// StartPubSub is the main startup function for the PubSub subsystem
func StartPubSub(config Configuration) error {
	wasabee.Log.Debugf("starting PubSub: [%s] %s %s", config.Cert, config.Project, config.Topic)

	// var err error
	// var client *pubsub.Client
	var cctx context.Context

	ctx := context.Background()
	cctx, cancel = context.WithCancel(ctx)

	opt := option.WithCredentialsFile(config.Cert)

	client, err := pubsub.NewClient(ctx, config.Project, opt)
	if err != nil {
		wasabee.Log.Errorf("error initializing pubsub: %v", err)
		return err
	}
	defer client.Close()

	topic = client.Topic(config.Topic)
	// defer topic.Stop()

	if !wasabee.GetvEnlOne() && !wasabee.GetEnlRocks() {
		// use the subscription for those who only request
		config.subscription = fmt.Sprintf("%s-requester", config.Topic)
		config.responder = false
	} else {
		// use the subscription for those who respond to requests
		config.subscription = fmt.Sprintf("%s-responder", config.Topic)
		config.responder = true
	}

	wasabee.Log.Infof("using subscription: %s", config.subscription)
	mainsub := client.Subscription(config.subscription)
	mainsub.ReceiveSettings.Synchronous = false
	// mainsub.ReceiveSettings.NumGoroutines = 2 // let the library auto-determine

	mainchan := make(chan *pubsub.Message)
	defer close(mainchan)

	// spin up listener to listen for messages on the main channel
	go listenForPubSubMessages(mainchan, config)

	// spin up listener for commands from wasabee
	go listenForWasabeeCommands()

	// send some heartbeats for testing
	go heartbeats()

	// Receive blocks until the context is cancelled or an error occurs.
	err = mainsub.Receive(cctx, func(ctx context.Context, msg *pubsub.Message) {
		// push any messages received into the main channel
		mainchan <- msg
	})
	if err != nil {
		wasabee.Log.Errorf("receive: %v", err)
		return err
	}
	wasabee.Log.Notice("PubSub exiting")
	return nil
}

func listenForPubSubMessages(mainchan chan *pubsub.Message, config Configuration) {
	hostname, _ := os.Hostname()

	for msg := range mainchan {
		if msg.Attributes["Sender"] == hostname {
			wasabee.Log.Debug("ignoring message from me")
			continue
		}
		switch msg.Attributes["Type"] {
		case "heartbeat":
			// anyone on the subscription can ack it
			wasabee.Log.Debugf("received heartbeat from [%s]", msg.Attributes["Sender"])
			msg.Ack()
			break
		case "request":
			if !config.responder {
				// anyone on the requester subscription can Ack it
				msg.Ack()
				break
			}
			wasabee.Log.Debugf("[%s] requesing [%s]", msg.Attributes["Sender"], msg.Attributes["Gid"])
			ack, err := respond(msg.Attributes["Gid"], msg.Attributes["Sender"])
			if err != nil {
				wasabee.Log.Debug(err)
				msg.Nack()
				break
			}
			if ack {
				msg.Ack()
			} else {
				wasabee.Log.Debugf("Nacking response for [%s] requested by [%s]", msg.Attributes["Gid"], msg.Attributes["Sender"])
				msg.Nack()
			}
			break
		case "agent":
			if config.responder {
				// anyone on the responder subscription can Ack it
				msg.Ack()
				break
			}
			if msg.Attributes["RespondingTo"] != hostname {
				wasabee.Log.Debug("Nacking response not intended for me")
				msg.Nack()
				break
			}
			wasabee.Log.Debugf("response for [%s]", msg.Attributes["Gid"])
			if msg.Attributes["Authorative"] == "true" {
				wasabee.Log.Debug("authoritative")
			}
			var ad wasabee.AgentData
			err := json.Unmarshal(msg.Data, &ad)
			if err != nil {
				wasabee.Log.Error(err)
				msg.Nack()
				break
			}
			err = ad.Save()
			if err != nil {
				wasabee.Log.Error(err)
				msg.Nack()
				break
			}
			msg.Ack()
			break
		default:
			wasabee.Log.Debugf("unknown message type [%s]", msg.Attributes["Type"])
			// get it off the subscription quickly
			msg.Ack()
		}
	}
}

func listenForWasabeeCommands() {
	cmdchan := wasabee.PubSubInit()
	for cmd := range cmdchan {
		switch cmd.Command {
		case "request":
			err := request(cmd.Param)
			if err != nil {
				wasabee.Log.Error(err)
			}
			break
		default:
			wasabee.Log.Notice("unknown PubSub command: %s", cmd.Command)
		}

	}
	Shutdown()
}

// shutdown calls the subscription receive cancel function, triggering Start() to return
func Shutdown() error {
	cancel()
	return nil
}

func request(gid string) error {
	ctx := context.Background()
	hostname, _ := os.Hostname()

	var atts map[string]string
	atts = make(map[string]string)
	atts["Type"] = "request"
	atts["Gid"] = gid
	atts["Sender"] = hostname

	topic.Publish(ctx, &pubsub.Message{
		Attributes: atts,
		Data:       []byte(""),
	})

	return nil
}

func respond(g string, sender string) (bool, error) {
	// if the APIs are not running, don't respond
	if !wasabee.GetvEnlOne() && !wasabee.GetEnlRocks() {
		return false, nil
	}

	ctx := context.Background()
	hostname, _ := os.Hostname()

	gid := wasabee.GoogleID(g)

	// make sure we have the most current info from the APIs
	_, err := gid.InitAgent()
	if err != nil {
		wasabee.Log.Error(err)
		return false, err
	}

	var ad wasabee.AgentData
	err = gid.GetAgentData(&ad)
	if err != nil {
		wasabee.Log.Error(err)
		return false, err
	}

	// we do not need to send team/op data across
	ad.OwnedTeams = nil
	ad.Teams = nil
	ad.Ops = nil
	ad.OwnedOps = nil
	ad.Assignments = nil

	d, err := json.Marshal(ad)
	if err != nil {
		wasabee.Log.Error(err)
		return false, err
	}

	var atts map[string]string
	atts = make(map[string]string)
	atts["Type"] = "agent"
	atts["Gid"] = g
	atts["Sender"] = hostname
	atts["RespondingTo"] = sender
	if wasabee.GetvEnlOne() && wasabee.GetEnlRocks() {
		atts["Authoratative"] = "true"
	}

	wasabee.Log.Debugf("publishing GoogleID %s [%s]", ad.GoogleID, ad.IngressName)
	topic.Publish(ctx, &pubsub.Message{
		Attributes: atts,
		Data:       d,
	})
	return true, nil
}

func heartbeats() {
	ticker := time.NewTicker(30 * time.Minute)
	defer ticker.Stop()

	var atts map[string]string
	atts = make(map[string]string)
	atts["Type"] = "heartbeat"
	atts["Sender"], _ = os.Hostname()

	for {
		t := <-ticker.C
		atts["Time"] = t.String()

		topic.Publish(context.Background(), &pubsub.Message{
			Attributes: atts,
		})
	}
}
