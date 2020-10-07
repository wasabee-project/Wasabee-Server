package wasabeepubsub

import (
	"context"
	"encoding/json"
	"os"
	"strings"
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
	wasabee.Log.Infow("startup", "subsystem", "PubSub", "credentials", c.Cert, "GC project", c.Project, "message", "starting PubSub")
	c.hostname, _ = os.Hostname()

	var cctx context.Context

	ctx := context.Background()
	cctx, cancel = context.WithCancel(ctx)

	opt := option.WithCredentialsFile(c.Cert)

	client, err := pubsub.NewClient(ctx, c.Project, opt)
	if err != nil {
		wasabee.Log.Errorw("startup", "subsystem", "PubSub", "error", err.Error())
		return err
	}
	defer client.Close()

	// we will use one topic for sending, one for receiving
	c.responseTopic = client.Topic("responses")
	c.requestTopic = client.Topic("requests")

	// are we a responder or a requester
	if !wasabee.GetvEnlOne() && !wasabee.GetEnlRocks() {
		// use a subscription for one who requests
		c.responder = false
		c.subscription = c.hostname
	} else {
		// use the topic/subscription for all those who respond to requests
		c.responder = true
		c.subscription = "requests"
	}

	wasabee.Log.Infow("startup", "subsystem", "PubSub", "subscription", c.subscription, "message", "using PubSub subscription "+c.subscription)
	// if !responder ... create subscription if it does not exist
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
		wasabee.Log.Error(err)
		return err
	}
	wasabee.Log.Infow("shutdown", "subsystem", "PubSub", "message", "shutting down PubSub")
	return nil
}

func listenForPubSubMessages() {
	for msg := range c.mc {
		// always ignore messages from me
		if msg.Attributes["Sender"] == c.hostname {
			msg.Ack()
			continue
		}
		switch msg.Attributes["Type"] {
		case "heartbeat":
			// anyone on the subscription can ack it
			msg.Ack()
		case "request":
			if !c.responder {
				// anyone on the requester subscription can Ack it
				wasabee.Log.Errorw("request on the response topic?", "subsystem", "PubSub")
				msg.Ack()
				break
			}
			// wasabee.Log.Debugw("request", "subsystem", "PubSub", "requester", msg.Attributes["Sender"], "GID", msg.Attributes["Gid"])
			ack, err := respond(msg.Attributes["Gid"], msg.Attributes["Sender"])
			if err != nil {
				wasabee.Log.Error(err)
				msg.Nack()
				break
			}
			if ack {
				// wasabee.Log.Debugw("ACK request", "subsystem", "PubSub", "requester", msg.Attributes["Sender"], "GID", msg.Attributes["Gid"])
				msg.Ack()
			} else {
				wasabee.Log.Warnw("NACK request", "subsystem", "PubSub", "requester", msg.Attributes["Sender"], "GID", msg.Attributes["Gid"])
				msg.Nack()
			}
		case "location":
			// the responder should amplify to all requesters
			if c.responder {
				location(msg.Attributes["Gid"], msg.Attributes["ll"])
			}
			tokens := strings.Split(msg.Attributes["ll"], ",")
			gid := wasabee.GoogleID(msg.Attributes["Gid"])
			if err := gid.AgentLocation(tokens[0], tokens[1]); err != nil {
				wasabee.Log.Error(err)
				msg.Nack()
				break
			}
			msg.Ack()
		case "agent":
			if c.responder {
				// anyone on the responder subscription can Ack it
				wasabee.Log.Error("agent response on the request topic", "subsystem", "PubSub", "responder", msg.Attributes["Sender"])
				msg.Ack()
				break
			}
			// if msg.Attributes["RespondingTo"] != c.hostname { msg.Ack() break }
			// wasabee.Log.Debugw("response", "subsystem", "PubSub", "GID", msg.Attributes["Gid"], "responder", msg.Attributes["Sender"])
			// if msg.Attributes["Authoritative"] == "true" { wasabee.Log.Debug("Authoritative") }
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
			wasabee.Log.Warnw("unknown message", "subsystem", "PubSub", "type", msg.Attributes["Type"], "sender", msg.Attributes["Sender"])
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
		case "location":
			err := location(cmd.Param, cmd.Data)
			if err != nil {
				wasabee.Log.Error(err)
			}
		default:
			wasabee.Log.Warnw("unknown PubSub command", "command", cmd.Command)
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

func location(gid, ll string) error {
	ctx := context.Background()

	atts := make(map[string]string)
	atts["Type"] = "location"
	atts["Gid"] = gid
	atts["Sender"] = c.hostname
	atts["ll"] = ll

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
	// ad.OwnedTeams = nil
	ad.Teams = nil
	ad.Ops = nil
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

	// wasabee.Log.Debugw("publishing", "subsystem", "PubSub", "GID", ad.GoogleID, "name", ad.IngressName)
	c.responseTopic.Publish(ctx, &pubsub.Message{
		Attributes: atts,
		Data:       d,
	})
	return true, nil
}

func heartbeats() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	atts := make(map[string]string)
	atts["Type"] = "heartbeat"
	atts["Sender"] = c.hostname

	if c.responder {
		c.responseTopic.Publish(context.Background(), &pubsub.Message{
			Attributes: atts,
		})
	} else {
		c.requestTopic.Publish(context.Background(), &pubsub.Message{
			Attributes: atts,
		})
	}

	for {
		t := <-ticker.C
		atts["Time"] = t.String()

		if c.responder {
			c.responseTopic.Publish(context.Background(), &pubsub.Message{
				Attributes: atts,
			})
		} else {
			c.requestTopic.Publish(context.Background(), &pubsub.Message{
				Attributes: atts,
			})
		}
	}
}
