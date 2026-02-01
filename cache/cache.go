package cache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/asdine/storm/v3"
	"github.com/jonhadfield/gosn-v2/common"
	"github.com/jonhadfield/gosn-v2/items"
	log "github.com/jonhadfield/gosn-v2/log"
	"github.com/jonhadfield/gosn-v2/session"
	"github.com/mitchellh/go-homedir"
)

const batchSize = 500

// Sync management variables
var (
	lastSyncTime time.Time
	syncMutex    sync.Mutex
)

// SyncErrorType represents different types of sync errors
type SyncErrorType int

const (
	SyncErrorUnknown SyncErrorType = iota
	SyncErrorRateLimit
	SyncErrorValidation
	SyncErrorAuthentication
	SyncErrorItemsKey
	SyncErrorConflict
	SyncErrorNetwork
)

// SyncError provides detailed error information
type SyncError struct {
	Type      SyncErrorType
	Original  error
	Message   string
	Retryable bool
	BackoffMs int64
}

func (e *SyncError) Error() string {
	return e.Message
}

// RateLimitBackoff tracks exponential backoff for rate limiting
type RateLimitBackoff struct {
	attempts    int64
	baseDelayMs int64
	maxDelayMs  int64
}

type Item struct {
	// String fields (16 bytes each) - ordered for optimal memory layout
	UUID        string `storm:"id,unique"`
	Content     string
	ContentType string `storm:"index"`
	ItemsKeyID  string
	EncItemKey  string
	CreatedAt   string
	UpdatedAt   string

	// Pointer field (8 bytes)
	DuplicateOf *string

	// time.Time is 24 bytes (3 words on 64-bit)
	DirtiedDate time.Time

	// Integer fields (8 bytes each)
	CreatedAtTimestamp int64
	UpdatedAtTimestamp int64

	// Boolean fields packed together (1 byte each, minimal padding)
	Deleted bool
	Dirty   bool
}

type SyncToken struct {
	SyncToken string `storm:"id,unique"`
	CreatedAt time.Time
}

type SyncInput struct {
	*Session
	Close bool
}

type SyncOutput struct {
	DB *storm.DB
}

type Items []Item

func (i Items) UUIDs() []string {
	var uuids []string

	for _, ii := range i {
		if ii.Deleted {
			continue
		}

		uuids = append(uuids, ii.UUID)
	}

	return uuids
}

func (s *Session) gosn() *session.Session {
	gs := session.Session{
		Debug:             s.Debug,
		HTTPClient:        s.HTTPClient,
		SchemaValidation:  s.SchemaValidation,
		Server:            s.Server,
		FilesServerUrl:    s.FilesServerUrl,
		Token:             s.Token,
		MasterKey:         s.MasterKey,
		ItemsKeys:         s.ItemsKeys,
		DefaultItemsKey:   s.DefaultItemsKey,
		KeyParams:         s.KeyParams,
		AccessToken:       s.AccessToken,
		RefreshToken:      s.RefreshToken,
		AccessExpiration:  s.AccessExpiration,
		RefreshExpiration: s.RefreshExpiration,
		ReadOnlyAccess:    s.ReadOnlyAccess,
		PasswordNonce:     s.PasswordNonce,
		Schemas:           s.Schemas,
	}

	return &gs
}

func (pi Items) ToItems(s *Session) (items.Items, error) {
	var its items.Items
	var err error
	//log.DebugPrint(s.Debug, fmt.Sprintf("ToItems | Converting %d cache items to gosn items", len(pi)), common.MaxDebugChars)

	// start := time.Now()

	var eItems items.EncryptedItems

	for _, ei := range pi {
		// we'd never need to decrypt an item in the db that needs to be deleted
		if ei.Deleted {
			continue
		}

		// remove items encrypted using legacy version of SN
		if strings.HasPrefix(ei.Content, "003") {
			continue
		}

		if ei.EncItemKey == "" {
			// TODO: should I ignore or return an error?
			log.DebugPrint(s.Debug, fmt.Sprintf("ToItems | ignoring invalid item due to missing encrypted items key: %+v", ei), common.MaxDebugChars)
		}

		eiik := ei.ItemsKeyID

		eItems = append(eItems, items.EncryptedItem{
			UUID:               ei.UUID,
			Content:            ei.Content,
			ContentType:        ei.ContentType,
			ItemsKeyID:         eiik,
			EncItemKey:         ei.EncItemKey,
			Deleted:            ei.Deleted,
			CreatedAt:          ei.CreatedAt,
			CreatedAtTimestamp: ei.CreatedAtTimestamp,
			UpdatedAtTimestamp: ei.UpdatedAtTimestamp,
			UpdatedAt:          ei.UpdatedAt,
			DuplicateOf:        ei.DuplicateOf,
		})

		// Log duplicate items for debugging without recursion
		if ei.ContentType == common.SNItemTypeNote && ei.DuplicateOf != nil {
			log.DebugPrint(s.Debug, fmt.Sprintf("%s: %s is duplicate of %s", ei.ContentType, ei.UUID, *ei.DuplicateOf), 120)
		}
	}

	if eItems != nil {
		if len(s.Session.ItemsKeys) == 0 {
			panic("trying to convert cache items to items with no items keys")
		}

		if s.Session.DefaultItemsKey.ItemsKey == "" {
			panic("trying to convert cache items to items with no default items key")
		}

		its, err = eItems.DecryptAndParse(s.Session)
		if err != nil {
			return items.Items{}, err
		}
	}

	// log.DebugPrint(s.Debug, fmt.Sprintf("ToItems took: %s", time.Since(start).String()), 50)

	return its, nil
}

// func (s *Session) Export(path string) error {
// 	log.DebugPrint(s.Debug, fmt.Sprintf("Exporting to path: %s", path),common.MaxDebugChars)
//
// 	return s.Session.Export(path)
// }

func (s *Session) RemoveDB() {
	if err := os.Remove(s.CacheDBPath); err != nil {
		if runtime.GOOS == "linux" || runtime.GOOS == "darwin" {
			if !strings.Contains(err.Error(), "no such file or directory") {
				panic(err)
			}
		}

		if runtime.GOOS == "windows" && !(strings.Contains(err.Error(), "cannot find the file specified") || strings.Contains(err.Error(), "cannot find the path specified")) {
			panic(err)
		}
	}
}

// Import reads a json export file into SN and then syncs with the db.
// func (s *Session) Import(path string, persist bool) error {
// 	so, err := Sync(SyncInput{
// 		Session: s,
// 		Close:   false,
// 	})
// 	if err != nil {
// 		return err
// 	}
//
// 	var syncTokens []SyncToken
//
// 	err = so.DB.All(&syncTokens)
// 	if err != nil && err.Error() != "not found" {
// 		return err
// 	}
//
// 	syncToken := ""
//
// 	if len(syncTokens) > 0 {
// 		syncToken = syncTokens[0].SyncToken
// 	}
//
// 	log.DebugPrint(s.Debug, fmt.Sprintf("importing from %s", path))
//
// 	ii, ifk, err := s.Session.Import(path, syncToken, "")
// 	if err != nil {
// 		return err
// 	}
//
// 	log.DebugPrint(s.Debug, fmt.Sprintf("importing loaded: %d items and %s key", len(ii), ifk.UUID))
//
// 	err = so.DB.Close()
// 	if err != nil {
// 		return err
// 	}
//
// 	_, err = Sync(SyncInput{
// 		Session: s,
// 		Close:   true,
// 	})
//
// 	return err
// }

func ToCacheItems(items items.EncryptedItems, clean bool) (pitems Items) {
	for _, i := range items {
		var cItem Item
		cItem.UUID = i.UUID
		cItem.Content = i.Content
		cItem.UpdatedAtTimestamp = i.UpdatedAtTimestamp
		cItem.CreatedAtTimestamp = i.CreatedAtTimestamp

		if !clean {
			cItem.Dirty = true
			cItem.DirtiedDate = time.Now()
		}

		cItem.ContentType = i.ContentType

		iik := ""

		if !i.Deleted && i.ItemsKeyID == "" && !(i.ContentType == common.SNItemTypeItemsKey || strings.HasPrefix(i.ContentType, "SF")) {
			panic(fmt.Sprintf("we've received %s %s from SN without ItemsKeyID", i.ContentType, i.UUID))
		}

		if i.ItemsKeyID != "" {
			iik = i.ItemsKeyID
		}

		if i.ContentType == common.SNItemTypeItemsKey && iik != "" {
			log.Fatalf("ItemsKey with UUID: %s has ItemsKeyID %s", i.UUID, iik)
		}

		cItem.ItemsKeyID = i.ItemsKeyID
		cItem.EncItemKey = i.EncItemKey
		cItem.Deleted = i.Deleted
		cItem.CreatedAt = i.CreatedAt
		cItem.UpdatedAt = i.UpdatedAt
		cItem.DuplicateOf = i.DuplicateOf
		pitems = append(pitems, cItem)
	}

	return pitems
}

// SaveNotes encrypts, converts to cache items, and then persists to db.
func SaveNotes(s *Session, db *storm.DB, notes items.Notes, close bool) error {
	eItems, err := notes.Encrypt(*s.gosn())
	if err != nil {
		return err
	}

	cItems := ToCacheItems(eItems, false)

	return SaveCacheItems(db, cItems, close)
}

// SaveTags encrypts, converts to cache items, and then persists to db.
func SaveTags(db *storm.DB, s *Session, tags items.Tags, close bool) error {
	eItems, err := tags.Encrypt(*s.gosn())
	if err != nil {
		return err
	}

	cItems := ToCacheItems(eItems, false)

	return SaveCacheItems(db, cItems, close)
}

// SaveEncryptedItems converts to cache items and persists to db.
func SaveEncryptedItems(db *storm.DB, items items.EncryptedItems, close bool) error {
	cItems := ToCacheItems(items, false)

	return SaveCacheItems(db, cItems, close)
}

// SaveItems saves Items to the provided database.
func SaveItems(s *Session, db *storm.DB, its items.Items, close bool) error {
	if len(its) == 0 {
		return errors.New("no items provided to SaveItems")
	}

	if db == nil {
		return errors.New("db not passed to SaveItems")
	}

	var eItems items.EncryptedItems

	for x := range its {
		eItem, err := items.EncryptItem(its[x], s.DefaultItemsKey, s.Session)
		if err != nil {
			return fmt.Errorf("saveItems | %w", err)
		}

		eItems = append(eItems, eItem)
	}

	cItems := ToCacheItems(eItems, false)

	return SaveCacheItems(db, cItems, close)
}

// SaveCacheItems saves Cache Items to the provided database.
func SaveCacheItems(db *storm.DB, items Items, close bool) error {
	if len(items) == 0 {
		return errors.New("no items provided to SaveCacheItems")
	}

	if db == nil {
		return errors.New("db not passed to SaveCacheItems")
	}

	// CRITICAL SAFEGUARD: Filter out any deleted protected items
	var safeItems Items
	for _, item := range items {
		if item.ContentType == common.SNItemTypeItemsKey && item.Deleted {
			log.DebugPrint(false, fmt.Sprintf("SaveCacheItems | WARNING: Refusing to save deleted SN|ItemsKey %s", item.UUID), common.MaxDebugChars)
			continue
		}
		if item.ContentType == common.SNItemTypeUserPreferences && item.Deleted {
			log.DebugPrint(false, fmt.Sprintf("SaveCacheItems | WARNING: Refusing to save deleted SN|UserPreferences %s", item.UUID), common.MaxDebugChars)
			continue
		}
		safeItems = append(safeItems, item)
	}
	items = safeItems

	total := len(items)

	for i := 0; i < total; i += batchSize {
		j := i + batchSize
		if j > total {
			j = total
		}

		tx, err := db.Begin(true)
		if err != nil {
			return err
		}

		sl := items[i:j]

		for v := range sl {
			err = tx.Save(&sl[v])
			if err != nil {
				if rErr := tx.Rollback(); rErr != nil {
					return fmt.Errorf("saveCacheItems | save error: %s | rollback error: %w",
						err.Error(), rErr)
				}

				return fmt.Errorf("saveCacheItems | save error: %w", err)
			}
		}

		err = tx.Commit()
		if err != nil {
			return err
		}
	}

	if close {
		if err := db.Close(); err != nil {
			return fmt.Errorf("saveCacheItems | close error: %w", err)
		}
	}

	return nil
}

// DeleteCacheItems deletes Cache Items from the provided database.
func DeleteCacheItems(db *storm.DB, items Items, close bool) error {
	if len(items) == 0 {
		return errors.New("no items provided to DeleteCacheItems")
	}

	if db == nil {
		return errors.New("db not passed to DeleteCacheItems")
	}

	// CRITICAL SAFEGUARD: Never delete protected items from cache
	var safeItems Items
	for _, item := range items {
		if item.ContentType == common.SNItemTypeItemsKey {
			log.DebugPrint(false, fmt.Sprintf("DeleteCacheItems | WARNING: Refusing to delete SN|ItemsKey %s from cache", item.UUID), common.MaxDebugChars)
			continue
		}
		if item.ContentType == common.SNItemTypeUserPreferences {
			log.DebugPrint(false, fmt.Sprintf("DeleteCacheItems | WARNING: Refusing to delete SN|UserPreferences %s from cache", item.UUID), common.MaxDebugChars)
			continue
		}
		safeItems = append(safeItems, item)
	}
	items = safeItems

	if len(items) == 0 {
		return nil // Nothing left to delete after filtering
	}

	total := len(items)

	for i := 0; i < total; i += batchSize {
		j := i + batchSize
		if j > total {
			j = total
		}

		tx, err := db.Begin(true)
		if err != nil {
			return err
		}

		sl := items[i:j]

		for v := range sl {
			err = tx.DeleteStruct(&sl[v])
			if err != nil {
				if strings.Contains(err.Error(), "not found") {
					continue
				}

				err = fmt.Errorf("rolling back due to: %+v", err)

				if tx.Rollback() != nil {
					origErr := err
					err = fmt.Errorf("%+v and rollback failed with: %+v", err, origErr)

					return err
				}
				return err
			}
		}

		err = tx.Commit()
		if err != nil {
			return err
		}
	}

	if close {
		return db.Close()
	}

	return nil
}

// CleanCacheItems marks Cache Items as clean (Dirty = false) and resets dirtied date to the provided database.
func CleanCacheItems(db *storm.DB, items Items, close bool) error {
	if len(items) == 0 {
		return errors.New("no items provided to DeleteCacheItems")
	}

	if db == nil {
		return errors.New("db not passed to DeleteCacheItems")
	}

	total := len(items)

	for i := 0; i < total; i += batchSize {
		j := i + batchSize
		if j > total {
			j = total
		}

		tx, err := db.Begin(true)
		if err != nil {
			return err
		}

		sl := items[i:j]

		for v := range sl {
			uuid := sl[v].UUID

			if err = tx.UpdateField(&Item{UUID: uuid}, "Dirty", false); err != nil {
				if strings.Contains(err.Error(), "not found") {
					continue
				}
			}

			if err = tx.UpdateField(&Item{UUID: uuid}, "DirtiedDate", time.Time{}); err != nil {
				if strings.Contains(err.Error(), "not found") {
					continue
				}
			}

			if err != nil {
				err = fmt.Errorf("rolling back due to: %+v", err)

				if tx.Rollback() != nil {
					origErr := err
					err = fmt.Errorf("%+v and rollback failed with: %+v", err, origErr)
				}

				return err
			}
		}

		err = tx.Commit()
		if err != nil {
			return err
		}
	}

	if close {
		return db.Close()
	}

	return nil
}

type CleanInput struct {
	*Session
	Close                 bool
	UnreferencedItemsKeys bool
}

func (i Items) Validate() error {
	for x := range i {
		// // fmt.Printf("validating: %+v\n", i[x])
		// 	continue
		// }

		switch {
		case strings.HasPrefix(i[x].Content, "003"):
			// TODO: Instructions incorrect
			// return fmt.Errorf("found cache item %s: %s created with legacy StandardNotes version: 003\n     "+
			// 	"please export and then import content using official app to upgrade to 004 and then run 'sn resync' to upgrade cache to version 004", i[x].ContentType, i[x].UUID)
		case i[x].UUID == "":
			return fmt.Errorf("cache item is missing uuid: %+v", i[x])
		case i[x].ContentType == "":
			return fmt.Errorf("cache item is missing content_type: %+v", i[x])
		case i[x].Content == "":
			return fmt.Errorf("cache item is missing content: %+v", i[x])
		case i[x].EncItemKey == "" && i[x].ContentType != common.SNItemTypeSFExtension:
			return fmt.Errorf("cache item is missing enc_item_key: %+v", i[x])
		case i[x].ContentType != common.SNItemTypeItemsKey && i[x].ContentType != common.SNItemTypeSFExtension && i[x].ItemsKeyID == "":
			return fmt.Errorf("cache item is missing items_key_id: %+v", i[x])
		}
	}

	return nil
}

func (i Items) ValidateSaved() error {
	for x := range i {
		// fmt.Printf("validating saved: %+v\n", i[x])
		if i[x].Deleted {
			continue
		}

		switch {
		case i[x].UUID == "":
			return fmt.Errorf("cache item is missing uuid: %+v", i[x])
		case i[x].ContentType == "":
			return fmt.Errorf("cache item is missing content_type: %+v", i[x])
			// case i[x].Content == "":
			// 	return fmt.Errorf("cache item is missing content: %+v", i[x])
			// case i[x].EncItemKey == "" && i[x].ContentType != common.SNItemTypeSFExtension:
			// 	return fmt.Errorf("cache item is missing enc_item_key: %+v", i[x])
			// case i[x].ContentType != common.SNItemTypeItemsKey && i[x].ContentType != common.SNItemTypeSFExtension && i[x].ItemsKeyID == "":
			// 	return fmt.Errorf("cache item is missing items_key_id: %+v", i[x])
			// }
		}
	}

	return nil
}

func retrieveItemsKeysFromCache(s *session.Session, i Items) (items.EncryptedItems, error) {
	var encryptedItemKeys items.EncryptedItems
	log.DebugPrint(s.Debug, "retrieveItemsKeysFromCache | attempting to retrieve items key(s) from cache", common.MaxDebugChars)

	for x := range i {
		if i[x].ContentType == common.SNItemTypeItemsKey && !i[x].Deleted {
			log.DebugPrint(s.Debug, fmt.Sprintf("retrieved items key %s from cache", i[x].UUID), common.MaxDebugChars)

			encryptedItemKeys = append(encryptedItemKeys, items.EncryptedItem{
				UUID:               i[x].UUID,
				Content:            i[x].Content,
				ContentType:        i[x].ContentType,
				ItemsKeyID:         i[x].ItemsKeyID,
				EncItemKey:         i[x].EncItemKey,
				Deleted:            i[x].Deleted,
				CreatedAt:          i[x].CreatedAt,
				UpdatedAt:          i[x].UpdatedAt,
				CreatedAtTimestamp: i[x].CreatedAtTimestamp,
				UpdatedAtTimestamp: i[x].UpdatedAtTimestamp,
				DuplicateOf:        i[x].DuplicateOf,
			})
		}
	}

	return encryptedItemKeys, nil
}

// enforceMinimumSyncDelay prevents rapid consecutive sync operations with adaptive timing
func enforceMinimumSyncDelay() {
	syncMutex.Lock()
	defer syncMutex.Unlock()

	// Enforce minimum delay between sync operations
	minDelay := common.SyncDelayMinimum
	if elapsed := time.Since(lastSyncTime); elapsed < minDelay {
		sleepDuration := minDelay - elapsed
		log.DebugPrint(false, fmt.Sprintf("Sync | Enforcing %v delay before next sync (elapsed: %v)", sleepDuration, elapsed), common.MaxDebugChars)
		time.Sleep(sleepDuration)
	}
	lastSyncTime = time.Now()
}

// enforceRateLimitBackoff implements exponential backoff for rate limit responses
func enforceRateLimitBackoff(backoff *RateLimitBackoff) {
	// Initialize defaults if not set
	if backoff.attempts == 0 && backoff.baseDelayMs == 0 {
		backoff.baseDelayMs = common.RateLimitBaseDelay
		backoff.maxDelayMs = common.RateLimitMaxDelay
	}

	backoff.attempts++
	delayMs := backoff.baseDelayMs * (1 << (backoff.attempts - 1)) // Exponential: base, 2*base, 4*base, 8*base...
	if delayMs > backoff.maxDelayMs {
		delayMs = backoff.maxDelayMs
	}

	time.Sleep(time.Duration(delayMs) * time.Millisecond)
}

// classifySyncError analyzes error to determine type and retry strategy
func classifySyncError(err error) *SyncError {
	if err == nil {
		return nil
	}

	errMsg := err.Error()
	errLower := strings.ToLower(errMsg)

	// HTTP 429 Rate Limiting
	if strings.Contains(errLower, "429") || strings.Contains(errLower, "rate limit") ||
		strings.Contains(errLower, "exceeded the maximum bandwidth") {
		return &SyncError{
			Type:      SyncErrorRateLimit,
			Original:  err,
			Message:   "Rate limit exceeded - server requested backoff",
			Retryable: true,
			BackoffMs: common.RateLimitInitialBackoff,
		}
	}

	// ItemsKey issues
	if strings.Contains(errLower, "items key") || strings.Contains(errLower, "itemskey") ||
		strings.Contains(errLower, "empty default items key") {
		return &SyncError{
			Type:      SyncErrorItemsKey,
			Original:  err,
			Message:   "Missing or invalid ItemsKey - account setup required",
			Retryable: false,
		}
	}

	// Authentication issues
	if strings.Contains(errLower, "auth") || strings.Contains(errLower, "unauthorized") ||
		strings.Contains(errLower, "invalid session") || strings.Contains(errLower, "token") {
		return &SyncError{
			Type:      SyncErrorAuthentication,
			Original:  err,
			Message:   "Authentication failed - session may be expired",
			Retryable: false,
		}
	}

	// Validation errors
	if strings.Contains(errLower, "validation") || strings.Contains(errLower, "invalid") ||
		strings.Contains(errLower, "malformed") || strings.Contains(errLower, "bad request") {
		return &SyncError{
			Type:      SyncErrorValidation,
			Original:  err,
			Message:   "Validation failed - check item data format",
			Retryable: false,
		}
	}

	// Network connectivity issues
	if strings.Contains(errLower, "network") || strings.Contains(errLower, "connection") ||
		strings.Contains(errLower, "timeout") || strings.Contains(errLower, "unreachable") {
		return &SyncError{
			Type:      SyncErrorNetwork,
			Original:  err,
			Message:   "Network connectivity issue - temporary failure",
			Retryable: true,
			BackoffMs: common.NetworkErrorBackoff,
		}
	}

	// Sync conflicts
	if strings.Contains(errLower, "conflict") {
		return &SyncError{
			Type:      SyncErrorConflict,
			Original:  err,
			Message:   "Sync conflict detected - items may need resolution",
			Retryable: true,
			BackoffMs: common.ConflictErrorBackoff,
		}
	}

	// Default unknown error
	return &SyncError{
		Type:      SyncErrorUnknown,
		Original:  err,
		Message:   fmt.Sprintf("Unknown sync error: %v", err),
		Retryable: true,
		BackoffMs: common.UnknownErrorBackoff,
	}
}

// validateSessionItemsKey checks if session has valid ItemsKey before sync
func validateSessionItemsKey(s *Session) error {
	if s == nil {
		return errors.New("session is nil")
	}
	if s.Session == nil {
		return errors.New("session.Session is nil")
	}
	// Check if DefaultItemsKey is zero value (empty struct)
	if s.DefaultItemsKey.ItemsKey == "" {
		return &SyncError{
			Type:      SyncErrorItemsKey,
			Original:  errors.New("no default ItemsKey available"),
			Message:   "Session lacks valid ItemsKey for encryption operations",
			Retryable: false,
		}
	}
	if s.DefaultItemsKey.ItemsKey == "" {
		return &SyncError{
			Type:      SyncErrorItemsKey,
			Original:  errors.New("empty default ItemsKey"),
			Message:   "Session has empty ItemsKey - account setup required",
			Retryable: false,
		}
	}
	return nil
}

// SyncWithRetry performs sync with intelligent retry logic
func SyncWithRetry(si SyncInput, maxRetries int) (so SyncOutput, err error) {
	var rateLimitBackoff RateLimitBackoff

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			log.DebugPrint(si.Session.Debug,
				fmt.Sprintf("SyncWithRetry | Attempt %d/%d", attempt+1, maxRetries+1),
				common.MaxDebugChars)
		}

		so, err = Sync(si)
		if err == nil {
			// Success - reset any backoff state
			rateLimitBackoff = RateLimitBackoff{}
			return so, nil
		}

		// Analyze error for retry decision
		syncErr, ok := err.(*SyncError)
		if !ok {
			// Non-classified error, apply basic classification
			syncErr = classifySyncError(err)
		}

		log.DebugPrint(si.Session.Debug,
			fmt.Sprintf("SyncWithRetry | Error type %d: %s (retryable: %t)",
				syncErr.Type, syncErr.Message, syncErr.Retryable),
			common.MaxDebugChars)

		// Don't retry non-retryable errors
		if !syncErr.Retryable {
			return so, syncErr
		}

		// Don't retry on final attempt
		if attempt >= maxRetries {
			return so, syncErr
		}

		// Apply appropriate backoff based on error type
		switch syncErr.Type {
		case SyncErrorRateLimit:
			log.DebugPrint(si.Session.Debug,
				"SyncWithRetry | Rate limit detected, applying exponential backoff",
				common.MaxDebugChars)
			enforceRateLimitBackoff(&rateLimitBackoff)
		case SyncErrorNetwork:
			time.Sleep(time.Duration(syncErr.BackoffMs) * time.Millisecond)
		case SyncErrorConflict:
			// For conflicts, shorter delay as they may resolve quickly
			time.Sleep(time.Duration(syncErr.BackoffMs) * time.Millisecond)
		default:
			// Standard backoff for unknown errors
			time.Sleep(time.Duration(syncErr.BackoffMs) * time.Millisecond)
		}
	}

	return so, err
}

// calculateSyncTimeout returns appropriate timeout based on dataset size
func calculateSyncTimeout(itemCount int64) time.Duration {
	baseTimeout := 30 * time.Second

	switch {
	case itemCount < 10:
		return baseTimeout
	case itemCount < 100:
		return baseTimeout * 2
	case itemCount < 1000:
		return baseTimeout * 4
	default:
		return baseTimeout * 8 // Max 4 minutes for very large datasets
	}
}

// getSyncConfiguration returns timeout and retry configuration based on environment and dataset
func getSyncConfiguration(si SyncInput, itemCount int64) (timeout time.Duration, retries int) {
	// Check environment overrides first
	if envTimeout := os.Getenv("SN_SYNC_TIMEOUT"); envTimeout != "" {
		if t, err := time.ParseDuration(envTimeout); err == nil {
			timeout = t
		}
	}

	// Default to calculated timeout based on dataset size
	if timeout == 0 {
		timeout = calculateSyncTimeout(itemCount)
	}

	retries = 3 // Reduced from 5 for faster failure detection
	return
}

// validateAndCleanSyncToken validates and cleans stale sync tokens
func validateAndCleanSyncToken(db *storm.DB, session *session.Session) (string, error) {
	var syncTokens []SyncToken
	if err := db.All(&syncTokens); err != nil {
		return "", nil // No tokens, start fresh
	}

	if len(syncTokens) > 1 {
		// Multiple tokens indicate corruption - reset
		log.DebugPrint(session.Debug, "Sync | Multiple sync tokens found, resetting", common.MaxDebugChars)
		if dropErr := db.Drop("SyncToken"); dropErr != nil {
			return "", dropErr
		}
		return "", nil
	}

	// Validate token age with graduated approach
	if len(syncTokens) == 1 {
		token := syncTokens[0]
		age := time.Since(token.CreatedAt)

		// Hard expiry: reset token if older than 24 hours
		if age > common.SyncTokenMaxAge {
			log.DebugPrint(session.Debug,
				fmt.Sprintf("Sync | Sync token expired (%v old), resetting", age),
				common.MaxDebugChars)
			if dropErr := db.Drop("SyncToken"); dropErr != nil {
				return "", dropErr
			}
			return "", nil
		}

		// Soft warning: log if token is aging (>12 hours)
		if age > common.SyncTokenSoftAge {
			log.DebugPrint(session.Debug,
				fmt.Sprintf("Sync | Sync token aging (%v old), consider refresh soon", age),
				common.MaxDebugChars)
		}

		return token.SyncToken, nil
	}

	return "", nil
}

// handleSyncError implements specific error recovery strategies
func handleSyncError(err error, si SyncInput) (shouldRetry bool, newSi SyncInput, finalErr error) {
	if err == nil {
		return false, si, nil
	}

	errStr := strings.ToLower(err.Error())

	switch {
	case strings.Contains(errStr, "giving up"):
		// HTTP client exhausted retries - this is final
		return false, si, fmt.Errorf("sync timed out after maximum retries: %w", err)

	case strings.Contains(errStr, "database is locked"):
		// Database contention - retry with delay
		time.Sleep(1 * time.Second)
		return true, si, nil

	case strings.Contains(errStr, "invalid sync token"):
		// Reset sync token and retry
		newSi := si
		if si.Session.CacheDBPath != "" {
			if db, dbErr := storm.Open(si.Session.CacheDBPath); dbErr == nil {
				_ = db.Drop("SyncToken")
				db.Close()
			}
		}
		return true, newSi, nil

	default:
		return false, si, err
	}
}

// shouldUseBatchedSync determines if we need progressive sync for large datasets
func shouldUseBatchedSync(db *storm.DB) bool {
	// Check if we have a large dataset that needs batched processing
	if db == nil {
		return false
	}

	var itemCount int64
	if count, err := db.Count(&Items{}); err == nil {
		itemCount = int64(count)
	}
	return itemCount > 100 // Threshold for batched sync
}

// Sync will push any dirty items to SN and make database cache consistent with SN.
func Sync(si SyncInput) (so SyncOutput, err error) {
	// Validate session and warn about ItemsKey issues (but don't fail)
	if validationErr := validateSessionItemsKey(si.Session); validationErr != nil {
		if syncErr, ok := validationErr.(*SyncError); ok {
			log.DebugPrint(si.Session.Debug,
				fmt.Sprintf("Sync | ItemsKey validation warning: %s - sync will continue but may not be fully functional", syncErr.Message),
				common.MaxDebugChars)
			// Log warning but don't return error - allow sync to proceed
		} else {
			// For non-SyncError validation issues (like nil session), still fail
			return so, validationErr
		}
	}

	// Prevent rapid consecutive syncs
	enforceMinimumSyncDelay()

	// Track sync timing for health monitoring
	syncStart := time.Now()

	// Database connection timeout
	dbTimeout := 30 * time.Second

	// Enhanced error handling with classification
	defer func() {
		if si.Close && so.DB != nil {
			if closeErr := so.DB.Close(); closeErr != nil {
				log.DebugPrint(si.Session.Debug,
					fmt.Sprintf("Sync | WARNING: Failed to close DB: %v", closeErr),
					common.MaxDebugChars)
			}
		}

		duration := time.Since(syncStart)
		if duration > 5*time.Second {
			log.DebugPrint(si.Session.Debug, fmt.Sprintf("Sync | WARNING: Sync took %v, exceeding healthy duration of 5s", duration), common.MaxDebugChars)
		}

		// Classify and enhance error information
		if err != nil {
			if syncErr := classifySyncError(err); syncErr != nil {
				log.DebugPrint(si.Session.Debug,
					fmt.Sprintf("Sync | Error classified as %d: %s (retryable: %t)",
						syncErr.Type, syncErr.Message, syncErr.Retryable),
					common.MaxDebugChars)
				err = syncErr // Return enhanced error
			}
		}
	}()

	// Add database operation context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	// check session is valid
	if si.Session == nil || !si.Session.Valid() {
		err = &SyncError{
			Type:      SyncErrorAuthentication,
			Original:  errors.New("invalid session"),
			Message:   "Session is invalid or expired",
			Retryable: false,
		}
		return
	}

	log.DebugPrint(si.Session.Debug, fmt.Sprintf("Sync | session is valid"), common.MaxDebugChars)

	// only path should be passed
	if si.Session.CacheDBPath == "" {
		err = errors.New("database path is required")
		return
	}

	var db *storm.DB
	// open DB if path provided
	if si.Session.CacheDBPath != "" {
		log.DebugPrint(si.Session.Debug, fmt.Sprintf("Sync | using db in '%s'", si.Session.CacheDBPath), common.MaxDebugChars)

		// Open database with timeout handling
		type openResult struct {
			db  *storm.DB
			err error
		}

		done := make(chan openResult, 1)
		go func() {
			database, openErr := storm.Open(si.Session.CacheDBPath)
			done <- openResult{db: database, err: openErr}
		}()

		select {
		case result := <-done:
			if result.err != nil {
				err = result.err
				return
			}
			db = result.db
		case <-ctx.Done():
			err = fmt.Errorf("database open timed out after %v", dbTimeout)
			return
		}

		si.CacheDB = db
		so.DB = db
	}

	var all Items
	err = db.All(&all)
	if err != nil && !strings.Contains(err.Error(), "not found") {
		log.DebugPrint(si.Session.Debug, fmt.Sprintf("Sync | error retrieving all items from db: %s", err.Error()), common.MaxDebugChars)

		return
	}
	log.DebugPrint(si.Session.Debug, fmt.Sprintf("Sync | retrieved %d existing Items from db", len(all)), common.MaxDebugChars)

	// validate
	if err = all.Validate(); err != nil {
		return so, err
	}
	if len(all) > 0 {
		var cachedKeys items.EncryptedItems

		cachedKeys, err = retrieveItemsKeysFromCache(si.Session.Session, all)
		if err != nil {
			return
		}

		if err = processCachedItemsKeys(si.Session, cachedKeys); err != nil {
			return
		}
		log.DebugPrint(si.Session.Debug, fmt.Sprintf("Sync | processed %d items keys from cache", len(cachedKeys)), common.MaxDebugChars)
	}

	// look for dirty items to push to SN with the gosn sync
	var dirty []Item

	err = db.Find("Dirty", true, &dirty)
	if err != nil {
		if !strings.Contains(err.Error(), "not found") {
			return
		}
	}

	log.DebugPrint(si.Session.Debug, fmt.Sprintf("Sync | retrieved %d dirty Items from db", len(dirty)), common.MaxDebugChars)

	// Get resource-aware sync configuration
	itemCount := int64(len(all))
	timeout, retries := getSyncConfiguration(si, itemCount)
	log.DebugPrint(si.Session.Debug, fmt.Sprintf("Sync | Using timeout %v, retries %d for %d items", timeout, retries, itemCount), common.MaxDebugChars)

	// Validate and clean sync token using new validation logic
	syncToken, err := validateAndCleanSyncToken(db, si.Session.Session)
	if err != nil {
		log.DebugPrint(si.Session.Debug, fmt.Sprintf("Sync | Error validating sync token: %v", err), common.MaxDebugChars)
		return
	}

	// if session doesn't contain items keys then remove sync token so we bring all items in
	if si.Session.DefaultItemsKey.ItemsKey == "" {
		fmt.Printf("Sync | no default items key in session so resetting sync token\n")
		log.DebugPrint(si.Session.Debug, "Sync | no default items key in session so resetting sync token", common.MaxDebugChars)
		syncToken = ""
	} else if syncToken != "" {
		log.DebugPrint(si.Session.Debug, fmt.Sprintf("Sync | loaded sync token %s from db", syncToken), common.MaxDebugChars)
	}

	// convert dirty to items.Items
	// Pre-allocate with capacity to avoid reallocations (some items may be filtered out)
	dirtyItemsToPush := make(items.EncryptedItems, 0, len(dirty))

	for _, d := range dirty {
		// CRITICAL SAFEGUARD: Never push deleted or modified SN|ItemsKey items
		if d.ContentType == common.SNItemTypeItemsKey {
			if d.Deleted {
				log.DebugPrint(si.Session.Debug, fmt.Sprintf("Sync | WARNING: Blocking attempt to delete SN|ItemsKey %s from cache", d.UUID), common.MaxDebugChars)
				continue // Skip this item entirely
			}
			if d.Content == "" {
				panic("dirty items key is empty")
			}
			// Skip any attempt to modify existing ItemsKeys
			if d.UUID != "" && d.UpdatedAt != "" {
				log.DebugPrint(si.Session.Debug, fmt.Sprintf("Sync | WARNING: Blocking attempt to modify SN|ItemsKey %s from cache", d.UUID), common.MaxDebugChars)
				continue // Skip this item entirely
			}
		}

		// CRITICAL SAFEGUARD: Never push deleted SN|UserPreferences items
		if d.ContentType == common.SNItemTypeUserPreferences && d.Deleted {
			log.DebugPrint(si.Session.Debug, fmt.Sprintf("Sync | WARNING: Blocking attempt to delete SN|UserPreferences %s from cache", d.UUID), common.MaxDebugChars)
			continue // Skip this item entirely
		}

		dirtyItemsToPush = append(dirtyItemsToPush, items.EncryptedItem{
			UUID:               d.UUID,
			Content:            d.Content,
			ContentType:        d.ContentType,
			ItemsKeyID:         d.ItemsKeyID,
			EncItemKey:         d.EncItemKey,
			Deleted:            d.Deleted,
			CreatedAt:          d.CreatedAt,
			UpdatedAt:          d.UpdatedAt,
			CreatedAtTimestamp: d.CreatedAtTimestamp,
			UpdatedAtTimestamp: d.UpdatedAtTimestamp,
			DuplicateOf:        d.DuplicateOf,
		})

		// fmt.Printf("dirty item: %+v\n", dirtyItemsToPush)
	}

	// TODO: add all the items keys in the session to SN (dupes will be handled)?

	// Optimization: Skip API call if no changes and recent sync token exists
	if len(dirtyItemsToPush) == 0 && syncToken != "" {
		var syncTokens []SyncToken
		if err = db.All(&syncTokens); err == nil && len(syncTokens) == 1 {
			tokenAge := time.Since(syncTokens[0].CreatedAt)
			if tokenAge < common.MinSyncInterval {
				log.DebugPrint(si.Debug,
					fmt.Sprintf("Sync | Skipping API call - no changes and recent sync (age: %v)", tokenAge),
					common.MaxDebugChars)
				so.DB = db
				return so, nil
			}
		}
	}

	// call gosn sync with dirty items to push
	var gSI items.SyncInput

	if len(dirtyItemsToPush) > 0 {
		log.DebugPrint(si.Debug, fmt.Sprintf("Sync | pushing %d dirty items", len(dirtyItemsToPush)), common.MaxDebugChars)

		gSI = items.SyncInput{
			Session:   si.Session.Session,
			Items:     dirtyItemsToPush,
			SyncToken: syncToken,
		}
	} else {
		log.DebugPrint(si.Debug, "Sync | no dirty items to push", common.MaxDebugChars)

		gSI = items.SyncInput{
			Session:   si.Session.Session,
			SyncToken: syncToken,
		}
	}

	if !gSI.Session.Valid() {
		panic("invalid Session")
	}

	var gSO items.SyncOutput

	// Retry logic with enhanced error handling
	log.DebugPrint(si.Session.Debug, fmt.Sprintf("Sync | calling items.Sync with syncToken %s", syncToken), common.MaxDebugChars)

	for attempt := 0; attempt < retries; attempt++ {
		if attempt > 0 {
			log.DebugPrint(si.Session.Debug, fmt.Sprintf("Sync | Retry attempt %d/%d", attempt+1, retries), common.MaxDebugChars)
		}

		gSO, err = items.Sync(gSI)
		if err == nil {
			break // Success, exit retry loop
		}

		// Check if we should retry based on error type
		shouldRetry, newSI, finalErr := handleSyncError(err, si)
		if !shouldRetry {
			err = finalErr
			return
		}

		// Update sync input if recovery strategy modified it
		if newSI.Session != si.Session {
			gSI.Session = newSI.Session.Session
		}

		// Don't sleep on the last attempt
		if attempt < retries-1 {
			sleepTime := time.Duration(attempt+1) * time.Second
			// Cap delay at 5 seconds maximum
			if sleepTime > 5*time.Second {
				sleepTime = 5 * time.Second
			}
			log.DebugPrint(si.Session.Debug, fmt.Sprintf("Sync | Sleeping %v before retry", sleepTime), common.MaxDebugChars)
			time.Sleep(sleepTime)
		}
	}

	if err != nil {
		log.DebugPrint(si.Session.Debug, fmt.Sprintf("Sync | Failed after %d retries: %v", retries, err), common.MaxDebugChars)
		return
	}

	log.DebugPrint(si.Session.Debug, fmt.Sprintf("Sync | initial sync retrieved sync token %s from SN", gSO.SyncToken), common.MaxDebugChars)

	if len(gSO.Conflicts) > 0 {
		panic("conflicts should have been resolved by gosn sync")
	}

	// check items are valid
	// TODO: work out need for this
	// we expect deleted items to be returned so why check?
	// check saved items are valid and remove any from db that are now deleted
	var itemsToDeleteFromDB Items

	log.DebugPrint(si.Debug, "Sync | checking items.Sync Items for any unsupported", common.MaxDebugChars)

	for _, x := range gSO.Items {
		// if x.EncItemKey == "" {
		// 	fmt.Printf("item missing encitemkey: %+v\n", x)
		// }
		// TODO: we chould just do a 'Validate' method on items and find any without encItemKey (that are meant to be encrypted)
		components := strings.Split(x.EncItemKey, ":")

		if len(components) <= 1 {
			log.DebugPrint(si.Debug, fmt.Sprintf("Sync | ignoring item %s of type %s deleted: %t", x.UUID, x.ContentType, x.Deleted), common.MaxDebugChars)
			continue
		}
	}

	if len(gSO.SavedItems) > 0 {
		log.DebugPrint(si.Debug, fmt.Sprintf("Sync | checking %d items.Sync SavedItems for items to add or remove from db", len(gSO.SavedItems)), common.MaxDebugChars)
	}

	var savedItems Items

	for _, x := range gSO.SavedItems {
		// don't add deleted
		if x.ContentType == common.SNItemTypeItemsKey && x.Deleted {
			log.Fatalf("Attempting to delete SN|ItemsKey: %s", x.UUID)
		}

		if x.Deleted {
			// handle deleted item by removing from db
			var itemToDeleteFromDB Items

			err = db.Find("UUID", x.UUID, &itemToDeleteFromDB)
			if err != nil && err.Error() != "not found" {
				return
			}

			log.DebugPrint(si.Debug, fmt.Sprintf("Sync | adding %s %s to list of items to delete", x.ContentType, x.UUID), common.MaxDebugChars)

			itemsToDeleteFromDB = append(itemsToDeleteFromDB, itemToDeleteFromDB...)

			continue
		}

		log.DebugPrint(si.Debug, fmt.Sprintf("adding %s %s to db", x.ContentType, x.UUID), common.MaxDebugChars)
		// fmt.Printf("ITEM IS: %+v\n", x)

		item := Item{
			UUID:               x.UUID,
			Content:            x.Content,
			ContentType:        x.ContentType,
			ItemsKeyID:         x.ItemsKeyID,
			EncItemKey:         x.EncItemKey,
			Deleted:            x.Deleted,
			CreatedAt:          x.CreatedAt,
			UpdatedAt:          x.UpdatedAt,
			CreatedAtTimestamp: x.CreatedAtTimestamp,
			UpdatedAtTimestamp: x.UpdatedAtTimestamp,
			DuplicateOf:        x.DuplicateOf,
		}

		if item.Deleted {
			panic(fmt.Sprintf("adding deleted item to db: %+v", item))
		}

		// find item (from function input) matching saved item and update original with datestamps
		// the saved item is stripped of content
		var updatedSavedItems Items
		func() {
			for _, di := range dirtyItemsToPush {
				for _, ii := range savedItems {
					if di.UUID == ii.UUID {
						updatedSavedItems = append(updatedSavedItems, Item{
							UUID:               di.UUID,
							Content:            di.Content,
							ContentType:        di.ContentType,
							ItemsKeyID:         di.ItemsKeyID,
							EncItemKey:         di.EncItemKey,
							Deleted:            false,
							CreatedAt:          ii.CreatedAt,
							UpdatedAt:          ii.UpdatedAt,
							CreatedAtTimestamp: ii.CreatedAtTimestamp,
							UpdatedAtTimestamp: ii.UpdatedAtTimestamp,
							DuplicateOf:        di.DuplicateOf,
							Dirty:              false,
							DirtiedDate:        time.Now(),
						})
					}
				}
			}
		}()

		savedItems = append(savedItems, item)
	}

	if len(savedItems) > 0 {
		// fmt.Printf("saving items: %+v\n", savedItems)
		if err = SaveCacheItems(si.CacheDB, savedItems, false); err != nil {
			return
		}
	}

	// remove items from db
	if len(itemsToDeleteFromDB) > 0 {
		if err = DeleteCacheItems(db, itemsToDeleteFromDB, false); err != nil {
			return
		}
	}

	// unset dirty flag and date on anything that has now been synced back to SN
	if len(dirty) > 0 {
		log.DebugPrint(si.Debug, fmt.Sprintf("Sync | removing dirty flag on %d db items now synced back to SN", len(dirty)), common.MaxDebugChars)

		if err = CleanCacheItems(db, dirty, false); err != nil {
			return
		}
	}

	// get the items keys returned by SN
	var eiks []items.EncryptedItem

	if len(gSO.Items) > 0 {
		log.DebugPrint(si.Debug, fmt.Sprintf("Sync | saving %d items from items.Sync Items to db", len(gSO.Items)), common.MaxDebugChars)
	}

	// put new Items in db
	var itemsToDelete Items

	var newItems Items

	for _, i := range gSO.Items {
		// CRITICAL SAFEGUARD: Never delete SN|ItemsKey items from cache
		if i.ContentType == common.SNItemTypeItemsKey && i.Deleted {
			log.DebugPrint(si.Debug, fmt.Sprintf("Sync | WARNING: Refusing to delete SN|ItemsKey %s from cache", i.UUID), common.MaxDebugChars)
			continue // Skip deletion of ItemsKey
		}

		// CRITICAL SAFEGUARD: Never delete SN|UserPreferences items from cache
		if i.ContentType == common.SNItemTypeUserPreferences && i.Deleted {
			log.DebugPrint(si.Debug, fmt.Sprintf("Sync | WARNING: Refusing to delete SN|UserPreferences %s from cache", i.UUID), common.MaxDebugChars)
			continue // Skip deletion of UserPreferences
		}

		// if the item has been deleted in SN, then delete from db
		if i.Deleted {
			log.DebugPrint(si.Debug, fmt.Sprintf("Sync | adding uuid for deletion %s %s and skipping addition to db", i.ContentType, i.UUID), common.MaxDebugChars)

			di := Item{
				UUID:               i.UUID,
				Content:            i.Content,
				ContentType:        i.ContentType,
				ItemsKeyID:         i.ItemsKeyID,
				EncItemKey:         i.EncItemKey,
				Deleted:            i.Deleted,
				CreatedAt:          i.CreatedAt,
				UpdatedAt:          i.UpdatedAt,
				CreatedAtTimestamp: i.CreatedAtTimestamp,
				UpdatedAtTimestamp: i.UpdatedAtTimestamp,
				DuplicateOf:        i.DuplicateOf,
			}
			itemsToDelete = append(itemsToDelete, di)

			continue
		}

		if i.ContentType == common.SNItemTypeItemsKey {
			eiks = append(eiks, i)
		}

		item := Item{
			UUID:               i.UUID,
			Content:            i.Content,
			ContentType:        i.ContentType,
			ItemsKeyID:         i.ItemsKeyID,
			EncItemKey:         i.EncItemKey,
			Deleted:            i.Deleted,
			CreatedAt:          i.CreatedAt,
			UpdatedAt:          i.UpdatedAt,
			CreatedAtTimestamp: i.CreatedAtTimestamp,
			UpdatedAtTimestamp: i.UpdatedAtTimestamp,
			DuplicateOf:        i.DuplicateOf,
		}

		newItems = append(newItems, item)
	}

	if len(newItems) > 0 {
		log.DebugPrint(si.Debug, fmt.Sprintf("Sync | %d new items to saved to db", len(newItems)), common.MaxDebugChars)

		if err = SaveCacheItems(db, newItems, false); err != nil {
			return
		}
	}

	if len(itemsToDelete) > 0 {
		if err = DeleteCacheItems(db, itemsToDelete, false); err != nil {
			return
		}
	}

	if len(eiks) > 0 && processCachedItemsKeys(si.Session, eiks) != nil {
		return
	}

	log.DebugPrint(si.Debug, "Sync | retrieving all items from db in preparation for decryption", common.MaxDebugChars)

	err = db.All(&all)

	if len(all) > 0 {
		log.DebugPrint(si.Debug, fmt.Sprintf("Sync | retrieved %d items from db in preparation for decryption", len(all)), common.MaxDebugChars)
	}

	log.DebugPrint(si.Debug, fmt.Sprintf("Sync | replacing db SyncToken with latest from items.Sync: %s", gSO.SyncToken), common.MaxDebugChars)

	// drop the SyncToken db if it exists
	err = db.Drop("SyncToken")
	if err != nil && !strings.Contains(err.Error(), "not found") {
		err = fmt.Errorf("dropping sync token bucket before replacing: %w", err)

		return
	}
	// replace with latest from server
	sv := SyncToken{
		SyncToken: gSO.SyncToken,
		CreatedAt: time.Now(),
	}

	if err = db.Save(&sv); err != nil {
		log.DebugPrint(si.Session.Debug, fmt.Sprintf("Sync | saving sync token %s to db", sv.SyncToken), common.MaxDebugChars)
		err = fmt.Errorf("saving token to db %w", err)

		return
	}

	err = db.All(&all)

	so.DB = db

	return
}

func processCachedItemsKeys(s *Session, eiks items.EncryptedItems) error {
	// merge the items keys returned by SN with the items keys in the session
	if len(eiks) > 0 {
		log.DebugPrint(s.Debug, fmt.Sprintf("Sync | decrypting and parsing %d ItemsKeys", len(eiks)), common.MaxDebugChars)
	}

	iks, err := items.DecryptAndParseItemKeys(s.MasterKey, eiks)
	if err != nil {
		return err
	}

	var syncedItemsKeys []session.SessionItemsKey

	for x := range iks {
		syncedItemsKeys = append(syncedItemsKeys, session.SessionItemsKey{
			UUID:               iks[x].UUID,
			ItemsKey:           iks[x].ItemsKey,
			UpdatedAtTimestamp: iks[x].UpdatedAtTimestamp,
			CreatedAtTimestamp: iks[x].CreatedAtTimestamp,
		})
	}

	if len(eiks) > 0 {
		log.DebugPrint(s.Debug, fmt.Sprintf("Sync | merging %d new ItemsKeys with existing stored in session", len(eiks)), common.MaxDebugChars)
	}

	if len(s.Session.ItemsKeys) > 0 {
		log.DebugPrint(s.Debug, fmt.Sprintf("Sync | pre-merge total of %d ItemsKeys in session", len(s.Session.ItemsKeys)), common.MaxDebugChars)
	}

	s.Session.ItemsKeys = mergeItemsKeysSlices(s.Session.ItemsKeys, syncedItemsKeys)

	// Set default items key using Standard Notes priority logic:
	// 1. Prioritize keys marked as default
	// 2. Fall back to most recent by timestamp
	var defaultItemsKey session.SessionItemsKey
	var latestItemsKey session.SessionItemsKey

	for x := range s.Session.ItemsKeys {
		key := s.Session.ItemsKeys[x]

		// Track the most recent key regardless
		if key.CreatedAtTimestamp > latestItemsKey.CreatedAtTimestamp {
			latestItemsKey = key
		}

		// Prefer keys marked as default
		if key.Default {
			defaultItemsKey = key
			break
		}
	}

	// Use default key if found, otherwise use most recent
	if defaultItemsKey.UUID != "" {
		s.Session.DefaultItemsKey = defaultItemsKey
		log.DebugPrint(s.Debug, fmt.Sprintf("Sync | setting Default Items Key to %s (marked as default)", defaultItemsKey.UUID), common.MaxDebugChars)
	} else if latestItemsKey.UUID != "" {
		s.Session.DefaultItemsKey = latestItemsKey
		log.DebugPrint(s.Debug, fmt.Sprintf("Sync | setting Default Items Key to %s (most recent)", latestItemsKey.UUID), common.MaxDebugChars)
	}

	log.DebugPrint(s.Debug, fmt.Sprintf("Sync | post-merge total of %d ItemsKeys in session", len(s.Session.ItemsKeys)), common.MaxDebugChars)
	return err
}

func mergeItemsKeysSlices(sessionList, another []session.SessionItemsKey) []session.SessionItemsKey {
	var out []session.SessionItemsKey
	var holdingList []session.SessionItemsKey

	for _, s := range sessionList {
		var found bool

		for _, a := range another {
			// if we have a match add the latest to the holding list
			if a.UUID == s.UUID {
				found = true

				if a.UpdatedAtTimestamp > s.UpdatedAtTimestamp {
					// if other is newer, add that to holding list
					holdingList = append(holdingList, a)
				} else {
					// otherwise, session is same or equal, so add that to holding list
					holdingList = append(holdingList, s)
				}

				break
			}
		}
		// existing session key not found in the other list, so keep it
		if !found {
			holdingList = append(holdingList, s)
		}
	}

	// copy holding list to final list
	out = append(out, holdingList...)

	// add anything in other list that doesn't exist in final list until now
	for _, a := range another {
		var found bool

		for _, n := range holdingList {
			if a.UUID == n.UUID {
				found = true
				break
			}
		}

		if !found {
			out = append(out, a)
		}
	}

	return out
}

// GenCacheDBPath generates a path to a database file to be used as a cache of encrypted items
// The filename is a SHA2 hash of a concatenation of the following in order to be both unique
// and avoid concurrent usage:
// - part of the session authentication key (so that caches are unique to a user)
// - the server URL (so that caches are server specific)
// - the requesting application name (so that caches are application specific).
func GenCacheDBPath(session Session, dir, appName string) (string, error) {
	if !session.Valid() || appName == "" {
		return "", errors.New("invalid session or appName")
	}

	if dir == "" {
		homeDir, err := homedir.Dir()
		if err != nil {
			return "", err
		}
		dir = filepath.Join(homeDir, "."+appName)
	}

	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("failed to make cache directory: %s", dir)
	}

	h := sha256.New()
	h.Write([]byte(session.MasterKey[:2] + session.MasterKey[len(session.MasterKey)-2:] + session.MasterKey + appName))
	hexedDigest := hex.EncodeToString(h.Sum(nil))[:8]

	return filepath.Join(dir, appName+"-"+hexedDigest+".db"), nil
}
