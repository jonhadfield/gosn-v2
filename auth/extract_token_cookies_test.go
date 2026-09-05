package auth

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// extractTokenCookies feeds the cookie values that RequestRefreshTokenWithSession
// surfaces so cookie-based sessions can refresh their manually-tracked cookies
// after a token rotation.
func TestExtractTokenCookies(t *testing.T) {
	t.Run("extracts both access and refresh cookies", func(t *testing.T) {
		headers := []string{
			"access_token_abc=AT123; Path=/; HttpOnly; Secure; Partitioned",
			"refresh_token_abc=RT456; Path=/; HttpOnly; Secure; Partitioned",
		}

		access, refresh := extractTokenCookies(headers)
		require.Equal(t, "access_token_abc=AT123", access)
		require.Equal(t, "refresh_token_abc=RT456", refresh)
	})

	t.Run("ignores unrelated cookies", func(t *testing.T) {
		headers := []string{
			"session_id=zzz; Path=/",
			"access_token_x=AT; Secure",
		}

		access, refresh := extractTokenCookies(headers)
		require.Equal(t, "access_token_x=AT", access)
		require.Empty(t, refresh)
	})

	t.Run("returns empty for no headers", func(t *testing.T) {
		access, refresh := extractTokenCookies(nil)
		require.Empty(t, access)
		require.Empty(t, refresh)
	})

	t.Run("skips blank header values", func(t *testing.T) {
		headers := []string{"", "   ", "refresh_token_y=RT; Path=/"}

		access, refresh := extractTokenCookies(headers)
		require.Empty(t, access)
		require.Equal(t, "refresh_token_y=RT", refresh)
	})
}
