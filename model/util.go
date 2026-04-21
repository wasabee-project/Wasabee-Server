package model

import (
	"context"
	"errors"

	"github.com/wasabee-project/Wasabee-Server/log"
	"github.com/wasabee-project/Wasabee-Server/util"
)

// GenerateSafeName generates a slug that doesn't exist in the database yet.
// It checks across multiple tables to ensure global uniqueness within the app.
func GenerateSafeName(ctx context.Context) (string, error) {
	name := ""
	exists := true

	// Loop until we find a name that doesn't exist in any sensitive column
	for exists {
		name = util.GenerateName()
		if name == "" {
			return "", errors.New(ErrNameGenFailed)
		}

		var count, total int
		// Check OTT
		err := db.QueryRowContext(ctx, "SELECT COUNT(OneTimeToken) FROM agent WHERE OneTimeToken = ?", name).Scan(&count)
		if err != nil {
			return "", err
		}
		total = count

		// Check teamID
		err = db.QueryRowContext(ctx, "SELECT COUNT(teamID) FROM team WHERE teamID = ?", name).Scan(&count)
		if err != nil {
			return "", err
		}
		total += count

		// Check joinLinkToken
		err = db.QueryRowContext(ctx, "SELECT COUNT(joinLinkToken) FROM team WHERE joinLinkToken = ?", name).Scan(&count)
		if err != nil {
			return "", err
		}
		total += count

		if total == 0 {
			exists = false
		}
	}

	return name, nil
}

// LocationClean is called from the background process to remove out-dated agent locations (privacy)
// It resets locations to (0,0) if they haven't been updated in 3 hours.
func LocationClean(ctx context.Context) {
	rows, err := db.QueryContext(ctx, "SELECT gid FROM locations WHERE NOT ST_Equals(loc, Point(0, 0)) AND upTime < DATE_SUB(UTC_TIMESTAMP(), INTERVAL 3 HOUR)")
	if err != nil {
		log.Error(err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var gid GoogleID
		if err = rows.Scan(&gid); err != nil {
			log.Error(err)
			continue
		}
		// Reset to "Null Island"
		if _, err = db.ExecContext(ctx, "UPDATE locations SET loc = Point(0, 0), upTime = UTC_TIMESTAMP() WHERE gid = ?", gid); err != nil {
			log.Error(err)
			continue
		}
	}
}
