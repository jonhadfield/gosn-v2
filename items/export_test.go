package items

import (
	"path/filepath"
	"testing"

	"github.com/jonhadfield/gosn-v2/auth"
	"github.com/jonhadfield/gosn-v2/common"
	"github.com/jonhadfield/gosn-v2/session"
	"github.com/stretchr/testify/require"
)

func TestDecryptExportFixture(t *testing.T) {
	t.Parallel()

	// Fixture password matches the account used to create
	// testuser-encrypted-backup.txt (see historically commented import tests).
	const password = "testuser" // ggignore

	s := &session.Session{
		KeyParams: auth.KeyParams{
			Identifier: "other-account@example.com",
		},
	}

	path := filepath.Join("..", "testuser-encrypted-backup.txt")
	got, err := decryptExport(s, path, password)
	require.NoError(t, err)
	require.NotEmpty(t, got)
	require.Nil(t, s.ImporterItemsKeys, "ImporterItemsKeys should be cleared after decrypt")

	var foundNoteOrTag bool
	for _, item := range got {
		ct := item.GetContentType()
		require.NotEqual(t, common.SNItemTypeItemsKey, ct, "items keys stay in the encrypted key list, not decrypted payload")
		if ct == common.SNItemTypeNote || ct == common.SNItemTypeTag {
			foundNoteOrTag = true
		}
	}
	require.True(t, foundNoteOrTag, "export fixture should decrypt at least one note or tag")
}

func TestSessionItemsKeyFromItemsKey(t *testing.T) {
	t.Parallel()

	ik := NewItemsKey()
	ik.UpdatedAt = "2026-01-01T00:00:00.000Z"
	ik.UpdatedAtTimestamp = 1
	sik := SessionItemsKeyFromItemsKey(ik)
	require.Equal(t, ik.UUID, sik.UUID)
	require.Equal(t, ik.ItemsKey, sik.ItemsKey)
	require.Equal(t, ik.CreatedAtTimestamp, sik.CreatedAtTimestamp)
	require.Equal(t, ik.UpdatedAtTimestamp, sik.UpdatedAtTimestamp)
}
