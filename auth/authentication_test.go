package auth

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/go-retryablehttp"
	"github.com/jonhadfield/gosn-v2/common"
	"github.com/stretchr/testify/assert"

	"github.com/stretchr/testify/require"
)

var (
	testSession      *SignInResponseDataSession
	testUserEmail    string
	testUserPassword string
	sInput           = SignInInput{
		Email:     os.Getenv("SN_EMAIL"),
		Password:  os.Getenv("SN_PASSWORD"),
		APIServer: os.Getenv("SN_SERVER"),
		Debug:     true,
	}
)

func localTestMain() {
	localServer := "http://ramea:3000"
	testUserEmail = fmt.Sprintf("ramea-%s", strconv.FormatInt(time.Now().UnixNano(), 16))
	testUserPassword = "secretsanta"

	rInput := RegisterInput{
		Client:    retryablehttp.NewClient(),
		Password:  testUserPassword,
		Email:     testUserEmail,
		APIServer: localServer,
		Version:   common.DefaultSNVersion,
		Debug:     true,
	}

	_, err := rInput.Register()
	if err != nil {
		panic(fmt.Sprintf("failed to register with: %s", localServer))
	}

	// auth.SignIn(localServer, testUserEmail, testUserPassword)
	SignIn(SignInInput{
		HTTPClient: retryablehttp.NewClient(),
		Email:      testUserEmail,
		Password:   testUserPassword,
		APIServer:  localServer,
		Debug:      false,
	})
}

func TestMain(m *testing.M) {
	if os.Getenv("SN_SERVER") == "" || strings.Contains(os.Getenv("SN_SERVER"), "ramea") {
		localTestMain()
	} else {
		httpClient := common.NewHTTPClient()
		out, err := SignIn(SignInInput{
			HTTPClient: httpClient,
			Email:      os.Getenv("SN_EMAIL"),
			Password:   os.Getenv("SN_PASSWORD"),
			APIServer:  os.Getenv("SN_SERVER"),
			Debug:      true,
		})
		if err != nil {
			panic(fmt.Sprintf("failed to sign-in with: %s", os.Getenv("SN_SERVER")))
		}

		testSession = &SignInResponseDataSession{
			Debug:             true,
			HTTPClient:        httpClient,
			SchemaValidation:  false,
			Server:            "",
			FilesServerUrl:    "",
			Token:             "",
			MasterKey:         "",
			KeyParams:         out.KeyParams,
			AccessToken:       "",
			RefreshToken:      "",
			AccessExpiration:  0,
			RefreshExpiration: 0,
			ReadOnlyAccess:    false,
			PasswordNonce:     "",
		}

		if strings.ToLower(os.Getenv("SN_DEBUG")) == "true" {
			testSession.Debug = true
		}

		// if _, err := items.Sync(items.SyncInput{Session: testSession}); err != nil {
		// 	log.Fatal(err)
		// }
		//
		// var err error
		// testSession.Schemas, err = session.LoadSchemas()
		// if err != nil {
		// 	log.Fatal(err)
		// }
		//
		// if len(testSession.Schemas) == 0 {
		// 	log.Fatal("failed to load schemas")
		// }

		os.Exit(m.Run())
	}
}

var (
	testEmailAddr         = fmt.Sprintf("testuser-%s@example.com", time.Now().Format("20060102150405"))
	testEmailAddrWithPlus = fmt.Sprintf("test+user-%s@example.com", time.Now().Format("20060102150405"))
)

// func TestGenerateEncryptedPasswordWithValidInput(t *testing.T) {
//	var testInput generateEncryptedPasswordInput
//	testInput.userPassword = "oWB7c&77Zahw8XK$AUy#"
//	testInput.Identifier = "soba@lessknown.co.uk"
//	testInput.PasswordNonce = "9e88fc67fb8b1efe92deeb98b5b6a801c78bdfae08eecb315f843f6badf60aef"
//	testInput.PasswordCost = 110000
//	testInput.Version = "003"
//	testInput.PasswordSalt = ""
//	result, _, _, _ := generateEncryptedPasswordAndKeys(testInput)
//	require.Equal(t, result, "1312fe421aa49a6444684b58cbd5a43a55638cd5bf77514c78d50c7f3ae9c4e7")
// }

// server required for following tests.
func TestSignIn(t *testing.T) {
	sInput.HTTPClient = retryablehttp.NewClient()
	sOut, err := SignIn(sInput)
	require.NoError(t, err, "sign-in failed", err)

	testSession = &sOut.Session

	if testSession.AccessToken == "" || testSession.RefreshToken == "" || testSession.RefreshExpiration == 0 || testSession.AccessExpiration == 0 {
		t.Errorf("SignIn Failed with %s", os.Getenv("SN_SERVER"))
	}
}

func TestRefreshSession(t *testing.T) {
	so, err := SignIn(sInput)
	require.NoError(t, err, "sign-in failed", err)

	preAccessToken := so.Session.AccessToken
	preAccessExpiration := so.Session.AccessExpiration
	preRefreshToken := so.Session.RefreshToken
	preRefreshExpiration := so.Session.RefreshExpiration

	// wait for 2 seconds to ensure that the expiration times are different
	time.Sleep(1 * time.Second)
	rt, err := RequestRefreshToken(so.Session.HTTPClient, os.Getenv("SN_SERVER")+common.AuthRefreshPath, so.Session.AccessToken, so.Session.RefreshToken, true)
	require.NoError(t, err)
	require.NotEmpty(t, rt.Data.Session.AccessToken)
	require.NotEmpty(t, rt.Data.Session.RefreshToken)
	require.NotEmpty(t, rt.Data.Session.RefreshExpiration)
	require.NotEmpty(t, rt.Data.Session.AccessExpiration)
	assert.NotEqual(t, preAccessToken, rt.Data.Session.AccessToken)
	assert.NotEqual(t, preAccessExpiration, rt.Data.Session.AccessExpiration)
	assert.NotEqual(t, preRefreshToken, rt.Data.Session.RefreshToken)
	assert.NotEqual(t, preRefreshExpiration, rt.Data.Session.RefreshExpiration)
}

func TestRegistrationWithInvalidShortPassword(t *testing.T) {
	password := "secret"
	rInput := RegisterInput{
		Email:     testEmailAddr,
		Password:  password,
		APIServer: os.Getenv("SN_SERVER"),
	}
	_, err := rInput.Register()
	require.Error(t, err)
	require.Equal(t, err.Error(), fmt.Sprintf("password must be at least %d characters", common.MinPasswordLength))
}

func TestRegistrationAndSignInWithNewCredentials(t *testing.T) {
	if strings.Contains(os.Getenv("SN_SERVER"), "ramea") {
		emailAddr := testEmailAddr
		password := "secretsanta"

		rInput := RegisterInput{
			Password:  password,
			Email:     emailAddr,
			Version:   common.DefaultSNVersion,
			APIServer: os.Getenv("SN_SERVER"),
			Debug:     true,
		}

		_, err := rInput.Register()
		require.NoError(t, err, "registration failed")

		postRegSignInInput := SignInInput{
			APIServer: os.Getenv("SN_SERVER"),
			Email:     emailAddr,
			Password:  password,
		}
		_, err = SignIn(postRegSignInInput)
		require.NoError(t, err, err)

		//
		// so, err := items.Sync(items.SyncInput{
		// 	Session: &sio.Session,
		// })
		// require.NoError(t, err)
		// require.GreaterOrEqual(t, len(so.Items), 1)
	}
}

func TestRegistrationWithPreRegisteredEmail(t *testing.T) {
	if strings.Contains(os.Getenv("SN_SERVER"), "ramea") {
		password := "secret"
		rInput := RegisterInput{
			Email:     testEmailAddr,
			Password:  password,
			APIServer: os.Getenv("SN_SERVER"),
		}
		_, err := rInput.Register()
		require.Error(t, err, "email is already registered")
	}
}

func TestRegistrationAndSignInWithEmailWithPlusSign(t *testing.T) {
	if strings.Contains(os.Getenv("SN_SERVER"), "ramea") {
		_, err := SignIn(SignInInput{
			Email:     testEmailAddrWithPlus,
			Password:  "secret",
			APIServer: os.Getenv("SN_SERVER"),
			Debug:     true,
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid email or password")
	}
}

// func TestSignInWithInvalidEmail(t *testing.T) {
// 	_, err := SignIn(SignInInput{
// 		Email:     "invalid@example.com",
// 		Password:  "secret",
// 		APIServer: os.Getenv("SN_SERVER"),
// 		Debug:     true,
// 	})
// 	require.Error(t, err)
// 	require.Contains(t, err.Error(), "invalid email or password")
// }

// func TestSignInWithBadPassword(t *testing.T) {
// 	_, err := SignIn(SignInInput{
// 		Email:     "invalid@lessknown.co.uk",
// 		Password:  "invalid",
// 		APIServer: os.Getenv("SN_SERVER"),
// 		Debug:     true,
// 	})
// 	require.NoError(t, err)
// 	require.Contains(t, err.Error(), "invalid email or password")
// }

func TestSignInWithUnresolvableHost(t *testing.T) {
	_, err := SignIn(SignInInput{
		Email:     "sn@lessknown.co.uk",
		Password:  "invalid",
		APIServer: "https://standardnotes.example.com:443",
		Debug:     true,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "standardnotes.example.com cannot be resolved")
}

func TestSignInWithInvalidURL(t *testing.T) {
	_, err := SignIn(SignInInput{
		Email:     "sn@lessknown.co.uk",
		Password:  "invalid",
		APIServer: "standardnotes.example.com:443",
		Debug:     true,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "protocol is missing from API server URL: standardnotes.example.com")
}

// Need to revise this test as different test platforms respond differently to the request
//  func TestSignInWithServerActivelyRefusing(t *testing.T) {
//	  password := "invalid"
//	  sInput := SignInInput{
// 		Email:     "sn@lessknown.co.uk",
//		Password:  password,
//		APIServer: "http://255.255.255.255",
//    }
//	  _, err := SignIn(sInput)
//	  require.Error(t, err)
//	  require.Equal(t, fmt.Sprintf("failed to connect to http://255.255.255.255/v1/login-params"), err.Error())
//   }

func TestSignInWithUnavailableServer(t *testing.T) {
	if os.Getenv("SN_SERVER") == "http://ramea:3000" {
		return
	}

	client := retryablehttp.NewClient()
	client.RetryMax = 1
	client.RetryWaitMin = 1 * time.Second
	client.RetryWaitMax = 2 * time.Second
	client.Logger = nil

	_, err := SignIn(SignInInput{
		HTTPClient: client,
		Email:      "sn@lessknown.co.uk",
		Password:   "invalid",
		APIServer:  "https://10.10.10.10:6000",
		Debug:      true,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to connect")
}
