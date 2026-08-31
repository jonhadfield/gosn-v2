package items

import (
	"fmt"
	"testing"

	"github.com/jonhadfield/gosn-v2/common"
	"github.com/jonhadfield/gosn-v2/mock"
	"github.com/jonhadfield/gosn-v2/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockSession starts a mock server and returns a session that has completed its
// first sync, so it holds the account's items key.
func mockSession(t *testing.T, opts ...mock.Option) (*mock.Server, *session.Session) {
	t.Helper()

	srv, err := mock.New(opts...)
	require.NoError(t, err)
	t.Cleanup(srv.Close)

	sess, err := srv.Session(false)
	require.NoError(t, err)

	_, err = Sync(SyncInput{Session: sess})
	require.NoError(t, err)
	require.NotEmpty(t, sess.DefaultItemsKey.ItemsKey)

	return srv, sess
}

func encryptNotes(t *testing.T, sess *session.Session, titles ...string) EncryptedItems {
	t.Helper()

	var out EncryptedItems

	for _, title := range titles {
		note, err := NewNote(title, title+" content", nil)
		require.NoError(t, err)

		encrypted, err := EncryptItem(&note, sess.DefaultItemsKey, sess)
		require.NoError(t, err)

		out = append(out, encrypted)
	}

	return out
}

func TestSyncItemsKeyIsRetrievedAndMadeDefault(t *testing.T) {
	srv, sess := mockSession(t)

	assert.Equal(t, srv.ItemsKeyUUID(), sess.DefaultItemsKey.UUID)
	assert.True(t, sess.DefaultItemsKey.Default)
	require.Len(t, sess.ItemsKeys, 1)
	assert.Equal(t, common.DefaultSNVersion, sess.DefaultItemsKey.Version)
}

func TestSyncPushesAndRetrievesNotes(t *testing.T) {
	srv, sess := mockSession(t)

	so, err := Sync(SyncInput{Session: sess, Items: encryptNotes(t, sess, "apple", "lemon")})
	require.NoError(t, err)
	require.Len(t, so.SavedItems, 2)
	require.Len(t, srv.ItemsOfType(common.SNItemTypeNote), 2)

	// a second client sees both, and can decrypt them
	_, reader := mockSessionFor(t, srv)

	ro, err := Sync(SyncInput{Session: reader})
	require.NoError(t, err)

	parsed, err := ro.Items.DecryptAndParse(reader)
	require.NoError(t, err)

	titles := make([]string, 0, 2)
	for _, n := range parsed.Notes() {
		titles = append(titles, n.Content.GetTitle())
	}

	assert.ElementsMatch(t, []string{"apple", "lemon"}, titles)
}

// mockSessionFor returns a second synced session against an existing server.
func mockSessionFor(t *testing.T, srv *mock.Server) (*mock.Server, *session.Session) {
	t.Helper()

	sess, err := srv.Session(false)
	require.NoError(t, err)

	_, err = Sync(SyncInput{Session: sess})
	require.NoError(t, err)

	return srv, sess
}

func TestSyncTokenOnlyReturnsWhatIsNew(t *testing.T) {
	srv, sess := mockSession(t)

	first, err := Sync(SyncInput{Session: sess, Items: encryptNotes(t, sess, "apple")})
	require.NoError(t, err)
	require.NotEmpty(t, first.SyncToken)

	// nothing has changed, so a sync with that token brings nothing back
	second, err := Sync(SyncInput{Session: sess, SyncToken: first.SyncToken})
	require.NoError(t, err)
	require.Empty(t, second.Items)

	// but an item another client writes afterwards does
	_, other := mockSessionFor(t, srv)

	_, err = Sync(SyncInput{Session: other, Items: encryptNotes(t, other, "lemon")})
	require.NoError(t, err)

	third, err := Sync(SyncInput{Session: sess, SyncToken: second.SyncToken})
	require.NoError(t, err)
	require.Len(t, third.Items, 1)

	parsed, err := third.Items.DecryptAndParse(sess)
	require.NoError(t, err)
	require.Len(t, parsed.Notes(), 1)
	assert.Equal(t, "lemon", parsed.Notes()[0].Content.GetTitle())
}

func TestSyncPropagatesDeletion(t *testing.T) {
	srv, sess := mockSession(t)

	notes := encryptNotes(t, sess, "apple")

	_, err := Sync(SyncInput{Session: sess, Items: notes})
	require.NoError(t, err)
	require.Len(t, srv.ItemsOfType(common.SNItemTypeNote), 1)

	notes[0].Deleted = true

	_, err = Sync(SyncInput{Session: sess, Items: notes})
	require.NoError(t, err)

	assert.Empty(t, srv.ItemsOfType(common.SNItemTypeNote))

	// a client syncing from scratch is told about the deletion
	_, reader := mockSessionFor(t, srv)

	ro, err := Sync(SyncInput{Session: reader})
	require.NoError(t, err)

	var seen bool

	for _, i := range ro.Items {
		if i.UUID == notes[0].UUID {
			seen = true

			assert.True(t, i.Deleted)
		}
	}

	assert.True(t, seen, "deleted item should be reported to other clients")
}

func TestSyncPagesThroughLargeResponses(t *testing.T) {
	srv, sess := mockSession(t, mock.WithPageSize(3))

	var titles []string
	for i := range 11 {
		titles = append(titles, fmt.Sprintf("note-%02d", i))
	}

	_, err := Sync(SyncInput{Session: sess, Items: encryptNotes(t, sess, titles...)})
	require.NoError(t, err)
	require.Len(t, srv.ItemsOfType(common.SNItemTypeNote), len(titles))

	// the reader has to follow the cursor across pages to see them all
	_, reader := mockSessionFor(t, srv)

	ro, err := Sync(SyncInput{Session: reader})
	require.NoError(t, err)

	parsed, err := ro.Items.DecryptAndParse(reader)
	require.NoError(t, err)
	assert.Len(t, parsed.Notes(), len(titles))
}

func TestSyncReportsServerConflicts(t *testing.T) {
	srv, sess := mockSession(t)

	notes := encryptNotes(t, sess, "apple")

	_, err := Sync(SyncInput{Session: sess, Items: notes})
	require.NoError(t, err)

	stored := srv.ItemsOfType(common.SNItemTypeNote)
	require.Len(t, stored, 1)
	original := stored[0].Content

	// the server now rejects writes to that item, as it would when the item has
	// moved on underneath the client
	srv.ConflictOn(notes[0].UUID, "sync_conflict")

	changed := encryptNotes(t, sess, "apple updated")
	changed[0].UUID = notes[0].UUID

	_, err = Sync(SyncInput{Session: sess, Items: changed})
	require.NoError(t, err, "a conflict should be resolved rather than surfaced as an error")

	// whatever the client does to resolve it, the server's copy must survive
	// untouched: losing it would lose the other side of the conflict
	var found bool

	for _, i := range srv.ItemsOfType(common.SNItemTypeNote) {
		if i.UUID == notes[0].UUID {
			found = true

			assert.Equal(t, original, i.Content, "the conflicted write must not overwrite the server's copy")
		}
	}

	assert.True(t, found, "the server's copy of the conflicted item should still be there")
}

func TestSyncRecoversFromExpiredToken(t *testing.T) {
	srv, sess := mockSession(t)

	before := sess.AccessToken
	srv.ExpireAccessToken()

	_, err := Sync(SyncInput{Session: sess, Items: encryptNotes(t, sess, "apple")})
	require.NoError(t, err)

	assert.NotEqual(t, before, sess.AccessToken, "the session should hold the refreshed token")
	assert.Len(t, srv.ItemsOfType(common.SNItemTypeNote), 1, "the write should have been retried")
}

func TestSyncFailsWithoutAValidSession(t *testing.T) {
	_, sess := mockSession(t)

	sess.AccessToken = ""

	_, err := Sync(SyncInput{Session: sess})
	require.Error(t, err)
}
