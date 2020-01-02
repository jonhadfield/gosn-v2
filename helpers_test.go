package gosn

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStringInSliceCaseSensitive(t *testing.T) {
	assert.True(t, stringInSlice("Marmite", []string{"Cheese", "Marmite", "Toast"}, true))
}

func TestStringInSliceCaseInsensitive(t *testing.T) {
	assert.True(t, stringInSlice("marmite", []string{"Cheese", "Marmite", "Toast"}, false))
}
