package wasabihttps

import (
	"math"
	"strconv"
	"time"
)

// formatTime is used only in legacy simple route
func formatTime(t time.Time, relative bool) string {
	if relative {
		if (t == time.Time{}) {
			return "never"
		}

		seconds := int(math.Floor(time.Now().Sub(t).Seconds()))
		context := "ago"
		if seconds <= 0 {
			context = "left"
			seconds = -seconds
		}
		if seconds >= 2630016*12 {
			v := seconds / (2630016 * 12)
			return strconv.Itoa(v) + " year" + iif(v == 1, "", "s").(string) + " " + context
		} else if seconds >= 2630016 {
			v := seconds / (2630016) // An average month in any year (including leap years)
			return strconv.Itoa(v) + " month" + iif(v == 1, "", "s").(string) + " " + context
		} else if seconds >= 60*60*24 {
			v := seconds / (60 * 60 * 24)
			return strconv.Itoa(v) + " day" + iif(v == 1, "", "s").(string) + " " + context
		} else if seconds >= 60*60 {
			v := seconds / (60 * 60)
			return strconv.Itoa(v) + " hour" + iif(v == 1, "", "s").(string) + " " + context
		} else if seconds >= 60 {
			v := seconds / (60)
			return strconv.Itoa(v) + " minute" + iif(v == 1, "", "s").(string) + " " + context
		}
		return "a few seconds " + context
	}
	if (t == time.Time{}) {
		return ""
	}
	return t.UTC().Format("2006-01-02 15:04 (UTC)")
}

func iif(cond bool, onTrue interface{}, onFalse interface{}) interface{} {
	if cond {
		return onTrue
	}
	return onFalse
}
