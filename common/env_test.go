package common

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseEnvInt64Unset(t *testing.T) {
	os.Unsetenv("TEST_INT64")
	val, ok, err := ParseEnvInt64("TEST_INT64")
	require.NoError(t, err)
	require.False(t, ok)
	require.Equal(t, int64(0), val)
}

func TestParseEnvInt64Valid(t *testing.T) {
	os.Setenv("TEST_INT64", "42")
	defer os.Unsetenv("TEST_INT64")

	val, ok, err := ParseEnvInt64("TEST_INT64")
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, int64(42), val)
}

func TestParseEnvInt64Invalid(t *testing.T) {
	os.Setenv("TEST_INT64", "abc")
	defer os.Unsetenv("TEST_INT64")

	_, ok, err := ParseEnvInt64("TEST_INT64")
	require.Error(t, err)
	require.False(t, ok)
}
