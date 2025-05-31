package session

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"

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

func (k MockKeyRingDodgy) DeleteAll(service string) error {
	return fmt.Errorf("failed to delete all sessions")
}

type MockKeyRingDefined struct{}

func (k MockKeyRingDefined) Set(user, service, password string) error {
	return nil
}

func (k MockKeyRingDefined) Get(service, user string) (r string, err error) {
	return "{\"Server\":\"http://ramea:3000\",\"Token\":\"\",\"MasterKey\":\"03f7f410be71838897d35cacec799503355d486a0ef4a1e3e5f64abf262a640f\",\"keyParams\":{\"created\":\"1693053961818\",\"identifier\":\"ramea@lessknown.co.uk\",\"origination\":\"registration\",\"pw_nonce\":\"93ed73375de052cb233fc9914fe8a2a264492b74ebc2968e17eb44d451ced614\",\"version\":\"004\"},\"access_token\":\"1:7d90df15-7d74-46e0-a7c6-4cfa1b70f42e:ODAyZWRlMTg2ZjM5\",\"refresh_token\":\"1:7d90df15-7d74-46e0-a7c6-4cfa1b70f42e:YTU4NWY3MmIxMmIz\",\"access_expiration\":1698262966000,\"refresh_expiration\":1724635892000}", nil
}

func (k MockKeyRingDefined) Delete(service, user string) error {
	return nil
}

func (k MockKeyRingDefined) DeleteAll(service string) error {
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

func (k MockKeyRingUnDefined) DeleteAll(service string) error {
	return nil
}

func TestMakeSessionString(t *testing.T) {
	sessionString := "{\"Server\":\"http://ramea:3000\",\"Token\":\"\",\"MasterKey\":\"5319f9c148ee3dbe78fc149e8643775242d7e83216060ee5e228ab2ec3d88a76\",\"keyParams\":{\"created\":\"1608473387799\",\"identifier\":\"test-user\",\"origination\":\"registration\",\"pw_nonce\":\"yJTyMmLr3KqOc7ifRfe1H7Pu8591n7Sj\",\"version\":\"004\"},\"access_token\":\"1:3dc699a1-451d-4de3-b01d-fca32554292b:Io9MOsc.WDIq0JBt\",\"refresh_token\":\"1:3dc699a1-451d-4de3-b01d-fca32554292b:-ixQV.-RMCCSPG0M\",\"access_expiration\":1648647400000,\"refresh_expiration\":1675020326000,\"SchemaValidation\":false}"

	var session Session

	require.NoError(t, json.Unmarshal([]byte(sessionString), &session))

	ss := makeMinimalSessionString(session)
	require.Equal(t, sessionString, ss)
}

func TestWriteSession(t *testing.T) {
	if os.Getenv(common.EnvSkipSessionTests) != "" {
		t.Skip("skipping session test")
	}

	var kEmpty MockKeyRingDodgy

	require.Error(t, writeSession("example", kEmpty))

	var kDefined MockKeyRingDefined
	require.NoError(t, writeSession("example", kDefined))

	_, _, _ = GetSession(nil, true, "SNServer", os.Getenv(common.EnvServer), true)

	require.NoError(t, SessionExists(kDefined))
}

// func TestAddSession(t *testing.T) {
// 	if os.Getenv("SN_SKIP_SESSION_TESTS") != "" {
// 		t.Skip("skipping session test")
// 	}
//
// 	viper.SetEnvPrefix("sn")
// 	require.NoError(t, viper.BindEnv("email"))
// 	require.NoError(t, viper.BindEnv("password"))
// 	require.NoError(t, viper.BindEnv("server"))
//
// 	serverURL := os.Getenv("SN_SERVER")
// 	if serverURL == "" {
// 		serverURL = SNServerURL
// 	}
//
// 	k := MockKeyRingDefined{}
// 	RemoveSession(k)
// 	_, err := AddSession(nil, serverURL, "", k, true)
// 	require.NoError(t, err)
// 	_, _, err = GetSession(nil, true, "", serverURL, true)
// 	require.NoError(t, err)
// }

func TestSessionExists(t *testing.T) {
	if os.Getenv(common.EnvSkipSessionTests) != "" {
		t.Skip("skipping session test")
	}

	var kEmpty MockKeyRingUnDefined
	require.Error(t, SessionExists(kEmpty))

	var kDefined MockKeyRingDefined
	require.NoError(t, SessionExists(kDefined))
}

func TestRemoveSession(t *testing.T) {
	if os.Getenv(common.EnvSkipSessionTests) != "" {
		t.Skip("skipping session test")
	}

	var kUndefined MockKeyRingUnDefined

	require.Contains(t, RemoveSession(kUndefined), "failed")

	var kDefined MockKeyRingDefined

	require.Contains(t, RemoveSession(kDefined), "success")
}

func TestSessionStatus(t *testing.T) {
	if os.Getenv(common.EnvSkipSessionTests) != "" {
		t.Skip("skipping session test")
	}

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
	require.Contains(t, s, "user: ramea@lessknown.co.uk")

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

func TestAddSessionWithoutExistingEnvVars(t *testing.T) {
	if os.Getenv(common.EnvSkipSessionTests) != "" {
		t.Skip("skipping session test")
	}

	_ = os.Unsetenv(common.EnvServer)
	_ = os.Unsetenv(common.EnvEmail)
	_ = os.Unsetenv(common.EnvPassword)

	serverURL := os.Getenv(common.EnvServer)
	if serverURL == "" {
		serverURL = SNServerURL
	}

	_, err := AddSession(nil, serverURL, "", MockKeyRingUnDefined{}, true)
	require.NoError(t, err)
}
