package model

import (
	"fmt"

	"github.com/wasabee-project/Wasabee-Server/generatename"
	"github.com/wasabee-project/Wasabee-Server/log"
)

// GenerateSafeName generates a slug (like GenerateName()) that doesn't exist in the database yet.
func GenerateSafeName() (string, error) {
	name := ""
	rows := 1

	for rows > 0 {
		var i, total int
		name = generatename.GenerateName()
		if name == "" {
			err := fmt.Errorf("name generation failed")
			return "", err
		}
		err := db.QueryRow("SELECT COUNT(OneTimeToken) FROM agent WHERE OneTimeToken = ?", name).Scan(&i)
		if err != nil {
			return "", err
		}
		total = i
		err = db.QueryRow("SELECT COUNT(teamID) FROM team WHERE teamID = ?", name).Scan(&i)
		if err != nil {
			return "", err
		}
		total += i
		err = db.QueryRow("SELECT COUNT(joinLinkToken) FROM team WHERE joinLinkToken = ?", name).Scan(&i)
		if err != nil {
			return "", err
		}
		total += i
		rows = total
	}

	return name, nil
}

// called from the background process
func LocationClean() {
	r, err := db.Query("SELECT gid FROM locations WHERE loc != POINTFROMTEXT(?) AND upTime < DATE_SUB(UTC_TIMESTAMP(), INTERVAL 3 HOUR)", "POINT(0 0)")
	if err != nil {
		log.Error(err)
		return
	}
	defer r.Close()

	var gid GoogleID
	for r.Next() {
		err = r.Scan(&gid)
		if err != nil {
			log.Error(err)
			continue
		}
		_, err = db.Exec("UPDATE locations SET loc = POINTFROMTEXT(?), upTime = UTC_TIMESTAMP() WHERE gid = ?", "POINT(0 0)", gid)
		if err != nil {
			log.Error(err)
			continue
		}
	}
}
