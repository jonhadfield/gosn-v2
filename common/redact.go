package common

import (
	"strings"
)

// redactedPlaceholder replaces a credential value in debug output.
const redactedPlaceholder = "[REDACTED]"

// sensitiveHeaders are headers whose values carry credentials and must never
// be written to a log in full.
var sensitiveHeaders = map[string]bool{
	"authorization":       true,
	"cookie":              true,
	"set-cookie":          true,
	"proxy-authorization": true,
	"x-auth-token":        true,
}

// RedactCookieHeader takes a Cookie or Set-Cookie header value and replaces
// each cookie's value with a placeholder, keeping the cookie name and any
// attributes. Debug output stays useful for diagnosing domain, path and flag
// problems without exposing the token itself.
//
// "access_token_abc=secret; HttpOnly; Path=/" becomes
// "access_token_abc=[REDACTED]; HttpOnly; Path=/".
func RedactCookieHeader(header string) string {
	if header == "" {
		return ""
	}

	parts := strings.Split(header, ";")

	// Only the first part is the cookie's name=value pair; the rest are
	// attributes, which are safe to keep and useful to see.
	nameValue := strings.TrimSpace(parts[0])
	if nameValue != "" {
		if name, _, found := strings.Cut(nameValue, "="); found {
			parts[0] = strings.TrimSpace(name) + "=" + redactedPlaceholder
		} else {
			// No "=" at all, so there is no value to leak; leave it be.
			parts[0] = nameValue
		}
	}

	for i := 1; i < len(parts); i++ {
		parts[i] = strings.TrimSpace(parts[i])
	}

	return strings.Join(parts, "; ")
}

// RedactHeaderValue returns a header value safe to write to a log. Values of
// headers that carry credentials are replaced wholesale, except cookie headers,
// which keep their names and attributes via RedactCookieHeader.
func RedactHeaderValue(name, value string) string {
	switch {
	case !sensitiveHeaders[strings.ToLower(strings.TrimSpace(name))]:
		return value
	case strings.EqualFold(strings.TrimSpace(name), "cookie"),
		strings.EqualFold(strings.TrimSpace(name), "set-cookie"):
		return RedactCookieHeader(value)
	default:
		return redactedPlaceholder
	}
}
