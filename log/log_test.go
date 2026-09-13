package log

import (
	"bytes"
	stdlog "log"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func captureDebugPrint(t *testing.T, msg string, maxChars int) string {
	t.Helper()

	var buf bytes.Buffer

	out, flags := stdlog.Writer(), stdlog.Flags()
	stdlog.SetOutput(&buf)
	stdlog.SetFlags(0)

	t.Cleanup(func() {
		stdlog.SetOutput(out)
		stdlog.SetFlags(flags)
	})

	DebugPrint(true, msg, maxChars)

	return buf.String()
}

// TestDebugPrintTruncatedURL guards against the regression that put part of a private server
// address into public CI logs: the second URL in a sign-in error was cut by truncation, leaving
// a fragment that GitHub's secret masking did not recognise. The address below is a documentation
// address (RFC 5737) in the same shape as the real one.
func TestDebugPrintTruncatedURL(t *testing.T) {
	const host = "192.0.2.10"

	msg := `getAuthParams error: POST http://` + host + `:3000/v2/login-params giving up after 6 attempt(s): ` +
		`Post "http://` + host + `:3000/v2/login-params": context deadline exceeded`

	for maxChars := 1; maxChars <= len(msg); maxChars++ {
		got := captureDebugPrint(t, msg, maxChars)

		require.NotContains(t, got, "192.0", "maxChars %d leaked part of the host: %s", maxChars, got)
	}

	got := captureDebugPrint(t, msg, len(msg))
	require.Contains(t, got, "http://[REDACTED]/v2/login-params")
}

func TestDebugPrintHidden(t *testing.T) {
	var buf bytes.Buffer

	out := stdlog.Writer()
	stdlog.SetOutput(&buf)
	t.Cleanup(func() { stdlog.SetOutput(out) })

	DebugPrint(false, "http://192.0.2.10/v2/login", 120)

	require.Empty(t, strings.TrimSpace(buf.String()))
}
