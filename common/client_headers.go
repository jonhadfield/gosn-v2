package common

import "net/http"

// Client identification headers required by the Standard Notes API gateway.
//
// As of 2026-05, requests missing X-SNJS-Version or X-Application-Version are
// rejected with HTTP 400 and the message "Your client version is no longer
// supported." The gateway gates on these headers rather than the api field in
// the request body, so they must be present on every auth and sync request.
//
// The values below mirror what the official Standard Notes client sends. When
// the gateway bumps its minimum-supported client version again, update these
// constants to track the current standardnotes/app release.
const (
	SNUserAgent  = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) StandardNotes/3.198.18 Chrome/134.0.6998.205 Electron/35.2.0 Safari/537.36"
	SNJSVersion  = "2.211.7"
	SNAppVersion = "Desktop-3.201.27"
)

// SetStandardClientHeaders sets the User-Agent and SN-specific version headers
// the API gateway requires to accept a request. Call this on any outbound
// request to api.standardnotes.com (auth, sync, refresh, registration).
func SetStandardClientHeaders(h http.Header) {
	h.Set("User-Agent", SNUserAgent)
	h.Set("X-SNJS-Version", SNJSVersion)
	h.Set("X-Application-Version", SNAppVersion)
}
