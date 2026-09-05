package common

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRedactCookieHeader(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name   string
		header string
		want   string
	}{
		{
			name:   "empty header",
			header: "",
			want:   "",
		},
		{
			name:   "name and value only",
			header: "access_token_abc=supersecret",
			want:   "access_token_abc=[REDACTED]",
		},
		{
			name:   "attributes are preserved",
			header: "access_token_abc=supersecret; HttpOnly; Path=/; Secure",
			want:   "access_token_abc=[REDACTED]; HttpOnly; Path=/; Secure",
		},
		{
			name:   "attribute containing equals is preserved",
			header: "refresh_token_abc=secret; Domain=standardnotes.com; SameSite=None",
			want:   "refresh_token_abc=[REDACTED]; Domain=standardnotes.com; SameSite=None",
		},
		{
			name:   "value containing equals is fully redacted",
			header: "token=abc==padding; Path=/",
			want:   "token=[REDACTED]; Path=/",
		},
		{
			name:   "no equals sign means no value to leak",
			header: "justaflag; HttpOnly",
			want:   "justaflag; HttpOnly",
		},
		{
			name:   "empty value",
			header: "access_token_abc=; Path=/",
			want:   "access_token_abc=[REDACTED]; Path=/",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, RedactCookieHeader(tc.header))
		})
	}
}

// TestRedactCookieHeaderLeakedExample guards against the regression that put
// live session tokens into public CI logs: a full Set-Cookie header from the
// sign-in response was written to the debug log verbatim. The token and UUID
// below are synthetic, but match the real ones in shape and length so the
// name/value boundary is exercised as it occurs in practice.
func TestRedactCookieHeaderLeakedExample(t *testing.T) {
	t.Parallel()

	const secret = "not-a-real-token" // ggignore
	header := "access_token_00000000-0000-4000-8000-000000000000=" + secret +
		"; HttpOnly;Secure; Path=/;Partitioned; SameSite=None; Domain=standardnotes.com"

	got := RedactCookieHeader(header)

	require.NotContains(t, got, secret, "the session token must never survive redaction")
	require.Contains(t, got, "access_token_00000000-0000-4000-8000-000000000000", "the cookie name is useful and should survive")
	require.Contains(t, got, "Domain=standardnotes.com", "attributes are useful and should survive")
	require.Contains(t, got, redactedPlaceholder)
}

func TestRedactHeaderValue(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name        string
		headerName  string
		headerValue string
		want        string
	}{
		{
			name:        "non-sensitive header passes through",
			headerName:  "Content-Type",
			headerValue: "application/json",
			want:        "application/json",
		},
		{
			name:        "authorization is replaced wholesale",
			headerName:  "Authorization",
			headerValue: "Bearer sometoken",
			want:        redactedPlaceholder,
		},
		{
			name:        "matching is case insensitive",
			headerName:  "AUTHORIZATION",
			headerValue: "Bearer sometoken",
			want:        redactedPlaceholder,
		},
		{
			name:        "set-cookie keeps name and attributes",
			headerName:  "Set-Cookie",
			headerValue: "access_token_abc=secret; Path=/",
			want:        "access_token_abc=[REDACTED]; Path=/",
		},
		{
			name:        "cookie keeps name and attributes",
			headerName:  "Cookie",
			headerValue: "refresh_token_abc=secret",
			want:        "refresh_token_abc=[REDACTED]",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, RedactHeaderValue(tc.headerName, tc.headerValue))
		})
	}
}

// TestRedactHeaderValueCoversSensitiveHeaders asserts that no header listed as
// sensitive can return its value unchanged.
func TestRedactHeaderValueCoversSensitiveHeaders(t *testing.T) {
	t.Parallel()

	const secret = "not-a-real-value" // ggignore

	for header := range sensitiveHeaders {
		got := RedactHeaderValue(header, "name="+secret)
		require.NotContains(t, got, secret, "header %q leaked its value", header)
		require.True(t, strings.Contains(got, redactedPlaceholder), "header %q was not redacted", header)
	}
}
