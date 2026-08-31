// These tests run against the mock server. They live in the external
// session_test package because the mock imports session, so using it from the
// internal tests would be an import cycle.
package session_test

import (
	"testing"

	"github.com/jonhadfield/gosn-v2/common"
	"github.com/jonhadfield/gosn-v2/mock"
	"github.com/jonhadfield/gosn-v2/session"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// keyring is an in-memory stand-in for the OS keyring.
type keyring struct {
	stored map[string]string
}

func newKeyring() *keyring { return &keyring{stored: map[string]string{}} }

func (k *keyring) Set(service, user, password string) error {
	k.stored[service+"/"+user] = password

	return nil
}

func (k *keyring) Get(service, user string) (string, error) {
	return k.stored[service+"/"+user], nil
}

func (k *keyring) Delete(service, user string) error {
	delete(k.stored, service+"/"+user)

	return nil
}

func (k *keyring) DeleteAll(service string) error {
	k.stored = map[string]string{}

	return nil
}

// useMockServer points the environment at a mock server for the duration of a
// test. GetCredentials reads the account through viper, so the environment has
// to be bound to it the way a client does at startup.
func useMockServer(t *testing.T) *mock.Server {
	t.Helper()

	srv, err := mock.New()
	require.NoError(t, err)
	t.Cleanup(srv.Close)

	t.Setenv(common.EnvServer, srv.URL)
	t.Setenv(common.EnvEmail, srv.Email)
	t.Setenv(common.EnvPassword, srv.Password)

	viper.SetEnvPrefix("sn")

	for _, key := range []string{"server", "email", "password"} {
		require.NoError(t, viper.BindEnv(key))
	}

	return srv
}

func TestGetSessionFromUserAgainstMockServer(t *testing.T) {
	srv := useMockServer(t)

	sess, email, err := session.GetSessionFromUser(nil, srv.URL, false)
	require.NoError(t, err)

	assert.Equal(t, srv.Email, email)
	assert.Equal(t, srv.URL, sess.Server)
	assert.NotEmpty(t, sess.AccessToken)
	assert.NotEmpty(t, sess.MasterKey)
	assert.True(t, sess.Valid(), "a freshly signed in session should be valid")
}

func TestAddSessionAgainstMockServer(t *testing.T) {
	srv := useMockServer(t)

	k := newKeyring()

	msg, err := session.AddSession(nil, srv.URL, "", k, false)
	require.NoError(t, err)
	assert.NotEmpty(t, msg)

	// the session should now be readable back out of the keyring
	stored, err := session.GetSessionFromKeyring(k)
	require.NoError(t, err)
	assert.Contains(t, stored, srv.URL)

	require.NoError(t, session.SessionExists(k))
}

func TestSessionStatusAgainstMockServer(t *testing.T) {
	srv := useMockServer(t)

	k := newKeyring()

	_, err := session.AddSession(nil, srv.URL, "", k, false)
	require.NoError(t, err)

	msg, err := session.SessionStatus("", k)
	require.NoError(t, err)
	assert.Contains(t, msg, srv.Email)
}

func TestRemoveSessionAgainstMockServer(t *testing.T) {
	srv := useMockServer(t)

	k := newKeyring()

	_, err := session.AddSession(nil, srv.URL, "", k, false)
	require.NoError(t, err)
	require.NoError(t, session.SessionExists(k))

	assert.Equal(t, session.MsgSessionRemovalSuccess, session.RemoveSession(k))
	require.Error(t, session.SessionExists(k))
}

// TestSessionRefreshAgainstMockServer covers Session.Refresh, which could not
// succeed while the refresh response failed to unmarshal.
func TestSessionRefreshAgainstMockServer(t *testing.T) {
	srv := useMockServer(t)

	sess, _, err := session.GetSessionFromUser(nil, srv.URL, false)
	require.NoError(t, err)

	before := sess.AccessToken

	require.NoError(t, sess.Refresh())

	assert.NotEmpty(t, sess.AccessToken)
	assert.NotEqual(t, before, sess.AccessToken, "refresh should replace the access token")
	assert.True(t, sess.Valid())
}
