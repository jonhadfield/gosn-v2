package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"github.com/asdine/storm/v3"
	"github.com/jonhadfield/gosn-v2"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	// LOGGING
	libName       = "gosn-v2 cache" // name of library used in logging
	maxDebugChars = 120    // number of characters to display when logging API response body
)


type Item struct {
	UUID        string `storm:"id,unique"`
	Content     string
	ContentType string `storm:"index"`
	EncItemKey  string
	Deleted     bool `storm:"index"`
	CreatedAt   string
	UpdatedAt   string
	Dirty       bool
	DirtiedDate time.Time
}

type SyncToken struct {
	SyncToken string `storm:"id,unique"`
}

type SyncInput struct {
	Session gosn.Session
	DB      *storm.DB // pointer to an existing DB
	DBPath  string    // path to create new DB
	Debug   bool
}

type SyncOutput struct {
	Items, SavedItems, Unsaved gosn.EncryptedItems
	DB                         *storm.DB // pointer to DB
}

type Items []Item

func (pi Items) ToItems(session gosn.Session) (items gosn.Items, err error) {
	var eItems gosn.EncryptedItems
	for _, ei := range pi {
		eItems = append(eItems, gosn.EncryptedItem{
			UUID:        ei.UUID,
			Content:     ei.Content,
			ContentType: ei.ContentType,
			EncItemKey:  ei.EncItemKey,
			Deleted:     ei.Deleted,
			CreatedAt:   ei.CreatedAt,
			UpdatedAt:   ei.UpdatedAt,
		})
	}
	if eItems != nil {
		items, err = eItems.DecryptAndParse(session.Mk, session.Ak, false)
	}

	return
}

func ConvertItemsToPersistItems(items gosn.EncryptedItems) (pitems Items) {
	for _, i := range items {
		pitems = append(pitems, Item{
			UUID:        i.UUID,
			Content:     i.Content,
			ContentType: i.ContentType,
			EncItemKey:  i.EncItemKey,
			Deleted:     i.Deleted,
			CreatedAt:   i.CreatedAt,
			UpdatedAt:   i.UpdatedAt,
		})
	}

	return
}

func initialiseDB(si SyncInput) (db *storm.DB, err error) {
	// create new DB in provided path
	db, err = storm.Open(si.DBPath)
	if err != nil {
		return
	}

	// call gosn sync to get existing items
	gSI := gosn.SyncInput{
		Session: si.Session,
	}

	var gSO gosn.SyncOutput

	gSO, err = gosn.Sync(gSI)
	if err != nil {
		return
	}

	// put new Items in db
	for _, i := range gSO.Items {
		item := Item{
			UUID:        i.UUID,
			Content:     i.Content,
			ContentType: i.ContentType,
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

	// update sync values in db for next time
	sv := SyncToken{
		SyncToken: gSO.SyncToken,
	}
	if err = db.Save(&sv); err != nil {
		return
	}

	return
}

func Sync(si SyncInput) (so SyncOutput, err error) {
	// check session is valid
	if !si.Session.Valid() {
		err = fmt.Errorf("invalid session")
		return
	}

	// check if a DB or a path to a DB was passed
	if si.DB != nil && si.DBPath != "" {
		err = fmt.Errorf("passing a DB pointer and DB path does not make sense")
		return
	}

	// open existing DB if no path provided
	if si.DBPath != "" {
		debugPrint(si.Debug, fmt.Sprintf("Sync | using db in '%s'", si.DBPath))
		si.DB, err = storm.Open(si.DBPath)
		if err != nil {
			return
		}
	}

	// if DB isn't passed, create a new one to update and return
	if si.DB == nil {
		if si.DBPath == "" {
			err = fmt.Errorf("DB pointer or DB path are required")
			return
		}
		var db *storm.DB
		db, err = initialiseDB(si)
		return SyncOutput{
			DB: db,
		}, err
	}

	// look for dirty items to save with the gosn sync
	var dirty []Item
	err = si.DB.Find("Dirty", true, &dirty)
	if err != nil {
		if !strings.Contains(err.Error(), "not found") {
			return
		}
	}

	// get sync token from previous operation
	var syncTokens []SyncToken
	err = si.DB.All(&syncTokens)
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

	// convert dirty to gosn.Items
	var dirtyItemsToPush gosn.EncryptedItems
	for _, d := range dirty {
		dirtyItemsToPush = append(dirtyItemsToPush, gosn.EncryptedItem{
			UUID:        d.UUID,
			Content:     d.Content,
			ContentType: d.ContentType,
			EncItemKey:  d.EncItemKey,
			Deleted:     d.Deleted,
			CreatedAt:   d.CreatedAt,
			UpdatedAt:   d.UpdatedAt,
		})
	}
	// call gosn sync with dirty items to push
	var gSI gosn.SyncInput
	if len(dirtyItemsToPush) > 0 {
		gSI = gosn.SyncInput{
			Session:   si.Session,
			Items:     dirtyItemsToPush,
			SyncToken: syncToken,
		}
	} else {
		gSI = gosn.SyncInput{
			Session:   si.Session,
			SyncToken: syncToken,
		}
	}

	var gSO gosn.SyncOutput

	gSO, err = gosn.Sync(gSI)
	if err != nil {

		return
	}

	for _, d := range dirty {
		err = si.DB.UpdateField(&Item{UUID: d.UUID}, "Dirty", false)
		if err != nil {

			return
		}
		err = si.DB.UpdateField(&Item{UUID: d.UUID}, "DirtiedDate", time.Time{})
		if err != nil {

			return
		}
	}
	so.Items = gSO.Items
	so.SavedItems = gSO.SavedItems
	so.Unsaved = gSO.Unsaved
	so.DB = si.DB

	// put new Items in db
	for _, i := range gSO.Items {
		item := Item{
			UUID:        i.UUID,
			Content:     i.Content,
			ContentType: i.ContentType,
			EncItemKey:  i.EncItemKey,
			Deleted:     i.Deleted,
			CreatedAt:   i.CreatedAt,
			UpdatedAt:   i.UpdatedAt,
		}
		err = si.DB.Save(&item)
		if err != nil {
			return
		}
	}

	// drop the SyncToken db if it exists
	_ = si.DB.Drop("SyncToken")

	sv := SyncToken{
		SyncToken: gSO.SyncToken,
	}
	if err = si.DB.Save(&sv); err != nil {

		return
	}

	return
}

// GenCacheDBPath generates a path to a database file to be used as a cache of encrypted items
// The filename is a SHA2 hash of a concatenation of the following in order to be both unique
// and avoid concurrent usage:
// - part of the session authentication key (so that caches are unique to a user)
// - the server URL (so that caches are server specific)
// - the requesting application name (so that caches are application specific)
func GenCacheDBPath(session gosn.Session, dir, appName string) (string, error) {
	var err error
	if !session.Valid() {
		return "", fmt.Errorf("invalid session")
	}

	if appName == "" {
		return "", fmt.Errorf("appName is a required")
	}

	if dir == "" {
		dir = os.TempDir()
	} else if _, err := os.Stat(dir); os.IsNotExist(err) {
		return "", fmt.Errorf("directory does not exist: '%s'", dir)
	}

	h := sha256.New()

	h.Write([]byte(session.Ak[:2] + session.Ak[len(session.Ak)-2:] + session.Server + appName))
	bs := h.Sum(nil)
	hexedDigest := hex.EncodeToString(bs)[:8]

	if dir == "" {
		dir = os.TempDir()
	}

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
