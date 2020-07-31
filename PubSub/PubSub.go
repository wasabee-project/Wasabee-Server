package wasabeepubsub

import (
	"context"
	"encoding/json"
	"os"
	"time"

	"cloud.google.com/go/pubsub"
	"github.com/wasabee-project/Wasabee-Server"
	// "google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

var cancel context.CancelFunc

// Configuration is the data passed into StartPubSub
type Configuration struct {
	Cert          string
	Project       string
	hostname      string
	subscription  string
	responder     bool
	requestTopic  *pubsub.Topic
	responseTopic *pubsub.Topic
	sub           *pubsub.Subscription
	mc            chan *pubsub.Message
}

var c Configuration

// StartPubSub is the main startup function for the PubSub subsystem
func StartPubSub(config Configuration) error {
	c = config
	wasabee.Log.Debugf("starting PubSub: [%s] %s", c.Cert, c.Project)
	c.hostname, _ = os.Hostname()

	var cctx context.Context

	ctx := context.Background()
	cctx, cancel = context.WithCancel(ctx)

	opt := option.WithCredentialsFile(c.Cert)

	client, err := pubsub.NewClient(ctx, c.Project, opt)
	if err != nil {
		wasabee.Log.Errorf("error initializing pubsub: %v", err)
		return err
	}
	defer client.Close()
	c.responseTopic = client.Topic("responses")
	c.requestTopic = client.Topic("requests")
	// defer c.requestTopic.Stop()
	// defer c.responseTopic.Stop()

	if !wasabee.GetvEnlOne() && !wasabee.GetEnlRocks() {
		// use a subscription for one who requests
		c.subscription = c.hostname
		c.responder = false
	} else {
		// use the topic/subscription for all those who respond to requests
		c.responder = true
		c.subscription = "requests"
	}

	wasabee.Log.Infof("using subscription: %s", c.subscription)
	c.sub = client.Subscription(c.subscription)
	c.sub.ReceiveSettings.Synchronous = false

	c.mc = make(chan *pubsub.Message)
	defer close(c.mc)

	// spin up listener to listen for messages on the main channel
	go listenForPubSubMessages()

	// spin up listener for commands from wasabee
	if !c.responder {
		go listenForWasabeeCommands()
	}

	// send some heartbeats for testing
	go heartbeats()

	// Receive blocks until the context is cancelled or an error occurs.
	err = c.sub.Receive(cctx, func(ctx context.Context, msg *pubsub.Message) {
		// push any messages received into the main channel
		c.mc <- msg
	})
	if err != nil {
		wasabee.Log.Errorf("receive: %v", err)
		return err
	}
	wasabee.Log.Notice("PubSub exiting")
	return nil
}

func listenForPubSubMessages() {
	for msg := range c.mc {
		if msg.Attributes["Sender"] == c.hostname {
			// wasabee.Log.Debug("acking message from me")
			msg.Ack()
			continue
		}
		switch msg.Attributes["Type"] {
		case "heartbeat":
			// anyone on the subscription can ack it
			// wasabee.Log.Debugf("received heartbeat from [%s]", msg.Attributes["Sender"])
			msg.Ack()
		case "request":
			if !c.responder {
				// anyone on the requester subscription can Ack it
				wasabee.Log.Error("request on the response topic?")
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
				wasabee.Log.Infof("Nacking response for [%s] requested by [%s]", msg.Attributes["Gid"], msg.Attributes["Sender"])
				msg.Nack()
			}
		case "agent":
			if c.responder {
				// anyone on the responder subscription can Ack it
				wasabee.Log.Error("agent response on the request topic?")
				msg.Ack()
				break
			}
			if msg.Attributes["RespondingTo"] != c.hostname {
				// wasabee.Log.Debug("acking response not intended for me")
				msg.Ack()
				break
			}
			wasabee.Log.Debugf("response for [%s]", msg.Attributes["Gid"])
			if msg.Attributes["Authoritative"] == "true" {
				wasabee.Log.Debug("Authoritative")
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
		default:
			wasabee.Log.Noticef("unknown message type [%s]", msg.Attributes["Type"])
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
		default:
			wasabee.Log.Notice("unknown PubSub command: %s", cmd.Command)
		}

	}
	Shutdown()
}

// Shutdown calls the subscription receive cancel function, triggering Start() to return
func Shutdown() {
	cancel()
}

func request(gid string) error {
	ctx := context.Background()

	atts := make(map[string]string)
	atts["Type"] = "request"
	atts["Gid"] = gid
	atts["Sender"] = c.hostname

	c.requestTopic.Publish(ctx, &pubsub.Message{
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

	atts := make(map[string]string)
	atts["Type"] = "agent"
	atts["Gid"] = g
	atts["Sender"] = c.hostname
	atts["RespondingTo"] = sender
	if wasabee.GetvEnlOne() && wasabee.GetEnlRocks() {
		atts["Authoritative"] = "true"
	}

	wasabee.Log.Debugf("publishing GoogleID %s [%s]", ad.GoogleID, ad.IngressName)
	c.responseTopic.Publish(ctx, &pubsub.Message{
		Attributes: atts,
		Data:       d,
	})
	return true, nil
}

func heartbeats() {
	ticker := time.NewTicker(30 * time.Minute)
	defer ticker.Stop()

	atts := make(map[string]string)
	atts["Type"] = "heartbeat"
	atts["Sender"] = c.hostname

	c.requestTopic.Publish(context.Background(), &pubsub.Message{
		Attributes: atts,
	})
	c.responseTopic.Publish(context.Background(), &pubsub.Message{
		Attributes: atts,
	})

	for {
		t := <-ticker.C
		atts["Time"] = t.String()

		c.requestTopic.Publish(context.Background(), &pubsub.Message{
			Attributes: atts,
		})
		c.responseTopic.Publish(context.Background(), &pubsub.Message{
			Attributes: atts,
		})
	}
}
