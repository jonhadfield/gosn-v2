package session

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
	"time"

	"github.com/jonhadfield/gosn-v2/auth"
	"github.com/jonhadfield/gosn-v2/common"
	"github.com/jonhadfield/gosn-v2/crypto"
	"github.com/stretchr/testify/require"
	"github.com/zalando/go-keyring"
)

// The session key and refreshed access token used by the tests.
const (
	testSessionKey      = "test-session-key"
	testRefreshedAccess = "1:refreshed:access"
)

func testSession(server string, accessExpiry time.Time) Session {
	return Session{ //nolint:gosec // fake credentials for tests
		Server:            server,
		MasterKey:         "test-master-key",
		KeyParams:         auth.KeyParams{Identifier: "user@example.com", Version: "004"},
		AccessToken:       "1:original:access",
		RefreshToken:      "1:original:refresh",
		AccessExpiration:  accessExpiry.UnixMilli(),
		RefreshExpiration: time.Now().Add(24 * time.Hour).UnixMilli(),
	}
}

// useDefaultKeyring sets the default keyring for the duration of a test.
func useDefaultKeyring(t *testing.T, k keyring.Keyring) {
	t.Helper()

	SetDefaultKeyring(k)
	t.Cleanup(func() { SetDefaultKeyring(nil) })
}

func TestFileKeyringRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "session")
	fk := NewFileKeyring(path)

	_, err := fk.Get(KeyringService, KeyringApplicationName)
	require.ErrorIs(t, err, keyring.ErrNotFound)

	require.NoError(t, fk.Set(KeyringService, KeyringApplicationName, "first"))
	require.NoError(t, fk.Set(KeyringService, KeyringApplicationName, "second"))

	got, err := fk.Get(KeyringService, KeyringApplicationName)
	require.NoError(t, err)
	require.Equal(t, "second", got)

	if runtime.GOOS != "windows" {
		info, statErr := os.Stat(path)
		require.NoError(t, statErr)
		require.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	}

	// only the session file is left behind, no temporary files
	entries, err := os.ReadDir(filepath.Dir(path))
	require.NoError(t, err)
	require.Len(t, entries, 1)

	require.NoError(t, fk.Delete(KeyringService, KeyringApplicationName))
	require.ErrorIs(t, fk.Delete(KeyringService, KeyringApplicationName), keyring.ErrNotFound)
}

func TestFileKeyringEmptyFileIsNotFound(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session")
	require.NoError(t, os.WriteFile(path, []byte("\n"), 0o600))

	_, err := NewFileKeyring(path).Get(KeyringService, KeyringApplicationName)
	require.ErrorIs(t, err, keyring.ErrNotFound)
}

func TestDefaultKeyringIsUsedWhenNilPassed(t *testing.T) {
	fk := NewFileKeyring(filepath.Join(t.TempDir(), "session"))
	useDefaultKeyring(t, fk)

	require.Error(t, SessionExists(nil))

	require.NoError(t, writeSession(makeMinimalSessionString(testSession("", time.Now())), nil))
	require.NoError(t, SessionExists(nil))

	require.Equal(t, MsgSessionRemovalSuccess, RemoveSession(nil))
	require.Error(t, SessionExists(nil))
}

func TestGetSessionFromFileKeyring(t *testing.T) {
	fk := NewFileKeyring(filepath.Join(t.TempDir(), "session"))
	useDefaultKeyring(t, fk)

	// an access token well within its expiry is used as is, without a refresh
	stored := testSession("http://127.0.0.1:1", time.Now().Add(time.Hour))
	require.NoError(t, fk.Set("", "", makeMinimalSessionString(stored)))

	sess, email, err := GetSession(nil, true, "", "", false)
	require.NoError(t, err)
	require.Equal(t, "user@example.com", email)
	require.Equal(t, stored.AccessToken, sess.AccessToken)
}

func TestGetSessionFromFileKeyringEncrypted(t *testing.T) {
	fk := NewFileKeyring(filepath.Join(t.TempDir(), "session"))
	useDefaultKeyring(t, fk)

	stored := testSession("http://127.0.0.1:1", time.Now().Add(time.Hour))
	enc, err := crypto.Encrypt([]byte(testSessionKey), makeMinimalSessionString(stored))
	require.NoError(t, err)
	require.NoError(t, fk.Set("", "", enc))

	sess, _, err := GetSession(nil, true, testSessionKey, "", false)
	require.NoError(t, err)
	require.Equal(t, stored.AccessToken, sess.AccessToken)
}

// A refreshed session must be written back to the default keyring, encrypted
// with the key it was loaded with, and without prompting for that key again.
func TestGetSessionRefreshSavesEncryptedToFileKeyring(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != common.AuthRefreshPath {
			http.NotFound(w, r)

			return
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"session":{"access_token":"` + testRefreshedAccess +
			`","refresh_token":"1:refreshed:refresh","access_expiration":` +
			jsonInt(time.Now().Add(time.Hour).UnixMilli()) +
			`,"refresh_expiration":` + jsonInt(time.Now().Add(24*time.Hour).UnixMilli()) + `}}}`))
	}))
	t.Cleanup(srv.Close)

	fk := NewFileKeyring(filepath.Join(t.TempDir(), "session"))
	useDefaultKeyring(t, fk)

	// an expired access token forces a refresh
	stored := testSession(srv.URL, time.Now().Add(-time.Hour))
	enc, err := crypto.Encrypt([]byte(testSessionKey), makeMinimalSessionString(stored))
	require.NoError(t, err)
	require.NoError(t, fk.Set("", "", enc))

	sess, _, err := GetSession(nil, true, testSessionKey, "", false)
	require.NoError(t, err)
	require.Equal(t, testRefreshedAccess, sess.AccessToken)

	raw, err := fk.Get("", "")
	require.NoError(t, err)
	require.False(t, isUnencryptedSession(raw), "refreshed session should still be encrypted")

	plain, err := crypto.Decrypt([]byte(testSessionKey), raw)
	require.NoError(t, err)

	saved, err := ParseSessionString(plain)
	require.NoError(t, err)
	require.Equal(t, testRefreshedAccess, saved.AccessToken)
}

func jsonInt(i int64) string {
	return strconv.FormatInt(i, 10)
}
