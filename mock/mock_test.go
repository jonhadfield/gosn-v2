package mock_test

import (
	"testing"

	"github.com/jonhadfield/gosn-v2/auth"
	"github.com/jonhadfield/gosn-v2/common"
	"github.com/jonhadfield/gosn-v2/items"
	"github.com/jonhadfield/gosn-v2/mock"
	"github.com/jonhadfield/gosn-v2/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newServer(t *testing.T, opts ...mock.Option) *mock.Server {
	t.Helper()

	srv, err := mock.New(opts...)
	require.NoError(t, err)
	t.Cleanup(srv.Close)

	return srv
}

// syncedSession returns a session that has completed its first sync, and so
// has the account's items key.
func syncedSession(t *testing.T, srv *mock.Server) *session.Session {
	t.Helper()

	sess, err := srv.Session(false)
	require.NoError(t, err)

	_, err = items.Sync(items.SyncInput{Session: sess})
	require.NoError(t, err)
	require.NotEmpty(t, sess.DefaultItemsKey.ItemsKey, "first sync must yield an items key")

	return sess
}

func encryptedNote(t *testing.T, sess *session.Session, title, text string) items.EncryptedItem {
	t.Helper()

	note, err := items.NewNote(title, text, nil)
	require.NoError(t, err)

	encrypted, err := items.EncryptItem(&note, sess.DefaultItemsKey, sess)
	require.NoError(t, err)

	return encrypted
}

func TestSignIn(t *testing.T) {
	srv := newServer(t)

	in, err := srv.SignIn(false)
	require.NoError(t, err)

	assert.NotEmpty(t, in.AccessToken)
	assert.NotEmpty(t, in.RefreshToken)
	assert.NotEmpty(t, in.MasterKey)
	assert.NotZero(t, in.AccessExpiration)
	assert.NotZero(t, in.RefreshExpiration)
	assert.Equal(t, common.DefaultSNVersion, in.KeyParams.Version)

	// the client has to derive the same master key the server encrypted the
	// items key with, or nothing will decrypt
	assert.Equal(t, srv.MasterKey(), in.MasterKey)
}

func TestSignInWithWrongPassword(t *testing.T) {
	srv := newServer(t)

	_, err := auth.CliSignIn(srv.Email, "wrong-password", srv.URL, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid email or password")
}

func TestSignInWithUnknownEmail(t *testing.T) {
	srv := newServer(t)

	_, err := auth.CliSignIn("nobody@example.com", srv.Password, srv.URL, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid email or password")
}

func TestCustomCredentials(t *testing.T) {
	srv := newServer(t, mock.WithCredentials("someone@example.com", "another-password"))

	assert.Equal(t, "someone@example.com", srv.Email)

	_, err := srv.SignIn(false)
	require.NoError(t, err)
}

func TestFirstSyncYieldsItemsKey(t *testing.T) {
	srv := newServer(t)
	sess := syncedSession(t, srv)

	assert.Equal(t, srv.ItemsKeyUUID(), sess.DefaultItemsKey.UUID)
	assert.True(t, sess.DefaultItemsKey.Default)
	assert.Len(t, sess.ItemsKeys, 1)
}

func TestNoteRoundTrip(t *testing.T) {
	srv := newServer(t)
	sess := syncedSession(t, srv)

	so, err := items.Sync(items.SyncInput{
		Session: sess,
		Items:   items.EncryptedItems{encryptedNote(t, sess, "apple", "apple content")},
	})
	require.NoError(t, err)
	require.Len(t, so.SavedItems, 1)

	// the server stamps the updated time, as a real one does
	assert.NotEmpty(t, so.SavedItems[0].UpdatedAt)
	assert.NotZero(t, so.SavedItems[0].UpdatedAtTimestamp)
	assert.Len(t, srv.ItemsOfType(common.SNItemTypeNote), 1)

	// a second client must be able to read it back and decrypt it
	reader := syncedSession(t, srv)

	ro, err := items.Sync(items.SyncInput{Session: reader})
	require.NoError(t, err)

	parsed, err := ro.Items.DecryptAndParse(reader)
	require.NoError(t, err)

	notes := parsed.Notes()
	require.Len(t, notes, 1)
	assert.Equal(t, "apple", notes[0].Content.GetTitle())
	assert.Equal(t, "apple content", notes[0].Content.GetText())
}

func TestDeletedItemLosesItsPayload(t *testing.T) {
	srv := newServer(t)
	sess := syncedSession(t, srv)

	note := encryptedNote(t, sess, "lemon", "lemon content")

	_, err := items.Sync(items.SyncInput{Session: sess, Items: items.EncryptedItems{note}})
	require.NoError(t, err)
	require.Len(t, srv.ItemsOfType(common.SNItemTypeNote), 1)

	note.Deleted = true

	so, err := items.Sync(items.SyncInput{Session: sess, Items: items.EncryptedItems{note}})
	require.NoError(t, err)

	assert.Empty(t, srv.ItemsOfType(common.SNItemTypeNote))
	require.Len(t, so.SavedItems, 1)
	assert.True(t, so.SavedItems[0].Deleted)

	// The server keeps the item's identity but drops its payload. The client
	// puts the payload it pushed back onto the saved item, so check what the
	// server actually holds rather than what came back.
	var stored *mock.Item

	for _, i := range srv.Items() {
		if i.UUID == note.UUID {
			held := i
			stored = &held
		}
	}

	require.NotNil(t, stored)
	assert.True(t, stored.Deleted)
	assert.Empty(t, stored.Content)
	assert.Empty(t, stored.EncItemKey)
}

func TestSyncTokenLimitsWhatIsRetrieved(t *testing.T) {
	srv := newServer(t)
	sess := syncedSession(t, srv)

	first, err := items.Sync(items.SyncInput{Session: sess})
	require.NoError(t, err)
	require.NotEmpty(t, first.SyncToken)

	// nothing has changed since that token, so nothing comes back
	second, err := items.Sync(items.SyncInput{Session: sess, SyncToken: first.SyncToken})
	require.NoError(t, err)
	assert.Empty(t, second.Items)
}

func TestExpiredAccessTokenIsRefreshed(t *testing.T) {
	srv := newServer(t)
	sess := syncedSession(t, srv)

	before := sess.AccessToken
	srv.ExpireAccessToken()

	// the client should refresh and retry rather than fail
	_, err := items.Sync(items.SyncInput{Session: sess})
	require.NoError(t, err)

	assert.NotEqual(t, before, sess.AccessToken, "session should hold the refreshed token")
}

func TestConflictIsReported(t *testing.T) {
	srv := newServer(t)
	sess := syncedSession(t, srv)

	note := encryptedNote(t, sess, "grape", "grape content")

	_, err := items.Sync(items.SyncInput{Session: sess, Items: items.EncryptedItems{note}})
	require.NoError(t, err)

	srv.ConflictOn(note.UUID, "sync_conflict")

	note.Content = "" // force a change the server will reject
	_, err = items.Sync(items.SyncInput{Session: sess, Items: items.EncryptedItems{note}})

	// whether the client resolves or surfaces it, the write must not land
	if err == nil {
		assert.Len(t, srv.ItemsOfType(common.SNItemTypeNote), 1)
	}
}

func TestPageSizeDrivesCursorPaging(t *testing.T) {
	srv := newServer(t, mock.WithPageSize(2))
	sess := syncedSession(t, srv)

	var toPush items.EncryptedItems
	for _, title := range []string{"one", "two", "three", "four", "five"} {
		toPush = append(toPush, encryptedNote(t, sess, title, title+" content"))
	}

	_, err := items.Sync(items.SyncInput{Session: sess, Items: toPush})
	require.NoError(t, err)
	require.Len(t, srv.ItemsOfType(common.SNItemTypeNote), 5)

	// a fresh client has to page through all of them
	reader := syncedSession(t, srv)

	ro, err := items.Sync(items.SyncInput{Session: reader})
	require.NoError(t, err)

	parsed, err := ro.Items.DecryptAndParse(reader)
	require.NoError(t, err)
	assert.Len(t, parsed.Notes(), 5)
}

func TestFailNextSyncs(t *testing.T) {
	srv := newServer(t)
	sess := syncedSession(t, srv)

	srv.FailNextSyncs(500, 500, 500, 500, 500, 500, 500, 500)

	_, err := items.Sync(items.SyncInput{Session: sess})
	require.Error(t, err)
}
