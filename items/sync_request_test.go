package items

import (
	"context"
	"net/http"
	"testing"

	"github.com/jonhadfield/gosn-v2/common"
	"github.com/jonhadfield/gosn-v2/session"
	"github.com/stretchr/testify/require"
)

// newSyncRequest is the single source of request construction shared by the
// initial sync request and both retry paths (HTTP 429 backoff and
// post-token-refresh). These tests pin the contract that every constructed
// request carries authentication and the client-identification headers the SN
// gateway requires — in particular that cookie-based sessions always send the
// Cookie header, which earlier hand-rolled retry paths dropped.

const testSyncURL = "https://api.standardnotes.com/v1/items"

func TestNewSyncRequestCookieBasedAuth(t *testing.T) {
	sess := &session.Session{
		AccessToken:       "2:abc.def.ghi",
		AccessTokenCookie: "access_token_xyz=cookievalue",
	}

	req, err := newSyncRequest(sess, testSyncURL, []byte(`{"api":"x"}`), nil)
	require.NoError(t, err)

	// Cookie-based sessions send BOTH Cookie and Authorization headers.
	require.Equal(t, "access_token_xyz=cookievalue", req.Header.Get("Cookie"))
	require.Equal(t, "Bearer 2:abc.def.ghi", req.Header.Get("Authorization"))

	assertStandardHeaders(t, req)
}

func TestNewSyncRequestHeaderBasedAuth(t *testing.T) {
	sess := &session.Session{
		AccessToken: "plain-access-token",
	}

	req, err := newSyncRequest(sess, testSyncURL, []byte(`{"api":"x"}`), nil)
	require.NoError(t, err)

	// Header-based sessions send no Cookie header.
	require.Empty(t, req.Header.Get("Cookie"))
	require.Equal(t, "Bearer plain-access-token", req.Header.Get("Authorization"))

	assertStandardHeaders(t, req)
}

// TestNewSyncRequestCookieOmittedWhenEmpty guards the regression where a
// "2:"-prefixed token with no stored cookie must not emit an empty Cookie
// header.
func TestNewSyncRequestCookieOmittedWhenEmpty(t *testing.T) {
	sess := &session.Session{
		AccessToken:       "2:abc.def.ghi",
		AccessTokenCookie: "",
	}

	req, err := newSyncRequest(sess, testSyncURL, nil, nil)
	require.NoError(t, err)

	require.Empty(t, req.Header.Get("Cookie"))
	require.Equal(t, "Bearer 2:abc.def.ghi", req.Header.Get("Authorization"))
}

func TestNewSyncRequestAppliesContext(t *testing.T) {
	sess := &session.Session{AccessToken: "t"}

	type ctxKey string
	ctx := context.WithValue(context.Background(), ctxKey("k"), "v")

	req, err := newSyncRequest(sess, testSyncURL, nil, ctx)
	require.NoError(t, err)
	require.Equal(t, "v", req.Context().Value(ctxKey("k")))
}

// assertStandardHeaders verifies the request carries the content type and the
// client-identification headers the SN gateway requires, so retried requests
// are indistinguishable from the initial one to the gateway.
func assertStandardHeaders(t *testing.T, req *http.Request) {
	t.Helper()
	require.Equal(t, common.SNAPIContentType, req.Header.Get(common.HeaderContentType))
	require.Equal(t, common.SNUserAgent, req.Header.Get("User-Agent"))
	require.Equal(t, common.SNJSVersion, req.Header.Get("X-SNJS-Version"))
	require.Equal(t, common.SNAppVersion, req.Header.Get("X-Application-Version"))
}
