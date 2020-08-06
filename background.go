package wasabee

import (
	"os"
	"time"
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
			locationClean()
		}
	}
}

func locationClean() {
	r, err := db.Query("SELECT gid FROM locations WHERE loc != POINTFROMTEXT(?) AND upTime < DATE_SUB(UTC_TIMESTAMP(), INTERVAL 3 HOUR)", "POINT(0 0)")
	if err != nil {
		Log.Error(err)
		return
	}
	defer r.Close()

	var gid GoogleID
	for r.Next() {
		err = r.Scan(&gid)
		if err != nil {
			Log.Error(err)
			continue
		}
		_, err = db.Exec("UPDATE locations SET loc = POINTFROMTEXT(?), upTime = UTC_TIMESTAMP() WHERE gid = ?", "POINT(0 0)", gid)
		if err != nil {
			Log.Error(err)
			continue
		}
	}
}
