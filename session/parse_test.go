package session

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseSessionStringValid(t *testing.T) {
	sessionString := "{\"Server\":\"http://ramea:3000\",\"Token\":\"\",\"MasterKey\":\"5319f9c148ee3dbe78fc149e8643775242d7e83216060ee5e228ab2ec3d88a76\",\"keyParams\":{\"created\":\"1608473387799\",\"identifier\":\"test-user\",\"origination\":\"registration\",\"pw_nonce\":\"yJTyMmLr3KqOc7ifRfe1H7Pu8591n7Sj\",\"version\":\"004\"},\"access_token\":\"1:3dc699a1-451d-4de3-b01d-fca32554292b:Io9MOsc.WDIq0JBt\",\"refresh_token\":\"1:3dc699a1-451d-4de3-b01d-fca32554292b:-ixQV.-RMCCSPG0M\",\"access_expiration\":1648647400000,\"refresh_expiration\":1675020326000,\"SchemaValidation\":false}"

	sess, err := ParseSessionString(sessionString)
	require.NoError(t, err)
	require.Equal(t, "http://ramea:3000", sess.Server)
	require.Equal(t, "5319f9c148ee3dbe78fc149e8643775242d7e83216060ee5e228ab2ec3d88a76", sess.MasterKey)
	require.Equal(t, int64(1648647400000), sess.AccessExpiration)
}

func TestParseSessionStringInvalid(t *testing.T) {
	_, err := ParseSessionString("not-json")
	require.Error(t, err)
}
