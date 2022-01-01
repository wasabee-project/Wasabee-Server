package background

import (
	"context"
	"time"

	"github.com/wasabee-project/Wasabee-Server/log"
	"github.com/wasabee-project/Wasabee-Server/model"
)

// Start runs the database cleaning tasks such as expiring stale user locations
func Start(ctx context.Context) {
	log.Infow("startup", "message", "running initial background tasks")
	model.LocationClean()

	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Infow("shutdown", "message", "background tasks shutting down")
			return
		case <-ticker.C:
			model.LocationClean()
		}
	}
}
