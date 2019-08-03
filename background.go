package wasabee

import (
	"os"
	"time"
)

// BackgroundTasks runs the database cleaning tasks such as expiring stale user locations
func BackgroundTasks(c chan os.Signal) {
	Log.Debug("running initial tasks")
	locationClean()
	simpleDocClean()

	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()

	for {
		select {
		case x := <-c:
			Log.Debugf("signal received: %s", x)
			return
		case <-ticker.C:
			locationClean()
			simpleDocClean()
		}
	}
}

func locationClean() {
	r, err := db.Query("SELECT gid FROM locations WHERE loc != POINTFROMTEXT(?) AND upTime < DATE_SUB(NOW(), INTERVAL 3 HOUR)", "POINT(0 0)")
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
		_, err = db.Exec("UPDATE locations SET loc = POINTFROMTEXT(?), upTime = NOW() WHERE gid = ?", "POINT(0 0)", gid)
		if err != nil {
			Log.Error(err)
			continue
		}
	}
}
