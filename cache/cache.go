package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"github.com/jonhadfield/gosn-v2/common"
	"github.com/jonhadfield/gosn-v2/items"
	logging2 "github.com/jonhadfield/gosn-v2/logging"
	"github.com/jonhadfield/gosn-v2/session"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/asdine/storm/v3"
	"github.com/fatih/color"
	"github.com/mitchellh/go-homedir"
)

const (
	// LOGGING.
	libName       = "gosn-v2 | cache" // name of library used in logging
	maxDebugChars = 120               // number of characters to display when logging API response body
)

var HiWhite = color.New(color.FgHiWhite).SprintFunc()

type Item struct {
	UUID               string `storm:"id,unique"`
	Content            string
	ContentType        string `storm:"index"`
	ItemsKeyID         string
	EncItemKey         string
	Deleted            bool
	CreatedAt          string
	UpdatedAt          string
	CreatedAtTimestamp int64
	UpdatedAtTimestamp int64
	DuplicateOf        *string
	Dirty              bool
	DirtiedDate        time.Time
}

type SyncToken struct {
	SyncToken string `storm:"id,unique"`
}

type SyncInput struct {
	*Session
	Close bool
}

type SyncOutput struct {
	DB *storm.DB
}

type Items []Item

func (s *Session) gosn() *session.Session {
	gs := session.Session{
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

func (pi Items) ToItems(s *Session) (its items.Items, err error) {
	logging2.DebugPrint(s.Debug, fmt.Sprintf("ToItems | Converting %d cache items to gosn items", len(pi)), common.MaxDebugChars)

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
			logging2.DebugPrint(s.Debug, fmt.Sprintf("ToItems | ignoring invalid item due to missing encrypted items key: %+v", ei), common.MaxDebugChars)
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
			return
		}
	}

	return
}

// func (s *Session) Export(path string) error {
// 	logging2.DebugPrint(s.Debug, fmt.Sprintf("Exporting to path: %s", path),common.MaxDebugChars)
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
// 	logging2.DebugPrint(s.Debug, fmt.Sprintf("importing from %s", path))
//
// 	ii, ifk, err := s.Session.Import(path, syncToken, "")
// 	if err != nil {
// 		return err
// 	}
//
// 	logging2.DebugPrint(s.Debug, fmt.Sprintf("importing loaded: %d items and %s key", len(ii), ifk.UUID))
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

		if !i.Deleted && i.ItemsKeyID == "" && !(i.ContentType == "SN|ItemsKey" || strings.HasPrefix(i.ContentType, "SF")) {
			panic(fmt.Sprintf("we've received %s %s from SN without ItemsKeyID", i.ContentType, i.UUID))
		}

		if i.ItemsKeyID != "" {
			iik = i.ItemsKeyID
		}

		if i.ContentType == "SN|ItemsKey" && iik != "" {
			log.Fatal("ItemsKey with UUID:", i.UUID, "has ItemsKeyID:", iik)
		}

		cItem.ItemsKeyID = i.ItemsKeyID
		cItem.EncItemKey = i.EncItemKey
		cItem.Deleted = i.Deleted
		cItem.CreatedAt = i.CreatedAt
		cItem.UpdatedAt = i.UpdatedAt
		cItem.DuplicateOf = i.DuplicateOf
		pitems = append(pitems, cItem)
	}

	return
}

// SaveNotes encrypts, converts to cache items, and then persists to db.
func SaveNotes(s *Session, db *storm.DB, items items.Notes, close bool) error {
	eItems, err := items.Encrypt(*s.gosn())
	if err != nil {
		return err
	}

	cItems := ToCacheItems(eItems, false)

	return SaveCacheItems(db, cItems, close)
}

// SaveTags encrypts, converts to cache items, and then persists to db.
func SaveTags(db *storm.DB, s *Session, items items.Tags, close bool) error {
	eItems, err := items.Encrypt(*s.gosn())
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
		return fmt.Errorf("no items provided to SaveItems")
	}

	if db == nil {
		return fmt.Errorf("db not passed to SaveItems")
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
		return fmt.Errorf("no items provided to SaveCacheItems")
	}

	if db == nil {
		return fmt.Errorf("db not passed to SaveCacheItems")
	}

	batchSize := 500

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

// DeleteCacheItems saves Cache Items to the provided database.
func DeleteCacheItems(db *storm.DB, items Items, close bool) error {
	if len(items) == 0 {
		return fmt.Errorf("no items provided to DeleteCacheItems")
	}

	if db == nil {
		return fmt.Errorf("db not passed to DeleteCacheItems")
	}

	batchSize := 500

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
		return fmt.Errorf("no items provided to DeleteCacheItems")
	}

	if db == nil {
		return fmt.Errorf("db not passed to DeleteCacheItems")
	}

	batchSize := 500

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
			// 	"please export and then import content using offical app to upgrade to 004 and then run 'sn resync' to upgrade cache to version 004", i[x].ContentType, i[x].UUID)
		case i[x].UUID == "":
			return fmt.Errorf("cache item is missing uuid: %+v", i[x])
		case i[x].ContentType == "":
			return fmt.Errorf("cache item is missing content_type: %+v", i[x])
		case i[x].Content == "":
			return fmt.Errorf("cache item is missing content: %+v", i[x])
		case i[x].EncItemKey == "" && i[x].ContentType != "SF|Extension":
			return fmt.Errorf("cache item is missing enc_item_key: %+v", i[x])
		case i[x].ContentType != "SN|ItemsKey" && i[x].ContentType != "SF|Extension" && i[x].ItemsKeyID == "":
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
			// case i[x].EncItemKey == "" && i[x].ContentType != "SF|Extension":
			// 	return fmt.Errorf("cache item is missing enc_item_key: %+v", i[x])
			// case i[x].ContentType != "SN|ItemsKey" && i[x].ContentType != "SF|Extension" && i[x].ItemsKeyID == "":
			// 	return fmt.Errorf("cache item is missing items_key_id: %+v", i[x])
			// }
		}
	}

	return nil
}

func retrieveItemsKeysFromCache(s *session.Session, i Items) (encryptedItemKeys items.EncryptedItems, err error) {
	logging2.DebugPrint(s.Debug, "retrieveItemsKeysFromCache | attempting to retrieve items key(s) from cache", common.MaxDebugChars)

	for x := range i {
		if i[x].ContentType == "SN|ItemsKey" && !i[x].Deleted {
			logging2.DebugPrint(s.Debug, fmt.Sprintf("retrieved items key %s from cache", i[x].UUID), common.MaxDebugChars)

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

	return
}

// Sync will push any dirty items to SN and make database cache consistent with SN.
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
		logging2.DebugPrint(si.Session.Debug, fmt.Sprintf("Sync | using db in '%s'", si.Session.CacheDBPath), common.MaxDebugChars)

		db, err = storm.Open(si.Session.CacheDBPath)
		if err != nil {
			return
		}

		si.CacheDB = db

		if si.Close {
			defer func(db *storm.DB) {
				if err = db.Close(); err != nil {
					panic(err)
				}
			}(db)
		}
	}

	var all Items
	err = db.All(&all)
	logging2.DebugPrint(si.Session.Debug, fmt.Sprintf("Sync | retrieved %d existing Items from db", len(all)), common.MaxDebugChars)

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
	}

	// look for dirty items to push to SN with the gosn sync
	var dirty []Item

	err = db.Find("Dirty", true, &dirty)
	if err != nil {
		if !strings.Contains(err.Error(), "not found") {
			return
		}
	}

	logging2.DebugPrint(si.Session.Debug, fmt.Sprintf("Sync | retrieved %d dirty Items from db", len(dirty)), common.MaxDebugChars)

	// get sync token from previous operation to prevent syncing all data each time
	var syncTokens []SyncToken
	err = db.All(&syncTokens)

	if err != nil && !strings.Contains(err.Error(), "not found") {
		// on first ever sync, we won't have a sync token

		return
	}

	var syncToken string

	if len(syncTokens) > 1 {
		err = fmt.Errorf("expected maximum of one sync token but %d returned", len(syncTokens))

		return
	}

	if len(syncTokens) == 1 {
		syncToken = syncTokens[0].SyncToken
		logging2.DebugPrint(si.Session.Debug, fmt.Sprintf("Sync | loaded sync token %s from db", syncToken), common.MaxDebugChars)
	}

	// if session doesn't contain items keys then remove sync token so we bring all items in
	if si.Session.DefaultItemsKey.ItemsKey == "" {
		logging2.DebugPrint(si.Session.Debug, "Sync | no default items key in session so resetting sync token", common.MaxDebugChars)

		syncToken = ""
	}

	// convert dirty to items.Items
	var dirtyItemsToPush items.EncryptedItems

	for _, d := range dirty {
		if d.ContentType == "SN|ItemsKey" && d.Content == "" {
			panic("dirty items key is empty")
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

	// call gosn sync with dirty items to push
	var gSI items.SyncInput

	if len(dirtyItemsToPush) > 0 {
		logging2.DebugPrint(si.Debug, fmt.Sprintf("Sync | pushing %d dirty items", len(dirtyItemsToPush)), common.MaxDebugChars)

		gSI = items.SyncInput{
			Session:   si.Session.Session,
			Items:     dirtyItemsToPush,
			SyncToken: syncToken,
		}
	} else {
		logging2.DebugPrint(si.Debug, "Sync | no dirty items to push", common.MaxDebugChars)

		gSI = items.SyncInput{
			Session:   si.Session.Session,
			SyncToken: syncToken,
		}
	}

	if !gSI.Session.Valid() {
		panic("invalid Session")
	}

	var gSO items.SyncOutput

	logging2.DebugPrint(si.Session.Debug, fmt.Sprintf("Sync | calling items.Sync with syncToken %s", syncToken), common.MaxDebugChars)

	gSO, err = items.Sync(gSI)
	if err != nil {
		return
	}

	logging2.DebugPrint(si.Session.Debug, fmt.Sprintf("Sync | initial sync retrieved sync token %s from SN", gSO.SyncToken), common.MaxDebugChars)

	if len(gSO.Conflicts) > 0 {
		panic("conflicts should have been resolved by gosn sync")
	}

	// check items are valid
	// TODO: work out need for this
	// we expect deleted items to be returned so why check?
	// check saved items are valid and remove any from db that are now deleted
	var itemsToDeleteFromDB Items

	logging2.DebugPrint(si.Debug, "Sync | checking items.Sync Items for any unsupported", common.MaxDebugChars)

	for _, x := range gSO.Items {
		// if x.EncItemKey == "" {
		// 	fmt.Printf("item missing encitemkey: %+v\n", x)
		// }
		// TODO: we chould just do a 'Validate' method on items and find any without encItemKey (that are meant to be encrypted)
		components := strings.Split(x.EncItemKey, ":")

		if len(components) <= 1 {
			logging2.DebugPrint(si.Debug, fmt.Sprintf("Sync | ignoring item %s of type %s deleted: %t", x.UUID, x.ContentType, x.Deleted), common.MaxDebugChars)
			continue
		}
	}

	if len(gSO.SavedItems) > 0 {
		logging2.DebugPrint(si.Debug, fmt.Sprintf("Sync | checking %d items.Sync SavedItems for items to add or remove from db", len(gSO.SavedItems)), common.MaxDebugChars)
	}

	var savedItems Items

	for _, x := range gSO.SavedItems {
		// don't add deleted
		if x.ContentType == "SN|ItemsKey" && x.Deleted {
			log.Fatal("Attempting to delete SN|ItemsKey:", x.UUID)
		}

		if x.Deleted {
			// handle deleted item by removing from db
			var itemToDeleteFromDB Items

			err = db.Find("UUID", x.UUID, &itemToDeleteFromDB)
			if err != nil && err.Error() != "not found" {
				return
			}

			logging2.DebugPrint(si.Debug, fmt.Sprintf("Sync | adding %s %s to list of items to delete", x.ContentType, x.UUID), common.MaxDebugChars)

			itemsToDeleteFromDB = append(itemsToDeleteFromDB, itemToDeleteFromDB...)

			continue
		}

		logging2.DebugPrint(si.Debug, fmt.Sprintf("adding %s %s to db", x.ContentType, x.UUID), common.MaxDebugChars)
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
		logging2.DebugPrint(si.Debug, fmt.Sprintf("Sync | removing dirty flag on %d db items now synced back to SN", len(dirty)), common.MaxDebugChars)

		if err = CleanCacheItems(db, dirty, false); err != nil {
			return
		}
	}

	// get the items keys returned by SN
	var eiks []items.EncryptedItem

	if len(gSO.Items) > 0 {
		logging2.DebugPrint(si.Debug, fmt.Sprintf("Sync | saving %d items from items.Sync Items to db", len(gSO.Items)), common.MaxDebugChars)
	}

	// put new Items in db
	var itemsToDelete Items

	var newItems Items

	for _, i := range gSO.Items {
		// if the item has been deleted in SN, then delete from db
		if i.Deleted {
			logging2.DebugPrint(si.Debug, fmt.Sprintf("Sync | adding uuid for deletion %s %s and skipping addition to db", i.ContentType, i.UUID), common.MaxDebugChars)

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

		if i.ContentType == "SN|ItemsKey" {
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
		logging2.DebugPrint(si.Debug, fmt.Sprintf("Sync | %d new items to saved to db", len(newItems)), common.MaxDebugChars)

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

	logging2.DebugPrint(si.Debug, "Sync | retrieving all items from db in preparation for decryption", common.MaxDebugChars)

	err = db.All(&all)

	if len(all) > 0 {
		logging2.DebugPrint(si.Debug, fmt.Sprintf("Sync | retrieved %d items from db in preparation for decryption", len(all)), common.MaxDebugChars)
	}

	logging2.DebugPrint(si.Debug, fmt.Sprintf("Sync | replacing db SyncToken with latest from items.Sync: %s", gSO.SyncToken), common.MaxDebugChars)

	// drop the SyncToken db if it exists
	err = db.Drop("SyncToken")
	if err != nil && !strings.Contains(err.Error(), "not found") {
		err = fmt.Errorf("dropping sync token bucket before replacing: %w", err)

		return
	}
	// replace with latest from server
	sv := SyncToken{
		SyncToken: gSO.SyncToken,
	}

	if err = db.Save(&sv); err != nil {
		logging2.DebugPrint(si.Session.Debug, fmt.Sprintf("Sync | saving sync token %s to db", sv.SyncToken), common.MaxDebugChars)
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
		logging2.DebugPrint(s.Debug, fmt.Sprintf("Sync | decrypting and parsing %d ItemsKeys", len(eiks)), common.MaxDebugChars)
	}

	// fmt.Printf("PRE-SYNC ITEMS KEYS === START\n")
	// for _, x := range eiks {
	// fmt.Printf("ITEMS KEY: %+v\n", x)
	// }
	// fmt.Printf("PRE-SYNC ITEMS KEYS === END\n")

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
		logging2.DebugPrint(s.Debug, fmt.Sprintf("Sync | merging %d new ItemsKeys with existing stored in session", len(eiks)), common.MaxDebugChars)
	}

	if len(s.Session.ItemsKeys) > 0 {
		logging2.DebugPrint(s.Debug, fmt.Sprintf("Sync | pre-merge total of %d ItemsKeys in session", len(s.Session.ItemsKeys)), common.MaxDebugChars)
	}

	s.Session.ItemsKeys = mergeItemsKeysSlices(s.Session.ItemsKeys, syncedItemsKeys)
	// set default items key to most recent
	var latestItemsKey session.SessionItemsKey
	for x := range s.Session.ItemsKeys {
		if s.Session.ItemsKeys[x].CreatedAtTimestamp > latestItemsKey.CreatedAtTimestamp {
			latestItemsKey = s.Session.ItemsKeys[x]
		}
	}

	logging2.DebugPrint(s.Debug, fmt.Sprintf("Sync | setting Default Items Key to %s", latestItemsKey.UUID), common.MaxDebugChars)
	s.Session.DefaultItemsKey = latestItemsKey

	logging2.DebugPrint(s.Debug, fmt.Sprintf("Sync | post-merge total of %d ItemsKeys in session", len(s.Session.ItemsKeys)), common.MaxDebugChars)
	// fmt.Printf("POST-SYNC ITEMS KEYS === START\n")
	// for _, x := range s.Session.ItemsKeys {
	// 	fmt.Printf("ITEMS KEY: %+v\n", x)
	// }
	// fmt.Printf("POST-SYNC ITEMS KEYS === END\n")
	return err
}

func mergeItemsKeysSlices(sessionList, another []session.SessionItemsKey) (out []session.SessionItemsKey) {
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

	return
}

// GenCacheDBPath generates a path to a database file to be used as a cache of encrypted items
// The filename is a SHA2 hash of a concatenation of the following in order to be both unique
// and avoid concurrent usage:
// - part of the session authentication key (so that caches are unique to a user)
// - the server URL (so that caches are server specific)
// - the requesting application name (so that caches are application specific).
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

func DebugPrint(show bool, msg string) {
	if show {
		if len(msg) > maxDebugChars {
			msg = msg[:maxDebugChars] + "..."
		}

		log.Println(libName, "|", msg)
	}
}
