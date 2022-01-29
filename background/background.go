package background

import (
	"context"
	"time"

	"github.com/wasabee-project/Wasabee-Server/Firebase"
	"github.com/wasabee-project/Wasabee-Server/config"
	"github.com/wasabee-project/Wasabee-Server/log"
	"github.com/wasabee-project/Wasabee-Server/model"
)

// Start runs the database cleaning tasks such as expiring stale user locations
func Start(ctx context.Context) {
	log.Infow("startup", "message", "running initial background tasks")
	model.LocationClean()

	hourly := time.NewTicker(time.Hour)
	defer hourly.Stop()

	weekly := time.NewTicker(time.Hour * 24 * 7)
	defer weekly.Stop()

	if config.IsFirebaseRunning() {
		wfb.Resubscribe() // prevent a crash if background starts before firebase
	}

	for {
		select {
		case <-ctx.Done():
			log.Infow("shutdown", "message", "background tasks shutting down")
			return
		case <-hourly.C:
			model.LocationClean()
		case <-weekly.C:
			wfb.Resubscribe()
		}
	}
}
