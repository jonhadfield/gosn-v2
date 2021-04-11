package gosn

import (
	"log"
	"os"
	"testing"
)

var testSession *Session

func TestMain(m *testing.M) {
	gs, err := CliSignIn(os.Getenv("SN_EMAIL"), os.Getenv("SN_PASSWORD"), os.Getenv("SN_SERVER"))
	if err != nil {
		log.Fatal(err)
	}

	testSession = &Session{
		Debug:             true,
		Server:            gs.Server,
		Token:             gs.Token,
		MasterKey:         gs.MasterKey,
		RefreshExpiration: gs.RefreshExpiration,
		RefreshToken:      gs.RefreshToken,
		AccessToken:       gs.AccessToken,
		AccessExpiration:  gs.AccessExpiration,
	}

	_, err = Sync(SyncInput{
		Session: testSession,
		Debug:   true,
	})

	if testSession.DefaultItemsKey.ItemsKey == "" {
		panic("failed in TestMain due to empty default items key")
	}

	os.Exit(m.Run())
}
