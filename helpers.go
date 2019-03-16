package PhDevBin

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ParseExpiration creates a time.Time object from an expiration string, taking the units m, h, d, w into account.
func ParseExpiration(expiration string) (time.Time, error) {
	expiration = strings.ToLower(strings.TrimSpace(expiration))
	if expiration == "volatile" {
		return time.Unix(-1, 0), nil
	}

	var multiplier int64

	if strings.HasSuffix(expiration, "h") {
		expiration = strings.TrimSuffix(expiration, "h")
		multiplier = 60
	} else if strings.HasSuffix(expiration, "d") {
		expiration = strings.TrimSuffix(expiration, "d")
		multiplier = 60 * 24
	} else if strings.HasSuffix(expiration, "w") {
		expiration = strings.TrimSuffix(expiration, "w")
		multiplier = 60 * 24 * 7
	} else {
		expiration = strings.TrimSuffix(expiration, "m")
		multiplier = 1
	}

	value, err := strconv.ParseInt(expiration, 10, 0)
	if err != nil {
		return time.Time{}, err
	}

	if multiplier*value == 0 {
		return time.Time{}, nil
	}

	expirationTime := time.Now().Add(time.Duration(multiplier*value) * time.Minute)

	return expirationTime, nil
}

// EscapeHTML removes all special HTML characters (namely, &<>") in a string and replaces them with their entities (e.g. &amp;).
func EscapeHTML(content string) string {
	content = strings.Replace(content, "&", "&amp;", -1)
	content = strings.Replace(content, "<", "&lt;", -1)
	content = strings.Replace(content, ">", "&gt;", -1)
	content = strings.Replace(content, "\"", "&quot;", -1)
	return content
}

// This does not match all HTML tags, but those created by Prism.js are fine for us.
var htmlTags = regexp.MustCompile(`<[^>]+>`)

// StripHTML strips all HTML tags and replaces the entities from escapeHTML backwards.
func StripHTML(content string) string {
	content = htmlTags.ReplaceAllString(content, "")
	content = strings.Replace(content, "&quot;", "\"", -1)
	content = strings.Replace(content, "&gt;", ">", -1)
	content = strings.Replace(content, "&lt;", "<", -1)
	content = strings.Replace(content, "&amp;", "&", -1)
	return content
}

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

/* getLanguages reads the existing languages from the prism-server for use with SyntaxExists.
func Slice2map(s []string) map[string]bool {
	r := map[string]bool{}
	for i := 0; i < len(s); i++ {
		r[s[i]] = true
	}
	return r
} */
