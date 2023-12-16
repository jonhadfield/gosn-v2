package items

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jonhadfield/gosn-v2/schemas"

	"github.com/hashicorp/go-retryablehttp"
	"github.com/jonhadfield/gosn-v2/auth"
	"github.com/jonhadfield/gosn-v2/common"
	"github.com/jonhadfield/gosn-v2/session"
)

var (
	testSession      *session.Session
	testUserEmail    string
	testUserPassword string
)

func localTestMain() {
	localServer := "http://ramea:3000"
	testUserEmail = fmt.Sprintf("ramea-%s", strconv.FormatInt(time.Now().UnixNano(), 16))
	testUserPassword = "secretsanta"

	rInput := auth.RegisterInput{
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
	auth.SignIn(auth.SignInInput{
		Email:     testUserEmail,
		Password:  testUserPassword,
		APIServer: localServer,
		Debug:     false,
	})
}

func TestMain(m *testing.M) {
	if strings.Contains(os.Getenv("SN_SERVER"), "ramea") {
		localTestMain()

		os.Exit(m.Run())
	}

	httpClient := retryablehttp.NewClient()

	sOutput, err := auth.SignIn(auth.SignInInput{
		HTTPClient: httpClient,
		Email:      os.Getenv("SN_EMAIL"),
		Password:   os.Getenv("SN_PASSWORD"),
		APIServer:  os.Getenv("SN_SERVER"),
		Debug:      true,
	})

	testSession = &session.Session{
		Debug:             true,
		HTTPClient:        httpClient,
		SchemaValidation:  false,
		Server:            os.Getenv("SN_SERVER"),
		FilesServerUrl:    sOutput.Session.FilesServerUrl,
		Token:             "",
		MasterKey:         sOutput.Session.MasterKey,
		ItemsKeys:         nil,
		DefaultItemsKey:   session.SessionItemsKey{},
		KeyParams:         sOutput.Session.KeyParams,
		AccessToken:       sOutput.Session.AccessToken,
		RefreshToken:      sOutput.Session.RefreshToken,
		AccessExpiration:  sOutput.Session.AccessExpiration,
		RefreshExpiration: sOutput.Session.RefreshExpiration,
		ReadOnlyAccess:    sOutput.Session.ReadOnlyAccess,
		PasswordNonce:     sOutput.Session.PasswordNonce,
		Schemas:           nil,
	}

	// sOutput.Session

	// if strings.ToLower(os.Getenv("SN_DEBUG")) == "true" {
	// 	testSession.Debug = true
	// }

	if _, err = Sync(SyncInput{Session: testSession}); err != nil {
		log.Fatal(err)
	}

	testSession.Schemas, err = schemas.LoadSchemas()
	if err != nil {
		log.Fatal(err)
	}

	if len(testSession.Schemas) == 0 {
		log.Fatal("failed to load schemas")
	}

	os.Exit(m.Run())
}
