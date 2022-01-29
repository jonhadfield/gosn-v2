package gosn

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"syscall"

	"github.com/spf13/viper"
	keyring "github.com/zalando/go-keyring"
	"golang.org/x/crypto/ssh/terminal"
)

const (
	SNServerURL              = "https://api.standardnotes.com"
	KeyringApplicationName   = "Session"
	KeyringService           = "StandardNotesCLI"
	MsgSessionRemovalSuccess = "session removed successfully"
	MsgSessionRemovalFailure = "failed to remove session"
)

// Session holds authentication and encryption parameters required
// to communicate with the API and process transferred data.
type Session struct {
	Debug     bool
	Server    string
	Token     string
	MasterKey string
	ItemsKeys []ItemsKey
	// ImporterItemsKey is the key used to encrypt exported items and set during import only
	ImporterItemsKey  ItemsKey
	DefaultItemsKey   ItemsKey
	KeyParams         KeyParams `json:"keyParams"`
	AccessToken       string    `json:"access_token"`
	RefreshToken      string    `json:"refresh_token"`
	AccessExpiration  int64     `json:"access_expiration"`
	RefreshExpiration int64     `json:"refresh_expiration"`
	PasswordNonce     string
}

type MinimalSession struct {
	Server            string
	Token             string
	MasterKey         string
	KeyParams         KeyParams `json:"keyParams"`
	AccessToken       string    `json:"access_token"`
	RefreshToken      string    `json:"refresh_token"`
	AccessExpiration  int64     `json:"access_expiration"`
	RefreshExpiration int64     `json:"refresh_expiration"`
}

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
		bytePassword, err := terminal.ReadPassword(int(syscall.Stdin))
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

// Encrypt string to base64 crypto using AES.
func Encrypt(key []byte, text string) string {
	key = padToAESBlockSize(key)
	plaintext := []byte(text)

	block, err := aes.NewCipher(key)
	if err != nil {
		panic(err)
	}

	ciphertext := make([]byte, aes.BlockSize+len(plaintext))

	iv := ciphertext[:aes.BlockSize]
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		panic(err)
	}

	stream := cipher.NewCFBEncrypter(block, iv)
	stream.XORKeyStream(ciphertext[aes.BlockSize:], plaintext)

	// convert to base64
	return base64.URLEncoding.EncodeToString(ciphertext)
}

func GetSessionFromKeyring(k keyring.Keyring) (s string, err error) {
	if k == nil {
		return keyring.Get(KeyringService, KeyringApplicationName)
	}

	return k.Get(KeyringService, KeyringApplicationName)
}

func AddSession(snServer, inKey string, k keyring.Keyring, debug bool) (res string, err error) {
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

		byteKey, err = terminal.ReadPassword(int(syscall.Stdin))
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

	session, _, err = GetSessionFromUser(snServer, debug)
	if err != nil {
		return fmt.Sprint("failed to get Session: ", err), err
	}

	rS := makeMinimalSessionString(session)

	if inKey != "" {
		key := []byte(inKey)
		rS = Encrypt(key, makeMinimalSessionString(session))
	}

	err = writeSession(rS, k)
	if err != nil {
		return fmt.Sprint("failed to set Session: ", err), err
	}

	return "session added successfully", err
}

func makeMinimalSessionString(s Session) string {
	ms := MinimalSession{
		Server:            s.Server,
		Token:             s.Token,
		MasterKey:         s.MasterKey,
		KeyParams:         s.KeyParams,
		AccessToken:       s.AccessToken,
		RefreshToken:      s.RefreshToken,
		AccessExpiration:  s.AccessExpiration,
		RefreshExpiration: s.RefreshExpiration,
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

	return k.Set(KeyringService, KeyringApplicationName, s)
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

func GetSessionFromUser(server string, debug bool) (Session, string, error) {
	var sess Session

	var err error

	var email, password, apiServer, errMsg string

	email, password, apiServer, errMsg = GetCredentials(server)
	if errMsg != "" {
		if strings.Contains(errMsg, "password not defined") {
			err = fmt.Errorf("password not defined")
		} else {
			fmt.Printf("\nerror: %s\n\n", errMsg)
		}

		return sess, email, err
	}

	debugPrint(debug, fmt.Sprintf("attempting cli sign-in with email: '%s' %d char password and server '%s'", email, len(password), apiServer))

	sess, err = CliSignIn(email, password, apiServer, debug)
	if err != nil {
		debugPrint(debug, fmt.Sprintf("CliSignIn failed with: %+v", err))

		return sess, email, err
	}

	return sess, email, err
}

func GetSession(loadSession bool, sessionKey, server string, debug bool) (session Session, email string, err error) {
	if loadSession {
		var rawSess string

		rawSess, err = keyring.Get(KeyringService, KeyringApplicationName)
		if err != nil {
			return
		}

		var loadedSession Session

		if !isUnencryptedSession(rawSess) {
			if sessionKey == "" {
				var byteKey []byte

				fmt.Print("session key: ")

				byteKey, err = terminal.ReadPassword(int(syscall.Stdin))
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

			if rawSess, err = Decrypt([]byte(sessionKey), rawSess); err != nil {
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
	} else {
		session, email, err = GetSessionFromUser(server, debug)
		if err != nil {
			return
		}
	}

	session.Debug = debug

	return session, email, err
}

func ParseSessionString(ss string) (sess Session, err error) {
	var ms MinimalSession
	err = json.Unmarshal([]byte(ss), &ms)

	sess.Server = ms.Server
	sess.AccessToken = ms.AccessToken
	sess.AccessExpiration = ms.AccessExpiration
	sess.RefreshToken = ms.RefreshToken
	sess.RefreshExpiration = ms.RefreshExpiration
	sess.MasterKey = ms.MasterKey
	sess.Server = ms.Server
	sess.KeyParams = ms.KeyParams
	sess.PasswordNonce = ms.KeyParams.PwNonce

	return

}

func isUnencryptedSession(in string) bool {
	return strings.HasPrefix(in, "{")
}

// Decrypt from base64 to decrypted string.
func Decrypt(key []byte, cryptoText string) (pt string, err error) {
	var ciphertext []byte

	if ciphertext, err = base64.URLEncoding.DecodeString(cryptoText); err != nil {
		return
	}

	key = padToAESBlockSize(key)

	var block cipher.Block

	if block, err = aes.NewCipher(key); err != nil {
		return
	}

	if len(ciphertext) < aes.BlockSize {
		return "", errors.New("ciphertext too short")
	}

	iv := ciphertext[:aes.BlockSize]
	ciphertext = ciphertext[aes.BlockSize:]

	stream := cipher.NewCFBDecrypter(block, iv)
	stream.XORKeyStream(ciphertext, ciphertext)

	pt = fmt.Sprintf("%s", ciphertext)

	return
}

func getSessionContent(key, rawSession string) (session string, err error) {
	// check if Session is encrypted
	if !isUnencryptedSession(rawSession) {
		if key == "" {
			fmt.Print("session key: ")

			var byteKey []byte
			byteKey, err = terminal.ReadPassword(int(syscall.Stdin))

			fmt.Println()

			if err == nil {
				key = string(byteKey)
			}

			if len(strings.TrimSpace(key)) == 0 {
				err = fmt.Errorf("key required")

				return
			}
		}

		session, err = Decrypt([]byte(key), rawSession)

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
			err = fmt.Errorf("stored session is invalid and needs to be replaced")
		}

		return
	}

	s, err := ParseSessionString(session)
	if err != nil {
		msg = fmt.Sprint("failed to parse Session: ", err)

		return
	}

	return fmt.Sprint("session found: ", s.KeyParams.Identifier), err
}
