package cache

import (
	"testing"

	"github.com/jonhadfield/gosn-v2/common"
	"github.com/stretchr/testify/require"
)

func TestItemsValidateRejectsProtocol003(t *testing.T) {
	t.Parallel()

	items := Items{
		{
			UUID:        "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
			ContentType: common.SNItemTypeNote,
			Content:     "003:deadbeef:ciphertext:auth",
			EncItemKey:  "003:nonce:key:auth",
			ItemsKeyID:  "ik-1",
		},
	}

	err := items.Validate()
	require.Error(t, err)
	require.Contains(t, err.Error(), "legacy protocol 003")
	require.Contains(t, err.Error(), common.SNItemTypeNote)
}

func TestItemsValidateAcceptsProtocol004(t *testing.T) {
	t.Parallel()

	items := Items{
		{
			UUID:        "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
			ContentType: common.SNItemTypeNote,
			Content:     "004:deadbeef:ciphertext:auth",
			EncItemKey:  "004:nonce:key:auth",
			ItemsKeyID:  "ik-1",
		},
	}

	require.NoError(t, items.Validate())
}
