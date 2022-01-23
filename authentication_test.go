package gosn

import (
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

var (
	testEmailAddr         = fmt.Sprintf("testuser-%s@example.com", time.Now().Format("20060102150405"))
	testEmailAddrWithPlus = fmt.Sprintf("test+user-%s@example.com", time.Now().Format("20060102150405"))
)

// ### server not required for following tests.
func TestGenerateSalt004(t *testing.T) {
	identifier := "sn004@lessknown.co.uk"
	nonce := "2c409996650e46c748856fbd6aa549f89f35be055a8f9bfacdf0c4b29b2152e9"
	decodedHex64, _ := hex.DecodeString("7129955dbbbfb376fdcac49890ef17bc")
	require.Equal(t, decodedHex64, generateSalt(identifier, nonce))
}

func TestGenerateEncryptedPasswordWithValidInput004(t *testing.T) {
	var testInput generateEncryptedPasswordInput
	testInput.userPassword = "debugtest"
	testInput.Identifier = "sn004@lessknown.co.uk"
	testInput.PasswordNonce = "2c409996650e46c748856fbd6aa549f89f35be055a8f9bfacdf0c4b29b2152e9"
	masterKey, serverPassword, err := generateMasterKeyAndServerPassword004(testInput)
	require.NoError(t, err)
	require.Equal(t, "2396d6ac0bc70fe45db1d2bcf3daa522603e9c6fcc88dc933ce1a3a31bbc08ed", masterKey)
	require.Equal(t, "a5eb9fbc767eafd6e54fd9d3646b19520e038ba2ccc9cceddf2340b37b788b47", serverPassword)
}

func TestGenerateEncryptedPasswordWithValidInput(t *testing.T) {
	var testInput generateEncryptedPasswordInput
	testInput.userPassword = "oWB7c&77Zahw8XK$AUy#"
	testInput.Identifier = "soba@lessknown.co.uk"
	testInput.PasswordNonce = "9e88fc67fb8b1efe92deeb98b5b6a801c78bdfae08eecb315f843f6badf60aef"
	testInput.PasswordCost = 110000
	testInput.Version = "003"
	testInput.PasswordSalt = ""
	result, _, _, _ := generateEncryptedPasswordAndKeys(testInput)
	require.Equal(t, result, "1312fe421aa49a6444684b58cbd5a43a55638cd5bf77514c78d50c7f3ae9c4e7")
}

// server required for following tests.
func TestSignIn(t *testing.T) {
	_, err := SignIn(sInput)
	require.NoError(t, err, "sign-in failed", err)

	if testSession.AccessToken == "" || testSession.RefreshToken == "" || testSession.RefreshExpiration == 0 || testSession.AccessExpiration == 0 {
		t.Errorf("SignIn Failed")
	}
}

func TestRegistrationAndSignInWithNewCredentials(t *testing.T) {
	if strings.Contains(os.Getenv("SN_SERVER"), "ramea") {
		emailAddr := testEmailAddr
		password := "secretsanta"

		rInput := RegisterInput{
			Password:   password,
			Email:      emailAddr,
			Identifier: emailAddr,
			Version:    defaultSNVersion,
			APIServer:  os.Getenv("SN_SERVER"),
			Debug:      true,
		}

		_, err := rInput.Register()
		require.NoError(t, err, "registration failed")

		postRegSignInInput := SignInInput{
			APIServer: os.Getenv("SN_SERVER"),
			Email:     emailAddr,
			Password:  password,
		}
		sio, err := SignIn(postRegSignInInput)
		require.NoError(t, err, err)

		so, err := Sync(SyncInput{
			Session: &sio.Session,
		})
		require.NoError(t, err)
		require.GreaterOrEqual(t, len(so.Items), 1)
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
		password := "secret"
		sInput := SignInInput{
			Email:     testEmailAddrWithPlus,
			Password:  password,
			APIServer: os.Getenv("SN_SERVER"),
			Debug:     true,
		}
		_, err := SignIn(sInput)
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid email or password")
	}
}

func TestSignInWithInvalidEmail(t *testing.T) {
	password := "secret"
	sInput := SignInInput{
		Email:     "invalid@example.com",
		Password:  password,
		APIServer: os.Getenv("SN_SERVER"),
		Debug:     true,
	}
	_, err := SignIn(sInput)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid email or password")
}

func TestSignInWithBadPassword(t *testing.T) {
	password := "invalid"
	sInput := SignInInput{
		Email:     "sn@lessknown.co.uk",
		Password:  password,
		APIServer: os.Getenv("SN_SERVER"),
		Debug:     true,
	}
	_, err := SignIn(sInput)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid email or password")
}

func TestSignInWithUnresolvableHost(t *testing.T) {
	password := "invalid"
	sInput := SignInInput{
		Email:     "sn@lessknown.co.uk",
		Password:  password,
		APIServer: "https://standardnotes.example.com:443",
		Debug:     true,
	}
	_, err := SignIn(sInput)
	require.Error(t, err)
	require.Contains(t, err.Error(), "standardnotes.example.com cannot be resolved")
}

func TestSignInWithInvalidURL(t *testing.T) {
	password := "invalid"
	sInput := SignInInput{
		Email:     "sn@lessknown.co.uk",
		Password:  password,
		APIServer: "standardnotes.example.com:443",
		Debug:     true,
	}
	_, err := SignIn(sInput)
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
	password := "invalid"
	sInput := SignInInput{
		Email:     "sn@lessknown.co.uk",
		Password:  password,
		APIServer: "https://10.10.10.10:6000",
		Debug:     true,
	}
	_, err := SignIn(sInput)
	require.Error(t, err)
	require.Equal(t, fmt.Sprintf("failed to connect to %s within %d seconds",
		"https://10.10.10.10:6000/v1/login-params", connectionTimeout), err.Error())
}
