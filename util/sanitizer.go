package util

import (
	"strings"

	"github.com/microcosm-cc/bluemonday"

	"github.com/wasabee-project/Wasabee-Server/log"
)

var sanitizer *bluemonday.Policy

func init() {
	sanitizer = bluemonday.StrictPolicy()
}

// Sanitize removes all HTML/CSS from an input string, making it safe for re-display in an HTML/JS context
func Sanitize(in string) string {
	out := strings.TrimSpace(sanitizer.Sanitize(in))
	if in != out {
		log.Debugw("sanitize", "in", in, "out", out)
	}
	return out
}
