// Package mock provides an in-memory Standard Notes server for use in tests.
//
// It implements the endpoints a client needs in order to authenticate and
// sync -- login parameters, sign-in, session refresh, registration and the
// items endpoint -- and holds items as the opaque encrypted blobs the client
// uploads. The account is seeded with a real SN|ItemsKey encrypted with the
// master key derived from the mock credentials, because a client never creates
// one during a sync.
//
// Only the server is a stand-in. Sign-in, key derivation, encryption,
// decryption and the sync round trip all run the code they run in production,
// so tests exercise the real client rather than a reimplementation of it.
//
//	srv, err := mock.New()
//	if err != nil {
//	        t.Fatal(err)
//	}
//	defer srv.Close()
//
//	sess, err := srv.Session()
//
// The package deliberately depends only on auth, common, crypto and session,
// so that the items and cache packages can use it in their own tests without
// creating an import cycle.
package mock

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/jonhadfield/gosn-v2/auth"
	"github.com/jonhadfield/gosn-v2/common"
	"github.com/jonhadfield/gosn-v2/crypto"
	"github.com/jonhadfield/gosn-v2/session"
)

const (
	// DefaultEmail is the account a mock server accepts unless told otherwise.
	DefaultEmail = "test@example.com"
	// DefaultPassword is the password a mock server accepts unless told
	// otherwise. It is over the API's minimum length so that registration
	// against the mock behaves like registration against a real server.
	DefaultPassword = "mock-server-password"

	itemsKeyContentVersion = "004"
)

// Item is an item as it travels over the wire. The server keeps the encrypted
// payload exactly as the client sent it and only ever sets the timestamps.
type Item struct {
	UUID               string  `json:"uuid"`
	ItemsKeyID         string  `json:"items_key_id,omitempty"`
	Content            string  `json:"content"`
	ContentType        string  `json:"content_type"`
	EncItemKey         string  `json:"enc_item_key"`
	CreatedAt          string  `json:"created_at"`
	UpdatedAt          string  `json:"updated_at"`
	DuplicateOf        *string `json:"duplicate_of,omitempty"`
	CreatedAtTimestamp int64   `json:"created_at_timestamp"`
	UpdatedAtTimestamp int64   `json:"updated_at_timestamp"`
	Deleted            bool    `json:"deleted"`
}

// Server is a Standard Notes API served over HTTP from memory.
type Server struct {
	// URL is the address to point a session at.
	URL string
	// Email and Password are the credentials the server accepts.
	Email    string
	Password string

	httpServer *httptest.Server
	pageSize   int

	masterKey      string
	serverPassword string
	keyParams      auth.KeyParams
	itemsKeyUUID   string

	mu sync.Mutex
	// revision advances on every write and doubles as the sync token, so a
	// client is only sent what it has not already seen.
	revision  int64
	stored    map[string]*storedItem
	syncs     int
	tokenSeq  int
	expired   bool
	failSync  []int
	conflicts map[string]string
}

type storedItem struct {
	item     Item
	revision int64
}

type config struct {
	email    string
	password string
	pageSize int
}

// Option configures a mock server.
type Option func(*config)

// WithCredentials sets the email and password the server accepts.
func WithCredentials(email, password string) Option {
	return func(c *config) {
		c.email = email
		c.password = password
	}
}

// WithPageSize caps how many items a single sync response carries, so that
// callers can exercise the client's cursor handling. Zero means no cap.
func WithPageSize(n int) Option {
	return func(c *config) {
		c.pageSize = n
	}
}

// New starts a mock server holding an empty account with a single items key.
// Close must be called when the caller is done with it.
func New(opts ...Option) (*Server, error) {
	cfg := config{email: DefaultEmail, password: DefaultPassword}
	for _, opt := range opts {
		opt(&cfg)
	}

	nonceBytes, err := crypto.GenerateNonce()
	if err != nil {
		return nil, fmt.Errorf("mock: generating password nonce: %w", err)
	}

	nonce := hex.EncodeToString(nonceBytes)

	masterKey, serverPassword, err := crypto.GenerateMasterKeyAndServerPassword004(crypto.GenerateEncryptedPasswordInput{
		UserPassword:  cfg.password,
		Identifier:    cfg.email,
		PasswordNonce: nonce,
	})
	if err != nil {
		return nil, fmt.Errorf("mock: deriving master key: %w", err)
	}

	s := &Server{
		Email:          cfg.email,
		Password:       cfg.password,
		pageSize:       cfg.pageSize,
		masterKey:      masterKey,
		serverPassword: serverPassword,
		keyParams: auth.KeyParams{
			Created:     strconv.FormatInt(time.Now().UTC().UnixMilli(), 10),
			Identifier:  cfg.email,
			Origination: "registration",
			PwNonce:     nonce,
			Version:     common.DefaultSNVersion,
		},
		stored:    make(map[string]*storedItem),
		conflicts: make(map[string]string),
	}

	if err = s.seedItemsKey(); err != nil {
		return nil, err
	}

	mux := http.NewServeMux()
	mux.HandleFunc(common.AuthParamsPath, s.handleAuthParams)
	mux.HandleFunc(common.SignInPath, s.handleSignIn)
	mux.HandleFunc(common.AuthRefreshPath, s.handleRefresh)
	mux.HandleFunc(common.AuthRegisterPath, s.handleRegister)
	mux.HandleFunc(common.SyncPath, s.handleSync)

	s.httpServer = httptest.NewServer(mux)
	s.URL = s.httpServer.URL

	return s, nil
}

// Close shuts the server down.
func (s *Server) Close() {
	s.httpServer.Close()
}

// MasterKey returns the master key the account's items key is encrypted with.
func (s *Server) MasterKey() string {
	return s.masterKey
}

// KeyParams returns the account's key parameters.
func (s *Server) KeyParams() auth.KeyParams {
	return s.keyParams
}

// ItemsKeyUUID returns the uuid of the items key the account was seeded with.
func (s *Server) ItemsKeyUUID() string {
	return s.itemsKeyUUID
}

// SignIn authenticates against the server as a client would.
func (s *Server) SignIn(debug bool) (auth.SignInResponseDataSession, error) {
	return auth.CliSignIn(s.Email, s.Password, s.URL, debug)
}

// Session signs in and returns a session ready to sync with. The session has
// not synced yet, so its default items key is only populated once the caller
// performs a sync.
func (s *Server) Session(debug bool) (*session.Session, error) {
	in, err := s.SignIn(debug)
	if err != nil {
		return nil, fmt.Errorf("mock: signing in: %w", err)
	}

	return &session.Session{
		Debug:             debug,
		HTTPClient:        in.HTTPClient,
		Server:            s.URL,
		Token:             in.Token,
		MasterKey:         in.MasterKey,
		KeyParams:         in.KeyParams,
		AccessToken:       in.AccessToken,
		AccessExpiration:  in.AccessExpiration,
		RefreshToken:      in.RefreshToken,
		RefreshExpiration: in.RefreshExpiration,
		ReadOnlyAccess:    in.ReadOnlyAccess,
		PasswordNonce:     in.PasswordNonce,
	}, nil
}

// Items returns everything the account holds, deleted items included, ordered
// by the revision at which each was last written.
func (s *Server) Items() []Item {
	s.mu.Lock()
	defer s.mu.Unlock()

	stored := make([]*storedItem, 0, len(s.stored))
	for _, si := range s.stored {
		stored = append(stored, si)
	}

	sort.Slice(stored, func(i, j int) bool { return stored[i].revision < stored[j].revision })

	out := make([]Item, 0, len(stored))
	for _, si := range stored {
		out = append(out, si.item)
	}

	return out
}

// ItemsOfType returns the undeleted items of the given content type.
func (s *Server) ItemsOfType(contentType string) []Item {
	var out []Item

	for _, i := range s.Items() {
		if i.ContentType == contentType && !i.Deleted {
			out = append(out, i)
		}
	}

	return out
}

// Syncs returns the number of sync requests handled so far.
func (s *Server) Syncs() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.syncs
}

// ExpireAccessToken makes the next sync request answer 401, so that callers can
// exercise the client's token refresh. The token issued by the refresh works.
func (s *Server) ExpireAccessToken() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.expired = true
}

// FailNextSyncs queues up statuses to answer the next sync requests with,
// one status per request, before serving normally again. Use it to exercise
// the client's retry and error handling.
func (s *Server) FailNextSyncs(statuses ...int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.failSync = append(s.failSync, statuses...)
}

// ConflictOn makes the server reject writes to the given item with a conflict
// of the given type, as it would when the item has changed underneath the
// client. Types are those the API uses, e.g. "sync_conflict" and
// "uuid_conflict".
func (s *Server) ConflictOn(uuid, conflictType string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.conflicts[uuid] = conflictType
}

// seedItemsKey creates the items key the account's items are encrypted with. A
// real account is given one at registration, and a client never creates one
// while syncing, so without it nothing can be encrypted.
func (s *Server) seedItemsKey() error {
	key, err := crypto.GenerateItemKey(64)
	if err != nil {
		return fmt.Errorf("mock: generating items key: %w", err)
	}

	uuid, err := newUUID()
	if err != nil {
		return err
	}

	encrypted, err := s.encryptItemsKey(uuid, key)
	if err != nil {
		return err
	}

	s.itemsKeyUUID = uuid
	s.put(encrypted)

	return nil
}

// itemsKeyContent is the plaintext of an SN|ItemsKey, matching the shape the
// client unmarshals after decrypting one.
type itemsKeyContent struct {
	ItemsKey string `json:"itemsKey"`
	Version  string `json:"version"`
	Default  bool   `json:"isDefault"`
}

// encryptItemsKey builds an SN|ItemsKey encrypted with the account master key,
// following the 004 protocol: the content is encrypted with a per-item key, and
// that key is encrypted with the master key.
func (s *Server) encryptItemsKey(uuid, key string) (Item, error) {
	content, err := json.Marshal(itemsKeyContent{
		ItemsKey: key,
		Version:  itemsKeyContentVersion,
		Default:  true,
	})
	if err != nil {
		return Item{}, fmt.Errorf("mock: marshalling items key content: %w", err)
	}

	itemEncryptionKey, err := crypto.GenerateItemKey(64)
	if err != nil {
		return Item{}, fmt.Errorf("mock: generating item encryption key: %w", err)
	}

	authData := base64.StdEncoding.EncodeToString(
		[]byte(auth.GenerateAuthData(common.SNItemTypeItemsKey, uuid, s.keyParams)),
	)

	encryptedContent, err := encryptString(string(content), itemEncryptionKey, authData)
	if err != nil {
		return Item{}, fmt.Errorf("mock: encrypting items key content: %w", err)
	}

	encryptedKey, err := encryptString(itemEncryptionKey, s.masterKey, authData)
	if err != nil {
		return Item{}, fmt.Errorf("mock: encrypting items key: %w", err)
	}

	return Item{
		UUID:        uuid,
		ContentType: common.SNItemTypeItemsKey,
		Content:     encryptedContent,
		EncItemKey:  encryptedKey,
	}, nil
}

// encryptString produces a 004 encrypted string: the version, the nonce, the
// ciphertext and the authenticated data, colon separated.
func encryptString(plainText, key, b64AuthData string) (string, error) {
	nonceBytes, err := crypto.GenerateNonce()
	if err != nil {
		return "", err
	}

	nonce := hex.EncodeToString(nonceBytes)

	cipherText, err := crypto.EncryptString(plainText, key, nonce, b64AuthData, 32)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s:%s:%s:%s", itemsKeyContentVersion, nonce, cipherText, b64AuthData), nil
}

// newUUID returns a random identifier in the format the API uses.
func newUUID() (string, error) {
	b, err := crypto.GenerateItemKey(32)
	if err != nil {
		return "", fmt.Errorf("mock: generating uuid: %w", err)
	}

	return fmt.Sprintf("%s-%s-%s-%s-%s", b[0:8], b[8:12], b[12:16], b[16:20], b[20:32]), nil
}

// put stores an item at a new revision, stamping it as the server would. The
// caller holds the lock, except during construction.
func (s *Server) put(item Item) Item {
	s.revision++

	now := time.Now().UTC()
	item.UpdatedAt = now.Format(common.TimeLayout)
	item.UpdatedAtTimestamp = now.UnixMicro()

	if existing, ok := s.stored[item.UUID]; ok {
		item.CreatedAt = existing.item.CreatedAt
		item.CreatedAtTimestamp = existing.item.CreatedAtTimestamp
	}

	if item.CreatedAt == "" {
		item.CreatedAt = item.UpdatedAt
	}

	if item.CreatedAtTimestamp == 0 {
		item.CreatedAtTimestamp = item.UpdatedAtTimestamp
	}

	s.stored[item.UUID] = &storedItem{item: item, revision: s.revision}

	return item
}
