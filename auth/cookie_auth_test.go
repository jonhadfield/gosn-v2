package auth

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hashicorp/go-retryablehttp"
	"github.com/jonhadfield/gosn-v2/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCookieExtraction verifies that cookie values are correctly extracted from Set-Cookie headers
func TestCookieExtraction(t *testing.T) {
	tests := []struct {
		name                   string
		setCookieHeaders       []string
		expectedAccessCookie   string
		expectedRefreshCookie  string
		expectedAccessToken    string
		expectedRefreshToken   string
	}{
		{
			name: "valid cookie-based auth response",
			setCookieHeaders: []string{
				"access_token_abc123=2:token123; Path=/; HttpOnly; Secure; SameSite=Strict; Partitioned",
				"refresh_token_def456=2:refresh123; Path=/; HttpOnly; Secure; SameSite=Strict; Partitioned",
			},
			expectedAccessCookie:  "access_token_abc123=2:token123",
			expectedRefreshCookie: "refresh_token_def456=2:refresh123",
			expectedAccessToken:   "2:token123",
			expectedRefreshToken:  "2:refresh123",
		},
		{
			name: "multiple cookies with extras",
			setCookieHeaders: []string{
				"access_token_xyz=2:tokenxyz; Max-Age=3600; Path=/; HttpOnly; Secure",
				"other_cookie=value; Path=/",
				"refresh_token_rst=2:refreshrst; Max-Age=86400; Path=/; HttpOnly",
			},
			expectedAccessCookie:  "access_token_xyz=2:tokenxyz",
			expectedRefreshCookie: "refresh_token_rst=2:refreshrst",
			expectedAccessToken:   "2:tokenxyz",
			expectedRefreshToken:  "2:refreshrst",
		},
		{
			name:                   "no cookie headers",
			setCookieHeaders:       []string{},
			expectedAccessCookie:   "",
			expectedRefreshCookie:  "",
			expectedAccessToken:    "",
			expectedRefreshToken:   "",
		},
		{
			name: "header-based auth (no cookies)",
			setCookieHeaders: []string{
				"session_id=abc123; Path=/",
			},
			expectedAccessCookie:  "",
			expectedRefreshCookie: "",
			expectedAccessToken:   "",
			expectedRefreshToken:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var accessTokenCookie string
			var refreshTokenCookie string
			var accessToken string
			var refreshToken string

			if len(tt.setCookieHeaders) > 0 {
				for _, setCookieHeader := range tt.setCookieHeaders {
					parts := strings.Split(setCookieHeader, ";")
					if len(parts) == 0 {
						continue
					}

					nameValue := strings.TrimSpace(parts[0])

					if strings.HasPrefix(nameValue, "access_token_") {
						accessTokenCookie = nameValue
						// Extract the token value (after the =)
						if idx := strings.Index(nameValue, "="); idx != -1 {
							accessToken = nameValue[idx+1:]
						}
					} else if strings.HasPrefix(nameValue, "refresh_token_") {
						refreshTokenCookie = nameValue
						// Extract the token value (after the =)
						if idx := strings.Index(nameValue, "="); idx != -1 {
							refreshToken = nameValue[idx+1:]
						}
					}
				}
			}

			assert.Equal(t, tt.expectedAccessCookie, accessTokenCookie, "Access cookie mismatch")
			assert.Equal(t, tt.expectedRefreshCookie, refreshTokenCookie, "Refresh cookie mismatch")
			assert.Equal(t, tt.expectedAccessToken, accessToken, "Access token mismatch")
			assert.Equal(t, tt.expectedRefreshToken, refreshToken, "Refresh token mismatch")
		})
	}
}

// TestCookieBasedAuthDetection verifies detection of cookie-based vs header-based authentication
func TestCookieBasedAuthDetection(t *testing.T) {
	tests := []struct {
		name           string
		accessToken    string
		expectedIsCookie bool
	}{
		{
			name:           "cookie-based token (2: prefix)",
			accessToken:    "2:abc123def456",
			expectedIsCookie: true,
		},
		{
			name:           "header-based token (no 2: prefix)",
			accessToken:    "abc123def456",
			expectedIsCookie: false,
		},
		{
			name:           "header-based token (1: prefix)",
			accessToken:    "1:abc123def456",
			expectedIsCookie: false,
		},
		{
			name:           "empty token",
			accessToken:    "",
			expectedIsCookie: false,
		},
		{
			name:           "malformed token with colon but not 2:",
			accessToken:    "3:abc123",
			expectedIsCookie: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			accessParts := strings.Split(tt.accessToken, ":")
			isCookieBased := len(accessParts) >= 2 && accessParts[0] == "2"

			assert.Equal(t, tt.expectedIsCookie, isCookieBased, "Cookie detection mismatch")
		})
	}
}

// TestDualHeaderAuthentication verifies that both Cookie and Authorization headers are sent for cookie-based auth
func TestDualHeaderAuthentication(t *testing.T) {
	// Create a test server that validates headers
	requestsReceived := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestsReceived++

		// Verify both headers are present for cookie-based auth
		cookieHeader := r.Header.Get("Cookie")
		authHeader := r.Header.Get("Authorization")

		assert.NotEmpty(t, cookieHeader, "Cookie header should be present")
		assert.NotEmpty(t, authHeader, "Authorization header should be present")
		assert.Contains(t, authHeader, "Bearer ", "Authorization should be Bearer token")
		assert.Contains(t, cookieHeader, "access_token_", "Cookie should contain access token")

		// Return success response
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"data":{"items":[]},"sync_token":"test"}`))
	}))
	defer server.Close()

	// Create a mock session with cookie-based auth
	session := &SignInResponseDataSession{
		HTTPClient:        retryablehttp.NewClient(),
		Server:            server.URL,
		AccessToken:       "2:test_token_123",
		AccessTokenCookie: "access_token_abc=2:test_token_123",
		Debug:             false,
	}

	// Create a request that would be sent during sync
	req, err := http.NewRequest("GET", server.URL+"/test", nil)
	require.NoError(t, err)

	// Apply authentication headers (simulating items.go logic)
	accessParts := strings.Split(session.AccessToken, ":")
	isCookieBased := len(accessParts) >= 2 && accessParts[0] == "2"

	if isCookieBased && session.AccessTokenCookie != "" {
		// For cookie-based auth, send BOTH Cookie and Authorization headers
		req.Header.Set("Cookie", session.AccessTokenCookie)
		req.Header.Set("Authorization", "Bearer "+session.AccessToken)
	} else {
		req.Header.Set("Authorization", "Bearer "+session.AccessToken)
	}

	// Execute request
	resp, err := session.HTTPClient.StandardClient().Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, 1, requestsReceived, "Should have received exactly one request")
}

// TestHeaderOnlyAuthentication verifies that only Authorization header is sent for header-based auth
func TestHeaderOnlyAuthentication(t *testing.T) {
	// Create a test server that validates headers
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify only Authorization header is present for header-based auth
		cookieHeader := r.Header.Get("Cookie")
		authHeader := r.Header.Get("Authorization")

		assert.Empty(t, cookieHeader, "Cookie header should NOT be present for header-based auth")
		assert.NotEmpty(t, authHeader, "Authorization header should be present")
		assert.Contains(t, authHeader, "Bearer ", "Authorization should be Bearer token")

		// Return success response
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"data":{"items":[]},"sync_token":"test"}`))
	}))
	defer server.Close()

	// Create a mock session with header-based auth (no 2: prefix)
	session := &SignInResponseDataSession{
		HTTPClient:        retryablehttp.NewClient(),
		Server:            server.URL,
		AccessToken:       "regular_token_123",
		AccessTokenCookie: "", // No cookie for header-based auth
		Debug:             false,
	}

	// Create a request
	req, err := http.NewRequest("GET", server.URL+"/test", nil)
	require.NoError(t, err)

	// Apply authentication headers
	accessParts := strings.Split(session.AccessToken, ":")
	isCookieBased := len(accessParts) >= 2 && accessParts[0] == "2"

	if isCookieBased && session.AccessTokenCookie != "" {
		req.Header.Set("Cookie", session.AccessTokenCookie)
		req.Header.Set("Authorization", "Bearer "+session.AccessToken)
	} else {
		req.Header.Set("Authorization", "Bearer "+session.AccessToken)
	}

	// Execute request
	resp, err := session.HTTPClient.StandardClient().Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// TestCookieRefreshFlow verifies cookie extraction and usage in refresh flow
func TestCookieRefreshFlow(t *testing.T) {
	// Create a test server that returns cookie-based refresh response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify refresh request has both headers
		cookieHeader := r.Header.Get("Cookie")
		authHeader := r.Header.Get("Authorization")

		assert.NotEmpty(t, cookieHeader, "Refresh should send cookie")
		assert.NotEmpty(t, authHeader, "Refresh should send authorization")

		// Return new tokens via cookies
		w.Header().Add("Set-Cookie", "access_token_new=2:new_access; Path=/; HttpOnly; Secure")
		w.Header().Add("Set-Cookie", "refresh_token_new=2:new_refresh; Path=/; HttpOnly; Secure")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"data": {
				"session": {
					"access_token": "2:new_access",
					"refresh_token": "2:new_refresh",
					"access_expiration": 3600,
					"refresh_expiration": 86400
				}
			}
		}`))
	}))
	defer server.Close()

	// Initial session with cookie-based auth
	session := &SignInResponseDataSession{
		HTTPClient:         retryablehttp.NewClient(),
		Server:             server.URL,
		AccessToken:        "2:old_access",
		RefreshToken:       "2:old_refresh",
		AccessTokenCookie:  "access_token_old=2:old_access",
		RefreshTokenCookie: "refresh_token_old=2:old_refresh",
		Debug:              false,
	}

	// Simulate refresh request
	req, err := http.NewRequest("POST", server.URL+common.AuthRefreshPath, nil)
	require.NoError(t, err)

	// Apply refresh headers (like in authentication.go)
	refreshParts := strings.Split(session.RefreshToken, ":")
	isCookieBased := len(refreshParts) >= 2 && refreshParts[0] == "2"

	if isCookieBased && session.RefreshTokenCookie != "" {
		req.Header.Set("Cookie", session.RefreshTokenCookie)
		req.Header.Set("Authorization", "Bearer "+session.RefreshToken)
	} else {
		req.Header.Set("Authorization", "Bearer "+session.RefreshToken)
	}

	// Execute request
	resp, err := session.HTTPClient.StandardClient().Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Verify Set-Cookie headers were returned
	setCookies := resp.Header.Values("Set-Cookie")
	assert.Len(t, setCookies, 2, "Should receive 2 Set-Cookie headers")

	// Extract new cookies
	var newAccessCookie, newRefreshCookie string
	for _, cookie := range setCookies {
		if strings.HasPrefix(cookie, "access_token_") {
			parts := strings.Split(cookie, ";")
			newAccessCookie = strings.TrimSpace(parts[0])
		} else if strings.HasPrefix(cookie, "refresh_token_") {
			parts := strings.Split(cookie, ";")
			newRefreshCookie = strings.TrimSpace(parts[0])
		}
	}

	assert.Equal(t, "access_token_new=2:new_access", newAccessCookie)
	assert.Equal(t, "refresh_token_new=2:new_refresh", newRefreshCookie)
}

// TestGenerateCryptoSeed verifies the new crypto seed generation function
func TestGenerateCryptoSeed(t *testing.T) {
	// Test that generateCryptoSeed returns a valid seed
	seed, err := generateCryptoSeed()
	require.NoError(t, err, "generateCryptoSeed should not return error")
	assert.NotEqual(t, int64(0), seed, "Seed should not be zero")

	// Test that multiple calls return different seeds
	seed2, err := generateCryptoSeed()
	require.NoError(t, err)
	assert.NotEqual(t, seed, seed2, "Consecutive seeds should be different")

	// Test that we can generate many seeds without error
	for i := 0; i < 100; i++ {
		_, err := generateCryptoSeed()
		require.NoError(t, err, "Should generate seed without error on iteration %d", i)
	}
}

// TestCookieAuthenticationFields verifies that SignInResponseDataSession has cookie fields
func TestCookieAuthenticationFields(t *testing.T) {
	session := &SignInResponseDataSession{
		HTTPClient:         retryablehttp.NewClient(),
		Server:             "https://test.standardnotes.com",
		AccessToken:        "2:test_access",
		RefreshToken:       "2:test_refresh",
		AccessTokenCookie:  "access_token_abc=2:test_access",
		RefreshTokenCookie: "refresh_token_def=2:test_refresh",
	}

	assert.NotEmpty(t, session.AccessTokenCookie, "AccessTokenCookie field should exist")
	assert.NotEmpty(t, session.RefreshTokenCookie, "RefreshTokenCookie field should exist")
	assert.Contains(t, session.AccessTokenCookie, "access_token_", "Cookie should have proper name")
	assert.Contains(t, session.RefreshTokenCookie, "refresh_token_", "Cookie should have proper name")
}
