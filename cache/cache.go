package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/asdine/storm/v3"
	"github.com/fatih/color"
	"github.com/jonhadfield/gosn-v2"
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
	ItemsKeyID         *string
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
	debugPrint(s.Debug, fmt.Sprintf("ToItems | Converting %d cache items to gosn items", len(pi)))

	var eItems gosn.EncryptedItems

	for _, ei := range pi {
		// we'd never need to decrypt an item in the db that needs to be deleted
		if ei.Deleted {
			continue
		}

		if ei.EncItemKey == "" {
			// TODO: should I ignore or return an error?
			debugPrint(s.Debug, fmt.Sprintf("ToItems | ignoring invalid item due to missing encrypted items key: %+v", ei))
		}

		eiik := ei.ItemsKeyID

		eItems = append(eItems, gosn.EncryptedItem{
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

		items, err = eItems.DecryptAndParse(s.Session)

		if err != nil {
			return
		}
	}

	return
}

func (s *Session) Export(path string) error {
	debugPrint(s.Debug, fmt.Sprintf("Exporting to path: %s", path))

	err := s.Session.Export(path)
	if err != nil {
		return err
	}

	return nil
}

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
func (s *Session) Import(path string, persist bool) error {
	so, err := Sync(SyncInput{
		Session: s,
		Close:   false,
	})
	if err != nil {
		return err
	}

	var syncTokens []SyncToken

	err = so.DB.All(&syncTokens)
	if err != nil && err.Error() != "not found" {
		return err
	}

	syncToken := ""

	if len(syncTokens) > 0 {
		syncToken = syncTokens[0].SyncToken
	}

	debugPrint(s.Debug, fmt.Sprintf("importing from %s", path))

	ii, ifk, err := s.Session.Import(path, syncToken, "")
	if err != nil {
		return err
	}

	debugPrint(s.Debug, fmt.Sprintf("importing loaded: %d items and %s key", len(ii), ifk.UUID))

	err = so.DB.Close()
	if err != nil {
		return err
	}

	_, err = Sync(SyncInput{
		Session: s,
		Close:   true,
	})

	return err
}

func ToCacheItems(items gosn.EncryptedItems, clean bool) (pitems Items) {
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

		if !i.Deleted && i.ItemsKeyID == nil && !(i.ContentType == "SN|ItemsKey" || strings.HasPrefix(i.ContentType, "SF")) {
			panic(fmt.Sprintf("we've received %s %s from SN without ItemsKeyID", i.ContentType, i.UUID))
		}

		if i.ItemsKeyID != nil {
			iik = *i.ItemsKeyID
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

// SaveItems encrypts, converts to cache items, and then persists to db.
func SaveItems(db *storm.DB, s *Session, items gosn.Items, close bool) error {
	eItems, err := items.Encrypt(s.Session.DefaultItemsKey, s.MasterKey, s.Debug)
	if err != nil {
		return err
	}

	cItems := ToCacheItems(eItems, false)

	return SaveCacheItems(db, cItems, close)
}

// SaveNotes encrypts, converts to cache items, and then persists to db.
func SaveNotes(s *Session, db *storm.DB, items gosn.Notes, close bool) error {
	eItems, err := items.Encrypt(*s.gosn())
	if err != nil {
		return err
	}

	cItems := ToCacheItems(eItems, false)

	return SaveCacheItems(db, cItems, close)
}

// SaveTags encrypts, converts to cache items, and then persists to db.
func SaveTags(db *storm.DB, s *Session, items gosn.Tags, close bool) error {
	eItems, err := items.Encrypt(*s.gosn())
	if err != nil {
		return err
	}

	cItems := ToCacheItems(eItems, false)

	return SaveCacheItems(db, cItems, close)
}

// SaveEncryptedItems converts to cache items and persists to db.
func SaveEncryptedItems(db *storm.DB, items gosn.EncryptedItems, close bool) error {
	cItems := ToCacheItems(items, false)

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
	numItems := len(items)
	if numItems < 500 {
		batchSize = numItems
	}

	total := len(items)

	for i := 0; i < total/batchSize; i++ {
		tx, err := db.Begin(true)
		if err != nil {
			return err
		}

		for j := 0; j < batchSize; j++ {
			err = tx.Save(&items[j])
			if err != nil {
				tx.Rollback()
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

// DeleteCacheItems saves Cache Items to the provided database.
func DeleteCacheItems(db *storm.DB, items Items, close bool) error {
	if len(items) == 0 {
		return fmt.Errorf("no items provided to DeleteCacheItems")
	}

	if db == nil {
		return fmt.Errorf("db not passed to DeleteCacheItems")
	}

	batchSize := 500
	numItems := len(items)

	if numItems < 500 {
		batchSize = numItems
	}

	total := len(items)

	for i := 0; i < total/batchSize; i++ {
		tx, err := db.Begin(true)
		if err != nil {
			return err
		}

		for j := 0; j < batchSize; j++ {
			err = tx.DeleteStruct(&items[j])
			if err != nil {
				if strings.Contains(err.Error(), "not found") {
					continue
				}
				tx.Rollback()

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

// UpdateCacheItems updates Cache Items in the provided database.
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

type CleanInput struct {
	*Session
	Close                 bool
	UnreferencedItemsKeys bool
}

func Clean(ci CleanInput) (err error) {
	// check session is valid
	if ci.Session == nil || !ci.Session.Valid() {
		err = fmt.Errorf("invalid session")
		return
	}

	// only path should be passed
	if ci.Session.CacheDBPath == "" {
		err = fmt.Errorf("database path is required")
		return
	}

	var db *storm.DB

	// open DB if path provided
	if ci.Session.CacheDBPath != "" {
		debugPrint(ci.Session.Debug, fmt.Sprintf("Sync | using db in '%s'", ci.Session.CacheDBPath))

		db, err = storm.Open(ci.Session.CacheDBPath)
		if err != nil {
			return
		}

		ci.CacheDB = db

		if ci.Close {
			defer func(db *storm.DB) {
				if err = db.Close(); err != nil {
					panic(err)
				}
			}(db)
		}
	}

	var all []Item
	err = db.All(&all)
	debugPrint(ci.Session.Debug, fmt.Sprintf("Sync | retrieved %d existing Items from db", len(all)))

	seenItems := make(map[string]int)
	for x := range all {
		if seenItems[all[x].UUID] > 0 {
			panic(fmt.Sprintf("duplicate item in cache: %s %s", all[x].ContentType, all[x].UUID))
		}
		seenItems[all[x].UUID]++
	}

	return
}

func (i Items) Validate() error {
	for x := range i {
		if i[x].Deleted {
			continue
		}

		switch {
		case i[x].UUID == "":
			return fmt.Errorf("cache item is missing uuid: %+v", i[x])
		case i[x].ContentType == "":
			return fmt.Errorf("cache item is missing content_type: %+v", i[x])
		case i[x].Content == "":
			return fmt.Errorf("cache item is missing content: %+v", i[x])
		case i[x].EncItemKey == "" && i[x].ContentType != "SF|Extension":
			return fmt.Errorf("cache item is missing enc_item_key: %+v", i[x])
		case i[x].ContentType != "SN|ItemsKey" && i[x].ContentType != "SF|Extension" && i[x].ItemsKeyID == nil:
			return fmt.Errorf("cache item is missing items_key_id: %+v", i[x])
		}
	}

	return nil
}

func retrieveItemsKeysFromCache(s *gosn.Session, i Items) (encryptedItemKeys gosn.EncryptedItems, err error) {
	debugPrint(s.Debug, "retrieveItemsKeysFromCache| attempting to retrieve items key(s) from cache")

	for x := range i {
		if i[x].ContentType == "SN|ItemsKey" && !i[x].Deleted {
			debugPrint(s.Debug, fmt.Sprintf("retrieved items key %s from cache", i[x].UUID))

			encryptedItemKeys = append(encryptedItemKeys, gosn.EncryptedItem{
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

// Sync will push any dirty items to SN and make database cache consistent with SN
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
	debugPrint(si.Session.Debug, fmt.Sprintf("Sync | retrieved %d existing Items from db", len(all)))

	// validate
	if err = all.Validate(); err != nil {
		return so, err
	}

	if len(all) > 0 {
		var cachedKeys gosn.EncryptedItems

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

	debugPrint(si.Session.Debug, fmt.Sprintf("Sync | retrieved %d dirty Items from db", len(dirty)))

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
		debugPrint(si.Session.Debug, fmt.Sprintf("Sync | loaded sync token %s from db", syncToken))
	}

	// if session doesn't contain items keys then remove sync token so we bring all items in
	if si.Session.DefaultItemsKey.ItemsKey == "" {
		debugPrint(si.Session.Debug, "Sync | no default items key in session so resetting sync token")

		syncToken = ""
	}

	// convert dirty to gosn.Items
	var dirtyItemsToPush gosn.EncryptedItems

	for _, d := range dirty {
		if d.ContentType == "SN|ItemsKey" && d.Content == "" {
			panic("dirty items key is empty")
		}

		dirtyItemsToPush = append(dirtyItemsToPush, gosn.EncryptedItem{
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
	}

	// TODO: add all the items keys in the session to SN (dupes will be handled)?

	// call gosn sync with dirty items to push
	var gSI gosn.SyncInput

	if len(dirtyItemsToPush) > 0 {
		debugPrint(si.Debug, fmt.Sprintf("Sync | pushing %d dirty items", len(dirtyItemsToPush)))

		gSI = gosn.SyncInput{
			Session:   si.Session.Session,
			Items:     dirtyItemsToPush,
			SyncToken: syncToken,
		}
	} else {
		debugPrint(si.Debug, "Sync | no dirty items to push")

		gSI = gosn.SyncInput{
			Session:   si.Session.Session,
			SyncToken: syncToken,
		}
	}

	if !gSI.Session.Valid() {
		panic("invalid Session")
	}

	var gSO gosn.SyncOutput

	debugPrint(si.Session.Debug, fmt.Sprintf("Sync | calling gosn.Sync with syncToken %s", syncToken))

	gSO, err = gosn.Sync(gSI)
	if err != nil {
		return
	}

	debugPrint(si.Session.Debug, fmt.Sprintf("Sync | initial sync retrieved sync token %s from SN", gSO.SyncToken))

	if len(gSO.Conflicts) > 0 {
		panic("conflicts should have been resolved by gosn sync")
	}

	// check items are valid
	// TODO: work out need for this
	// we expect deleted items to be returned so why check?
	// check saved items are valid and remove any from db that are now deleted
	var itemsToDeleteFromDB Items

	debugPrint(si.Debug, "Sync | checking gosn.Sync Items for any unsupported")

	for _, x := range gSO.Items {
		// TODO: we chould just do a 'Validate' method on items and find any without encItemKey (that are meant to be encrypted)
		components := strings.Split(x.EncItemKey, ":")

		if len(components) <= 1 {
			debugPrint(si.Debug, fmt.Sprintf("Sync | ignoring item %s of type %s deleted: %t", x.UUID, x.ContentType, x.Deleted))
			continue
		}
	}

	debugPrint(si.Debug, fmt.Sprintf("Sync | checking %d gosn.Sync SavedItems for items to add or remove from db", len(gSO.SavedItems)))

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

			debugPrint(si.Debug, fmt.Sprintf("Sync | adding %s %s to list of items to delete", x.ContentType, x.UUID))

			itemsToDeleteFromDB = append(itemsToDeleteFromDB, itemToDeleteFromDB...)

			continue
		}

		debugPrint(si.Debug, fmt.Sprintf("adding %s %s to db", x.ContentType, x.UUID))

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

		savedItems = append(savedItems, item)
	}

	if len(savedItems) > 0 {
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

	debugPrint(si.Debug, fmt.Sprintf("Sync | removing dirty flag on %d db items now synced back to SN", len(dirty)))

	// unset dirty flag and date on anything that has now been synced back to SN
	for _, d := range dirty {
		if d.Deleted {
			// deleted items have been removed by now
			continue
		}

		err = db.UpdateField(&Item{UUID: d.UUID}, "Dirty", false)
		if err != nil {
			return
		}

		err = db.UpdateField(&Item{UUID: d.UUID}, "DirtiedDate", time.Time{})
		if err != nil {
			return
		}
	}

	// get the items keys returned by SN
	var eiks []gosn.EncryptedItem

	debugPrint(si.Debug, fmt.Sprintf("Sync | saving %d items from gosn.Sync Items to db", len(gSO.Items)))

	// put new Items in db
	var itemsToDelete Items
	var newItems Items

	for _, i := range gSO.Items {
		// if the item has been deleted in SN, then delete from db
		if i.Deleted {
			debugPrint(si.Debug, fmt.Sprintf("Sync | adding uuid for deletion %s %s and skipping addition to db", i.ContentType, i.UUID))

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
		if err = SaveCacheItems(db, newItems, false); err != nil {
			return
		}
	}

	if len(itemsToDelete) > 0 {
		if err = DeleteCacheItems(db, itemsToDelete, false); err != nil {
			return
		}
	}

	if err = processCachedItemsKeys(si.Session, eiks); err != nil {
		return
	}

	debugPrint(si.Debug, "Sync | retrieving all items from db in preparation for decryption")

	err = db.All(&all)

	debugPrint(si.Debug, fmt.Sprintf("Sync | retrieved %d items from db in preparation for decryption", len(all)))

	debugPrint(si.Debug, fmt.Sprintf("Sync | replacing db SyncToken with latest from gosn.Sync: %s", gSO.SyncToken))

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
		debugPrint(si.Session.Debug, fmt.Sprintf("Sync | saving sync token %s to db", sv.SyncToken))
		err = fmt.Errorf("saving token to db %w", err)

		return
	}

	err = db.All(&all)

	so.DB = db

	return
}

func processCachedItemsKeys(s *Session, eiks gosn.EncryptedItems) error {
	// merge the items keys returned by SN with the items keys in the session
	debugPrint(s.Debug, fmt.Sprintf("Sync | decrypting and parsing %d ItemsKeys", len(eiks)))

	iks, err := gosn.DecryptAndParseItemKeys(s.MasterKey, eiks)
	if err != nil {
		return err
	}

	debugPrint(s.Debug, fmt.Sprintf("Sync | merging %d new ItemsKeys with existing stored in session", len(eiks)))
	debugPrint(s.Debug, fmt.Sprintf("Sync | pre-merge total of %d ItemsKeys in session", len(s.Session.ItemsKeys)))

	s.Session.ItemsKeys = mergeItemsKeysSlices(s.Session.ItemsKeys, iks)
	// set default items key to most recent
	var latestItemsKey gosn.ItemsKey
	for x := range s.Session.ItemsKeys {
		if s.Session.ItemsKeys[x].CreatedAtTimestamp > latestItemsKey.CreatedAtTimestamp {
			latestItemsKey = s.Session.ItemsKeys[x]
		}
	}

	debugPrint(s.Debug, fmt.Sprintf("Sync | setting Default Items Key to %s", latestItemsKey.UUID))
	s.Session.DefaultItemsKey = latestItemsKey

	debugPrint(s.Debug, fmt.Sprintf("Sync | post-merge total of %d ItemsKeys in session", len(s.Session.ItemsKeys)))

	return err
}

func mergeItemsKeysSlices(sessionList, another []gosn.ItemsKey) (out []gosn.ItemsKey) {
	var holdingList []gosn.ItemsKey

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

func debugPrint(show bool, msg string) {
	if show {
		if len(msg) > maxDebugChars {
			msg = msg[:maxDebugChars] + "..."
		}

		log.Println(libName, "|", msg)
	}
}
