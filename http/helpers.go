package PhDevHTTP

import (
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/cloudkucooland/PhDevBin"
)

// replaceGlobal replaces all global frontend variables with their config value.
func replaceGlobal(content *string) {
	replaceVariable(content, "path", config.path)
	replaceVariable(content, "root", config.Root)
}

// replaceVariable replaces a single frontend variable with its value.
// For example, replaceVariable(x, "id", "12345") replaces "$$id$$" with "12345".
func replaceVariable(content *string, name string, value string) {
	*content = strings.Replace(*content, "$$"+name+"$$", value, -1)
}

// blockVariableExpressionCache contains regular expressions for all block variables to improve rendering speed
var blockVariableExpressionCache = map[string]*regexp.Regexp{}

// replaceBlockVariable removes if-style blocks if they're not matching the value, and cleans the variable remainders otherwise.
func replaceBlockVariable(content *string, name string, value bool) {
	// We will later replace every block that is only shown for the opposite value -> not seems to be inverted
	not := "!"
	if !value {
		not = ""
	}

	// Try loading regular expression from cache
	expression, exists := blockVariableExpressionCache[not+name]
	if exists == false {
		// Compile regular expression to match blocks for the opposite value
		expression = regexp.MustCompile(`\$\$` + not + name + `\$\$(?U:(?:.|\n)*\$\$/` + name + `\$\$)`)
		blockVariableExpressionCache[not+name] = expression
	}

	// Replace blocks for the opposite value
	*content = expression.ReplaceAllString(*content, "")

	// Replace the variables only (not including the block) if the value matches
	if value {
		replaceVariable(content, name, "")
	} else {
		replaceVariable(content, "!"+name, "")
	}
	replaceVariable(content, "/"+name, "")
}

func replaceDocumentVariables(content *string, doc *PhDevBin.Document) {
	replaceVariable(content, "id", doc.ID)

	replaceVariable(content, "creation", formatTime(doc.Upload, false))
	replaceVariable(content, "expiration", formatTime(doc.Expiration, false))
	replaceVariable(content, "expiration-remaining", formatTime(doc.Expiration, true))

	replaceVariable(content, "views", strconv.Itoa(doc.Views))

	if (doc.Expiration != time.Time{}) { // Don't store forever?
		replaceBlockVariable(content, "if_volatile", doc.Expiration.Before(time.Unix(0, 1)))
	} else {
		replaceBlockVariable(content, "if_volatile", false)
	}
}

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
