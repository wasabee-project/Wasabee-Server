package wasabi

import (
	"time"
)

// try runs a method up to howOften times until there's no error anymore, always waiting a second before trying again.
func try(what func() (interface{}, error), howOften int, sleepTime time.Duration) (interface{}, error) {
	var err error
	var result interface{}
	for i := 0; i < howOften; i++ {
		result, err = what()
		if err == nil {
			break
		} else {
			time.Sleep(sleepTime)
		}
	}
	if err != nil {
		return nil, err
	}
	return result, nil
}
