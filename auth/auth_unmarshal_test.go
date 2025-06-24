package auth_test

import (
	"net/http"
	"testing"

	auth "github.com/jonhadfield/gosn-v2/auth"
	"github.com/stretchr/testify/require"
)

func TestUnmarshalAuthRequestResponseOK(t *testing.T) {
	t.Parallel()

	body := []byte(`{"data":{"identifier":"id","pw_salt":"salt","pw_cost":1,"pw_nonce":"nonce","version":"004"}}`)
	out, errResp, err := auth.UnmarshalAuthRequestResponseForTest(http.StatusOK, body, false)
	require.NoError(t, err)
	require.Equal(t, "id", out.Data.Identifier)
	require.Empty(t, errResp.Data.Error.Message)
}

func TestUnmarshalAuthRequestResponseNotFound(t *testing.T) {
	t.Parallel()

	body := []byte(`{"meta":{},"data":{"error":{"tag":"invalid","message":"not found","payload":{"mfa_key":"abc"}}}}`)
	out, errResp, err := auth.UnmarshalAuthRequestResponseForTest(http.StatusNotFound, body, false)
	require.NoError(t, err)
	require.Equal(t, "abc", errResp.Data.Error.Payload.MFAKey)
	require.Empty(t, out.Data.Identifier)
}

func TestUnmarshalAuthRequestResponseForbidden(t *testing.T) {
	t.Parallel()

	_, _, err := auth.UnmarshalAuthRequestResponseForTest(http.StatusForbidden, []byte(`{}`), false)
	require.Error(t, err)
}
