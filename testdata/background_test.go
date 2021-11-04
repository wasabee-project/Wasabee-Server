package wasabee_test

import (
	"github.com/wasabee-project/Wasabee-Server"
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
	wasabee.Log.Info("Starting background thread")
	go wasabee.BackgroundTasks(sigch)

	// sleep a bit, then signal to stop
	wasabee.Log.Info("Sleeping 5 seconds")
	time.Sleep(5 * time.Second)
	wasabee.Log.Info("Sending interrupt signal")
	sigch <- syscall.SIGINT
}
