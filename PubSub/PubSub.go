package wasabeepubsub

import (
	"context"
	"fmt"
	"time"
	"os"

	"cloud.google.com/go/pubsub"
	"github.com/wasabee-project/Wasabee-Server"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

var topic *pubsub.Topic
var cancel context.CancelFunc

type Configuration struct {
	Cert	string
	Topic	string
	Project	string
	Subscription string
}

// StartPubSub is the main startup function for the PubSub subsystem
func StartPubSub(config Configuration) error {
	wasabee.Log.Debugf("starting PubSub: %s %s %s", config.Cert, config.Project, config.Topic)

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
		hostname, _ := os.Hostname()
		config.Subscription = fmt.Sprintf("%s-%s", config.Topic, hostname);
	}

	mainsub := client.Subscription(config.Subscription)
	ok, err = mainsub.Exists(context.Background())
	if err != nil {
		wasabee.Log.Error(err)
		return err
	}
	if !ok {
		wasabee.Log.Debugf("Creating subscription: %s", config.Subscription)

		duration, _ := time.ParseDuration("11m")
		mainsub, err = client.CreateSubscription(context.Background(), config.Subscription, pubsub.SubscriptionConfig{
			Topic: topic,
			RetainAckedMessages: false,
			RetentionDuration: duration,
		})
		if err != nil {
			wasabee.Log.Error(err)
			return err
		}
	} else {
		wasabee.Log.Infof("found subscription: %s - lingering from unclean shutdown?", config.Subscription)
	}
	mainsub.ReceiveSettings.Synchronous = false
	mainsub.ReceiveSettings.NumGoroutines = 2
	defer mainsub.Delete(context.Background())

	mainchan := make(chan *pubsub.Message)
	defer close(mainchan)

	// spin up listener to listen for messages on the main channel
	go func() {
		for msg := range mainchan {
			wasabee.Log.Debugf("Got pubsub message on topic %s: %q\n", config.Topic, string(msg.Data))
			msg.Ack()
		}
	}()

	// spin up listener for commands from wasabee
	go func () {
		cmdchan := wasabee.PubSubInit()
		for cmd := range cmdchan {
			wasabee.Log.Debugf("PubSub cmd from wasabee %s", cmd);
		}
		Shutdown()
	}()

	// Receive blocks until the context is cancelled or an error occurs.
	err = mainsub.Receive(cctx, func(ctx context.Context, msg *pubsub.Message) {
		// push any messages received into the main channel
		mainchan <- msg
	})
	if err != nil {
		wasabee.Log.Errorf("Receive: %v", err)
		return err
	}

	return nil
}

// shutdown calls the subscription receive cancel function, triggering Start() to return
func Shutdown() error {
	cancel()
	return nil
}

func Send() error {
	ctx := context.Background()

	var results []*pubsub.PublishResult
	r := topic.Publish(ctx, &pubsub.Message{
		Data: []byte("hello world"),
	})
	results = append(results, r)
	for _, r := range results {
		id, err := r.Get(ctx)
		if err != nil {
			wasabee.Log.Notice(err)
			//break
		}
		wasabee.Log.Debugf("Published a message with a message ID: %s\n", id)
	}

	return nil
}

func removeTopicIfNoSubscriptions() {
	i := 0
	for subs := topic.Subscriptions(context.Background()) ; ;  {
		sub, err := subs.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			wasabee.Log.Error(err)
		}
		wasabee.Log.Debugf("Remaining subscription: %s", sub)
		i++
	}
	// 1 because our own subscription might be still around 
	if i <= 1  {
		err := topic.Delete(context.Background())
		if err != nil {
			wasabee.Log.Error(err)
		}
	}
}
