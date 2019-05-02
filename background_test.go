package wasabi_test

import (
	"github.com/cloudkucooland/WASABI"
	"os"
	"os/signal"
	"syscall"
	"testing"
	"time"
)

func TestBackground(t *testing.T) {
	sigch := make(chan os.Signal, 3)
	signal.Notify(sigch, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGTERM, syscall.SIGHUP, os.Interrupt)

	// start reaper thread
	wasabi.Log.Info("Starting background thread")
	go wasabi.BackgroundTasks(sigch)

	// sleep a bit, then signal to stop
	wasabi.Log.Info("Sleeping 5 seconds")
	time.Sleep(5 * time.Second)
	wasabi.Log.Info("Sending interrupt signal")
	sigch <- syscall.SIGINT
}
