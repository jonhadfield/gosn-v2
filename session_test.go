package gosn

import (
	"fmt"
	"os"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

type MockKeyRingDodgy struct {
}

func (k MockKeyRingDodgy) Set(user, service, password string) error {
	return fmt.Errorf("failed to set Session")
}

func (k MockKeyRingDodgy) Get(service, user string) (r string, err error) {
	return "an invalid Session", nil
}

func (k MockKeyRingDodgy) Delete(service, user string) error {
	return nil
}

type MockKeyRingDefined struct {
}

func (k MockKeyRingDefined) Set(user, service, password string) error {
	return nil
}

func (k MockKeyRingDefined) Get(service, user string) (r string, err error) {
	return "someone@example.com;https://sync.standardnotes.org;eyJhbGciOiJKUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c;8f0f5166841ca4dee2975c74cc7e0a4345ce24b54d7b215677a3d540303aa203;1;6d5ffc6f8e337e6e3ae6d0c3201d9e2d00ffee64672bc4fe1886ad31770c19f1;2", nil
}
//testSession = fmt.Sprintf("%s;%s;%s;%s;%d;%s;%d", testSessionEmail, testSessionServer, testSessionMasterKey,
//testSessionAccessToken, testSessionAccessExpiration, testSessionRefreshToken, testSessionRefreshExpiration)

func (k MockKeyRingDefined) Delete(service, user string) error {
	return nil
}

type MockKeyRingUnDefined struct {
}

func (k MockKeyRingUnDefined) Set(user, service, password string) error {
	return nil
}

func (k MockKeyRingUnDefined) Get(service, user string) (r string, err error) {
	return "", nil
}

func (k MockKeyRingUnDefined) Delete(service, user string) error {
	return nil
}

// type Session struct {
//	Debug             bool
//	Server            string
//	Token             string
//	MasterKey         string
//	ItemsKeys         []ItemsKey
//	DefaultItemsKey   ItemsKey
//	AccessToken       string `json:"access_token"`
//	RefreshToken      string `json:"refresh_token"`
//	AccessExpiration  int64  `json:"access_expiration"`
//	RefreshExpiration int64  `json:"refresh_expiration"`
//}

var (
	testSessionEmail             = "me@home.com"
	testSessionServer            = "https://sync.server.com"
	testSessionMasterKey         = "testsessionmk"
	testSessionAccessToken       = "testsessionat"
	testSessionAccessExpiration  = 1
	testSessionRefreshToken      = "testsessionrt"
	testSessionRefreshExpiration = 2

	testSession = fmt.Sprintf("%s;%s;%s;%s;%d;%s;%d", testSessionEmail, testSessionServer, testSessionMasterKey,
		testSessionAccessToken, testSessionAccessExpiration, testSessionRefreshToken, testSessionRefreshExpiration)
)

func TestMakeSessionString(t *testing.T) {
	sess := Session{
		Debug:             false,
		Server:            testSessionServer,
		MasterKey:         testSessionMasterKey,
		AccessToken:       "testsessionat",
		RefreshToken:      "testsessionrt",
		AccessExpiration:  1,
		RefreshExpiration: 2,
	}
	ss := makeSessionString(testSessionEmail, sess)
	assert.Equal(t, testSession, ss)
}

func TestWriteSession(t *testing.T) {
	var kEmpty MockKeyRingDodgy

	assert.Error(t, writeSession("example", kEmpty))

	var kDefined MockKeyRingDefined

	assert.NoError(t, SessionExists(kDefined))
}

func TestAddSession(t *testing.T) {
	viper.SetEnvPrefix("sn")
	assert.NoError(t, viper.BindEnv("email"))
	assert.NoError(t, viper.BindEnv("password"))
	assert.NoError(t, viper.BindEnv("server"))

	serverURL := os.Getenv("SN_SERVER")
	if serverURL == "" {
		serverURL = SNServerURL
	}

	_, err := AddSession(serverURL, "", MockKeyRingUnDefined{})
	assert.NoError(t, err)
}

func TestSessionExists(t *testing.T) {
	var kEmpty MockKeyRingUnDefined

	assert.Error(t, SessionExists(kEmpty))

	var kDefined MockKeyRingDefined

	assert.NoError(t, SessionExists(kDefined))
}

func TestRemoveSession(t *testing.T) {
	var kUndefined MockKeyRingUnDefined

	assert.Contains(t, RemoveSession(kUndefined), "failed")

	var kDefined MockKeyRingDefined

	assert.Contains(t, RemoveSession(kDefined), "success")
}

func TestSessionStatus(t *testing.T) {
	// if Session is undefined then Session value should
	// be empty and error returned to reflect that
	var kUndefined MockKeyRingUnDefined
	s, err := SessionStatus("", kUndefined)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "empty")
	assert.Empty(t, s)

	// if Session is not empty but a value is found then
	// assume Session is not encrypted
	var kDefined MockKeyRingDefined
	s, err = SessionStatus("", kDefined)
	assert.NoError(t, err)
	assert.Contains(t, s, "Session found: someone@example.com")

	// if stored Session value is not immediately valid
	// then Session is assumed to be encrypted so ensure
	// a key, if not provided, is flagged
	var kDodgy MockKeyRingDodgy
	s, err = SessionStatus("", kDodgy)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "key required")
	assert.Empty(t, s)

	// if stored Session value is not immediately valid
	// then Session is assumed to be encrypted so ensure
	// Session that cannot be encrypted is flagged
	s, err = SessionStatus("somekey", kDodgy)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "corrupt")
	assert.Empty(t, s)
}
