package PhDevBin

import (
	"strconv"
)

func OwnTracksUpdate(lockey, jsonblob string, lat, lon float64) error {
    gid, err := LockeyToGid(lockey)
	if err != nil {
        Log.Notice(err)
		return err
	}
    _, err = db.Exec("UPDATE otdata SET otdata = ? WHERE gid = ?", jsonblob, gid)
	if err != nil {
	    Log.Notice(err)
	}
	err = UserLocation(gid, strconv.FormatFloat(lat, 'f', -1, 64), strconv.FormatFloat(lon, 'f', -1, 64))
	return err
}
