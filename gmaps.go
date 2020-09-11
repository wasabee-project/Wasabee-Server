package wasabee

import (
	// "context"
	"googlemaps.github.io/maps"
	// "google.golang.org/api/option"
)

var gMaps struct {
	client *maps.Client
}

func InitGMaps(creds string) error {
	// opts := option.WithCredentialsFile(creds)

	c, err := maps.NewClient(maps.WithRateLimit(1))
	if err != nil {
		Log.Error(err)
		return err
	}
	gMaps.client = c
	return nil
}
