package wasabeepubsub

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"cloud.google.com/go/pubsub"
	"github.com/wasabee-project/Wasabee-Server"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

var topic *pubsub.Topic
var cancel context.CancelFunc

type Configuration struct {
	Cert         string
	Topic        string
	Project      string
	Subscription string
}

// StartPubSub is the main startup function for the PubSub subsystem
func StartPubSub(config Configuration) error {
	wasabee.Log.Debugf("starting PubSub: %s %s %s", config.Cert, config.Project, config.Topic)
	hostname, _ := os.Hostname()

	// var err error
	// var client *pubsub.Client
	var cctx context.Context

	ctx := context.Background()
	cctx, cancel = context.WithCancel(ctx)

	client, err := pubsub.NewClient(ctx, config.Project, option.WithCredentialsFile(config.Cert))
	if err != nil {
		err := fmt.Errorf("error initializing pubsub: %v", err)
		wasabee.Log.Error(err)
		return err
	}
	defer client.Close()

	topics := client.Topics(ctx)
	for {
		t, err := topics.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			wasabee.Log.Error(err)
			break
		}
		wasabee.Log.Debugf("Found pubsub topic: %s", t)
	}

	topic = client.Topic(config.Topic)
	ok, err := topic.Exists(ctx)
	if err != nil {
		wasabee.Log.Error(err)
		return err
	}
	if !ok {
		wasabee.Log.Noticef("Creating topic: %s", config.Topic)
		topic, err = client.CreateTopic(ctx, config.Topic)
		if err != nil {
			wasabee.Log.Error(err)
		}
	} else {
		wasabee.Log.Debugf("Using existing topic: %s", config.Topic)
	}
	defer topic.Stop()
	defer removeTopicIfNoSubscriptions()

	if config.Subscription == "" {
		config.Subscription = fmt.Sprintf("%s-%s", config.Topic, hostname)
	}

	mainsub := client.Subscription(config.Subscription)
	ok, err = mainsub.Exists(context.Background())
	if err != nil {
		wasabee.Log.Error(err)
		return err
	}
	if !ok {
		wasabee.Log.Debugf("Creating subscription: %s", config.Subscription)
		// if I do not have the ENL APIs enabled, only listen for responses to me
		var filter string
		if !wasabee.GetvEnlOne() && !wasabee.GetEnlRocks() {
			filter = fmt.Sprintf("attributes:\"RespondingTo\" AND attributes.RespondingTo = \"%s\"", hostname)
			wasabee.Log.Debugf("only listening to responses to my requests: %s [not active yet]", filter)
		}

		duration, _ := time.ParseDuration("11m")
		mainsub, err = client.CreateSubscription(context.Background(), config.Subscription, pubsub.SubscriptionConfig{
			Topic:               topic,
			RetainAckedMessages: false,
			RetentionDuration:   duration,
			// Filter:              filter, // this is an alpha API and might not work
		})
		if err != nil {
			wasabee.Log.Error(err)
			return err
		}
	} else {
		wasabee.Log.Infof("found subscription: %s (lingering from unclean shutdown?)", config.Subscription)
	}
	mainsub.ReceiveSettings.Synchronous = false
	mainsub.ReceiveSettings.NumGoroutines = 2
	defer mainsub.Delete(context.Background())

	mainchan := make(chan *pubsub.Message)
	defer close(mainchan)

	// spin up listener to listen for messages on the main channel
	go listenForPubSubMessages(mainchan)

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
		wasabee.Log.Errorf("Receive: %v", err)
		return err
	}
	wasabee.Log.Notice("PubSub exiting")

	return nil
}

func listenForPubSubMessages(mainchan chan *pubsub.Message) {
	hostname, _ := os.Hostname()

	for msg := range mainchan {
		if msg.Attributes["Sender"] == hostname {
			// wasabee.Log.Debug("ignoring message from me")
			continue
		}
		switch msg.Attributes["Type"] {
		case "hearbeat":
			wasabee.Log.Debug("[%s] sent heartbeat", msg.Attributes["Sender"])
			msg.Ack()
			break
		case "request":
			wasabee.Log.Debugf("[%s] requesing [%s]", msg.Attributes["Sender"], msg.Attributes["Gid"])
			respond(msg.Attributes["Gid"], msg.Attributes["Sender"])
			msg.Ack()
			break
		case "agent":
			if msg.Attributes["RespondingTo"] != hostname {
				wasabee.Log.Debug("ignoring response not intended for me")
				break
			}
			wasabee.Log.Debugf("response for %s", msg.Attributes["Gid"])
			var ad wasabee.AgentData
			err := json.Unmarshal(msg.Data, &ad)
			if err != nil {
				wasabee.Log.Error(err)
				continue
			}
			err = ad.Save()
			if err != nil {
				wasabee.Log.Error(err)
				continue
			}
			msg.Ack()
			break
		default:
			wasabee.Log.Debugf("unknown message type %s", msg.Attributes["Type"])
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

func respond(g string, sender string) error {
	// if the APIs are not running, don't respond
	if !wasabee.GetvEnlOne() && !wasabee.GetEnlRocks() {
		return nil
	}

	ctx := context.Background()
	hostname, _ := os.Hostname()
	wasabee.Log.Debug(hostname)

	gid := wasabee.GoogleID(g)
	wasabee.Log.Debug(gid.String)

	// make sure we have the most current info from the APIs
	_, err := gid.InitAgent()
	if err != nil {
		wasabee.Log.Error(err)
		return err
	}

	var ad wasabee.AgentData
	err = gid.GetAgentData(&ad)
	if err != nil {
		wasabee.Log.Error(err)
		return err
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

	wasabee.Log.Debug("work done, publishing...")
	topic.Publish(ctx, &pubsub.Message{
		Attributes: atts,
		Data:       d,
	})
	return nil
}

func heartbeats() {
	ticker := time.NewTicker(120 * time.Second)
	defer ticker.Stop()

	var atts map[string]string
	atts = make(map[string]string)
	atts["Type"] = "heartbeat"
	atts["Sender"], _ = os.Hostname()

	// loop, sending a ping every 120 seconds
	for {
		t := <-ticker.C
		atts["Time"] = t.String()

		topic.Publish(context.Background(), &pubsub.Message{
			Attributes: atts,
		})
	}
}

func removeTopicIfNoSubscriptions() {
	i := 0
	for subs := topic.Subscriptions(context.Background()); ; {
		sub, err := subs.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			wasabee.Log.Error(err)
		}
		wasabee.Log.Debugf("remaining subscription: %s", sub)
		i++
	}
	if i == 0 {
		wasabee.Log.Debug("no remaining subscriptions, shutting down topic")
		err := topic.Delete(context.Background())
		if err != nil {
			wasabee.Log.Error(err)
		}
	}
}
