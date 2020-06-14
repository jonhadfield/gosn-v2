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

	session.Ak = gSession.Ak
	session.Mk = gSession.Mk
	session.Token = gSession.Token
	session.Server = gSession.Server

	return session, email, err
}
