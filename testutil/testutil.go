package testutil

import (
	"fmt"
	"strconv"
	"time"

	"github.com/hashicorp/go-retryablehttp"
	"github.com/jonhadfield/gosn-v2/auth"
	"github.com/jonhadfield/gosn-v2/common"
)

// RegisterAndSignInLocalUser registers and signs in a user on a local
// Standard Notes server used for testing. It returns the generated email
// and password used for the account.
func RegisterAndSignInLocalUser() (email, password string, err error) {
	localServer := "http://ramea:3000"
	email = fmt.Sprintf("ramea-%s", strconv.FormatInt(time.Now().UnixNano(), 16))
	password = "secretsanta"

	rInput := auth.RegisterInput{
		Client:    retryablehttp.NewClient(),
		Password:  password,
		Email:     email,
		APIServer: localServer,
		Version:   common.DefaultSNVersion,
		Debug:     true,
	}

	if _, err = rInput.Register(); err != nil {
		return "", "", fmt.Errorf("failed to register with: %s", localServer)
	}

	_, err = auth.SignIn(auth.SignInInput{
		HTTPClient: retryablehttp.NewClient(),
		Email:      email,
		Password:   password,
		APIServer:  localServer,
		Debug:      false,
	})
	if err != nil {
		return "", "", err
	}

	return email, password, nil
}
