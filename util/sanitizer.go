package util

import (
	"strings"

	"github.com/microcosm-cc/bluemonday"
)

var sanitizer *bluemonday.Policy

func init() {
	sanitizer = bluemonday.StrictPolicy()
}

func Sanitize(in string) string {
	return strings.TrimSpace(sanitizer.Sanitize(in))
}
