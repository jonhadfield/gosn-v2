package gosn

import (
	"encoding/hex"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

var testEmailAddr = fmt.Sprintf("testuser-%s@example.com", time.Now().Format("20060102150405"))

// ### server not required for following tests
func TestGenerateSalt004(t *testing.T) {
	identifier := "sn004@lessknown.co.uk"
	nonce := "2c409996650e46c748856fbd6aa549f89f35be055a8f9bfacdf0c4b29b2152e9"
	decodedHex64, _ := hex.DecodeString("7129955dbbbfb376fdcac49890ef17bc")
	assert.Equal(t, decodedHex64, generateSalt(identifier, nonce))
}

func TestGenerateEncryptedPasswordWithValidInput004(t *testing.T) {
	var testInput generateEncryptedPasswordInput
	testInput.userPassword = "debugtest"
	testInput.Identifier = "sn004@lessknown.co.uk"
	testInput.PasswordNonce = "2c409996650e46c748856fbd6aa549f89f35be055a8f9bfacdf0c4b29b2152e9"
	masterKey, serverPassword, err := generateMasterKeyAndServerPassword004(testInput)
	assert.NoError(t, err)
	assert.Equal(t, "2396d6ac0bc70fe45db1d2bcf3daa522603e9c6fcc88dc933ce1a3a31bbc08ed", masterKey)
	assert.Equal(t, "a5eb9fbc767eafd6e54fd9d3646b19520e038ba2ccc9cceddf2340b37b788b47", serverPassword)
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
	assert.Equal(t, result, "1312fe421aa49a6444684b58cbd5a43a55638cd5bf77514c78d50c7f3ae9c4e7")
}

// server required for following tests
func TestSignIn(t *testing.T) {
	sOutput, err := SignIn(sInput)
	assert.NoError(t, err, "sign-in failed", err)

	if sOutput.Session.AccessToken == "" || sOutput.Session.RefreshToken == "" || sOutput.Session.RefreshExpiration == 0 || sOutput.Session.AccessExpiration == 0 {
		t.Errorf("SignIn Failed")
	}
}

//
//func TestRegistrationAndSignInWithNewCredentials(t *testing.T) {
//	emailAddr := testEmailAddr
//	password := "secretsanta"
//
//	rInput := RegisterInput{
//		Password:   password,
//		Email:      emailAddr,
//		Identifier: emailAddr,
//		// PWNonce:     "",
//		Version: defaultSNVersion,
//		// Origination: "",
//		// Created:     0,
//		// API:         "",
//		APIServer: os.Getenv("SN_SERVER"),
//		Debug:     true,
//	}
//
//	// fmt.Printf("rInput: %+v\n", rInput)
//	_, err := rInput.Register()
//	// fmt.Println("X token:", token)
//	assert.NoError(t, err, "registration failed")
//
//	postRegSignInInput := SignInInput{
//		APIServer: os.Getenv("SN_SERVER"),
//		Email:     emailAddr,
//		Password:  password,
//	}
//	_, err = SignIn(postRegSignInInput)
//	assert.NoError(t, err, err)
//}

func TestRegistrationWithPreRegisteredEmail(t *testing.T) {
	password := "secret"
	rInput := RegisterInput{
		Email:     testEmailAddr,
		Password:  password,
		APIServer: os.Getenv("SN_SERVER"),
	}
	_, err := rInput.Register()
	assert.Error(t, err, "email is already registered")
}

//
//func TestSignInWithInvalidEmail(t *testing.T) {
//	password := "secret"
//	sInput := SignInInput{
//		Email:     "invalid@example.com",
//		Password:  password,
//		APIServer: os.Getenv("SN_SERVER"),
//	}
//	_, err := SignIn(sInput)
//	assert.Error(t, err)
//	assert.Contains(t, err.Error(), "Unable to locate account for email.")
//}

func TestSignInWithBadPassword(t *testing.T) {
	password := "invalid"
	sInput := SignInInput{
		Email:     "sn@lessknown.co.uk",
		Password:  password,
		APIServer: os.Getenv("SN_SERVER"),
	}
	_, err := SignIn(sInput)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid email or password")
}

func TestSignInWithUnresolvableHost(t *testing.T) {
	password := "invalid"
	sInput := SignInInput{
		Email:     "sn@lessknown.co.uk",
		Password:  password,
		APIServer: "https://standardnotes.example.com:443",
	}
	_, err := SignIn(sInput)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "standardnotes.example.com cannot be resolved")
}

func TestSignInWithInvalidURL(t *testing.T) {
	password := "invalid"
	sInput := SignInInput{
		Email:     "sn@lessknown.co.uk",
		Password:  password,
		APIServer: "standardnotes.example.com:443",
	}
	_, err := SignIn(sInput)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "protocol is missing from API server URL: standardnotes.example.com")
}

//func TestSignInWithServerActivelyRefusing(t *testing.T) {
//	password := "invalid"
//	sInput := SignInInput{
//		Email:     "sn@lessknown.co.uk",
//		Password:  password,
//		APIServer: "https://255.255.255.255:443",
//	}
//	_, err := SignIn(sInput)
//	assert.Error(t, err)
//	assert.Equal(t, fmt.Sprintf("failed to connect to https://255.255.255.255:443/auth/params"), err.Error())
//}
//
//func TestSignInWithUnavailableServer(t *testing.T) {
//	password := "invalid"
//	sInput := SignInInput{
//		Email:     "sn@lessknown.co.uk",
//		Password:  password,
//		APIServer: "https://10.10.10.10:6000",
//	}
//	_, err := SignIn(sInput)
//	assert.Error(t, err)
//	assert.Equal(t, err.Error(), fmt.Sprintf("failed to connect to %s within %d seconds",
//		"https://10.10.10.10:6000/auth/params", connectionTimeout))
//}
