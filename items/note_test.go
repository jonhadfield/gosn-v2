package items

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNoteContentGetTrashed(t *testing.T) {
	var nc NoteContent
	require.False(t, nc.GetTrashed())

	nc.SetTrashed(false)
	require.False(t, nc.GetTrashed())

	nc.SetTrashed(true)
	require.True(t, nc.GetTrashed())
}
