package cache

import (
	"github.com/jonhadfield/gosn-v2"
)

type Session struct {
	*gosn.Session
	CacheDBPath string
}

func GetSession(loadSession bool, sessionKey, server string) (s Session, email string, err error) {
	var gs gosn.Session

	gs, _, err = gosn.GetSession(loadSession, sessionKey, server)
	if err != nil {
		return
	}

	return Session{
		Session: &gosn.Session{
			gs.Debug,
			gs.Server,
			gs.Token,
			gs.MasterKey,
			gs.ItemsKeys,
			gs.DefaultItemsKey,
			gs.AccessToken,
			gs.RefreshToken,
			gs.AccessExpiration,
			gs.RefreshExpiration,
		},
		CacheDBPath: "",
	}, email, err

	return s, email, err
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
