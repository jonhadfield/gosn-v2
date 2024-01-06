package cache

import (
	"github.com/asdine/storm/v3"
	"github.com/hashicorp/go-retryablehttp"
	"github.com/jonhadfield/gosn-v2/auth"
	"github.com/jonhadfield/gosn-v2/common"
	"github.com/jonhadfield/gosn-v2/session"
)

type Session struct {
	*session.Session
	CacheDB     *storm.DB
	CacheDBPath string
}

// ImportSession creates a new Session from an existing gosn.Session instance
// with the option of specifying a path for the db other than the home folder.
func ImportSession(gs *auth.SignInResponseDataSession, path string) (s *Session, err error) {
	if gs == nil {
		panic("gs is nil")
	}

	s = &Session{}

	s.Session = &session.Session{}
	if gs.Server == "" {
		gs.Server = common.APIServer
	}

	// if !gs.Valid() {
	// 	return s, fmt.Errorf("invalid session")
	// }
	s.Session.HTTPClient = gs.HTTPClient
	if s.Session.HTTPClient == nil {
		s.Session.HTTPClient = retryablehttp.NewClient()
	}

	s.Session.Debug = gs.Debug
	s.Session.Server = gs.Server
	s.Session.Token = gs.Token
	s.Session.MasterKey = gs.MasterKey
	// s.Session.ItemsKeys = gs.ItemsKeys
	// s.Session.DefaultItemsKey = gs.DefaultItemsKey
	s.Session.AccessToken = gs.AccessToken
	s.Session.RefreshToken = gs.RefreshToken
	s.Session.AccessExpiration = gs.AccessExpiration
	s.Session.RefreshExpiration = gs.RefreshExpiration
	s.Session.SchemaValidation = gs.SchemaValidation
	s.Session.PasswordNonce = gs.PasswordNonce

	if path == "" {
		var dbPath string

		dbPath, err = GenCacheDBPath(*s, dbPath, common.LibName)
		if err != nil {
			return
		}

		s.CacheDBPath = dbPath

		return
	}

	s.CacheDBPath = path
	// }

	return s, err
}

// GetSession returns a cache session that encapsulates a gosn-v2 session with additional
// configuration for managing a local cache database.
func GetSession(httpClient *retryablehttp.Client, loadSession bool, sessionKey, server string, debug bool) (s Session, email string, err error) {
	var gs session.Session

	if httpClient == nil || httpClient.HTTPClient == nil {
		httpClient = common.NewHTTPClient()
	}

	gs, _, err = session.GetSession(httpClient, loadSession, sessionKey, server, debug)

	if err != nil {
		return
	}

	cs := Session{
		Session: &session.Session{
			HTTPClient:        httpClient,
			Debug:             gs.Debug,
			Server:            gs.Server,
			Token:             gs.Token,
			MasterKey:         gs.MasterKey,
			SchemaValidation:  gs.SchemaValidation,
			Schemas:           gs.Schemas,
			ItemsKeys:         gs.ItemsKeys,
			KeyParams:         gs.KeyParams,
			DefaultItemsKey:   gs.DefaultItemsKey,
			AccessToken:       gs.AccessToken,
			RefreshToken:      gs.RefreshToken,
			AccessExpiration:  gs.AccessExpiration,
			RefreshExpiration: gs.RefreshExpiration,
			PasswordNonce:     gs.PasswordNonce,
		},
		CacheDB:     nil,
		CacheDBPath: "",
	}

	return cs, email, err
}

func (s Session) Gosn() session.Session {
	return session.Session{
		Debug:             s.Debug,
		Server:            s.Server,
		Token:             s.Token,
		MasterKey:         s.MasterKey,
		ItemsKeys:         s.ItemsKeys,
		DefaultItemsKey:   s.DefaultItemsKey,
		AccessToken:       s.AccessToken,
		RefreshToken:      s.RefreshToken,
		AccessExpiration:  s.AccessExpiration,
		RefreshExpiration: s.RefreshExpiration,
		SchemaValidation:  s.SchemaValidation,
	}
}
