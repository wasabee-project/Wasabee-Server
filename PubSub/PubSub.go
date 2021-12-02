package wps

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"time"

	"cloud.google.com/go/pubsub"
	"github.com/wasabee-project/Wasabee-Server/auth"
	"github.com/wasabee-project/Wasabee-Server/config"
	"github.com/wasabee-project/Wasabee-Server/log"
	"github.com/wasabee-project/Wasabee-Server/model"
	// "google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

var cancel context.CancelFunc

// Configuration is the data passed into Start
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

// Start is the main startup function for the PubSub subsystem
func Start(incoming Configuration) error {
	c = incoming
	log.Infow("startup", "subsystem", "PubSub", "credentials", c.Cert, "GC project", c.Project, "message", "starting PubSub")
	c.hostname, _ = os.Hostname()

	var cctx context.Context

	ctx := context.Background()
	cctx, cancel = context.WithCancel(ctx)

	opt := option.WithCredentialsFile(c.Cert)

	client, err := pubsub.NewClient(ctx, c.Project, opt)
	if err != nil {
		log.Errorw("startup", "subsystem", "PubSub", "error", err.Error())
		return err
	}
	defer client.Close()

	// we will use one topic for sending, one for receiving
	c.responseTopic = client.Topic("responses")
	c.requestTopic = client.Topic("requests")

	// are we a responder or a requester
	w := config.Get()
	if !w.V && !w.Rocks {
		// use a subscription for one who requests
		c.responder = false
		c.subscription = c.hostname
	} else {
		// use the topic/subscription for all those who respond to requests
		c.responder = true
		c.subscription = "requests"
	}

	log.Infow("startup", "subsystem", "PubSub", "subscription", c.subscription, "message", "using PubSub subscription "+c.subscription)
	// if !responder ... create subscription if it does not exist
	c.sub = client.Subscription(c.subscription)
	c.sub.ReceiveSettings.Synchronous = false

	c.mc = make(chan *pubsub.Message)
	defer close(c.mc)

	// spin up listener to listen for messages on the main channel
	go listenForPubSubMessages()

	// spin up listener for commands from model
	// if !c.responder { }
	go listenForWasabeeCommands()

	// send some heartbeats for testing
	// go heartbeats()

	// Receive blocks until the context is cancelled or an error occurs.
	err = c.sub.Receive(cctx, func(ctx context.Context, msg *pubsub.Message) {
		// push any messages received into the main channel
		c.mc <- msg
	})
	if err != nil {
		log.Errorw("PubSub failure", "message", err.Error(), "subsystem", "PubSub")
		// return err
	}
	log.Infow("shutdown", "subsystem", "PubSub", "message", "shutting down PubSub")
	return err
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
				log.Errorw("request on the response topic?", "subsystem", "PubSub")
				msg.Ack()
				break
			}
			// log.Debugw("request", "subsystem", "PubSub", "requester", msg.Attributes["Sender"], "GID", msg.Attributes["Gid"])
			ack, err := respond(msg.Attributes["Gid"], msg.Attributes["Sender"])
			if err != nil {
				log.Warnw(err.Error(), "subsystem", "PubSub", "requester", msg.Attributes["Sender"], "GID", msg.Attributes["Gid"])
				msg.Ack()
				break
			}
			if ack {
				// log.Debugw("ACK request", "subsystem", "PubSub", "requester", msg.Attributes["Sender"], "GID", msg.Attributes["Gid"])
				msg.Ack()
			} else {
				log.Warnw("NACK request", "subsystem", "PubSub", "requester", msg.Attributes["Sender"], "GID", msg.Attributes["Gid"])
				msg.Nack()
			}
		case "location":
			if msg.Attributes["ll"] == "" {
				log.Error("pubsub location lat/lng not set")
				msg.Ack()
				break
			}

			// the responder should amplify to all requesters
			if c.responder {
				location(msg.Attributes["Gid"], msg.Attributes["ll"], msg.Attributes["Sender"])
			}

			tokens := strings.Split(msg.Attributes["ll"], ",")
			gid := model.GoogleID(msg.Attributes["Gid"])
			if len(tokens) != 2 {
				log.Errorw("pubsub location ll invalid", "ll", msg.Attributes["ll"])
				msg.Ack()
				break
			}
			if err := gid.AgentLocation(tokens[0], tokens[1]); err != nil {
				log.Error(err)
				msg.Nack()
				break
			}
			msg.Ack()
		case "inteldata":
			log.Debugw("got pubsub inteldata", "sender", msg.Attributes["Sender"])
			if msg.Attributes["Data"] == "" {
				log.Errorw("pubsub inteldata not set")
				msg.Ack()
				break
			}

			tokens := strings.Split(msg.Attributes["Data"], ",")
			if len(tokens) != 2 {
				log.Errorw("pubsub inteldata invalid", "Data", msg.Attributes["Data"])
				msg.Ack()
				break
			}
			gid := model.GoogleID(msg.Attributes["Gid"])
			if err := gid.SetIntelData(tokens[0], tokens[1]); err != nil {
				log.Error(err)
				msg.Nack()
				break
			}
			msg.Ack()
		case "agent":
			log.Debugw("got pubsub agent", "sender", msg.Attributes["Sender"])
			if c.responder {
				// anyone on the responder subscription can Ack it
				log.Errorw("agent response on the request topic", "subsystem", "PubSub", "responder", msg.Attributes["Sender"])
				msg.Ack()
				break
			}
			// if msg.Attributes["RespondingTo"] != c.hostname { msg.Ack() break }
			// log.Debugw("response", "subsystem", "PubSub", "GID", msg.Attributes["Gid"], "responder", msg.Attributes["Sender"], "data", msg.Data)
			// if msg.Attributes["Authoritative"] == "true" { log.Debug("Authoritative") }
			var ad model.Agent
			err := json.Unmarshal(msg.Data, &ad)
			if err != nil {
				log.Error(err)
				msg.Nack()
				break
			}
			err = ad.Save()
			if err != nil {
				log.Error(err)
				msg.Nack()
				break
			}
			msg.Ack()
		default:
			log.Warnw("unknown message", "subsystem", "PubSub", "type", msg.Attributes["Type"], "sender", msg.Attributes["Sender"])
			// get it off the subscription quickly
			msg.Ack()
		}
	}
}

func listenForWasabeeCommands() {
	cmdchan := startup()
	for cmd := range cmdchan {
		switch cmd.Command {
		case "request":
			request(cmd.Param)
		case "location":
			location(cmd.Param, cmd.Data, "")
		case "inteldata":
			inteldata(cmd.Param, cmd.Data, "")
		default:
			log.Warnw("unknown PubSub command", "command", cmd.Command)
		}

	}
	Shutdown()
}

// Shutdown calls the subscription receive cancel function, triggering Start() to return
func Shutdown() {
	cancel()
}

func request(gid string) {
	ctx := context.Background()

	atts := make(map[string]string)
	atts["Type"] = "request"
	atts["Gid"] = gid
	atts["Sender"] = c.hostname

	c.requestTopic.Publish(ctx, &pubsub.Message{
		Attributes: atts,
		Data:       []byte(""),
	})
}

func location(gid, ll, sender string) {
	ctx := context.Background()

	if sender == "" {
		sender = c.hostname
	}

	if ll == "" {
		log.Errorw("PubSub Location announce missing LL")
		return
	}

	atts := make(map[string]string)
	atts["Type"] = "location"
	atts["Gid"] = gid
	atts["Sender"] = sender
	atts["ll"] = ll

	if c.responder {
		c.responseTopic.Publish(ctx, &pubsub.Message{
			Attributes: atts,
		})
	}
	c.requestTopic.Publish(ctx, &pubsub.Message{
		Attributes: atts,
	})
}

func inteldata(gid, data, sender string) {
	ctx := context.Background()

	if sender == "" {
		sender = c.hostname
	}

	log.Debugw("publishing inteldata", "gid", gid, "data", data, "sender", sender)

	atts := make(map[string]string)
	atts["Type"] = "inteldata"
	atts["Gid"] = gid
	atts["Sender"] = sender
	atts["Data"] = data

	if c.responder {
		c.responseTopic.Publish(ctx, &pubsub.Message{
			Attributes: atts,
		})
	}
	c.requestTopic.Publish(ctx, &pubsub.Message{
		Attributes: atts,
	})
}

func respond(g string, sender string) (bool, error) {
	// if the APIs are not running, don't respond
	w := config.Get()
	if !w.V && !w.Rocks {
		return false, nil
	}

	ctx := context.Background()

	gid := model.GoogleID(g)

	// make sure we have the most current info from the APIs
	if _, err := auth.Authorize(gid); err != nil {
		log.Error(err)
		return false, err
	}

	ad, err := gid.GetAgent()
	if err != nil {
		log.Error(err)
		return false, err
	}

	// we do not need to send team/op data across
	// ad.OwnedTeams = nil
	ad.Teams = nil
	ad.Ops = nil
	// ad.Assignments = nil

	d, err := json.Marshal(ad)
	if err != nil {
		log.Error(err)
		return false, err
	}

	atts := make(map[string]string)
	atts["Type"] = "agent"
	atts["Gid"] = g
	atts["Sender"] = c.hostname
	atts["RespondingTo"] = sender
	// if w.V && w.Rocks { atts["Authoritative"] = "true" }

	// log.Debugw("publishing", "subsystem", "PubSub", "GID", ad.GoogleID, "name", ad.IngressName)
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
