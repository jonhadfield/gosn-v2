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
			debugPrint(s.Debug, fmt.Sprintf("ToItems | ignoring invalid ItemsKey: %s", *ei.ItemsKeyID))
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
	so, err := Sync(SyncInput{
		Session: s,
		Close:   false,
	})
	if err != nil {
		return err
	}

	var allPersistedItems Items
	err = so.DB.All(&allPersistedItems)
	if err != nil {
		return err
	}

	err = so.DB.Close()
	if err != nil {
		return err
	}

	items, err := allPersistedItems.ToItems(s)
	if err != nil {
		return err
	}

	debugPrint(s.Debug, fmt.Sprintf("Exporting to path: %s", path))

	err = items.Export(s.Session, path, false)
	if err != nil {
		return err
	}

	return nil
}

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
	syncToken := ""
	if len(syncTokens) > 0 {
		syncToken = syncTokens[0].SyncToken
	}

	_, _, err = s.Session.Import(path, true, syncToken)
	if err != nil {
		return err
	}

	err = so.DB.Close()
	if err != nil {
		return err
	}

	return nil
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
	for i := range items {
		if err := db.Save(&items[i]); err != nil {
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

// TODO: do I need to return SyncOutput with DB as SyncInput has a session with DB pointer already?
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

	var all []Item
	err = db.All(&all)

	// look for dirty items to push to SN with the gosn sync
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

	var syncToken string

	if len(syncTokens) > 1 {
		err = fmt.Errorf("expected maximum of one sync token but %d returned", len(syncTokens))

		return
	}

	if len(syncTokens) == 1 {
		syncToken = syncTokens[0].SyncToken
	}

	// if session doesn't contain items keys then set remove sync token
	if si.Session.Session.DefaultItemsKey.ItemsKey == "" {
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

	// add all the items keys in the session to SN (dupes will be handled)
	for x := range si.Session.Session.ItemsKeys {
		if si.Session.Session.ItemsKeys[x].Content.ItemsKey == "" {
			//panic(fmt.Sprintf("session items key %s is empty: %+v", si.Session.Session.ItemsKeys[x].UUID, si.Session.Session.ItemsKeys[x]))
			debugPrint(si.Session.Debug, fmt.Sprintf("skipping empty items key: %s", si.Session.Session.ItemsKeys[x].UUID))

			continue
		}

		var ek gosn.EncryptedItem

		ek, err = si.Session.Session.ItemsKeys[x].Encrypt(si.Session.Session)
		if err != nil {
			return
		}

		dirtyItemsToPush = append(dirtyItemsToPush, ek)
	}

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

	gSO, err = gosn.Sync(gSI)

	if err != nil {
		return
	}

	if len(gSO.Conflicts) > 0 {
		panic("conflicts should have been resolved by gosn sync")
	}

	// check items are valid
	// TODO: work out need for this
	// we expect deleted items to be returned to why check?
	// check saved items are valid and remove any from db that are now deleted
	var itemsToDeleteFromDB Items

	for _, x := range gSO.Items {
		// TODO: we chould just do a 'Validate' method on items and find any without encItemKey (that are meant to be encrypted)
		components := strings.Split(x.EncItemKey, ":")
		if len(components) <= 1 {
			debugPrint(si.Debug, fmt.Sprintf("ignoring item %s of type %s", x.UUID, x.ContentType))
			continue
		}
	}

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

			itemsToDeleteFromDB = append(itemsToDeleteFromDB, itemToDeleteFromDB...)
		}

		debugPrint(si.Debug, fmt.Sprintf("adding saved item %s of type %s to db", x.UUID, x.ContentType))

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

		err = db.Save(&item)
		if err != nil {
			return
		}
	}

	// remove items from db
	debugPrint(si.Debug, fmt.Sprintf("removing %d items from db", len(itemsToDeleteFromDB)))
	// TODO: put this in previous loop. why loop twice?
	for y := range itemsToDeleteFromDB {
		err = db.DeleteStruct(&itemsToDeleteFromDB[y])

		if err != nil {
			if err.Error() == "not found" {
				err = nil
			} else {
				return
			}
		}
	}

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

	// put new Items in db
	for _, i := range gSO.Items {
		// if the item has been deleted in SN, then no need to add it to db!!!
		if i.Deleted {
			continue
		}

		if i.ContentType == "SN|ItemsKey" {
			eiks = append(eiks, i)
		}

		item := Item{
			UUID:        i.UUID,
			Content:     i.Content,
			ContentType: i.ContentType,
			ItemsKeyID:  i.ItemsKeyID,
			EncItemKey:  i.EncItemKey,
			//Deleted:          i.Deleted, 		# No need as we exclude deleted items above
			CreatedAt:          i.CreatedAt,
			UpdatedAt:          i.UpdatedAt,
			CreatedAtTimestamp: i.CreatedAtTimestamp,
			UpdatedAtTimestamp: i.UpdatedAtTimestamp,
			DuplicateOf:        i.DuplicateOf,
		}

		err = db.Save(&item)
		if err != nil {
			return
		}

	}

	// merge the items keys returned by SN with the items keys in the session
	iks, err := gosn.DecryptAndParseItemKeys(si.MasterKey, eiks)
	if err != nil {
		return
	}

	si.Session.ItemsKeys = mergeItemsKeysSlices(si.Session.ItemsKeys, iks)

	err = db.All(&all)

	// drop the SyncToken db if it exists
	_ = db.Drop("SyncToken")
	// replace with latest from server
	sv := SyncToken{
		SyncToken: gSO.SyncToken,
	}

	if err = db.Save(&sv); err != nil {
		return
	}

	err = db.All(&all)

	so.DB = db

	return
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
