package risc

import (
	"bytes"
	"context"
	"net/http"

	"github.com/wasabee-project/Wasabee-Server/log"
)

func registerWebhook(ctx context.Context) {
	// If we are already running or don't have auth data, skip
	if running || len(authdata) == 0 {
		return
	}

	// This is the "Add Subject" call to Google to start the stream
	// We use the Discovery endpoint we found earlier
	apiurl := googleConfig.AddEndpoint

	// Google RISC usually expects a Bearer token or a specific Service Account JWT
	// in the authdata we loaded in Start()
	req, err := http.NewRequestWithContext(ctx, "POST", apiurl, bytes.NewBuffer(authdata))
	if err != nil {
		log.Error(err)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Errorw("failed to register RISC webhook", "error", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		log.Errorw("Google rejected RISC registration", "status", resp.Status)
		return
	}

	running = true
	log.Infow("RISC webhook registered with Google")

	// Wait for shutdown signal
	<-ctx.Done()
	disableWebhook(context.Background()) // Use a fresh background context for cleanup
}

func disableWebhook(ctx context.Context) {
	if !running {
		return
	}

	// Standard practice: tell Google to stop sending events before we die
	req, err := http.NewRequestWithContext(ctx, "POST", googleConfig.RemEndpoint, bytes.NewBuffer(authdata))
	if err != nil {
		log.Error(err)
		return
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Error(err)
		return
	}
	defer resp.Body.Close()

	running = false
	close(riscchan)
	log.Info("RISC webhook disabled")
}
