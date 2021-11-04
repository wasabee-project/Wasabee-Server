package background

import (
	"os"
	"time"

	"github.com/wasabee-project/Wasabee-Server/log"
	"github.com/wasabee-project/Wasabee-Server/model"
)

// BackgroundTasks runs the database cleaning tasks such as expiring stale user locations
func BackgroundTasks(c chan os.Signal) {
	Log.Infow("startup", "message", "running initial background tasks")
	locationClean()

	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()

	for {
		select {
		case x := <-c:
			Log.Infow("shutdown", "message", "background tasks shutting down", "signal", x)
			return
		case <-ticker.C:
			model.LocationClean()
		}
	}
}
