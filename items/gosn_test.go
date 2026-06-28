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
	"github.com/jonhadfield/gosn-v2/schemas"
	"github.com/jonhadfield/gosn-v2/session"
)

var (
	testSession      *session.Session
	testUserEmail    string
	testUserPassword string
)

// skipIfSessionTestsDisabled skips tests that require a live server session
// (and thus a non-nil testSession) when SN_SKIP_SESSION_TESTS is set. TestMain
// bypasses sign-in in that mode, leaving testSession nil.
func skipIfSessionTestsDisabled(t *testing.T) {
	t.Helper()
	if strings.EqualFold(os.Getenv(common.EnvSkipSessionTests), "true") {
		t.Skip("skipping test that requires server authentication (SN_SKIP_SESSION_TESTS is set)")
	}
}

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
	// Skip the live sign-in/sync setup when SN_SKIP_SESSION_TESTS is set so that
	// tests not requiring a server (e.g. request construction, content helpers)
	// can run offline. Matches the auth/session/cache packages and the documented
	// `SN_SKIP_SESSION_TESTS=true go test ./...` workflow. Session-dependent tests
	// rely on testSession and must be excluded via -run when this is set.
	if strings.EqualFold(os.Getenv(common.EnvSkipSessionTests), "true") {
		os.Exit(m.Run())
	}

	if strings.Contains(os.Getenv(common.EnvServer), "ramea") {
		localTestMain()

		os.Exit(m.Run())
	}

	httpClient := common.NewHTTPClient()

	sOutput, err := auth.SignIn(auth.SignInInput{
		HTTPClient: httpClient,
		Email:      os.Getenv(common.EnvEmail),
		Password:   os.Getenv(common.EnvPassword),
		APIServer:  os.Getenv(common.EnvServer),
		Debug:      true,
	})
	if err != nil {
		log.Fatal(err)
	}

	testSession = &session.Session{
		Debug:             true,
		HTTPClient:        httpClient,
		SchemaValidation:  false,
		Server:            os.Getenv(common.EnvServer),
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

	os.Exit(m.Run())
}
