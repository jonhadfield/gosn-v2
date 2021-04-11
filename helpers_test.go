package gosn

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStringInSliceCaseSensitive(t *testing.T) {
	require.True(t, stringInSlice("Marmite", []string{"Cheese", "Marmite", "Toast"}, true))
}

func TestStringInSliceCaseInsensitive(t *testing.T) {
	require.True(t, stringInSlice("marmite", []string{"Cheese", "Marmite", "Toast"}, false))
}
