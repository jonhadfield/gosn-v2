package auth

import (
	"encoding/json"
	"testing"

	"github.com/jonhadfield/gosn-v2/common"
	"github.com/stretchr/testify/require"
)

func TestGenerateAuthDataItemsKey(t *testing.T) {
	kp := KeyParams{
		Created:     "123",
		Identifier:  "user@example.com",
		Origination: "registration",
		PwNonce:     "nonce",
		Version:     "004",
	}

	data := GenerateAuthData(common.SNItemTypeItemsKey, "uuid1", kp)

	var out struct {
		KP KeyParams `json:"kp"`
		U  string    `json:"u"`
		V  string    `json:"v"`
	}

	require.NoError(t, json.Unmarshal([]byte(data), &out))
	require.Equal(t, kp, out.KP)
	require.Equal(t, "uuid1", out.U)
	require.Equal(t, kp.Version, out.V)
}

func TestGenerateAuthDataDefault(t *testing.T) {
	kp := KeyParams{Version: "004"}

	data := GenerateAuthData(common.SNItemTypeNote, "uuid2", kp)

	var out map[string]string
	require.NoError(t, json.Unmarshal([]byte(data), &out))
	require.Equal(t, "uuid2", out["u"])
	require.Equal(t, "004", out["v"])
}
