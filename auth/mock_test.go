// These tests run against the mock server. They live in the external auth_test
// package because the mock imports auth, so using it from the internal tests
// would be an import cycle.
package auth_test

import (
	"testing"

	"github.com/jonhadfield/gosn-v2/auth"
	"github.com/jonhadfield/gosn-v2/common"
	"github.com/jonhadfield/gosn-v2/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newMockServer(t *testing.T) *mock.Server {
	t.Helper()

	srv, err := mock.New()
	require.NoError(t, err)
	t.Cleanup(srv.Close)

	return srv
}

func TestSignInAgainstMockServer(t *testing.T) {
	srv := newMockServer(t)

	out, err := auth.SignIn(auth.SignInInput{
		Email:     srv.Email,
		Password:  srv.Password,
		APIServer: srv.URL,
	})
	require.NoError(t, err)

	assert.NotEmpty(t, out.Session.AccessToken)
	assert.NotEmpty(t, out.Session.RefreshToken)
	assert.NotZero(t, out.Session.AccessExpiration)
	assert.NotZero(t, out.Session.RefreshExpiration)
	assert.Equal(t, common.DefaultSNVersion, out.KeyParams.Version)
	assert.Equal(t, srv.Email, out.KeyParams.Identifier)

	// the master key has to match the one the account's items are encrypted
	// with, or nothing the client fetches will decrypt
	assert.Equal(t, srv.MasterKey(), out.Session.MasterKey)
}

// TestSignInSetsServerOnSession covers the session coming back without the
// address it was created against, which left anything that defaults an empty
// server pointing a self-hosted session at the live API.
func TestSignInSetsServerOnSession(t *testing.T) {
	srv := newMockServer(t)

	out, err := auth.SignIn(auth.SignInInput{
		Email:     srv.Email,
		Password:  srv.Password,
		APIServer: srv.URL,
	})
	require.NoError(t, err)

	assert.Equal(t, srv.URL, out.Session.Server)
}

func TestSignInWithBadCredentials(t *testing.T) {
	srv := newMockServer(t)

	for _, tc := range []struct{ name, email, password string }{
		{"wrong password", mock.DefaultEmail, "not-the-password"},
		{"unknown account", "someone-else@example.com", mock.DefaultPassword},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := auth.SignIn(auth.SignInInput{
				Email:     tc.email,
				Password:  tc.password,
				APIServer: srv.URL,
			})
			require.Error(t, err)
			assert.Contains(t, err.Error(), "invalid email or password")
		})
	}
}

func TestCliSignInAgainstMockServer(t *testing.T) {
	srv := newMockServer(t)

	sess, err := auth.CliSignIn(srv.Email, srv.Password, srv.URL, false)
	require.NoError(t, err)

	assert.NotEmpty(t, sess.AccessToken)
	assert.Equal(t, srv.URL, sess.Server)
}

// TestRequestRefreshTokenWithSession covers the refresh response failing to
// unmarshal, which meant an expired access token could never be recovered from.
func TestRequestRefreshTokenWithSession(t *testing.T) {
	srv := newMockServer(t)

	sess, err := auth.CliSignIn(srv.Email, srv.Password, srv.URL, false)
	require.NoError(t, err)

	out, err := auth.RequestRefreshTokenWithSession(&sess, srv.URL+common.AuthRefreshPath, false)
	require.NoError(t, err)

	assert.NotEmpty(t, out.Data.Session.AccessToken)
	assert.NotEmpty(t, out.Data.Session.RefreshToken)
	assert.NotZero(t, out.Data.Session.AccessExpiration)
	assert.NotZero(t, out.Data.Session.RefreshExpiration)

	// the server issues a new token on every refresh
	assert.NotEqual(t, sess.AccessToken, out.Data.Session.AccessToken)
}

func TestRegisterAgainstMockServer(t *testing.T) {
	srv := newMockServer(t)

	token, err := auth.RegisterInput{
		Client:    common.NewHTTPClient(),
		Email:     srv.Email,
		Password:  srv.Password,
		APIServer: srv.URL,
		Version:   common.DefaultSNVersion,
	}.Register()
	require.NoError(t, err)
	assert.NotEmpty(t, token)
}
