package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/asdine/storm/v3"
	"github.com/jonhadfield/gosn-v2"
	"github.com/mitchellh/go-homedir"
)

const (
	// LOGGING
	libName       = "gosn-v2 cache" // name of library used in logging
	maxDebugChars = 120             // number of characters to display when logging API response body
)

type Item struct {
	UUID        string `storm:"id,unique"`
	Content     string
	ContentType string `storm:"index"`
	ItemsKeyID  string
	EncItemKey  string
	Deleted     bool
	CreatedAt   string
	UpdatedAt   string
	Dirty       bool
	DirtiedDate time.Time
}

type SyncToken struct {
	SyncToken string `storm:"id,unique"`
}

type SyncInput struct {
	*Session
	Close bool
	Debug bool
}

type SyncOutput struct {
	DB *storm.DB
}

type Items []Item

func (s *Session) gosn() *gosn.Session {
	gs := gosn.Session{
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
	return &gs
}

func (pi Items) ToItems(s *Session) (items gosn.Items, err error) {
	var eItems gosn.EncryptedItems
	for _, ei := range pi {
		eItems = append(eItems, gosn.EncryptedItem{
			UUID:        ei.UUID,
			Content:     ei.Content,
			ContentType: ei.ContentType,
			ItemsKeyID:  ei.ItemsKeyID,
			EncItemKey:  ei.EncItemKey,
			Deleted:     ei.Deleted,
			CreatedAt:   ei.CreatedAt,
			UpdatedAt:   ei.UpdatedAt,
		})
	}

	if eItems != nil {
		gs := s.gosn()
		if len(gs.ItemsKeys) == 0 {
			panic("trying to convert cache items to items with no items keys")
		}
		if gs.DefaultItemsKey.ItemsKey == "" {
			panic("trying to convert cache items to items with no default items key")
		}
		items, err = eItems.DecryptAndParse(gs)
		if err != nil {
			return
		}
	}

	return
}

func ToCacheItems(items gosn.EncryptedItems, clean bool) (pitems Items) {
	for _, i := range items {
		var cItem Item
		cItem.UUID = i.UUID
		cItem.Content = i.Content

		if !clean {
			cItem.Dirty = true
			cItem.DirtiedDate = time.Now()
		}

		cItem.ContentType = i.ContentType
		cItem.ItemsKeyID = i.ItemsKeyID
		cItem.EncItemKey = i.EncItemKey
		cItem.Deleted = i.Deleted
		cItem.CreatedAt = i.CreatedAt
		cItem.UpdatedAt = i.UpdatedAt
		pitems = append(pitems, cItem)
	}

	return
}

// SaveItems encrypts, converts to cache items, and then persists to db
func SaveItems(db *storm.DB, s *Session, items gosn.Items, close, debug bool) error {
	gs := s.gosn()

	eItems, err := items.Encrypt(*gs)
	if err != nil {
		return err
	}

	cItems := ToCacheItems(eItems, false)

	return SaveCacheItems(db, cItems, close)
}

// SaveNotes encrypts, converts to cache items, and then persists to db
func SaveNotes(s *Session, db *storm.DB, items gosn.Notes, close bool) error {
	eItems, err := items.Encrypt(*s.gosn())
	if err != nil {
		return err
	}

	cItems := ToCacheItems(eItems, false)

	return SaveCacheItems(db, cItems, close)
}

// SaveTags encrypts, converts to cache items, and then persists to db
func SaveTags(db *storm.DB, s *Session, items gosn.Tags, close bool) error {
	eItems, err := items.Encrypt(*s.gosn())
	if err != nil {
		return err
	}

	cItems := ToCacheItems(eItems, false)

	return SaveCacheItems(db, cItems, close)
}

// SaveEncryptedItems converts to cache items and persists to db
func SaveEncryptedItems(db *storm.DB, items gosn.EncryptedItems, close bool) error {
	cItems := ToCacheItems(items, false)
	return SaveCacheItems(db, cItems, close)
}

// SaveCacheItems saves Cache Items to the provided database
func SaveCacheItems(db *storm.DB, items Items, close bool) error {
	for _, i := range items {
		if err := db.Save(&i); err != nil {
			return err
		}
	}

	if close {
		return db.Close()
	}

	return nil
}

// UpdateCacheItems updates Cache Items in the provided database
func UpdateCacheItems(db *storm.DB, items Items, close bool) error {
	for _, i := range items {
		if err := db.Update(&i); err != nil {
			return err
		}
	}

	if close {
		return db.Close()
	}

	return nil
}

func Sync(si SyncInput) (so SyncOutput, err error) {
	// check session is valid
	if si.Session == nil || !si.Session.Valid() {
		err = fmt.Errorf("invalid session")
		return
	}

	// only path should be passed
	if si.Session.CacheDBPath == "" {
		err = fmt.Errorf("database path is required")
		return
	}

	var db *storm.DB

	// open DB if path provided
	if si.Session.CacheDBPath != "" {
		debugPrint(si.Session.Debug, fmt.Sprintf("Sync | using db in '%s'", si.Session.CacheDBPath))

		db, err = storm.Open(si.Session.CacheDBPath)
		if err != nil {
			return
		}

		if si.Close {
			defer db.Close()
		}
	}

	// look for dirty items to save with the gosn sync
	var dirty []Item

	err = db.Find("Dirty", true, &dirty)
	if err != nil {
		if !strings.Contains(err.Error(), "not found") {
			return
		}
	}

	// get sync token from previous operation
	var syncTokens []SyncToken

	err = db.All(&syncTokens)
	if err != nil {
		if !strings.Contains(err.Error(), "not found") {
			return
		}
		return
	}

	// load items keys from previous operation
	var ciks []Item

	err = db.Find("ContentType", "SN|ItemsKey", &ciks)
	if err != nil {
		if !strings.Contains(err.Error(), "not found") {
			return
		}
	}

	var iks gosn.EncryptedItems
	for _, cik := range ciks {
		iks = append(iks, gosn.EncryptedItem{
			UUID:        cik.UUID,
			ItemsKeyID:  cik.ItemsKeyID,
			Content:     cik.Content,
			ContentType: cik.ContentType,
			EncItemKey:  cik.EncItemKey,
			Deleted:     cik.Deleted,
			CreatedAt:   cik.CreatedAt,
			UpdatedAt:   cik.UpdatedAt,
		})
	}

	if iks != nil && len(iks) > 0 {
		err = iks.Validate()
		if err != nil {
			return
		}

		gs := si.Session.gosn()
		_, err = iks.DecryptAndParseItemsKeys(gs)
		si.Session.ItemsKeys = gs.ItemsKeys
		si.Session.DefaultItemsKey = gs.DefaultItemsKey
		if err != nil {
			return
		}
		if len(si.Session.ItemsKeys) == 0 {
			panic("pants")
		}
	}

	var syncToken string

	if len(syncTokens) > 1 {
		err = fmt.Errorf("expected maximum of one sync token but %d returned", len(syncTokens))

		return
	}

	if len(syncTokens) == 1 {
		syncToken = syncTokens[0].SyncToken
	}

	// if session doesn't contain items keys then set remove sync token
	if si.DefaultItemsKey.ItemsKey == "" {
		syncToken = ""
	}

	// convert dirty to gosn.Items
	var dirtyItemsToPush gosn.EncryptedItems
	for _, d := range dirty {
		dirtyItemsToPush = append(dirtyItemsToPush, gosn.EncryptedItem{
			UUID:        d.UUID,
			Content:     d.Content,
			ContentType: d.ContentType,
			ItemsKeyID:  d.ItemsKeyID,
			EncItemKey:  d.EncItemKey,
			Deleted:     d.Deleted,
			CreatedAt:   d.CreatedAt,
			UpdatedAt:   d.UpdatedAt,
		})
	}

	// TODO: Only call if dirty items to push
	// call gosn sync with dirty items to push
	var gSI gosn.SyncInput

	gs := si.Session.gosn()
	if len(dirtyItemsToPush) > 0 {
		gSI = gosn.SyncInput{
			Session:   gs,
			Items:     dirtyItemsToPush,
			SyncToken: syncToken,
		}
	} else {
		gSI = gosn.SyncInput{
			Session:   gs,
			SyncToken: syncToken,
		}
	}

	if !gSI.Session.Valid() {
		fmt.Println("Invalid Session")
	}

	var gSO gosn.SyncOutput
	gSO, err = gosn.Sync(gSI)
	if err != nil {
		return
	}

	// check items are valid
	for _, x := range gSO.Items {
		if x.Deleted {
			continue
		}

		components := strings.Split(x.EncItemKey, ":")
		if len(components) <= 1 {
			debugPrint(si.Debug, fmt.Sprintf("ignoring item %s of type %s", x.UUID, x.ContentType))
			continue
		}
	}

	// check saved items are valid
	for _, x := range gSO.SavedItems {
		if !x.Deleted {
			components := strings.Split(x.EncItemKey, ":")
			if len(components) <= 1 {
				debugPrint(si.Debug, fmt.Sprintf("ignoring saved item %s of type %s", x.UUID, x.ContentType))
				continue
			}

			//if x.UUID != components[2] {
			//	err = fmt.Errorf("synced item with uuid: %s has uuid in enc key as: %s", string(x.UUID), components[2])
			//	return
			//}
		}
	}

	// check unsaved items are valid
	for _, x := range gSO.Unsaved {
		if !x.Deleted {
			components := strings.Split(x.EncItemKey, ":")
			if len(components) <= 1 {
				debugPrint(si.Debug, fmt.Sprintf("ignoring unsaved item %s of type %s", x.UUID, x.ContentType))
				continue
			}
		}
	}

	for _, d := range dirty {
		err = db.UpdateField(&Item{UUID: d.UUID}, "Dirty", false)
		if err != nil {
			return
		}

		err = db.UpdateField(&Item{UUID: d.UUID}, "DirtiedDate", time.Time{})
		if err != nil {
			return
		}
	}

	// put new Items in db
	for _, i := range gSO.Items {
		if i.ContentType == "SN|ItemsKey" && i.Deleted {
			continue
		}

		item := Item{
			UUID:        i.UUID,
			Content:     i.Content,
			ContentType: i.ContentType,
			ItemsKeyID:  i.ItemsKeyID,
			EncItemKey:  i.EncItemKey,
			Deleted:     i.Deleted,
			CreatedAt:   i.CreatedAt,
			UpdatedAt:   i.UpdatedAt,
		}

		err = db.Save(&item)
		if err != nil {
			return
		}
	}

	// updated db with saved items
	// put new Items in db
	for _, i := range gSO.SavedItems {
		if i.ContentType == "SN|ItemsKey" && i.Deleted {
			continue
		}

		item := Item{
			UUID:        i.UUID,
			Content:     i.Content,
			ContentType: i.ContentType,
			ItemsKeyID:  i.ItemsKeyID,
			EncItemKey:  i.EncItemKey,
			Deleted:     i.Deleted,
			CreatedAt:   i.CreatedAt,
			UpdatedAt:   i.UpdatedAt,
		}

		err = db.Save(&item)
		if err != nil {
			return
		}
	}

	// drop the SyncToken db if it exists
	_ = db.Drop("SyncToken")
	// replace with latest from server
	sv := SyncToken{
		SyncToken: gSO.SyncToken,
	}

	if err = db.Save(&sv); err != nil {
		return
	}

	if len(gSO.Items) > 0 {
		_, err = gSO.Items.DecryptAndParseItemsKeys(si.Session.Session)
	}

	so.DB = db
	si.Session.ItemsKeys = gs.ItemsKeys
	si.Session.DefaultItemsKey = gs.DefaultItemsKey
	si.Session.Debug = gs.Debug
	si.Session.MasterKey = gs.MasterKey
	si.Session.AccessExpiration = gs.AccessExpiration
	si.Session.RefreshToken = gs.RefreshToken
	si.Session.AccessToken = gs.AccessToken
	si.Session.RefreshExpiration = gs.RefreshExpiration
	si.Session.Token = gs.Token
	si.Session.Server = gs.Server

	return
}

// GenCacheDBPath generates a path to a database file to be used as a cache of encrypted items
// The filename is a SHA2 hash of a concatenation of the following in order to be both unique
// and avoid concurrent usage:
// - part of the session authentication key (so that caches are unique to a user)
// - the server URL (so that caches are server specific)
// - the requesting application name (so that caches are application specific)
func GenCacheDBPath(session Session, dir, appName string) (string, error) {
	var err error

	if !session.Valid() {
		return "", fmt.Errorf("invalid session")
	}

	if appName == "" {
		return "", fmt.Errorf("appName is a required")
	}

	// if cache directory not defined then create dot path in home directory
	if dir == "" {
		var homeDir string

		homeDir, err = homedir.Dir()
		if err != nil {
			return "", err
		}

		dir = filepath.Join(homeDir, "."+appName)
	}

	err = os.MkdirAll(dir, 0o700)
	if err != nil {
		return "", fmt.Errorf("failed to make cache directory: %s", dir)
	}

	h := sha256.New()

	h.Write([]byte(session.MasterKey[:2] + session.MasterKey[len(session.MasterKey)-2:] + session.MasterKey + appName))
	bs := h.Sum(nil)
	hexedDigest := hex.EncodeToString(bs)[:8]

	return filepath.Join(dir, appName+"-"+hexedDigest+".db"), err
}

func debugPrint(show bool, msg string) {
	if show {
		if len(msg) > maxDebugChars {
			msg = msg[:maxDebugChars] + "..."
		}

		log.Println(libName, "|", msg)
	}
}
