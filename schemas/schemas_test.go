package schemas

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadSchemas(t *testing.T) {
	ts, err := LoadSchemas()
	require.NoError(t, err)
	require.NotNil(t, ts)

	for k, v := range ts {
		require.NotEmpty(t, k)
		require.NotEmpty(t, v)
	}
}
