package PhDevBin

import (
	"time"
)

// BackgroundTasks runs the database cleaning tasks such as expiring waypoints and stale user locations
func BackgroundTasks() {
	locationClean()
	waypointClean()
	simpleDocClean()

	time.Sleep(3600 * time.Second)
	Log.Debug("Running Background Tasks")
}

func locationClean() {
	r, err := db.Query("SELECT gid FROM locations WHERE loc != POINTFROMTEXT(?) AND upTime < DATE_SUB(NOW(), INTERVAL 3 HOUR)", "POINT(0 0)")
	if err != nil {
		Log.Error(err)
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
		}
		err = gid.ownTracksExternalUpdate("0", "-180.1", "reaper") // invalid values pop the user off the map
		if err != nil {
			Log.Error(err)
		}
	}
	return
}

func waypointClean() {
	// give the clients 3 days to receive the invalid ones and remove them from the list
	_, err := db.Exec("DELETE FROM waypoints WHERE expiration < DATE_SUB(NOW(), INTERVAL 3 DAY)")
	if err != nil {
		Log.Error(err)
	}

	// Invalidate expired ones
	_, err = db.Exec("UPDATE waypoints SET loc = POINTFROMTEXT(?) WHERE expiration < NOW() AND X(loc) != -180.1", "POINT(-180.1 0)")
	if err != nil {
		Log.Error(err)
	}

	return
}
