package gosn

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

type MockKeyRingDodgy struct{}

func (k MockKeyRingDodgy) Set(user, service, password string) error {
	return fmt.Errorf("failed to set Session")
}

func (k MockKeyRingDodgy) Get(service, user string) (r string, err error) {
	return "an invalid Session", nil
}

func (k MockKeyRingDodgy) Delete(service, user string) error {
	return nil
}

type MockKeyRingDefined struct{}

func (k MockKeyRingDefined) Set(user, service, password string) error {
	return nil
}

func (k MockKeyRingDefined) Get(service, user string) (r string, err error) {
	return "{\"Server\":\"http://ramea:3000\",\"Token\":\"\",\"MasterKey\":\"5319f9c148ee3dbe78fc149e8643775242d7e83216060ee5e228ab2ec3d88a76\",\"keyParams\":{\"created\":\"1608473387799\",\"identifier\":\"test-user\",\"origination\":\"registration\",\"pw_nonce\":\"yJTyMmLr3KqOc7ifRfe1H7Pu8591n7Sj\",\"version\":\"004\"},\"access_token\":\"1:3dc699a1-451d-4de3-b01d-fca32554292b:Io9MOsc.WDIq0JBt\",\"refresh_token\":\"1:3dc699a1-451d-4de3-b01d-fca32554292b:-ixQV.-RMCCSPG0M\",\"access_expiration\":1648647400000,\"refresh_expiration\":1675020326000}", nil
}

func (k MockKeyRingDefined) Delete(service, user string) error {
	return nil
}

type MockKeyRingUnDefined struct{}

func (k MockKeyRingUnDefined) Set(user, service, password string) error {
	return nil
}

func (k MockKeyRingUnDefined) Get(service, user string) (r string, err error) {
	return "", nil
}

func (k MockKeyRingUnDefined) Delete(service, user string) error {
	return nil
}

func TestMakeSessionString(t *testing.T) {
	sessionString := "{\"Server\":\"http://ramea:3000\",\"Token\":\"\",\"MasterKey\":\"5319f9c148ee3dbe78fc149e8643775242d7e83216060ee5e228ab2ec3d88a76\",\"keyParams\":{\"created\":\"1608473387799\",\"identifier\":\"test-user\",\"origination\":\"registration\",\"pw_nonce\":\"yJTyMmLr3KqOc7ifRfe1H7Pu8591n7Sj\",\"version\":\"004\"},\"access_token\":\"1:3dc699a1-451d-4de3-b01d-fca32554292b:Io9MOsc.WDIq0JBt\",\"refresh_token\":\"1:3dc699a1-451d-4de3-b01d-fca32554292b:-ixQV.-RMCCSPG0M\",\"access_expiration\":1648647400000,\"refresh_expiration\":1675020326000}"

	var session Session
	require.NoError(t, json.Unmarshal([]byte(sessionString), &session))
	ss := makeMinimalSessionString(session)
	require.Equal(t, sessionString, ss)
}

func TestWriteSession(t *testing.T) {
	var kEmpty MockKeyRingDodgy

	require.Error(t, writeSession("example", kEmpty))

	var kDefined MockKeyRingDefined

	require.NoError(t, SessionExists(kDefined))
}

func TestAddSessionWithoutExistingEnvVars(t *testing.T) {
	_ = os.Unsetenv("SN_SERVER")
	_ = os.Unsetenv("SN_EMAIL")
	_ = os.Unsetenv("SN_PASSWORD")

	serverURL := os.Getenv("SN_SERVER")
	if serverURL == "" {
		serverURL = SNServerURL
	}

	_, err := AddSession(serverURL, "", MockKeyRingUnDefined{}, true)
	require.NoError(t, err)
}

func TestAddSession(t *testing.T) {
	viper.SetEnvPrefix("sn")
	require.NoError(t, viper.BindEnv("email"))
	require.NoError(t, viper.BindEnv("password"))
	require.NoError(t, viper.BindEnv("server"))

	serverURL := os.Getenv("SN_SERVER")
	if serverURL == "" {
		serverURL = SNServerURL
	}

	_, err := AddSession(serverURL, "", MockKeyRingUnDefined{}, true)
	require.NoError(t, err)
}

func TestSessionExists(t *testing.T) {
	var kEmpty MockKeyRingUnDefined

	require.Error(t, SessionExists(kEmpty))

	var kDefined MockKeyRingDefined

	require.NoError(t, SessionExists(kDefined))
}

func TestRemoveSession(t *testing.T) {
	var kUndefined MockKeyRingUnDefined

	require.Contains(t, RemoveSession(kUndefined), "failed")

	var kDefined MockKeyRingDefined

	require.Contains(t, RemoveSession(kDefined), "success")
}

func TestSessionStatus(t *testing.T) {
	// if Session is undefined then Session value should
	// be empty and error returned to reflect that
	var kUndefined MockKeyRingUnDefined
	s, err := SessionStatus("", kUndefined)
	require.Error(t, err)
	require.Contains(t, err.Error(), "empty")
	require.Empty(t, s)

	// if Session is not empty but a value is found then
	// assume Session is not encrypted
	var kDefined MockKeyRingDefined
	s, err = SessionStatus("", kDefined)
	require.NoError(t, err)
	require.Contains(t, s, "session found: test-user")

	// if stored Session value is not immediately valid
	// then Session is assumed to be encrypted so ensure
	// a key, if not provided, is flagged
	var kDodgy MockKeyRingDodgy
	s, err = SessionStatus("", kDodgy)
	require.Error(t, err)
	require.Contains(t, err.Error(), "key required")
	require.Empty(t, s)

	// if stored Session value is not immediately valid
	// then Session is assumed to be encrypted so ensure
	// Session that cannot be encrypted is flagged
	s, err = SessionStatus("somekey", kDodgy)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid")
	require.Empty(t, s)
}
