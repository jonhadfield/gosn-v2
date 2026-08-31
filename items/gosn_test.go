package items

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jonhadfield/gosn-v2/auth"
	"github.com/jonhadfield/gosn-v2/common"
	"github.com/jonhadfield/gosn-v2/mock"
	"github.com/jonhadfield/gosn-v2/schemas"
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
	os.Exit(run(m))
}

func run(m *testing.M) int {
	if strings.Contains(os.Getenv(common.EnvServer), "ramea") {
		localTestMain()

		return m.Run()
	}

	server, email, password := os.Getenv(common.EnvServer),
		os.Getenv(common.EnvEmail), os.Getenv(common.EnvPassword)

	// Without credentials for a real account, run against a mock server. It
	// speaks enough of the API for the client to authenticate and sync, so the
	// tests below exercise the real encryption and sync paths either way.
	if email == "" || password == "" {
		srv, err := mock.New()
		if err != nil {
			log.Fatal(err)
		}

		defer srv.Close()

		server, email, password = srv.URL, srv.Email, srv.Password
	}

	httpClient := common.NewHTTPClient()

	sOutput, err := auth.SignIn(auth.SignInInput{
		HTTPClient: httpClient,
		Email:      email,
		Password:   password,
		APIServer:  server,
		Debug:      true,
	})
	if err != nil {
		log.Fatal(err)
	}

	testSession = &session.Session{
		Debug:             true,
		HTTPClient:        httpClient,
		SchemaValidation:  false,
		Server:            server,
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

	return m.Run()
}
