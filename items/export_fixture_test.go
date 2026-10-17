package items

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/jonhadfield/gosn-v2/common"
	"github.com/stretchr/testify/require"
)

// TestReadExportJSONFixtures covers parsing Standard Notes encrypted backup
// files. Decrypt of fixtures is covered by TestDecryptExportFixture; live
// round-trip Export/Import still needs a Standard Notes account.
func TestReadExportJSONFixtures(t *testing.T) {
	t.Parallel()

	fixtures := []struct {
		name            string
		path            string
		minItems        int
		requireItemsKey bool
	}{
		{
			name:            "test.json",
			path:            filepath.Join("..", "test.json"),
			minItems:        2,
			requireItemsKey: true,
		},
		{
			name:            "testuser-encrypted-backup.txt",
			path:            filepath.Join("..", "testuser-encrypted-backup.txt"),
			minItems:        1,
			requireItemsKey: true,
		},
	}

	for _, tc := range fixtures {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			items, keyParams, err := readJSON(tc.path)
			require.NoError(t, err)
			require.GreaterOrEqual(t, len(items), tc.minItems)
			require.NotEmpty(t, keyParams.Identifier)
			require.Equal(t, common.DefaultSNVersion, keyParams.Version)
			require.NotEmpty(t, keyParams.PwNonce)

			if tc.requireItemsKey {
				var foundKey bool
				for _, item := range items {
					if item.ContentType == common.SNItemTypeItemsKey {
						foundKey = true
						require.True(t, strings.HasPrefix(item.Content, common.DefaultSNVersion+":"))
						break
					}
				}
				require.True(t, foundKey, "export fixture should include an SN|ItemsKey")
			}
		})
	}
}
