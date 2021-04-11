package cache

import (
	"fmt"

	"github.com/asdine/storm/v3"
	gosn "github.com/jonhadfield/gosn-v2"
)

type Session struct {
	*gosn.Session
	CacheDB     *storm.DB
	CacheDBPath string
}

// ImportSession creates a new Session from an existing gosn.Session instance
// with the option of specifying a path for the db other than the home folder
func ImportSession(gs *gosn.Session, path string) (s *Session, err error) {
	s = &Session{}

	if gs.Server != "" {
		if !gs.Valid() {
			return s, fmt.Errorf("invalid session")
		}

		s.Session = gs

		if path == "" {
			var dbPath string
			dbPath, err = GenCacheDBPath(*s, dbPath, libName)
			if err != nil {
				return
			}
			s.CacheDBPath = dbPath

			return
		}

		s.CacheDBPath = path
	}

	return s, err

}

func GetSession(loadSession bool, sessionKey, server string) (s Session, email string, err error) {
	var gs gosn.Session

	gs, _, err = gosn.GetSession(loadSession, sessionKey, server)
	if err != nil {
		return
	}

	return Session{
		Session: &gosn.Session{
			Debug:             gs.Debug,
			Server:            gs.Server,
			Token:             gs.Token,
			MasterKey:         gs.MasterKey,
			ItemsKeys:         gs.ItemsKeys,
			DefaultItemsKey:   gs.DefaultItemsKey,
			AccessToken:       gs.AccessToken,
			RefreshToken:      gs.RefreshToken,
			AccessExpiration:  gs.AccessExpiration,
			RefreshExpiration: gs.RefreshExpiration,
		},
		CacheDB:     nil,
		CacheDBPath: "",
	}, email, err
}

func (s Session) Gosn() gosn.Session {
	return gosn.Session{
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
	}
}
