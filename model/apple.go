package model

import (
	"fmt"
)

// AppleIDtoGID returns a GoogleID for a given AppleID
// for now this is very dumb and doesn't try to do anything other than ensure the database doesn't throw an error
// if we want to manually map
func AppleIDtoGID(id string) (GoogleID, error) {
	if len(id) > 18 {
		id = id[:18]
	}

	faked := fmt.Sprintf("A-%s", id)

	return GoogleID(faked), nil
}
