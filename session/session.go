package session

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/jonhadfield/gosn-v2/schemas"

	"github.com/hashicorp/go-retryablehttp"
	"github.com/jonhadfield/gosn-v2/auth"
	"github.com/jonhadfield/gosn-v2/common"
	"github.com/jonhadfield/gosn-v2/crypto"
	"github.com/jonhadfield/gosn-v2/log"
	"github.com/santhosh-tekuri/jsonschema/v5"
	"github.com/spf13/viper"
	"github.com/zalando/go-keyring"
	"golang.org/x/term"
)

const (
	SNServerURL              = "https://api.standardnotes.com"
	KeyringApplicationName   = "Session"
	KeyringService           = "StandardNotesCLI"
	MsgSessionRemovalSuccess = "session removed successfully"
	MsgSessionRemovalFailure = "failed to remove session"
	DefaultSessionExpiryTime = 12 * time.Hour
	RefreshSessionThreshold  = 10 * time.Minute
)

type SessionItemsKey struct {
	UUID               string `json:"uuid"`
	ItemsKey           string `json:"itemsKey"`
	Version            string `json:"version"`
	Default            bool   `json:"isDefault"`
	CreatedAt          string `json:"created_at"`
	UpdatedAt          string `json:"updated_at"`
	CreatedAtTimestamp int64  `json:"created_at_timestamp"`
	UpdatedAtTimestamp int64  `json:"updated_at_timestamp"`
	Deleted            bool   `json:"deleted"`
	// Note: ItemReferences and AppData are typically empty for ItemsKeys
	// but could be added if needed in the future
}

// Session holds authentication and encryption parameters required
// to communicate with the API and process transferred data.
type Session struct {
	Debug            bool
	HTTPClient       *retryablehttp.Client
	SchemaValidation bool
	Server           string
	FilesServerUrl   string `json:"filesServerUrl"`
	Token            string
	MasterKey        string
	ItemsKeys        []SessionItemsKey
	// ImporterItemsKeys is the key used to encrypt exported items and set during import only
	// ImporterItemsKeys []SessionItemsKey
	DefaultItemsKey   SessionItemsKey
	KeyParams         auth.KeyParams `json:"keyParams"`
	AccessToken       string         `json:"access_token"`
	RefreshToken      string         `json:"refresh_token"`
	AccessExpiration  int64          `json:"access_expiration"`
	RefreshExpiration int64          `json:"refresh_expiration"`
	ReadOnlyAccess     bool           `json:"readonly_access"`
	PasswordNonce      string
	Schemas            map[string]*jsonschema.Schema
	// Cookie values extracted from Set-Cookie headers for manual cookie handling
	AccessTokenCookie  string `json:"access_token_cookie,omitempty"`
	RefreshTokenCookie string `json:"refresh_token_cookie,omitempty"`
}

type MinimalSession struct {
	Server             string
	Token              string
	MasterKey          string
	KeyParams          auth.KeyParams `json:"keyParams"`
	AccessToken        string         `json:"access_token"`
	RefreshToken       string         `json:"refresh_token"`
	AccessExpiration   int64          `json:"access_expiration"`
	RefreshExpiration  int64          `json:"refresh_expiration"`
	SchemaValidation   bool
	AccessTokenCookie  string `json:"access_token_cookie,omitempty"`
	RefreshTokenCookie string `json:"refresh_token_cookie,omitempty"`
}

// func (s *Session) Export(path string) error {
// 	// we must export all items, or otherwise we will update the encryption key
// 	// for non exported items so they can no longer be encrypted
// 	so, err := Sync(SyncInput{
// 		Session: s,
// 	})
// 	if err != nil {
// 		return err
// 	}
//
// 	ik := s.DefaultItemsKey
//
// 	// create new items key, but copy across uuid and timestamps
// 	nk, err := auth.CreateItemsKey()
// 	if err != nil {
// 		return err
// 	}
//
// 	nk.UUID = ik.UUID
// 	nk.CreatedAt = ik.CreatedAt
// 	nk.UpdatedAt = ik.UpdatedAt
// 	nk.CreatedAtTimestamp = ik.CreatedAtTimestamp
// 	nk.UpdatedAtTimestamp = ik.UpdatedAtTimestamp
//
// 	// encrypt items with the new ItemsKey
// 	nei, err := so.Items.ReEncrypt(s, ItemsKey{}, nk, s.MasterKey)
// 	if err != nil {
// 		return err
// 	}
//
// 	// encrypt items key that encrypted the items
// 	eik, err := EncryptItemsKey(nk, s, false)
// 	if err != nil {
// 		return err
// 	}
//
// 	if eik.UpdatedAtTimestamp != ik.UpdatedAtTimestamp {
// 		panic("updated timestamp not consistent with original")
// 	}
//
// 	eik.UpdatedAtTimestamp = ik.UpdatedAtTimestamp
//
// 	eik.UpdatedAt = ik.UpdatedAt
//
// 	// prepend new items key to the export
// 	nei = append([]EncryptedItem{eik}, nei...)
//
// 	// add existing items keys to the export
// 	if err = writeJSON(writeJSONConfig{
// 		session: *s,
// 		Path:    path,
// 		Debug:   true,
// 	}, nei); err != nil {
// 		return err
// 	}
//
// 	return nil
// }

func GetCredentials(inServer string) (email, password, apiServer, errMsg string) {
	switch {
	case viper.GetString("email") != "":
		email = viper.GetString("email")
	default:
		fmt.Print("email: ")

		_, err := fmt.Scanln(&email)
		if err != nil || len(strings.TrimSpace(email)) == 0 {
			errMsg = "email required"
			return
		}
	}

	if viper.GetString("password") != "" {
		password = viper.GetString("password")
	} else {
		fmt.Print("password: ")
		bytePassword, err := term.ReadPassword(int(syscall.Stdin))
		fmt.Println()
		if err == nil {
			password = string(bytePassword)
		} else {
			errMsg = err.Error()
			return
		}
		if strings.TrimSpace(password) == "" {
			errMsg = "password not defined"
		}
	}

	switch {
	case inServer != "":
		apiServer = inServer
	case viper.GetString("server") != "":
		apiServer = viper.GetString("server")
	default:
		apiServer = SNServerURL
	}

	return email, password, apiServer, errMsg
}

func GetSessionFromKeyring(k keyring.Keyring) (s string, err error) {
	if k == nil {
		s, err = keyring.Get(KeyringService, KeyringApplicationName)
		if err != nil {
			return s, fmt.Errorf("GetSessionFromKeyring | %w", err)
		}
	}

	if k != nil {
		s, err = k.Get(KeyringService, KeyringApplicationName)
		if err != nil {
			err = fmt.Errorf("GetSessionFromKeyring | %w", err)
		}
	}

	return
}

func AddSession(httpClient *retryablehttp.Client, snServer, inKey string, k keyring.Keyring, debug bool) (res string, err error) {
	// check if Session exists in keyring
	var s string
	s, err = GetSessionFromKeyring(k)
	// only return an error if there's an issue accessing the keyring
	if err != nil && !strings.Contains(err.Error(), "secret not found in keyring") {
		return
	}

	if inKey == "." {
		var byteKey []byte

		fmt.Print("session key: ")

		byteKey, err = term.ReadPassword(int(syscall.Stdin))
		if err != nil {
			return
		}

		inKey = string(byteKey)

		fmt.Println()
	}

	if s != "" {
		fmt.Print("replace existing Session (y|n): ")

		var resp string

		_, err = fmt.Scanln(&resp)
		if err != nil || strings.ToLower(resp) != "y" {
			// do nothing
			return "", nil
		}
	}

	var session Session

	session, _, err = GetSessionFromUser(httpClient, snServer, debug)
	if err != nil {
		return fmt.Sprint("failed to get Session: ", err), err
	}

	rS := makeMinimalSessionString(session)
	if inKey != "" {
		key := []byte(inKey)
		rS = crypto.Encrypt(key, makeMinimalSessionString(session))
	}

	err = writeSession(rS, k)
	if err != nil {
		return fmt.Sprint("failed to set Session: ", err), err
	}

	return "session added successfully", err
}

func UpdateSession(sess *Session, k keyring.Keyring, debug bool) error {
	// check if Session exists in keyring
	existingRaw, err := GetSessionFromKeyring(k)
	// only return an error if there's an issue accessing the keyring
	if err != nil && !strings.Contains(err.Error(), "secret not found in keyring") {
		return err
	}

	var byteKey []byte

	var key string

	if existingRaw != "" && !isUnencryptedSession(existingRaw) {
		fmt.Print("encryption key for updated session: ")

		byteKey, err = term.ReadPassword(int(syscall.Stdin))
		if err != nil {
			return err
		}

		key = string(byteKey)

		fmt.Println()
	}

	rS := makeMinimalSessionString(*sess)
	if key != "" {
		rS = crypto.Encrypt(byteKey, makeMinimalSessionString(*sess))
	}

	err = writeSession(rS, k)
	if err != nil {
		return fmt.Errorf("failed to write refreshed session: %w", err)
	}

	// fmt.Println("session refreshed successfully")

	return nil
}

func makeMinimalSessionString(s Session) string {
	ms := MinimalSession{
		Server:             s.Server,
		Token:              s.Token,
		MasterKey:          s.MasterKey,
		KeyParams:          s.KeyParams,
		AccessToken:        s.AccessToken,
		RefreshToken:       s.RefreshToken,
		AccessExpiration:   s.AccessExpiration,
		RefreshExpiration:  s.RefreshExpiration,
		SchemaValidation:   s.SchemaValidation,
		AccessTokenCookie:  s.AccessTokenCookie,
		RefreshTokenCookie: s.RefreshTokenCookie,
	}

	sb, err := json.Marshal(ms)
	if err != nil {
		panic("failed to marshall session")
	}

	return string(sb)
}

func writeSession(s string, k keyring.Keyring) error {
	if k == nil {
		return keyring.Set(KeyringService, KeyringApplicationName, s)
	}

	if err := k.Set(KeyringService, KeyringApplicationName, s); err != nil {
		return fmt.Errorf("writeSession | %w", err)
	}

	return nil
}

func SessionExists(k keyring.Keyring) error {
	s, err := GetSessionFromKeyring(k)
	if err != nil {
		return err
	}

	if len(s) == 0 {
		return errors.New("session is empty")
	}

	return nil
}

// RemoveSession removes the SN Session from the keyring.
func RemoveSession(k keyring.Keyring) string {
	var err error
	if err = SessionExists(k); err != nil {
		return fmt.Sprintf("%s: %s", MsgSessionRemovalFailure, err.Error())
	}

	if k == nil {
		err = keyring.Delete(KeyringService, KeyringApplicationName)
	} else {
		err = k.Delete(KeyringService, KeyringApplicationName)
	}

	if err != nil {
		return fmt.Sprintf("%s: %s", MsgSessionRemovalFailure, err.Error())
	}

	return MsgSessionRemovalSuccess
}

func GetSessionFromUser(httpClient *retryablehttp.Client, server string, debug bool) (Session, string, error) {
	var sess Session

	sess.HTTPClient = common.NewHTTPClient()

	if server == "" {
		server = common.APIServer
	}

	var err error

	var email, password, apiServer, errMsg string

	email, password, apiServer, errMsg = GetCredentials(server)
	if errMsg != "" {
		if strings.Contains(errMsg, "password not defined") {
			err = fmt.Errorf("password not defined")
		} else {
			err = fmt.Errorf("%s", errMsg)
			fmt.Printf("\nerror: %s\n\n", errMsg)
		}

		return sess, email, err
	}

	log.DebugPrint(debug, fmt.Sprintf("attempting cli sign-in with email: '%s' %d char password and server '%s'", email, len(password), apiServer), common.MaxDebugChars)

	signInSession, err := auth.CliSignIn(email, password, server, debug)
	sess = Session{
		Debug:              debug,
		HTTPClient:         signInSession.HTTPClient, // Preserve HTTP client with cookies
		SchemaValidation:   signInSession.SchemaValidation,
		Server:             server,
		FilesServerUrl:     signInSession.FilesServerUrl,
		Token:              signInSession.Token,
		MasterKey:          signInSession.MasterKey,
		KeyParams:          signInSession.KeyParams,
		AccessToken:        signInSession.AccessToken,
		RefreshToken:       signInSession.RefreshToken,
		AccessExpiration:   signInSession.AccessExpiration,
		RefreshExpiration:  signInSession.RefreshExpiration,
		ReadOnlyAccess:     signInSession.ReadOnlyAccess,
		AccessTokenCookie:  signInSession.AccessTokenCookie,
		RefreshTokenCookie: signInSession.RefreshTokenCookie,
	}

	if err != nil {
		log.DebugPrint(debug, fmt.Sprintf("CliSignIn failed with: %+v", err), common.MaxDebugChars)

		return sess, email, err
	}

	return sess, email, err
}

// func GetHttpClient() *retryablehttp.Client {
// 	rc := retryablehttp.NewClient()
// 	rc.RetryMax = 3
// 	return rc
// }

func GetSession(httpClient *retryablehttp.Client, loadSession bool, sessionKey, server string, debug bool) (session Session, email string, err error) {
	if loadSession {
		var rawSess string

		rawSess, err = keyring.Get(KeyringService, KeyringApplicationName)
		if err != nil {
			return
		}

		if rawSess == "" {
			err = fmt.Errorf("session not found in keyring")

			return
		}

		if strings.Contains(rawSess, "\"access_token\":\"\"") {
			err = fmt.Errorf("invalid session found in keyring")

			return
		}

		var loadedSession Session

		if !isUnencryptedSession(rawSess) {
			if sessionKey == "" {
				var byteKey []byte

				fmt.Print("session key: ")

				byteKey, err = term.ReadPassword(int(syscall.Stdin))
				if err != nil {
					return
				}

				fmt.Println()

				if len(byteKey) == 0 {
					err = fmt.Errorf("key not provided")
					return
				}

				sessionKey = string(byteKey)
			}

			if rawSess, err = crypto.Decrypt([]byte(sessionKey), rawSess); err != nil {
				return
			}

			if !isUnencryptedSession(rawSess) {
				err = fmt.Errorf("incorrect key or invalid session")

				return
			}
		}

		loadedSession, err = ParseSessionString(rawSess)
		if err != nil {
			return
		}

		email = loadedSession.KeyParams.Identifier
		session = loadedSession
		session.HTTPClient = httpClient

		// if session is expired or close to expiry then refresh it
		if time.Unix(session.AccessExpiration/1000, 0).Add(-RefreshSessionThreshold).Before(time.Now().UTC()) {
			if err = session.Refresh(); err != nil {
				return Session{}, "", err
			}

			if err = UpdateSession(&session, nil, session.Debug); err != nil {
				return Session{}, "", err
			}
		}
	} else {
		session, email, err = GetSessionFromUser(httpClient, server, debug)
		if err != nil {
			return
		}
	}

	session.Debug = debug

	if slices.Contains([]string{"yes", "true", "1"}, os.Getenv(common.EnvSchemaValidation)) {
		session.SchemaValidation = true

		session.Schemas, err = schemas.LoadSchemas()
		if err != nil {
			return Session{}, "", err
		}
	}

	return session, email, nil
}

func ParseSessionString(ss string) (Session, error) {
	var ms MinimalSession

	if err := json.Unmarshal([]byte(ss), &ms); err != nil {
		return Session{}, err
	}

	return Session{
		Server:             ms.Server,
		AccessToken:        ms.AccessToken,
		AccessExpiration:   ms.AccessExpiration,
		RefreshToken:       ms.RefreshToken,
		RefreshExpiration:  ms.RefreshExpiration,
		MasterKey:          ms.MasterKey,
		KeyParams:          ms.KeyParams,
		PasswordNonce:      ms.KeyParams.PwNonce,
		AccessTokenCookie:  ms.AccessTokenCookie,
		RefreshTokenCookie: ms.RefreshTokenCookie,
	}, nil
}

func isUnencryptedSession(in string) bool {
	return strings.HasPrefix(in, "{")
}

func getSessionContent(key, rawSession string) (session string, err error) {
	// check if Session is encrypted
	if !isUnencryptedSession(rawSession) {
		if key == "" {
			fmt.Print("session key: ")

			var byteKey []byte
			byteKey, err = term.ReadPassword(int(syscall.Stdin))

			fmt.Println()

			if err == nil {
				key = string(byteKey)
			}

			if len(strings.TrimSpace(key)) == 0 {
				err = fmt.Errorf("key required")

				return
			}
		}

		session, err = crypto.Decrypt([]byte(key), rawSession)

		if err != nil {
			err = fmt.Errorf("invalid session or wrong key provided")
		}
	} else {
		session = rawSession
	}

	return session, err
}

func SessionStatus(sKey string, k keyring.Keyring) (msg string, err error) {
	var rawSession string

	rawSession, err = GetSessionFromKeyring(k)
	if err != nil {
		return
	}

	if len(rawSession) == 0 {
		return "", errors.New("keyring is empty")
	}
	// now decrypt if needed
	var session string
	session, err = getSessionContent(sKey, rawSession)

	if err != nil {
		if strings.Contains(err.Error(), "illegal base64") {
			err = errors.New("stored Session is corrupt")
		}

		return
	}

	if !isUnencryptedSession(session) {
		if sKey != "" {
			err = fmt.Errorf("incorrect key, or session is invalid and needs to be replaced")
		} else {
			err = fmt.Errorf("stored session is invalid and needs to be replaced, or is encrypted and requires a key to unlock")
		}

		return
	}

	s, err := ParseSessionString(session)
	if err != nil {
		msg = fmt.Sprint("failed to parse Session: ", err)

		return
	}

	sb := strings.Builder{}
	sb.WriteString(fmt.Sprintf("user: %s\n", s.KeyParams.Identifier))
	sb.WriteString(fmt.Sprintf("access token expires: %s\n",
		time.Unix(s.AccessExpiration/1000, 0).UTC().String()))
	sb.WriteString(fmt.Sprintf("refresh token expires: %s",
		time.Unix(s.RefreshExpiration/1000, 0).UTC().String()))

	return sb.String(), err
}

func (sess *Session) Refresh() error {
	if sess.HTTPClient == nil {
		sess.HTTPClient = retryablehttp.NewClient()
	}

	server := sess.Server
	if sess.Server == "" {
		server = common.APIServer
	}

	// request token
	var requestTokenFailure auth.ErrorResponse

	// Use the session-aware refresh function that handles both cookie-based and header-based sessions
	authSession := auth.SignInResponseDataSession{
		HTTPClient:        sess.HTTPClient,
		Debug:             sess.Debug,
		Server:            sess.Server,
		MasterKey:         sess.MasterKey,
		KeyParams:         sess.KeyParams,
		AccessToken:       sess.AccessToken,
		RefreshToken:      sess.RefreshToken,
		AccessExpiration:  sess.AccessExpiration,
		RefreshExpiration: sess.RefreshExpiration,
		PasswordNonce:     sess.PasswordNonce,
	}

	refreshSessionOutput, err := auth.RequestRefreshTokenWithSession(&authSession, server+common.AuthRefreshPath, sess.Debug)
	if err != nil {
		log.DebugPrint(sess.Debug, fmt.Sprintf("refresh session failure: %+v error: %+v", requestTokenFailure, err), common.MaxDebugChars)

		return err
	}

	sess.FilesServerUrl = refreshSessionOutput.Meta.Server.FilesServerURL
	sess.AccessToken = refreshSessionOutput.Data.Session.AccessToken
	sess.RefreshToken = refreshSessionOutput.Data.Session.RefreshToken
	sess.AccessExpiration = refreshSessionOutput.Data.Session.AccessExpiration
	sess.RefreshExpiration = refreshSessionOutput.Data.Session.RefreshExpiration
	x := 0
	sess.ReadOnlyAccess = x != refreshSessionOutput.Data.Session.ReadOnlyAccess

	return err
}

// createHTTPClient for connection re-use.
func createHTTPClient() *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			MaxIdleConnsPerHost: common.MaxIdleConnections,
			DisableKeepAlives:   false,
			DisableCompression:  false,
			DialContext: (&net.Dialer{
				Timeout:   common.ConnectionTimeout * time.Second,
				KeepAlive: common.KeepAliveTimeout * time.Second,
			}).DialContext,
		},
		Timeout: time.Duration(common.RequestTimeout) * time.Second,
	}
}

func (sess *Session) Valid() bool {
	if sess == nil {
		fmt.Print("session is nil\n")
		return false
	}

	switch {
	case sess.RefreshToken == "":
		log.DebugPrint(sess.Debug, "session is missing refresh token", common.MaxDebugChars)
		return false
	case sess.AccessToken == "":
		log.DebugPrint(sess.Debug, "session is missing access token", common.MaxDebugChars)
		return false
	case sess.MasterKey == "":
		log.DebugPrint(sess.Debug, "session is missing master key", common.MaxDebugChars)
		return false
	case sess.AccessExpiration == 0:
		log.DebugPrint(sess.Debug, "Access Expiration is 0", common.MaxDebugChars)
		return false
	case sess.RefreshExpiration == 0:
		log.DebugPrint(sess.Debug, "Refresh Expiration is 0", common.MaxDebugChars)
		return false
	}

	// check no duplicate item keys
	// s.ItemsKeys = items.DedupeItemsKeys(s.ItemsKeys)

	return true
}
