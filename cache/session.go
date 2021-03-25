package cache

import (
	"github.com/jonhadfield/gosn-v2"
)

type Session struct {
	gosn.Session
	CacheDBPath string
}

func GetSession(loadSession bool, sessionKey, server string) (session Session, email string, err error) {
	var gSession gosn.Session

	gSession, _, err = gosn.GetSession(loadSession, sessionKey, server)
	if err != nil {
		return
	}

	session.Token = gSession.Token
	session.Server = gSession.Server
	session.AccessExpiration = gSession.AccessExpiration
	session.RefreshExpiration = gSession.RefreshExpiration
	session.MasterKey = gSession.MasterKey
	session.AccessToken = gSession.AccessToken
	session.RefreshToken = gSession.RefreshToken

	return session, email, err
}
