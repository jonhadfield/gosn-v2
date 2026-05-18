package common

import (
	"net/http"
	"testing"
)

func TestSetStandardClientHeaders(t *testing.T) {
	h := http.Header{}
	SetStandardClientHeaders(h)

	tests := map[string]string{
		"User-Agent":            SNUserAgent,
		"X-SNJS-Version":        SNJSVersion,
		"X-Application-Version": SNAppVersion,
	}

	for header, expected := range tests {
		if got := h.Get(header); got != expected {
			t.Errorf("%s: got %q, want %q", header, got, expected)
		}
	}
}

func TestSetStandardClientHeaders_OverwritesExisting(t *testing.T) {
	h := http.Header{}
	h.Set("User-Agent", "stale")
	h.Set("X-SNJS-Version", "stale")
	h.Set("X-Application-Version", "stale")

	SetStandardClientHeaders(h)

	if got := h.Get("User-Agent"); got != SNUserAgent {
		t.Errorf("User-Agent not overwritten: got %q", got)
	}
	if got := h.Get("X-SNJS-Version"); got != SNJSVersion {
		t.Errorf("X-SNJS-Version not overwritten: got %q", got)
	}
	if got := h.Get("X-Application-Version"); got != SNAppVersion {
		t.Errorf("X-Application-Version not overwritten: got %q", got)
	}
}
