package items

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jonhadfield/gosn-v2/auth"
	"github.com/jonhadfield/gosn-v2/common"
	"github.com/jonhadfield/gosn-v2/crypto"
	"github.com/jonhadfield/gosn-v2/log"
	"github.com/jonhadfield/gosn-v2/session"
	"github.com/santhosh-tekuri/jsonschema/v5"
)

type syncResponseData struct {
	Items        EncryptedItems  `json:"retrieved_items"`
	SavedItems   EncryptedItems  `json:"saved_items"`
	Unsaved      EncryptedItems  `json:"unsaved"`
	Conflicts    ConflictedItems `json:"conflicts"`
	SyncToken    string          `json:"sync_token"`
	CursorToken  string          `json:"cursor_token"`
	LastItemPut  int             // the last item successfully put
	PutLimitUsed int             // the put limit used
}
type syncResponse struct {
	Data syncResponseData `json:"data"`
}

// AppTagConfig defines expected configuration structure for making Tag related operations.
type AppTagConfig struct {
	Email    string
	Token    string
	FindText string
	FindTag  string
	NewTags  []string
	Debug    bool
}

const (
	retryScaleFactor   = 0.25
	statusInvalidToken = 498
)

// syncMutex serializes sync requests to prevent race conditions with:
// 1. Cookie jar concurrent access (cookiejar is not thread-safe)
// 2. HTTP connection pool reuse conflicts
// 3. Response body handling races
var syncMutex sync.Mutex

type EncryptedItems []EncryptedItem

func (ei EncryptedItems) DecryptAndParseItemsKeys(mk string, debug bool) (o []session.SessionItemsKey, err error) {
	log.DebugPrint(debug, fmt.Sprintf("DecryptAndParseItemsKeys | encrypted items to check: %d", len(ei)), common.MaxDebugChars)

	if len(ei) == 0 {
		return
	}

	var eiks EncryptedItems

	for _, e := range ei {
		if e.ContentType == common.SNItemTypeItemsKey && !e.Deleted {
			if e.UUID == "" {
				panic("DecryptAndParseItemsKeys | items key has no uuid")
			}

			if e.EncItemKey == "" {
				panic(fmt.Sprintf("DecryptAndParseItemsKeys | items key uuid: %s has no encrypted item key", e.UUID))
			}

			eiks = append(eiks, e)
		}
	}

	if len(eiks) == 0 {
		// err = fmt.Errorf("no items keys were retrieved")

		return
	}

	dpiks, err := DecryptAndParseItemKeys(mk, eiks)
	if err != nil {
		err = fmt.Errorf("DecryptAndParseItemsKeys | %w", err)

		return
	}

	if len(dpiks) == 0 {
		err = fmt.Errorf("failed to decrypt and parse items keys")
		return
	}

	for _, dpik := range dpiks {
		o = append(o, session.SessionItemsKey{
			UUID:               dpik.UUID,
			ItemsKey:           dpik.ItemsKey,
			Version:            dpik.Version,
			Default:            dpik.Default,
			CreatedAt:          dpik.CreatedAt,
			UpdatedAt:          dpik.UpdatedAt,
			CreatedAtTimestamp: dpik.CreatedAtTimestamp,
			UpdatedAtTimestamp: dpik.UpdatedAtTimestamp,
			Deleted:            dpik.Deleted,
		})
	}

	return
}

func IsEncryptedType(ct string) bool {
	switch {
	case strings.HasPrefix(ct, "SF"):
		return false
	case ct == common.SNItemTypeItemsKey:
		return false
	default:
		return true
	}
}

func (ei *EncryptedItems) Validate() error {
	var err error

	dei := *ei

	for x := range dei {
		enc := IsEncryptedType(dei[x].ContentType)

		switch {
		case dei[x].IsDeleted():
			continue
		case enc && dei[x].ItemsKeyID == "":
			// ignore item in this scenario as the official app does so
		case enc && dei[x].EncItemKey == "":
			err = fmt.Errorf("validation failed for \"%s\" due to missing encrypted item key: \"%s\"",
				dei[x].ContentType, dei[x].UUID)
		}

		if err != nil {
			return err
		}
	}

	return err
}

func ReEncryptItem(ei EncryptedItem, decryptionItemsKey session.SessionItemsKey, newItemsKey ItemsKey, newMasterKey string, s *session.Session) (o EncryptedItem, err error) {
	log.DebugPrint(s.Debug, fmt.Sprintf("ReEncrypt | item to re-encrypt %s %s", ei.ContentType, ei.UUID), common.MaxDebugChars)

	var di DecryptedItem

	di, err = DecryptItem(ei, s, []session.SessionItemsKey{decryptionItemsKey})

	if err != nil {
		err = fmt.Errorf("ReEncryptItem | Decrypt | %w", err)
		return
	}

	return di.Encrypt(newItemsKey, s)
}

func (ei EncryptedItems) ReEncrypt(s *session.Session, decryptionItemsKey session.SessionItemsKey, newItemsKey ItemsKey, newMasterKey string) (o EncryptedItems, err error) {
	log.DebugPrint(s.Debug, fmt.Sprintf("ReEncrypt | items: %d", len(ei)), common.MaxDebugChars)

	var di DecryptedItems

	di, err = DecryptItems(s, ei, []session.SessionItemsKey{decryptionItemsKey})

	if err != nil {
		err = fmt.Errorf("ReEncrypt | Decrypt | %w", err)
		return
	}

	for x := range di {
		// items key handled separately
		if di[x].ContentType == common.SNItemTypeItemsKey {
			continue
		}

		var ri EncryptedItem

		ri, err = di[x].Encrypt(newItemsKey, s)
		if err != nil {
			err = fmt.Errorf("ReEncrypt | Encrypt | %w", err)

			return
		}

		o = append(o, ri)
	}

	return o, err
}

func DecryptAndParseItem(ei EncryptedItem, s *session.Session) (o Item, err error) {
	log.DebugPrint(s.Debug, fmt.Sprintf("DecryptAndParse | items: %s %s", ei.ContentType, ei.UUID), common.MaxDebugChars)

	var di DecryptedItem
	//
	// if len(s.ImporterItemsKeys) > 0 {
	// 	logging.DebugPrint(s.Debug, "DecryptAndParse | using ImportersItemsKey", common.MaxDebugChars)
	// 	ik := GetMatchingItem(ei.ItemsKeyID, s.ImporterItemsKeys)
	//
	// 	di, err = DecryptItem(ei, s, ItemsKeys{ik})
	// } else
	di, err = DecryptItem(ei, s, []session.SessionItemsKey{})
	// }

	if err != nil {
		err = fmt.Errorf("DecryptAndParse | Decrypt | %w", err)
		return
	}

	o, err = ParseItem(di)
	if err != nil {
		err = fmt.Errorf("DecryptAndParse | ParseItem | %w", err)

		return
	}

	if s.SchemaValidation {
		var contentSchema *jsonschema.Schema

		switch it := o.(type) {
		case *Note:
			contentSchema = s.Schemas[noteContentSchemaName]
			if contentSchema == nil {
				err = fmt.Errorf("failed to get schema for %s", noteContentSchemaName)
				return
			}

			if err = validateContentSchema(s.Schemas[noteContentSchemaName], it.Content); err != nil {
				return
			}
		}
	}

	return
}

// func DecryptAndParseItems(ei EncryptedItems, s *session.Session) (o Items, err error) {
// 	log.DebugPrint(s.Debug, fmt.Sprintf("DecryptAndParse | items: %d", len(ei)), common.MaxDebugChars)
//
// 	for x := range ei {
// 		var di Item
//
// 		di, err = DecryptAndParseItem(ei[x], s)
// 		if err != nil {
// 			return
// 		}
//
// 		o = append(o, di)
// 	}
//
// 	return
// }

func (ei EncryptedItems) DecryptAndParse(s *session.Session) (o Items, err error) {
	log.DebugPrint(s.Debug, fmt.Sprintf("DecryptAndParse | items: %d", len(ei)), common.MaxDebugChars)

	var di DecryptedItems

	// if len(s.ImporterItemsKeys) > 0 && s.ImporterItemsKeys.Latest().UUID != "" {
	// 	logging.DebugPrint(s.Debug, "DecryptAndParse | using ImportersItemsKeys", common.MaxDebugChars)
	// 	di, err = DecryptItems(s, ei, s.ImporterItemsKeys)
	// } else {
	log.DebugPrint(s.Debug, "DecryptAndParse | using Session's ItemsKeys", common.MaxDebugChars)
	di, err = DecryptItems(s, ei, []session.SessionItemsKey{})
	// }

	if err != nil {
		err = fmt.Errorf("DecryptAndParse | Decrypt | %w", err)
		return
	}

	o, err = di.Parse()
	if err != nil {
		err = fmt.Errorf("DecryptAndParse | ParseItem | %w", err)

		return
	}

	return
}

func (i *Items) Append(x []interface{}) {
	var all Items

	for _, y := range x {
		switch t := y.(type) {
		case Note:
			it := t
			all = append(all, &it)
		case Tag:
			it := t
			all = append(all, &it)
		case Component:
			it := t
			all = append(all, &it)
		}
	}

	*i = all
}

func (i *Items) Encrypt(s *session.Session, ik session.SessionItemsKey) (e EncryptedItems, err error) {
	// return empty if no items provided
	if len(*i) == 0 {
		return
	}

	// fmt.Printf("Encrypt | encrypting %d items\n", len(*i))
	// for _, x := range *i {
	// 	fmt.Printf("----- %s %s\n", x.GetContentType(), x.GetUUID())
	// }
	e, err = encryptItems(s, i, ik)
	if err != nil {
		return
	}

	if err = e.Validate(); err != nil {
		return e, err
	}

	return
}

type EncryptedItem struct {
	// String fields (16 bytes each on 64-bit) - ordered for better cache locality
	UUID        string `json:"uuid"`
	ItemsKeyID  string `json:"items_key_id,omitempty"`
	Content     string `json:"content"`
	ContentType string `json:"content_type"`
	EncItemKey  string `json:"enc_item_key"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`

	// Pointer fields (8 bytes each)
	DuplicateOf         *string `json:"duplicate_of,omitempty"`
	AuthHash            *string `json:"auth_hash,omitempty"`
	UpdatedWithSession  *string `json:"updated_with_session,omitempty"`
	KeySystemIdentifier *string `json:"key_system_identifier,omitempty"`
	SharedVaultUUID     *string `json:"shared_vault_uuid,omitempty"`
	UserUUID            *string `json:"user_uuid,omitempty"`
	LastEditedByUUID    *string `json:"last_edited_by_uuid,omitempty"`

	// Integer fields (8 bytes each)
	CreatedAtTimestamp int64 `json:"created_at_timestamp"`
	UpdatedAtTimestamp int64 `json:"updated_at_timestamp"`

	// Boolean field (1 byte, minimal padding to 8-byte boundary)
	Deleted bool `json:"deleted"`
	// Default            bool    `json:"isDefault"`
}

func (ei EncryptedItem) GetItemsKeyID() string {
	if ei.ItemsKeyID != "" {
		return ei.ItemsKeyID
	}

	return ""
}

func (ei EncryptedItem) IsDeleted() bool {
	return ei.Deleted
}

type DecryptedItem struct {
	UUID                string  `json:"uuid"`
	ItemsKeyID          string  `json:"items_key_id,omitempty"`
	Content             string  `json:"content"`
	ContentType         string  `json:"content_type"`
	DuplicateOf         string  `json:"duplicate_of,omitempty"`
	Deleted             bool    `json:"deleted"`
	Default             bool    `json:"isDefault"`
	CreatedAt           string  `json:"created_at"`
	UpdatedAt           string  `json:"updated_at"`
	CreatedAtTimestamp  int64   `json:"created_at_timestamp"`
	UpdatedAtTimestamp  int64   `json:"updated_at_timestamp"`
	AuthHash            *string `json:"auth_hash,omitempty"`
	UpdatedWithSession  *string `json:"updated_with_session,omitempty"`
	KeySystemIdentifier *string `json:"key_system_identifier,omitempty"`
	SharedVaultUUID     *string `json:"shared_vault_uuid,omitempty"`
	UserUUID            *string `json:"user_uuid,omitempty"`
	LastEditedByUUID    *string `json:"last_edited_by_uuid,omitempty"`
}

type DecryptedItems []DecryptedItem

type UpdateItemRefsInput struct {
	Items Items // Tags
	ToRef Items // Items To Reference
}

type UpdateItemRefsOutput struct {
	Items Items // Tags
}

func UpdateItemRefs(i UpdateItemRefsInput) UpdateItemRefsOutput {
	var updated Items // updated tags

	for _, item := range i.Items {
		var refs ItemReferences

		for _, tr := range i.ToRef {
			ref := ItemReference{
				UUID:        tr.GetUUID(),
				ContentType: tr.GetContentType(),
			}
			refs = append(refs, ref)
		}

		switch item.GetContent().(type) {
		case *NoteContent:
			ic := item.GetContent().(*NoteContent)
			ic.UpsertReferences(refs)
			item.SetContent(ic)
		case *TagContent:
			ic := item.GetContent().(*TagContent)
			ic.UpsertReferences(refs)
			item.SetContent(ic)
		}

		updated = append(updated, item)
	}

	return UpdateItemRefsOutput{
		Items: updated,
	}
}

func makeSyncRequest(session *session.Session, reqBody []byte) (responseBody []byte, status int, err error) {
	// Serialize sync requests to prevent race conditions
	// This prevents concurrent access to the cookie jar and HTTP connection pool
	syncMutex.Lock()
	defer syncMutex.Unlock()
	// time.Sleep(3 * time.Second) // REMOVED: This was causing unnecessary delays
	// fmt.Println(string(reqBody))
	// Create HTTP client with connection pooling optimization
	// Reuse the Transport from session.HTTPClient for connection pooling benefits
	// while creating a fresh http.Client instance to avoid request state corruption
	log.DebugPrint(session.Debug, "makeSyncRequest | creating http client with connection pool reuse", common.MaxDebugChars)

	// Preserve both cookie jar and transport from session's HTTP client
	var existingCookieJar http.CookieJar
	var existingTransport http.RoundTripper

	if session.HTTPClient != nil && session.HTTPClient.HTTPClient != nil {
		if session.HTTPClient.HTTPClient.Jar != nil {
			existingCookieJar = session.HTTPClient.HTTPClient.Jar
			log.DebugPrint(session.Debug, "makeSyncRequest | preserving existing cookie jar with authentication cookies", common.MaxDebugChars)
		}
		if session.HTTPClient.HTTPClient.Transport != nil {
			existingTransport = session.HTTPClient.HTTPClient.Transport
			log.DebugPrint(session.Debug, "makeSyncRequest | reusing HTTP transport for connection pool optimization", common.MaxDebugChars)
		}
	}

	// Create a new http.Client instance but reuse Transport for connection pooling
	// This gives us connection pool benefits while avoiding request state corruption
	client := &http.Client{
		Timeout:   time.Duration(common.RequestTimeout) * time.Second,
		Jar:       existingCookieJar,
		Transport: existingTransport, // Reuse transport for connection pooling
	}

	// Allow overriding timeout via environment variable
	if envTimeout, ok, err := common.ParseEnvInt64(common.EnvRequestTimeout); err == nil && ok {
		client.Timeout = time.Duration(envTimeout) * time.Second
	}

	// Configure debug logging
	if session.Debug {
		log.DebugPrint(session.Debug, fmt.Sprintf("Standard HTTP Client configured with timeout=%v", client.Timeout), common.MaxDebugChars)
	}

	u := session.Server + common.SyncPath
	log.DebugPrint(session.Debug, fmt.Sprintf("makeSyncRequest | URL: %s", u), common.MaxDebugChars)
	request, err := http.NewRequest(http.MethodPost, u, bytes.NewBuffer(reqBody))
	if err != nil {
		return
	}
	request.Header.Set(common.HeaderContentType, common.SNAPIContentType)

	// For cookie-based authentication (tokens starting with "2:"), set Cookie header manually
	// because Go's cookie jar doesn't properly handle the Partitioned attribute
	accessParts := strings.Split(session.AccessToken, ":")
	isCookieBased := len(accessParts) >= 2 && accessParts[0] == "2"

	if isCookieBased && session.AccessTokenCookie != "" {
		// For cookie-based auth, send BOTH Cookie and Authorization headers
		request.Header.Set("Cookie", session.AccessTokenCookie)
		request.Header.Set("Authorization", "Bearer "+session.AccessToken)
		log.DebugPrint(session.Debug, "Using cookie-based authentication (Cookie + Authorization headers)", common.MaxDebugChars)
	} else {
		// For header-based auth, send Authorization header only
		request.Header.Set("Authorization", "Bearer "+session.AccessToken)
		log.DebugPrint(session.Debug, "Using header-based authentication (Authorization header only)", common.MaxDebugChars)
	}

	request.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) StandardNotes/3.198.18 Chrome/134.0.6998.205 Electron/35.2.0 Safari/537.36")

	// Create a context with timeout for the request
	timeout := common.RequestTimeout
	if envTimeout, ok, err := common.ParseEnvInt64(common.EnvRequestTimeout); err == nil && ok {
		timeout = int(envTimeout)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()
	request = request.WithContext(ctx)

	// Print headers
	for name, values := range request.Header {
		for _, value := range values {
			// Mask the Authorization header for security
			if name == "Authorization" && len(value) > 20 {
				maskedValue := value[:7] + "..." + value[len(value)-4:]
				log.DebugPrint(session.Debug, fmt.Sprintf("  %s: %s", name, maskedValue), common.MaxDebugChars)
			} else {
				log.DebugPrint(session.Debug, fmt.Sprintf("  %s: %s", name, value), common.MaxDebugChars)
			}
		}
	}

	// Print request body size and summary
	log.DebugPrint(session.Debug, fmt.Sprintf("Request Body Size: %d bytes", len(reqBody)), common.MaxDebugChars)

	// Parse and log key request body details (for debugging sync requests)
	if len(reqBody) > 0 {
		var reqData map[string]interface{}
		if json.Unmarshal(reqBody, &reqData) == nil {
			if api, ok := reqData["api"].(string); ok {
				log.DebugPrint(session.Debug, fmt.Sprintf("API Version: %s", api), common.MaxDebugChars)
			}
			if limit, ok := reqData["limit"].(float64); ok {
				log.DebugPrint(session.Debug, fmt.Sprintf("Limit: %.0f", limit), common.MaxDebugChars)
			}
			if items, ok := reqData["items"].([]interface{}); ok {
				log.DebugPrint(session.Debug, fmt.Sprintf("Items to sync: %d", len(items)), common.MaxDebugChars)
			}
			if syncToken, ok := reqData["sync_token"].(string); ok && syncToken != "" {
				// Show only first and last few characters of sync token for privacy
				if len(syncToken) > 20 {
					maskedToken := syncToken[:8] + "..." + syncToken[len(syncToken)-8:]
					log.DebugPrint(session.Debug, fmt.Sprintf("Sync Token: %s", maskedToken), common.MaxDebugChars)
				} else {
					log.DebugPrint(session.Debug, fmt.Sprintf("Sync Token: %s", syncToken), common.MaxDebugChars)
				}
			}
			if cursorToken, ok := reqData["cursor_token"].(string); ok && cursorToken != "" && cursorToken != "null" {
				log.DebugPrint(session.Debug, fmt.Sprintf("Cursor Token: %s", cursorToken), common.MaxDebugChars)
			}
		}
	}

	// Log full request body for small payloads to debug differences between requests
	if len(reqBody) < 300 {
		log.DebugPrint(session.Debug, fmt.Sprintf("Full Request Body: %s", string(reqBody)), 1000)
	}

	log.DebugPrint(session.Debug, "=== END REQUEST DETAILS ===", common.MaxDebugChars)

	start := time.Now()
	log.DebugPrint(session.Debug, fmt.Sprintf("Making sync request at %s", time.Now().Format("15:04:05.000")), common.MaxDebugChars)
	log.DebugPrint(session.Debug, fmt.Sprintf("Standard HTTP Client Timeout: %v", client.Timeout), common.MaxDebugChars)
	log.DebugPrint(session.Debug, fmt.Sprintf("Request URL: %s", request.URL.String()), common.MaxDebugChars)
	log.DebugPrint(session.Debug, fmt.Sprintf("Request Method: %s", request.Method), common.MaxDebugChars)
	log.DebugPrint(session.Debug, fmt.Sprintf("Request Context Timeout: %v", request.Context().Value("timeout")), common.MaxDebugChars)
	deadline, hasDeadline := request.Context().Deadline()
	log.DebugPrint(session.Debug, fmt.Sprintf("Request Context Deadline: %v (has deadline: %v)", deadline, hasDeadline), common.MaxDebugChars)

	// Check if HTTP client has cookie jar
	if client.Jar != nil {
		log.DebugPrint(session.Debug, "Cookie jar is enabled", common.MaxDebugChars)
		log.DebugPrint(session.Debug, fmt.Sprintf("Checking cookies for URL: %s", request.URL.String()), common.MaxDebugChars)
		cookies := client.Jar.Cookies(request.URL)
		log.DebugPrint(session.Debug, fmt.Sprintf("Cookies for URL: %d cookies", len(cookies)), common.MaxDebugChars)
		for i, cookie := range cookies {
			cookieValue := cookie.Value
			if len(cookieValue) > 10 {
				cookieValue = cookieValue[:10] + "..."
			}
			log.DebugPrint(session.Debug, fmt.Sprintf("Cookie %d: %s=%s (Domain=%s Path=%s Secure=%v HttpOnly=%v)",
				i, cookie.Name, cookieValue, cookie.Domain, cookie.Path, cookie.Secure, cookie.HttpOnly), common.MaxDebugChars)
		}
	} else {
		log.DebugPrint(session.Debug, "Cookie jar is NOT enabled", common.MaxDebugChars)
	}

	log.DebugPrint(session.Debug, "=== STARTING HTTP REQUEST ===", common.MaxDebugChars)

	// Implement exponential backoff retry for HTTP 429 (Too Many Requests)
	var response *http.Response
	var requestErr error
	var elapsed time.Duration
	maxRetries := common.MaxRequestRetries

	for attempt := 0; attempt <= maxRetries; attempt++ {
		// Make the request with detailed error logging using standard HTTP client
		response, requestErr = client.Do(request)

		// Calculate elapsed time and add checkpoint after request
		elapsed = time.Since(start)

		if requestErr != nil {
			log.DebugPrint(session.Debug, fmt.Sprintf("HTTP request failed after %v (attempt %d/%d)", elapsed, attempt+1, maxRetries+1), common.MaxDebugChars)
			log.DebugPrint(session.Debug, fmt.Sprintf("Error type: %T", requestErr), common.MaxDebugChars)
			log.DebugPrint(session.Debug, fmt.Sprintf("Error details: %+v", requestErr), common.MaxDebugChars)
			// For non-HTTP errors, return immediately (network issues, etc.)
			return nil, 0, requestErr
		}

		// Check for HTTP 429 (Too Many Requests)
		if response.StatusCode == http.StatusTooManyRequests {
			if attempt < maxRetries {
				// Calculate exponential backoff delay: 1s, 2s, 4s, 8s, 16s
				delay := time.Duration(1<<uint(attempt)) * time.Second
				log.DebugPrint(session.Debug, fmt.Sprintf("HTTP 429 Too Many Requests - retrying in %v (attempt %d/%d)", delay, attempt+1, maxRetries+1), common.MaxDebugChars)

				// Close the current response body before retrying
				if response.Body != nil {
					_ = response.Body.Close()
				}

				// Wait with exponential backoff
				time.Sleep(delay)

				// Create a new request with fresh body for retry
				request, err = http.NewRequest(http.MethodPost, u, bytes.NewBuffer(reqBody))
				if err != nil {
					return nil, 0, fmt.Errorf("failed to create retry request: %w", err)
				}
				request.Header.Set(common.HeaderContentType, common.SNAPIContentType)
				request.Header.Set("Authorization", "Bearer "+session.AccessToken)
				request.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) StandardNotes/3.198.18 Chrome/134.0.6998.205 Electron/35.2.0 Safari/537.36")
				request = request.WithContext(ctx)

				continue // Retry the request
			} else {
				// Max retries exceeded for 429
				log.DebugPrint(session.Debug, fmt.Sprintf("HTTP 429 Too Many Requests - max retries (%d) exceeded", maxRetries), common.MaxDebugChars)
			}
		}

		// If we get here, either the request succeeded (not 429) or we've exceeded retries
		break
	}

	// Continue with existing error handling and response processing
	err = requestErr
	if err != nil {
		// Error handling already done in retry loop above
	} else {
		log.DebugPrint(session.Debug, fmt.Sprintf("HTTP request succeeded in %v", elapsed), common.MaxDebugChars)
		if response != nil {
			log.DebugPrint(session.Debug, fmt.Sprintf("Response status: %d %s", response.StatusCode, response.Status), common.MaxDebugChars)
		}
	}
	if err != nil {
		log.DebugPrint(session.Debug, "=== SYNC REQUEST FAILED ===", common.MaxDebugChars)
		log.DebugPrint(session.Debug, fmt.Sprintf("Request duration: %v", elapsed), common.MaxDebugChars)
		log.DebugPrint(session.Debug, fmt.Sprintf("Error: %v", err), common.MaxDebugChars)
		log.DebugPrint(session.Debug, "=== END REQUEST FAILURE ===", common.MaxDebugChars)

		// Check if context was cancelled (timeout)
		if errors.Is(err, context.DeadlineExceeded) {
			fmt.Printf("Request timeout after %d seconds\n", timeout)
			return nil, 0, fmt.Errorf("request timeout: %w", err)
		}
		return nil, 0, err
	}
	if response == nil {
		log.DebugPrint(session.Debug, "=== SYNC RESPONSE ERROR ===", common.MaxDebugChars)
		log.DebugPrint(session.Debug, "Response is nil", common.MaxDebugChars)
		log.DebugPrint(session.Debug, "=== END RESPONSE ERROR ===", common.MaxDebugChars)
		return nil, 0, errors.New("response is nil")
	}

	// Print full response details
	log.DebugPrint(session.Debug, "=== SYNC RESPONSE DETAILS ===", common.MaxDebugChars)
	log.DebugPrint(session.Debug, fmt.Sprintf("Request duration: %v", elapsed), common.MaxDebugChars)
	log.DebugPrint(session.Debug, fmt.Sprintf("Status Code: %d %s", response.StatusCode, response.Status), common.MaxDebugChars)
	log.DebugPrint(session.Debug, fmt.Sprintf("Response received at: %s", time.Now().Format("15:04:05.000")), common.MaxDebugChars)

	// Print response headers
	log.DebugPrint(session.Debug, "Response Headers:", common.MaxDebugChars)
	for name, values := range response.Header {
		for _, value := range values {
			log.DebugPrint(session.Debug, fmt.Sprintf("  %s: %s", name, value), common.MaxDebugChars)
		}
	}

	log.DebugPrint(session.Debug, "=== END RESPONSE DETAILS ===", common.MaxDebugChars)

	// Read the response body first to prevent race conditions
	responseBody, err = io.ReadAll(response.Body)
	if err != nil {
		log.DebugPrint(session.Debug, fmt.Sprintf("makeSyncRequest | failed to read response body: %v", err), common.MaxDebugChars)
		// Still need to close the body even if read failed
		if response.Body != nil {
			_ = response.Body.Close()
		}
		return nil, response.StatusCode, fmt.Errorf("failed to read response body: %w", err)
	}

	// Close the response body after successful read
	if response.Body != nil {
		if closeErr := response.Body.Close(); closeErr != nil {
			log.DebugPrint(session.Debug, fmt.Sprintf("makeSyncRequest | failed to close response body: %v", closeErr), common.MaxDebugChars)
		}
	}

	// Log response body details
	log.DebugPrint(session.Debug, "=== SYNC RESPONSE BODY ===", common.MaxDebugChars)
	log.DebugPrint(session.Debug, fmt.Sprintf("Response Body Size: %d bytes", len(responseBody)), common.MaxDebugChars)

	// Parse and log key response details for successful responses
	if response.StatusCode == http.StatusOK && len(responseBody) > 0 {
		var respData map[string]interface{}
		if json.Unmarshal(responseBody, &respData) == nil {
			if data, ok := respData["data"].(map[string]interface{}); ok {
				if items, ok := data["items"].([]interface{}); ok {
					log.DebugPrint(session.Debug, fmt.Sprintf("Items returned: %d", len(items)), common.MaxDebugChars)
				}
				if savedItems, ok := data["saved_items"].([]interface{}); ok {
					log.DebugPrint(session.Debug, fmt.Sprintf("Saved items: %d", len(savedItems)), common.MaxDebugChars)
				}
				if conflicts, ok := data["conflicts"].([]interface{}); ok && len(conflicts) > 0 {
					log.DebugPrint(session.Debug, fmt.Sprintf("Conflicts: %d", len(conflicts)), common.MaxDebugChars)
				}
				if syncToken, ok := data["sync_token"].(string); ok && syncToken != "" {
					// Show only first and last few characters of sync token for privacy
					if len(syncToken) > 20 {
						maskedToken := syncToken[:8] + "..." + syncToken[len(syncToken)-8:]
						log.DebugPrint(session.Debug, fmt.Sprintf("New Sync Token: %s", maskedToken), common.MaxDebugChars)
					} else {
						log.DebugPrint(session.Debug, fmt.Sprintf("New Sync Token: %s", syncToken), common.MaxDebugChars)
					}
				}
			}
		}
	} else if response.StatusCode != http.StatusOK {
		// For error responses, show the response body (truncated if too long)
		bodyStr := string(responseBody)
		if len(bodyStr) > 500 {
			bodyStr = bodyStr[:500] + "... (truncated)"
		}
		log.DebugPrint(session.Debug, fmt.Sprintf("Error Response Body: %s", bodyStr), 500)
	}

	log.DebugPrint(session.Debug, "=== END RESPONSE BODY ===", common.MaxDebugChars)

	if response.StatusCode == http.StatusRequestEntityTooLarge {
		err = errors.New("payload too large")
		return responseBody, response.StatusCode, err
	}

	if response.StatusCode == statusInvalidToken {
		err = errors.New("session token is invalid or has expired")
		return responseBody, response.StatusCode, err
	}

	if response.StatusCode == http.StatusUnauthorized {
		log.DebugPrint(session.Debug, fmt.Sprintf("makeSyncRequest | sync of %d req bytes failed with: %s", len(reqBody), response.Status), common.MaxDebugChars)
		log.DebugPrint(session.Debug, "makeSyncRequest | attempting to refresh access token due to 401 Unauthorized", common.MaxDebugChars)

		// Attempt to refresh the access token before failing
		refreshURL := session.Server + common.AuthRefreshPath
		refreshResp, refreshErr := auth.RequestRefreshTokenWithSession(&auth.SignInResponseDataSession{
			HTTPClient:        session.HTTPClient,
			Debug:             session.Debug,
			Server:            session.Server,
			Token:             session.Token,
			MasterKey:         session.MasterKey,
			KeyParams:         session.KeyParams,
			AccessToken:       session.AccessToken,
			RefreshToken:      session.RefreshToken,
			AccessExpiration:  session.AccessExpiration,
			RefreshExpiration: session.RefreshExpiration,
			ReadOnlyAccess:    session.ReadOnlyAccess,
			PasswordNonce:     session.PasswordNonce,
		}, refreshURL, session.Debug)

		if refreshErr != nil {
			log.DebugPrint(session.Debug, fmt.Sprintf("makeSyncRequest | token refresh failed: %v", refreshErr), common.MaxDebugChars)
			err = fmt.Errorf("server returned 401 unauthorized and token refresh failed: %w", refreshErr)
			return responseBody, response.StatusCode, err
		}

		// Update session with new tokens
		session.AccessToken = refreshResp.Data.Session.AccessToken
		session.RefreshToken = refreshResp.Data.Session.RefreshToken
		session.AccessExpiration = refreshResp.Data.Session.AccessExpiration
		session.RefreshExpiration = refreshResp.Data.Session.RefreshExpiration
		log.DebugPrint(session.Debug, "makeSyncRequest | successfully refreshed access token, retrying original request", common.MaxDebugChars)

		// Retry the original request with the new token
		// Create a new request with fresh body since the original body was already consumed
		retryRequest, retryReqErr := http.NewRequest(http.MethodPost, u, bytes.NewBuffer(reqBody))
		if retryReqErr != nil {
			log.DebugPrint(session.Debug, fmt.Sprintf("makeSyncRequest | failed to create retry request: %v", retryReqErr), common.MaxDebugChars)
			return nil, 0, fmt.Errorf("failed to create retry request: %w", retryReqErr)
		}
		retryRequest.Header.Set(common.HeaderContentType, common.SNAPIContentType)
		retryRequest.Header.Set("Authorization", "Bearer "+session.AccessToken)
		retryRequest.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) StandardNotes/3.198.18 Chrome/134.0.6998.205 Electron/35.2.0 Safari/537.36")
		retryRequest = retryRequest.WithContext(ctx)

		log.DebugPrint(session.Debug, "makeSyncRequest | retrying request with refreshed token", common.MaxDebugChars)
		retryResponse, retryErr := client.Do(retryRequest)
		if retryErr != nil {
			log.DebugPrint(session.Debug, fmt.Sprintf("makeSyncRequest | retry after token refresh failed: %v", retryErr), common.MaxDebugChars)
			return nil, 0, fmt.Errorf("retry after token refresh failed: %w", retryErr)
		}

		// Read retry response body
		retryResponseBody, retryReadErr := io.ReadAll(retryResponse.Body)
		if retryReadErr != nil {
			if retryResponse.Body != nil {
				_ = retryResponse.Body.Close()
			}
			return nil, retryResponse.StatusCode, fmt.Errorf("failed to read retry response body: %w", retryReadErr)
		}

		// Close retry response body
		if retryResponse.Body != nil {
			_ = retryResponse.Body.Close()
		}

		log.DebugPrint(session.Debug, fmt.Sprintf("makeSyncRequest | retry request completed with status: %d", retryResponse.StatusCode), common.MaxDebugChars)
		return retryResponseBody, retryResponse.StatusCode, nil
	}

	if response.StatusCode > http.StatusBadRequest {
		log.DebugPrint(session.Debug, fmt.Sprintf("makeSyncRequest | sync of %d req bytes failed with: %s", len(reqBody), response.Status), common.MaxDebugChars)
		return responseBody, response.StatusCode, fmt.Errorf("unexpected status code: %d", response.StatusCode)
	}

	if response.StatusCode >= http.StatusOK && response.StatusCode < http.StatusMultipleChoices {
		log.DebugPrint(session.Debug, fmt.Sprintf("makeSyncRequest | sync of %d req bytes succeeded with: %s", len(reqBody), response.Status), common.MaxDebugChars)
	}

	return responseBody, response.StatusCode, nil
}

// ItemReference defines a reference from one item to another.
type ItemReference struct {
	// unique identifier of the item being referenced
	UUID string `json:"uuid"`
	// type of item being referenced
	ContentType string `json:"content_type"`
	// type of reference, notetonote, tagtonote, etc
	ReferenceType string `json:"reference_type,omitempty"`
}

type OrgStandardNotesSNDetail struct {
	ClientUpdatedAt    string `json:"client_updated_at"`
	PrefersPlainEditor bool   `json:"prefersPlainEditor"`
	Pinned             bool   `json:"pinned"`
}

type OrgStandardNotesSNComponentsDetail map[string]interface{}

type AppDataContent struct {
	OrgStandardNotesSN           OrgStandardNotesSNDetail           `json:"org.standardnotes.sn"`
	OrgStandardNotesSNComponents OrgStandardNotesSNComponentsDetail `json:"org.standardnotes.sn.components,omitempty"`
}

type NoteAppDataContent struct {
	OrgStandardNotesSN           OrgStandardNotesSNDetail           `json:"org.standardnotes.sn"`
	OrgStandardNotesSNComponents OrgStandardNotesSNComponentsDetail `json:"org.standardnotes.sn.components,omitempty"`
}

type TagContent struct {
	Title          string         `json:"title"`
	ItemReferences ItemReferences `json:"references"`
	AppData        AppDataContent `json:"appData"`
	IconString     string         `json:"iconString,omitempty"` // Icon for the tag (emoji or icon name)
	Expanded       bool           `json:"expanded,omitempty"`   // Whether tag is expanded in UI
	ParentId       string         `json:"parentId,omitempty"`   // Parent tag for nested tags
	// Missing attributes from official Standard Notes
	Preferences interface{} `json:"preferences,omitempty"` // TagPreferences object
}

func removeStringFromSlice(inSt string, inSl []string) []string {
	return slices.DeleteFunc(inSl, func(s string) bool { return s == inSt })
}

type ItemReferences []ItemReference

type Items []Item

func (i Items) UUIDs() []string {
	var uuids []string

	for _, ii := range i {
		uuids = append(uuids, ii.GetUUID())
	}

	return uuids
}

func ParseItem(di DecryptedItem) (p Item, err error) {
	var pi Item

	switch di.ContentType {
	case common.SNItemTypeItemsKey:
		// TODO: To be implemented separately so we don't parse as a normal item and,
		// most importantly, don't return as a normal Item
	case common.SNItemTypeNote:
		pi = parseNote(di)
	case common.SNItemTypeTag:
		pi = parseTag(di)
	case common.SNItemTypeComponent:
		pi = parseComponent(di)
	case common.SNItemTypeTheme:
		pi = parseTheme(di)
	case common.SNItemTypePrivileges:
		pi = parsePrivileges(di)
	case common.SNItemTypeExtension:
		pi = parseExtension(di)
	case common.SNItemTypeSFExtension:
		pi = parseSFExtension(di)
	case common.SNItemTypeSFMFA:
		pi = parseSFMFA(di)
	case common.SNItemTypeSmartTag:
		pi = parseSmartView(di)
	case common.SNItemTypeFileSafeFileMetaData:
		pi = parseFileSafeFileMetadata(di)
	case common.SNItemTypeFileSafeIntegration:
		pi = parseFileSafeIntegration(di)
	case common.SNItemTypeUserPreferences:
		pi = parseUserPreferences(di)
	case common.SNItemTypeExtensionRepo:
		pi = parseExtensionRepo(di)
	case common.SNItemTypeFileSafeCredentials:
		pi = parseFileSafeCredentials(di)
	case common.SNItemTypeFile:
		pi = parseFile(di)
	case common.SNItemTypeTrustedContact:
		pi = parseTrustedContact(di)
	case common.SNItemTypeVaultListing:
		pi = parseVaultListing(di)
	case common.SNItemTypeKeySystemRootKey:
		pi = parseKeySystemRootKey(di)
	case common.SNItemTypeKeySystemItemsKey:
		pi = parseKeySystemItemsKey(di)
	default:
		return nil, fmt.Errorf("unhandled type1 '%s' %s", di.ContentType, di.Content)
	}

	return pi, err
}

func (di *DecryptedItems) Parse() (p Items, err error) {
	for _, i := range *di {
		var pi Item

		switch i.ContentType {
		case common.SNItemTypeItemsKey:
			// TODO: To be implemented separately so we don't parse as a normal item and,
			// most importantly, don't return as a normal Item
			continue
		case common.SNItemTypeNote:
			pi = parseNote(i)
		case common.SNItemTypeTag:
			pi = parseTag(i)
		case common.SNItemTypeComponent:
			pi = parseComponent(i)
		case common.SNItemTypeTheme:
			pi = parseTheme(i)
		case common.SNItemTypePrivileges:
			pi = parsePrivileges(i)
		case common.SNItemTypeExtension:
			pi = parseExtension(i)
		case common.SNItemTypeSFExtension:
			pi = parseSFExtension(i)
		case common.SNItemTypeSFMFA:
			pi = parseSFMFA(i)
		case common.SNItemTypeSmartTag:
			pi = parseSmartView(i)
		case common.SNItemTypeFileSafeFileMetaData:
			pi = parseFileSafeFileMetadata(i)
		case common.SNItemTypeFileSafeIntegration:
			pi = parseFileSafeIntegration(i)
		case common.SNItemTypeUserPreferences:
			pi = parseUserPreferences(i)
		case common.SNItemTypeExtensionRepo:
			pi = parseExtensionRepo(i)
		case common.SNItemTypeFileSafeCredentials:
			pi = parseFileSafeCredentials(i)
		case common.SNItemTypeFile:
			pi = parseFile(i)
		case common.SNItemTypeTrustedContact:
			pi = parseTrustedContact(i)
		case common.SNItemTypeVaultListing:
			pi = parseVaultListing(i)
		case common.SNItemTypeKeySystemRootKey:
			pi = parseKeySystemRootKey(i)
		case common.SNItemTypeKeySystemItemsKey:
			pi = parseKeySystemItemsKey(i)
		default:
			return nil, fmt.Errorf("unhandled type2 '%s' %s", i.ContentType, i.Content)
		}

		p = append(p, pi)
	}

	return p, err
}

func processContentModel(contentType, input string) (output Content, err error) {
	// identify content model
	// try and unmarshall Item
	switch contentType {
	case common.SNItemTypeNote:
		var nc NoteContent

		if err = json.Unmarshal([]byte(input), &nc); err != nil {
			err = fmt.Errorf("processContentModel note | %w", err)

			return output, err
		}

		return &nc, nil
	case common.SNItemTypeTag:
		var tc TagContent
		if err = json.Unmarshal([]byte(input), &tc); err != nil {
			err = fmt.Errorf("processContentModel tag | %w", err)

			return output, err
		}

		return &tc, nil
	case common.SNItemTypeComponent:
		var cc ComponentContent
		if err = json.Unmarshal([]byte(input), &cc); err != nil {
			err = fmt.Errorf("processContentModel component | %w", err)

			return
		}

		return &cc, nil
	case common.SNItemTypeTheme:
		var tc ThemeContent
		if err = json.Unmarshal([]byte(input), &tc); err != nil {
			err = fmt.Errorf("processContentModel theme | %w", err)

			return
		}

		return &tc, nil
	case common.SNItemTypePrivileges:
		var pc PrivilegesContent
		if err = json.Unmarshal([]byte(input), &pc); err != nil {
			if err = json.Unmarshal([]byte(input), &pc); err != nil {
				err = fmt.Errorf("processContentModel privileges | %w", err)

				return
			}
		}

		return &pc, nil
	case common.SNItemTypeExtension:
		var ec ExtensionContent
		if err = json.Unmarshal([]byte(input), &ec); err != nil {
			err = fmt.Errorf("processContentModel extension | %w", err)

			return
		}

		return &ec, nil
	case common.SNItemTypeSFExtension:
		var sfe SFExtensionContent

		if len(input) > 0 {
			if err = json.Unmarshal([]byte(input), &sfe); err != nil {
				err = fmt.Errorf("processContentModel sf extension | %w", err)

				return
			}
		}

		return &sfe, nil
	case common.SNItemTypeSFMFA:
		var sfm SFMFAContent

		if len(input) > 0 {
			if err = json.Unmarshal([]byte(input), &sfm); err != nil {
				err = fmt.Errorf("processContentModel sf mfa | %w", err)

				return
			}
		}

		return &sfm, nil
	case common.SNItemTypeSmartTag:
		var sv SmartViewContent

		if len(input) > 0 {
			if err = json.Unmarshal([]byte(input), &sv); err != nil {
				err = fmt.Errorf("processContentModel smart view | %w", err)

				return
			}
		}

		return &sv, nil

	case common.SNItemTypeFileSafeFileMetaData:
		var fsfm FileSafeFileMetaDataContent

		if len(input) > 0 {
			if err = json.Unmarshal([]byte(input), &fsfm); err != nil {
				err = fmt.Errorf("processContentModel sf metadata | %w", err)

				return
			}
		}

		return &fsfm, nil

	case common.SNItemTypeFileSafeIntegration:
		var fsi FileSafeIntegrationContent

		if len(input) > 0 {
			if err = json.Unmarshal([]byte(input), &fsi); err != nil {
				err = fmt.Errorf("processContentModel fs integration | %w", err)

				return
			}
		}

		return &fsi, nil
	case common.SNItemTypeUserPreferences:
		var upc UserPreferencesContent

		if len(input) > 0 {
			if err = json.Unmarshal([]byte(input), &upc); err != nil {
				err = fmt.Errorf("processContentModel user preferences | %w", err)

				return
			}
		}

		return &upc, nil
	case common.SNItemTypeExtensionRepo:
		var erc ExtensionRepoContent

		if len(input) > 0 {
			if err = json.Unmarshal([]byte(input), &erc); err != nil {
				err = fmt.Errorf("processContentModel extension repo | %w", err)

				return
			}
		}

		return &erc, nil
	case common.SNItemTypeFileSafeCredentials:
		var fsc FileSafeCredentialsContent

		if len(input) > 0 {
			if err = json.Unmarshal([]byte(input), &fsc); err != nil {
				err = fmt.Errorf("processContentModel fs credentials | %w", err)

				return
			}
		}

		return &fsc, nil
	case common.SNItemTypeFile:
		var fc FileContent

		if len(input) > 0 {
			if err = json.Unmarshal([]byte(input), &fc); err != nil {
				err = fmt.Errorf("processContentModel file | %w", err)

				return
			}
		}

		return &fc, nil
	case common.SNItemTypeTrustedContact:
		var tc TrustedContactContent

		if len(input) > 0 {
			if err = json.Unmarshal([]byte(input), &tc); err != nil {
				err = fmt.Errorf("processContentModel trusted contact | %w", err)

				return
			}
		}

		return &tc, nil
	case common.SNItemTypeVaultListing:
		var vl VaultListingContent

		if len(input) > 0 {
			if err = json.Unmarshal([]byte(input), &vl); err != nil {
				err = fmt.Errorf("processContentModel vault listing | %w", err)

				return
			}
		}

		return &vl, nil
	case common.SNItemTypeKeySystemRootKey:
		var ksrk KeySystemRootKeyContent

		if len(input) > 0 {
			if err = json.Unmarshal([]byte(input), &ksrk); err != nil {
				err = fmt.Errorf("processContentModel key system root key | %w", err)

				return
			}
		}

		return &ksrk, nil
	case common.SNItemTypeKeySystemItemsKey:
		var ksik KeySystemItemsKeyContent

		if len(input) > 0 {
			if err = json.Unmarshal([]byte(input), &ksik); err != nil {
				err = fmt.Errorf("processContentModel key system items key | %w", err)

				return
			}
		}

		return &ksik, nil
	default:
		return nil, fmt.Errorf("unexpected type '%s'", contentType)
	}
}

func (ei *EncryptedItems) DeDupe() {
	if ei == nil {
		return
	}

	uniqueItems := make(map[string]EncryptedItem)

	var deDuped EncryptedItems

	eis := *ei
	for _, ei1 := range eis {
		if _, ok := uniqueItems[ei1.UUID]; ok {
			if ei1.UpdatedAtTimestamp > uniqueItems[ei1.UUID].UpdatedAtTimestamp {
				uniqueItems[ei1.UUID] = ei1
			}
		} else {
			uniqueItems[ei1.UUID] = ei1
		}
	}

	for _, v := range uniqueItems {
		deDuped = append(deDuped, v)
	}

	*ei = deDuped
}

func (ei *EncryptedItems) RemoveUnsupported() {
	var supported EncryptedItems

	for _, i := range *ei {
		if !slices.Contains([]string{common.SNItemTypeSFExtension}, i.ContentType) && !strings.HasPrefix(i.Content, "003") {
			supported = append(supported, i)
		}
		// if !strings.HasPrefix(i.Content, "003") {
		// 	supported = append(supported, i)
		// }
	}

	*ei = supported
}

func (ei *EncryptedItems) RemoveDeleted() {
	var clean EncryptedItems

	for _, i := range *ei {
		if !i.Deleted {
			clean = append(clean, i)
		}
	}

	*ei = clean
}

func (i *Items) DeDupe() {
	encountered := make(map[string]struct{})
	deDuped := make(Items, 0, len(*i))

	for _, j := range *i {
		if _, ok := encountered[j.GetUUID()]; !ok {
			encountered[j.GetUUID()] = struct{}{}
			deDuped = append(deDuped, j)
		}
	}

	*i = deDuped
}

func (i *Items) RemoveDeleted() {
	var clean Items

	for _, j := range *i {
		if !j.IsDeleted() {
			clean = append(clean, j)
		}
	}

	*i = clean
}

func (di *DecryptedItems) RemoveDeleted() {
	var clean DecryptedItems

	for _, j := range *di {
		if !j.Deleted {
			clean = append(clean, j)
		}
	}

	*di = clean
}

type EncryptedItemExport struct {
	UUID        string `json:"uuid"`
	ItemsKeyID  string `json:"items_key_id,omitempty"`
	Content     string `json:"content"`
	ContentType string `json:"content_type"`
	// Deleted            bool    `json:"deleted"`
	EncItemKey         string  `json:"enc_item_key"`
	CreatedAt          string  `json:"created_at"`
	UpdatedAt          string  `json:"updated_at"`
	CreatedAtTimestamp int64   `json:"created_at_timestamp"`
	UpdatedAtTimestamp int64   `json:"updated_at_timestamp"`
	DuplicateOf        *string `json:"duplicate_of"`
}

type writeJSONConfig struct {
	session session.Session
	Path    string
	Debug   bool
}

func writeJSON(c writeJSONConfig, items EncryptedItems) error {
	// prepare for export
	var itemsExport []EncryptedItemExport

	for x := range items {
		itemsExport = append(itemsExport, EncryptedItemExport{
			UUID:               items[x].UUID,
			ItemsKeyID:         items[x].ItemsKeyID,
			Content:            items[x].Content,
			ContentType:        items[x].ContentType,
			EncItemKey:         items[x].EncItemKey,
			CreatedAt:          items[x].CreatedAt,
			UpdatedAt:          items[x].UpdatedAt,
			CreatedAtTimestamp: items[x].CreatedAtTimestamp,
			UpdatedAtTimestamp: items[x].UpdatedAtTimestamp,
			DuplicateOf:        items[x].DuplicateOf,
		})
	}

	file, err := os.Create(c.Path)
	if err != nil {
		return err
	}

	defer file.Close()

	var jsonExport []byte

	if err == nil {
		if jsonExport, err = json.MarshalIndent(itemsExport, "", "  "); err != nil {
			return fmt.Errorf("writeJSON | %w", err)
		}
	}

	content := strings.Builder{}
	content.WriteString("{\n  \"version\": \"004\",")
	content.WriteString("\n  \"items\": ")
	content.Write(jsonExport)
	content.WriteString(",")

	// add keyParams
	content.WriteString("\n  \"keyParams\": {")
	content.WriteString(fmt.Sprintf("\n    \"identifier\": \"%s\",", c.session.KeyParams.Identifier))
	content.WriteString(fmt.Sprintf("\n    \"version\": \"%s\",", c.session.KeyParams.Version))
	content.WriteString(fmt.Sprintf("\n    \"origination\": \"%s\",", c.session.KeyParams.Origination))
	content.WriteString(fmt.Sprintf("\n    \"created\": \"%s\",", c.session.KeyParams.Created))
	content.WriteString(fmt.Sprintf("\n    \"pw_nonce\": \"%s\"", c.session.KeyParams.PwNonce))
	content.WriteString("\n  }")

	content.WriteString("\n}")

	_, err = file.WriteString(content.String())
	if err != nil {
		return fmt.Errorf("writeJSON | %w", err)
	}

	return nil
}

type CompareEncryptedItemsInput struct {
	Session        *session.Session
	FirstItem      EncryptedItem
	FirstItemsKey  session.SessionItemsKey
	SecondItem     EncryptedItem
	SecondItemsKey session.SessionItemsKey
}

type CompareItemsInput struct {
	Session    *session.Session
	FirstItem  Item
	SecondItem Item
}

func compareItems(input CompareItemsInput) (same, unsupported bool, err error) {
	if input.FirstItem.GetContentType() != input.SecondItem.GetContentType() {
		return false, unsupported, nil
	}

	first := input.FirstItem
	second := input.SecondItem

	switch first.GetContentType() {
	case common.SNItemTypeNote:
		n1 := first.(*Note)
		n2 := second.(*Note)

		return n1.Content.Title == n2.Content.Title && n1.Content.Text == n2.Content.Text, unsupported, nil
	case common.SNItemTypeTag:
		t1 := first.(*Tag)
		t2 := second.(*Tag)

		// compare references
		var refsDiffer bool

		t1Refs := t1.Content.ItemReferences
		t2Refs := t2.Content.ItemReferences

		if len(t1Refs) == len(t2Refs) {
			for x := range t1Refs {
				if t1Refs[x] != t2Refs[x] {
					refsDiffer = true
					break
				}
			}
		} else {
			refsDiffer = true
		}

		return t1.Content.Title == t2.Content.Title && !refsDiffer, unsupported, nil
	}

	return false, true, nil
}

func compareEncryptedItems(input CompareEncryptedItemsInput) (same, unsupported bool, err error) {
	if input.FirstItem.ContentType != input.SecondItem.ContentType {
		return false, unsupported, nil
	}

	fDec, err := DecryptItems(input.Session, EncryptedItems{input.FirstItem}, []session.SessionItemsKey{input.FirstItemsKey})
	if err != nil {
		return
	}

	fPar, err := fDec.Parse()
	if err != nil {
		return
	}

	sDec, err := DecryptItems(input.Session, EncryptedItems{input.SecondItem}, []session.SessionItemsKey{input.SecondItemsKey})
	if err != nil {
		return
	}

	sPar, err := sDec.Parse()
	if err != nil {
		return
	}

	first := fPar[0]
	second := sPar[0]

	switch first.GetContentType() {
	case common.SNItemTypeNote:
		n1 := first.(*Note)
		n2 := second.(*Note)

		return n1.Content.Title == n2.Content.Title && n1.Content.Text == n2.Content.Text, unsupported, nil
	case common.SNItemTypeTag:
		t1 := first.(*Tag)
		t2 := second.(*Tag)

		// compare references
		var refsDiffer bool

		t1Refs := t1.Content.ItemReferences
		t2Refs := t2.Content.ItemReferences

		if len(t1Refs) == len(t2Refs) {
			for x := range t1Refs {
				if t1Refs[x] != t2Refs[x] {
					refsDiffer = true
					break
				}
			}
		} else {
			refsDiffer = true
		}

		return t1.Content.Title == t2.Content.Title && !refsDiffer, unsupported, nil
	}

	return false, true, nil
}

// func decryptExport(s *session.Session, path, password string) (items Items, err error) {
// 	encItemsToImport, keyParams, err := readJSON(path)
// 	if err != nil {
// 		return
// 	}
//
// 	logging.DebugPrint(s.Debug, fmt.Sprintf("Import | read %d items from export file", len(encItemsToImport)), common.MaxDebugChars)
//
// 	// set master key to session by default, but then check if new one is required
// 	mk := s.MasterKey
//
// 	// if export was for a different user (identifier used to generate salt)
// 	if keyParams.Identifier != s.KeyParams.Identifier || keyParams.PwNonce != s.KeyParams.PwNonce {
// 		if password == "" {
// 			logging.DebugPrint(s.Debug, "Import | export is from different account, so prompting for password", common.MaxDebugChars)
// 			fmt.Print("password: ")
//
// 			var bytePassword []byte
// 			bytePassword, err = term.ReadPassword(int(syscall.Stdin))
//
// 			fmt.Println()
//
// 			if err == nil {
// 				password = string(bytePassword)
// 			} else {
// 				return
// 			}
// 		} else {
// 			logging.DebugPrint(s.Debug, "Import | export is from different account and using supplied password", common.MaxDebugChars)
// 		}
//
// 		if strings.TrimSpace(password) == "" {
// 			err = fmt.Errorf("password not defined")
// 			return
// 		}
//
// 		mk, _, err = crypto.GenerateMasterKeyAndServerPassword004(crypto.GenerateEncryptedPasswordInput{
// 			UserPassword:  password,
// 			Identifier:    keyParams.Identifier,
// 			PasswordNonce: keyParams.PwNonce,
// 			// Version:       keyParams.Version,
// 			Debug: s.Debug,
// 		})
// 		if err != nil {
// 			return
// 		}
// 	}
//
// 	// retrieve items and itemskey from export
// 	var exportsEncItemsKeys EncryptedItems
//
// 	var exportedEncItems EncryptedItems
//
// 	for x := range encItemsToImport {
// 		if encItemsToImport[x].ContentType == common.SNItemTypeItemsKey {
// 			logging.DebugPrint(s.Debug, fmt.Sprintf("Import | SN|ItemsKey loaded from export %s", encItemsToImport[x].UUID), common.MaxDebugChars)
//
// 			exportsEncItemsKeys = append(exportsEncItemsKeys, encItemsToImport[x])
//
// 			continue
// 		}
//
// 		exportedEncItems = append(exportedEncItems, encItemsToImport[x])
// 		logging.DebugPrint(s.Debug, fmt.Sprintf("Import | getting exported item %s %s",
// 			encItemsToImport[x].ContentType,
// 			encItemsToImport[x].UUID), common.MaxDebugChars)
// 	}
//
// 	// re-encrypt items
// 	if len(exportedEncItems) == 0 {
// 		err = fmt.Errorf("no items were found in export")
//
// 		return
// 	}
//
// 	var exportsItemsKeys ItemsKeys
//
// 	if len(exportsEncItemsKeys) == 0 {
// 		err = fmt.Errorf("invalid export: no ItemsKey %w", err)
// 		return
// 	}
//
// 	exportsItemsKeys, err = exportsEncItemsKeys.DecryptAndParseItemsKeys(mk, s.Debug)
// 	if err != nil {
// 		err = fmt.Errorf("invalid export: failed to decrypt ItemsKey %w", err)
// 		return
// 	}
//
// 	// s.ImporterItemsKeys = exportsItemsKeys
// 	items, err = exportedEncItems.DecryptAndParse(s)
// 	// s.ImporterItemsKeys = ItemsKeys{}
//
// 	return
// }

// Import steps are:
// - decrypt items in current file (derive master key based on username, password nonce)
// - create a new items key and reencrypt all items
// - set items key to be same updatedtimestamp in order to replace existing.
// func (s *session.Session) Import(path string, syncToken string, password string) (items EncryptedItems, itemsKey ItemsKey, err error) {
// 	exportItems, err := decryptExport(s, path, password)
// 	if err != nil {
// 		return
// 	}
//
// 	logging.DebugPrint(s.Debug, fmt.Sprintf("Import | export file returned %d items", len(exportItems)), common.MaxDebugChars)
//
// 	// This is already set when decrypting Export
//
// 	// retrieve all existing items from SN
// 	so, err := Sync(SyncInput{
// 		Session:   s,
// 		SyncToken: "",
// 	})
// 	if err != nil {
// 		return
// 	}
//
// 	logging.DebugPrint(s.Debug, fmt.Sprintf("Import | initial sync loaded %d items from SN", len(so.Items)), common.MaxDebugChars)
//
// 	// sync will override the default items key with the initial one found
// 	existingItems, err := so.Items.DecryptAndParse(s)
// 	if err != nil {
// 		return
// 	}
//
// 	// determine whether existing or exported items should be resynced...
// 	// - if export and existing have same last updated time, then just choose exported version
// 	var existingToReencrypt Items
//
// 	var exportedToReencrypt Items
//
// 	for x := range existingItems {
// 		var match bool
//
// 		for y := range exportItems {
// 			// check if we have a match for existing item and exported item
// 			if existingItems[x].GetUUID() == exportItems[y].GetUUID() && exportItems[y].GetContentType() != common.SNItemTypeItemsKey {
// 				logging.DebugPrint(s.Debug, fmt.Sprintf("Import | matching item found %s %s",
// 					existingItems[x].GetContentType(), existingItems[x].GetUUID()), common.MaxDebugChars)
//
// 				match = true
//
// 				if existingItems[x].GetUpdatedAtTimestamp() > exportItems[y].GetUpdatedAtTimestamp() {
// 					logging.DebugPrint(s.Debug, fmt.Sprintf("Import | existing %s %s newer than item to encrypt",
// 						existingItems[x].GetContentType(),
// 						existingItems[x].GetUUID()), common.MaxDebugChars)
// 					// if existing item is newer, then re-encrypt existing and add to list
// 					existingToReencrypt = append(existingToReencrypt, existingItems[x])
//
// 					var identical, unsupported bool
// 					// if exported item's content differs, then add also, and deal with conflict during sync
// 					identical, unsupported, err = compareItems(CompareItemsInput{
// 						Session:   s,
// 						FirstItem: existingItems[x],
// 						// FirstItemsKey:  s.DefaultItemsKey,
// 						SecondItem: exportItems[y],
// 						// SecondItemsKey: exportsItemsKey,
// 					})
// 					if err != nil {
// 						return
// 					}
//
// 					// if we're able to compare items, and they differ, then we'll add this item to intentionally
// 					// conflict on sync and be created as a conflicted copy
// 					if !identical && !unsupported {
// 						exportedToReencrypt = append(exportedToReencrypt, exportItems[y])
// 					}
// 				} else if existingItems[x].GetUpdatedAtTimestamp() == exportItems[y].GetUpdatedAtTimestamp() {
// 					// if existing item same age, then choose exported version that's already encrypted with new key
// 					exportedToReencrypt = append(exportedToReencrypt, exportItems[y])
// 				} else {
// 					// (exported cannot be newer than existing item)
// 					panic(fmt.Sprintf("exported %s %s found to be newer than server version",
// 						existingItems[x].GetContentType(),
// 						existingItems[x].GetUUID()))
// 				}
// 			}
// 		}
//
// 		// if we didn't find a match for the item in the export (and it's not a key) then add to final list
// 		if !match && existingItems[x].GetContentType() != common.SNItemTypeItemsKey {
// 			logging.DebugPrint(s.Debug, fmt.Sprintf("Import | no match found for existing item %s %s so add to items to re-encrypt",
// 				existingItems[x].GetContentType(),
// 				existingItems[x].GetUUID()), common.MaxDebugChars)
//
// 			existingToReencrypt = append(existingToReencrypt, existingItems[x])
// 		}
// 	}
//
// 	// loop through items to import and import any non Items Key (already handled) that doesn't exist in cache
// 	for y := range exportItems {
// 		var found bool
//
// 		for x := range existingItems {
// 			if exportItems[y].GetUUID() == existingItems[x].GetUUID() {
// 				found = true
//
// 				break
// 			}
// 		}
//
// 		if !found {
// 			exportedToReencrypt = append(exportedToReencrypt, exportItems[y])
// 		}
// 	}
//
// 	// create new items key and encrypt using current session's master key
// 	nik := NewItemsKey()
// 	nik.UUID = s.DefaultItemsKey.UUID
// 	nik.UpdatedAtTimestamp = s.DefaultItemsKey.UpdatedAtTimestamp
// 	nik.UpdatedAt = s.DefaultItemsKey.UpdatedAt
//
// 	// combine all items to reencrypt
// 	f := append(exportedToReencrypt, existingToReencrypt...)
//
// 	rf, err := f.Encrypt(s, nik)
// 	if err != nil {
// 		return
// 	}
//
// 	eNik, err := EncryptItemsKey(nik, s, false)
// 	if err != nil {
// 		return
// 	}
//
// 	eNiks := EncryptedItems{
// 		eNik,
// 	}
//
// 	// add existing items (re-encrypted) to the re-encrypted exported items
// 	// preprend new items key to the list of re-encrypted items
// 	rfa := append(eNiks, rf...)
//
// 	// set default items key to new items key
// 	s.DefaultItemsKey = nik
// 	// reset items keys slice to have only new
// 	s.ItemsKeys = ItemsKeys{s.DefaultItemsKey}
//
// 	so2, err := Sync(SyncInput{
// 		Session:   s,
// 		SyncToken: so.SyncToken,
// 		Items:     rfa,
// 	})
// 	if err != nil {
// 		return
// 	}
//
// 	// check initial items key differs from the new
// 	for x := range so.SavedItems {
// 		if so.SavedItems[x].ContentType == common.SNItemTypeItemsKey {
// 			itemsKey, err = so.SavedItems[x].Decrypt(s.MasterKey)
// 			if err != nil {
// 				return
// 			}
// 		}
// 	}
//
// 	items = append(so2.SavedItems, so.SavedItems...)
// 	itemsKey = nik
//
// 	return
// }

func readJSON(filePath string) (items EncryptedItems, kp auth.KeyParams, err error) {
	file, err := os.ReadFile(filePath)
	if err != nil {
		err = fmt.Errorf("%w failed to open: %s", err, filePath)
		return
	}

	var eif EncryptedItemsFile

	err = json.Unmarshal(file, &eif)
	if err != nil {
		err = fmt.Errorf("failed to unmarshall json: %w", err)
		return
	}

	return eif.Items, eif.KeyParams, err
}

type EncryptedItemsFile struct {
	Items     EncryptedItems `json:"items"`
	KeyParams auth.KeyParams `json:"keyParams"`
}

func UpsertReferences(existing, new ItemReferences) ItemReferences {
	res := existing

	if len(existing) == 0 {
		return new
	}

	for _, newRef := range new {
		var found bool

		for _, existingRef := range existing {
			if existingRef.UUID == newRef.UUID {
				found = true
			}
		}

		if !found {
			res = append(res, newRef)
		}
	}

	return res
}

func (iks ItemsKeys) Latest() ItemsKey {
	var l ItemsKey
	for _, ik := range iks {
		if ik.CreatedAtTimestamp > l.CreatedAtTimestamp {
			l = ik
		}
	}

	return l
}

// GenUUID generates a unique identifier required when creating a new item.
func GenUUID() string {
	newUUID := uuid.New()
	return newUUID.String()
}

func DedupeItemsKeys(itemsKeys []ItemsKey) (output []ItemsKey) {
	seen := make(map[string]int)
	for x := range itemsKeys {
		if seen[itemsKeys[x].UUID] > 0 {
			continue
		}

		seen[itemsKeys[x].UUID]++
		output = append(output, itemsKeys[x])
	}

	return output
}

func DecryptEncryptedItemKey(e EncryptedItem, encryptionKey string) (itemKey []byte, err error) {
	_, nonce, cipherText, authData := crypto.SplitContent(e.EncItemKey)
	return crypto.DecryptCipherText(cipherText, encryptionKey, nonce, authData)
}

func DecryptContent(e EncryptedItem, encryptionKey string) (content []byte, err error) {
	_, nonce, cipherText, authData := crypto.SplitContent(e.Content)

	content, err = crypto.DecryptCipherText(cipherText, encryptionKey, nonce, authData)
	if err != nil {
		return
	}

	c := string(content)

	if !slices.Contains([]string{
		common.SNItemTypeFileSafeIntegration,
		common.SNItemTypeFileSafeCredentials,
		common.SNItemTypeComponent,
		common.SNItemTypeTheme,
	}, e.ContentType) && len(c) > 250 {
		return
	}

	return
}

func CreateItemsKey() (ItemsKey, error) {
	ik := NewItemsKey()
	// creating an items key is done during registration or when exporting, in which case it will always be default
	// ik.Default = true
	// ik.Content.Default = true
	ik.CreatedAtTimestamp = time.Now().UTC().UnixMicro()
	ik.CreatedAt = time.Now().UTC().Format(common.TimeLayout)

	return ik, nil
}
