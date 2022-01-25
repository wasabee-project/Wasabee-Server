package util

import (
	"strings"

	// "github.com/microcosm-cc/bluemonday"

	"github.com/wasabee-project/Wasabee-Server/log"
)

/*
var sanitizer *bluemonday.Policy

func init() {
	sanitizer = bluemonday.StrictPolicy()
}
*/

// Sanitize is a very light-weight attempt to reduce XSS potential in well-behaved clients
func Sanitize(in string) string {
	out := strings.Replace(in, "<", "&lt;", -1)
	out = strings.Replace(out, ">", "&gt;", -1)
	out = strings.TrimSpace(out)
	if in != out {
		log.Debugw("sanitize", "in", in, "out", out)
	}
	return out
}
