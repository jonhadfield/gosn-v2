package cache

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/asdine/storm/v3"
	gosn "github.com/jonhadfield/gosn-v2"
	"github.com/stretchr/testify/require"
)

var testSession *Session

func testSetup() {
	gs, err := gosn.CliSignIn(os.Getenv("SN_EMAIL"), os.Getenv("SN_PASSWORD"), os.Getenv("SN_SERVER"), true)
	if err != nil {
		panic(err)
	}

	testSession, err = ImportSession(&gs, "")
	if err != nil {
		return
	}

	var path string

	path, err = GenCacheDBPath(*testSession, "", gosn.LibName)
	if err != nil {
		panic(err)
	}

	testSession.CacheDBPath = path

	_, err = Sync(SyncInput{
		Session: testSession,
		Close:   true,
	})
	if err != nil {
		panic(err)
	}

	if testSession.Session.DefaultItemsKey.ItemsKey == "" {
		panic("failed in TestMain due to empty default items key")
	}
}

func TestMain(m *testing.M) {
	testSetup()
	os.Exit(m.Run())
}

// Create a note in cache and sync to SN
// Export, then import, and sync back.
func TestSyncThenExportImport(t *testing.T) {
	defer cleanup(testSession.Session)

	var so SyncOutput
	so, err := Sync(SyncInput{
		Session: testSession,
	})
	require.NoError(t, err)

	// create new note with random content
	newNote, _ := createNote("test", "")
	dItems := gosn.Items{&newNote}
	require.NoError(t, dItems.Validate())

	eItems, err := dItems.Encrypt(testSession.DefaultItemsKey, testSession.MasterKey, testSession.Debug)
	require.NoError(t, err)

	var allPersistedItems []Item

	// items convert new items to 'persist' items and mark as dirty
	itp := ToCacheItems(eItems, false)
	for _, i := range itp {
		require.NoError(t, so.DB.Save(&i))
		allPersistedItems = append(allPersistedItems, i)
	}

	require.Len(t, allPersistedItems, 1)
	require.NoError(t, so.DB.Close())

	so, err = Sync(SyncInput{
		Session: testSession,
	})
	require.NoError(t, err)
	require.NotNil(t, so)
	require.NotNil(t, so.DB)
	require.NoError(t, so.DB.All(&allPersistedItems))
	require.NoError(t, so.DB.Close())

	dir, err := ioutil.TempDir("", "test")
	require.NoError(t, err)

	tmpfn := filepath.Join(dir, "tmpfile")
	err = testSession.Export(tmpfn)
	require.NoError(t, err)

	err = testSession.Import(tmpfn, true)
	require.NoError(t, err)
}

// Create a note in cache and sync to SN
// Export, then import, and sync back.
func TestSyncThenExportImportCompare(t *testing.T) {
	defer cleanup(testSession.Session)
	testSession.CacheDB.Close()

	var so SyncOutput
	so, err := Sync(SyncInput{
		Session: testSession,
	})
	require.NoError(t, err)

	// create new note with random content
	newNote, _ := createNote("test", "ok")
	dItems := gosn.Items{&newNote}
	require.NoError(t, dItems.Validate())

	// encrypt note
	eItems, err := dItems.Encrypt(testSession.DefaultItemsKey, testSession.MasterKey, testSession.Debug)
	require.NoError(t, err)
	require.Len(t, eItems, 1)

	// items convert new items to 'persist' items and mark as dirty
	itp := ToCacheItems(eItems, false)
	require.Len(t, itp, 1)

	var allPersistedItems []Item

	for _, i := range itp {
		require.NoError(t, so.DB.Save(&i))
		allPersistedItems = append(allPersistedItems, i)
	}

	require.Len(t, allPersistedItems, 1)
	require.NoError(t, so.DB.Close())

	so, err = Sync(SyncInput{
		Session: testSession,
		Close:   false,
	})

	require.NoError(t, err)
	require.NotNil(t, so)
	require.NotNil(t, so.DB)
	require.NoError(t, so.DB.All(&allPersistedItems))

	dir, err := ioutil.TempDir("", "test")
	require.NoError(t, err)
	require.NoError(t, so.DB.Close())

	tmpfn := filepath.Join(dir, "tmpfile")
	require.NoError(t, err)

	err = testSession.Export(tmpfn)
	require.NoError(t, err)

	err = testSession.Import(tmpfn, true)
	require.NoError(t, err)
	require.NoError(t, testSession.CacheDB.Close())

	so, err = Sync(SyncInput{
		Session: testSession,
		Close:   false,
	})
	require.NoError(t, err)

	gso, err := gosn.Sync(gosn.SyncInput{
		Session: testSession.Session,
		// SyncToken: "",
	})
	require.NoError(t, err)

	var api Items

	require.NoError(t, so.DB.All(&api))
	require.GreaterOrEqual(t, len(api), 1)
	require.NoError(t, so.DB.Close())

	var dbItemsKeyCount int

	// check items in db are in sync with SN
	for x := range api {
		if api[x].ContentType == "SN|ItemsKey" {
			dbItemsKeyCount++
		}

		var gsoMatch gosn.EncryptedItem

		var dbMatch Item

		var found bool

		for y := range gso.Items {
			if api[x].UUID == gso.Items[y].UUID {
				found = true
				gsoMatch = gso.Items[y]
				dbMatch = api[x]

				break
			}
		}

		require.True(t, found)

		switch {
		case gsoMatch.ItemsKeyID != nil && dbMatch.ItemsKeyID != nil && *gsoMatch.ItemsKeyID != *dbMatch.ItemsKeyID:
			fmt.Printf("Mismatched ItemsKeyID for uuid %s type %s. db has %s and SN has %s\n", gsoMatch.UUID, gsoMatch.ContentType, *dbMatch.ItemsKeyID, *gsoMatch.ItemsKeyID)
		}
	}

	var snItemsKeyCount int

	for x := range gso.Items {
		if gso.Items[x].ContentType == "SN|ItemsKey" {
			snItemsKeyCount++
		}

		var gsoMatch gosn.EncryptedItem

		var dbMatch Item

		var found bool

		for y := range allPersistedItems {
			if gso.Items[x].UUID == allPersistedItems[y].UUID {
				found = true
				gsoMatch = gso.Items[x]
				dbMatch = allPersistedItems[y]

				break
			}
		}

		if !found && gso.Items[x].ContentType != "SN|ItemsKey" {
			panic(fmt.Sprintf("item %+v not found in DB", gso.Items[x]))
		}

		switch {
		case gsoMatch.ItemsKeyID != nil && dbMatch.ItemsKeyID != nil && *gsoMatch.ItemsKeyID != *dbMatch.ItemsKeyID:
			fmt.Printf("Mismatched ItemsKeyID for uuid %s type %s. db has %s and SN has %s\n", gsoMatch.UUID, gsoMatch.ContentType, *dbMatch.ItemsKeyID, *gsoMatch.ItemsKeyID)
		}
	}

	if snItemsKeyCount != dbItemsKeyCount {
		fmt.Printf("SN ItemsKey count: %d DB ItemsKey count: %d\n", snItemsKeyCount, dbItemsKeyCount)
	}
}

func TestSyncWithoutDatabase(t *testing.T) {
	sio, err := gosn.SignIn(sInput)
	require.NoError(t, err, "sign-in failed", err)

	session, err := ImportSession(&sio.Session, "")
	if err != nil {
		return
	}

	session.CacheDBPath = ""
	_, err = Sync(SyncInput{Session: session})
	require.EqualError(t, err, "database path is required")
}

func TestSyncWithInvalidSession(t *testing.T) {
	defer removeDB(tempDBPath)

	_, err := Sync(SyncInput{})
	require.EqualError(t, err, "invalid session")

	var invalidSession Session

	_, err = Sync(SyncInput{Session: &invalidSession})

	require.EqualError(t, err, "invalid session")
}

func TestInitialSyncWithItemButNoDB(t *testing.T) {
	defer cleanup(testSession.Session)

	defer removeDB(testSession.CacheDBPath)

	sio, err := gosn.SignIn(sInput)
	require.NoError(t, err)

	session, err := ImportSession(&sio.Session, tempDBPath)
	if err != nil {
		return
	}

	session.CacheDBPath = tempDBPath

	require.NoError(t, err, "sign-in failed", err)

	var so SyncOutput

	so, err = Sync(SyncInput{
		Session: session,
	})
	require.NoError(t, err)

	var syncTokens []SyncToken
	err = so.DB.All(&syncTokens)
	require.NoError(t, err)
	require.Len(t, syncTokens, 1)
	require.NotEmpty(t, syncTokens[0]) // tells us what time to sync from next time
	require.NoError(t, so.DB.Close())
}

func TestSyncWithNoItems(t *testing.T) {
	removeDB(testSession.CacheDBPath)
	defer cleanup(testSession.Session)

	defer removeDB(testSession.CacheDBPath)

	so, err := Sync(SyncInput{
		Session: testSession,
	})
	require.NoError(t, err)
	require.NotNil(t, so)
	require.NotNil(t, so.DB)

	var syncTokens []SyncToken
	err = so.DB.All(&syncTokens)
	require.NoError(t, err)
	require.Len(t, syncTokens, 1)
	require.NotEmpty(t, syncTokens[0]) // tells us what time to sync from next time
	require.NoError(t, so.DB.Close())
}

// NewCacheDB creates a storm database in the provided path and returns a pointer to the opened database.
func NewCacheDB(path string) (db *storm.DB, err error) {
	// TODO: check path syntax is file (not ending in file sep.)
	// TODO: check file doesn't exist
	db, err = storm.Open(path)
	if err != nil {
		return
	}

	return
}

func GetCacheDB(path string) (db *storm.DB, err error) {
	// TODO: check exists
	db, err = storm.Open(tempDBPath)
	if err != nil {
		return
	}

	return
}

// create a note in a storm DB, mark it dirty, and then sync to SN
// the returned DB should have the note returned as not dirty
// SN should now have that note.
func TestSyncWithNewNote(t *testing.T) {
	defer cleanup(testSession.Session)

	// create new note with random content
	newNote, _ := createNote("test", "")
	dItems := gosn.Items{&newNote}
	require.NoError(t, dItems.Validate())

	eItems, err := dItems.Encrypt(testSession.DefaultItemsKey, testSession.MasterKey, testSession.Debug)
	require.NoError(t, err)

	var so SyncOutput
	so, err = Sync(SyncInput{
		Session: testSession,
	})
	require.NoError(t, err)

	var allPersistedItems []Item
	// items convert new items to 'persist' items and mark as dirty
	itp := ToCacheItems(eItems, false)
	for _, i := range itp {
		require.NoError(t, so.DB.Save(&i))
		allPersistedItems = append(allPersistedItems, i)
	}

	require.Len(t, allPersistedItems, 1)
	require.NoError(t, so.DB.Close())

	so, err = Sync(SyncInput{
		Session: testSession,
	})
	require.NoError(t, err)
	require.NotNil(t, so)
	require.NotNil(t, so.DB)
	require.NoError(t, so.DB.All(&allPersistedItems))

	var foundNonDirtyNote bool

	for _, i := range allPersistedItems {
		if i.UUID == newNote.UUID {
			foundNonDirtyNote = true

			require.False(t, i.Dirty)
			require.Zero(t, i.DirtiedDate)
		}
	}

	require.True(t, foundNonDirtyNote)

	// check the item exists in SN

	// get sync token from previous operation
	var syncTokens []SyncToken

	require.NoError(t, so.DB.All(&syncTokens))
	require.Len(t, syncTokens, 1)
	require.NoError(t, so.DB.Close())
}

func TestSyncWithNewNoteExportReauthenticateImport(t *testing.T) {
	defer cleanup(testSession.Session)

	// create new note with random content
	newNote, _ := createNote("test", "")
	dItems := gosn.Items{&newNote}
	require.NoError(t, dItems.Validate())

	eItems, err := dItems.Encrypt(testSession.DefaultItemsKey, testSession.MasterKey, testSession.Debug)
	require.NoError(t, err)

	var so SyncOutput
	so, err = Sync(SyncInput{
		Session: testSession,
	})
	require.NoError(t, err)

	var allPersistedItems []Item
	// items convert new items to 'persist' items and mark as dirty
	itp := ToCacheItems(eItems, false)
	for _, i := range itp {
		require.NoError(t, so.DB.Save(&i))
		allPersistedItems = append(allPersistedItems, i)
	}

	require.Len(t, allPersistedItems, 1)
	require.NoError(t, so.DB.Close())

	so, err = Sync(SyncInput{
		Session: testSession,
	})
	require.NoError(t, err)
	require.NotNil(t, so)
	require.NotNil(t, so.DB)
	require.NoError(t, so.DB.All(&allPersistedItems))

	var foundNonDirtyNote bool

	for _, i := range allPersistedItems {
		if i.UUID == newNote.UUID {
			foundNonDirtyNote = true

			require.False(t, i.Dirty)
			require.Zero(t, i.DirtiedDate)
		}
	}

	require.True(t, foundNonDirtyNote)

	// check the item exists in SN

	// get sync token from previous operation
	var syncTokens []SyncToken

	require.NoError(t, so.DB.All(&syncTokens))
	require.Len(t, syncTokens, 1)
	require.NoError(t, so.DB.Close())
}

// create a note in SN directly
// call persist Sync and check DB contains the note.
func TestSyncOneExisting(t *testing.T) {
	cleanup(testSession.Session)
	defer cleanup(testSession.Session)

	// create new note with random content and push to SN (not DB)
	newNote, _ := createNote("test", "")
	dItems := gosn.Items{&newNote}
	require.NoError(t, dItems.Validate())

	eItems, err := dItems.Encrypt(testSession.DefaultItemsKey, testSession.MasterKey, testSession.Debug)

	require.NoError(t, err)
	require.NoError(t, eItems.Validate())

	// push to SN
	var gso gosn.SyncOutput
	gso, err = gosn.Sync(gosn.SyncInput{
		Session: testSession.Session,
		Items:   eItems,
	})

	require.NoError(t, err)
	require.Len(t, gso.SavedItems, 1)
	// call sync
	var so SyncOutput

	so, err = Sync(SyncInput{
		Session: testSession,
	})
	require.NoError(t, err)

	defer so.DB.Close()
	defer removeDB(tempDBPath)

	// get all items
	var allPersistedItems []Item

	err = so.DB.All(&allPersistedItems)
	require.NoError(t, err)

	var syncTokens []SyncToken
	err = so.DB.All(&syncTokens)
	require.NoError(t, err)
	require.NotEmpty(t, syncTokens)

	err = so.DB.All(&allPersistedItems)

	require.Greater(t, len(allPersistedItems), 0)

	var foundNotes int

	for _, pi := range allPersistedItems {
		if pi.ContentType == "Note" && pi.UUID == newNote.UUID {
			foundNotes++
		}
	}

	require.Equal(t, 1, foundNotes)
}

// create a note in SN directly
// call persist Sync and check DB contains the note.
func TestSyncRetainsSyncToken(t *testing.T) {
	cleanup(testSession.Session)
	defer cleanup(testSession.Session)

	so, err := Sync(SyncInput{
		Session: testSession,
	})
	require.NoError(t, err)

	var syncTokens []SyncToken
	err = so.DB.All(&syncTokens)
	require.NoError(t, err)
	require.Len(t, syncTokens, 1)
	require.NoError(t, so.DB.Close())

	so, err = Sync(SyncInput{
		Session: testSession,
	})
	require.NoError(t, err)

	err = so.DB.All(&syncTokens)
	defer so.DB.Close()
	defer removeDB(tempDBPath)
	require.NoError(t, err)
	require.Len(t, syncTokens, 1)

	//
	//// get all items
	//var allPersistedItems []Item
	//
	//err = so.DB.All(&allPersistedItems)
	//require.NoError(t, err)
	//
	//var syncTokens []SyncToken
	//err = so.DB.All(&syncTokens)
	//require.NoError(t, err)
	//require.NotEmpty(t, syncTokens)
	//
	//err = so.DB.All(&allPersistedItems)
	//
	//require.Greater(t, len(allPersistedItems), 0)
	//
	//var foundNotes int
	//
	//for _, pi := range allPersistedItems {
	//	if pi.ContentType == "Note" && pi.UUID == newNote.UUID {
	//		foundNotes++
	//	}
	//}
	//
	//require.Equal(t, 1, foundNotes)
}

// create a note in SN directly
// call persist, Sync, and check DB contains the note
// update new note, sync to SN, then check new content is persisted.
func TestSyncUpdateExisting(t *testing.T) {
	defer cleanup(testSession.Session)

	// create new note with random content and push to SN (not DB)
	newNote, _ := createNote("test title", "some content")
	dItems := gosn.Items{&newNote}
	require.NoError(t, dItems.Validate())

	eItems, err := dItems.Encrypt(testSession.Session.DefaultItemsKey, testSession.Session.MasterKey, testSession.Session.Debug)
	require.NoError(t, err)

	// push new note to SN
	var gso gosn.SyncOutput
	gso, err = gosn.Sync(gosn.SyncInput{
		Session: testSession.Session,
		Items:   eItems,
	})

	require.NoError(t, err)
	require.Len(t, gso.SavedItems, 1)

	// USE CACHE SYNC TO LOAD NEW NOTE INTO CACHE

	cso, err := Sync(SyncInput{
		Session: testSession,
		Close:   false,
	})

	require.NoError(t, err)

	// defer so.DB.Close()
	defer removeDB(tempDBPath)

	// get all items
	var allPersistedItems Items

	// get sync token from previous operation
	var syncTokens []SyncToken
	err = cso.DB.All(&syncTokens)
	require.NoError(t, err)
	require.NotEmpty(t, syncTokens)

	err = cso.DB.All(&allPersistedItems)
	require.NoError(t, err)
	require.Greater(t, len(allPersistedItems), 0)

	var items gosn.Items
	items, err = allPersistedItems.ToItems(testSession)
	require.NoError(t, err)
	require.NotEmpty(t, items)

	var newNoteFound bool

	for x := range items {
		if items[x].GetContentType() == "Note" && items[x].GetUUID() == newNote.UUID {
			newNote = *items[x].(*gosn.Note)
			if newNote.Content.Title == "test title" && newNote.Content.Text == "some content" {
				newNoteFound = true
			}
		}
	}

	require.True(t, newNoteFound)
	newNote.Content.SetTitle("Modified Title")
	require.Equal(t, newNote.Content.Title, "Modified Title")
	newNote.Content.SetText("Modified Text")
	// newNote.Content.SetUpdateTime(time.Now())
	require.Equal(t, newNote.Content.Text, "Modified Text")
	// check note content was saved
	time.Sleep(1 * time.Second)

	// items convert new items to 'persist' items and mark as dirty
	var ditems gosn.Items
	ditems = append(ditems, &newNote)
	eItems, err = dItems.Encrypt(testSession.Session.DefaultItemsKey, testSession.Session.MasterKey, testSession.Session.Debug)
	require.NoError(t, err)
	require.Len(t, eItems, 1)

	itp := ToCacheItems(eItems, false)
	for x := range itp {
		require.NoError(t, cso.DB.Save(&itp[x]))
	}

	require.True(t, checkItemInDBTemp(cso.DB, newNote, "Modified Title", "Modified Text"))

	require.NoError(t, cso.DB.Close())

	cso, err = Sync(SyncInput{
		Session: testSession,
	})

	require.NoError(t, err)
	require.True(t, checkItemInDBTemp(cso.DB, newNote, "Modified Title", "Modified Text"))

	// CHECK ITEM IS NOW IN DB
	err = cso.DB.All(&allPersistedItems)
	require.NoError(t, err)
	// require.Equal(t, len(allPersistedItems), 1)
	items, err = allPersistedItems.ToItems(testSession)
	require.NoError(t, err)
	require.NotEmpty(t, items)

	newNoteFound = false

	for x := range items {
		if items[x].GetContentType() == "Note" && items[x].GetUUID() == newNote.UUID {
			newNote1 := *items[x].(*gosn.Note)

			if newNote1.Content.Title == "Modified Title" && newNote1.Content.Text == "Modified Text" {
				newNoteFound = true
			}
		}
	}

	require.True(t, newNoteFound)

	// now do a gosn sync and check item has updates
	var newSO gosn.SyncOutput
	newSO, err = gosn.Sync(gosn.SyncInput{
		Session: testSession.Session,
	})
	require.NoError(t, err)

	var uItem gosn.EncryptedItem

	for _, i := range newSO.Items {
		if i.UUID == newNote.UUID && i.Deleted == false {
			uItem = i
		}
	}

	require.NotEmpty(t, uItem.UUID)
	require.Equal(t, newNote.UUID, uItem.UUID)
	// require.Equal(t, newNote.UpdatedAt, uItem.UpdatedAt)

	newEncItems := gosn.EncryptedItems{uItem}

	var newDecItems gosn.Items
	newDecItems, err = newEncItems.DecryptAndParse(testSession.Session)
	require.NoError(t, err)
	require.NotEmpty(t, newDecItems)
	uNote := *newDecItems[0].(*gosn.Note)
	require.Equal(t, "Modified Title", uNote.Content.GetTitle())
	require.Equal(t, "Modified Text", uNote.Content.GetText())
}

func checkItemInDBTemp(db *storm.DB, inNote gosn.Note, title, text string) (found bool) {
	var allPersistedItems Items

	err := db.All(&allPersistedItems)
	if err != nil {
		panic(fmt.Sprintf("couldn't get items from db: %+v", err))
	}

	// require.Equal(t, len(allPersistedItems), 1)
	items, err := allPersistedItems.ToItems(testSession)
	if err != nil {
		panic("couldn't turn cache items to gosn items")
	}

	for x := range items {
		if items[x].GetContentType() == "Note" && items[x].GetUUID() == inNote.UUID {
			// panic("found new note in cache")
			foundNote := *items[x].(*gosn.Note)

			if foundNote.Content.Title == title && foundNote.Content.Text == text {
				return true
			}
		}
	}

	return false
}

func _deleteAllTagsNotesComponents(session *gosn.Session) (err error) {
	gnf := gosn.Filter{
		Type: "Note",
	}
	gtf := gosn.Filter{
		Type: "Tag",
	}
	gcf := gosn.Filter{
		Type: "SN|Component",
	}
	f := gosn.ItemFilters{
		Filters:  []gosn.Filter{gnf, gtf, gcf},
		MatchAny: true,
	}
	si := gosn.SyncInput{
		Session: session,
	}

	var so gosn.SyncOutput

	so, err = gosn.Sync(si)
	if err != nil {
		return
	}

	var items gosn.Items

	items, err = so.Items.DecryptAndParse(session)
	if err != nil {
		return
	}

	items.Filter(f)

	var toDel gosn.Items

	for x := range items {
		md := items[x]
		switch md.GetContentType() {
		case "Note":
			md.SetContent(*gosn.NewNoteContent())
		case "Tag":
			md.SetContent(*gosn.NewTagContent())
		case "SN|Component":
			md.SetContent(*gosn.NewComponentContent())
		}

		md.SetDeleted(true)
		toDel = append(toDel, md)
	}

	if len(toDel) > 0 {
		eToDel, err := toDel.Encrypt(session.DefaultItemsKey, session.MasterKey, session.Debug)
		si = gosn.SyncInput{
			Session: session,
			Items:   eToDel,
		}

		_, err = gosn.Sync(si)
		if err != nil {
			return fmt.Errorf("PutItems Failed: %v", err)
		}
	}

	return err
}

func createNote(title, content string) (note gosn.Note, text string) {
	text = content
	if content == "" {
		text = testParas[randInt(0, len(testParas))]
	}

	newNoteContent := gosn.NoteContent{
		Title:          title,
		Text:           text,
		ItemReferences: nil,
	}

	newNoteContent.SetUpdateTime(time.Now())

	newNote := gosn.NewNote()
	newNote.Content = newNoteContent

	return newNote, text
}

func cleanup(session *gosn.Session) {
	if err := _deleteAllTagsNotesComponents(session); err != nil {
		panic(err)
	}

	removeDB(testSession.CacheDBPath)
}

var (
	sInput = gosn.SignInInput{
		Email:     os.Getenv("SN_EMAIL"),
		Password:  os.Getenv("SN_PASSWORD"),
		APIServer: os.Getenv("SN_SERVER"),
		Debug:     true,
	}
	testParas = []string{
		"Lorem ipsum dolor sit amet, consectetur adipiscing elit. Ut venenatis est sit amet lectus aliquam, ac rutrum nibh vulputate. Etiam vel nulla dapibus, lacinia neque et, porttitor elit. Nulla scelerisque elit sem, ac posuere est gravida dignissim. Fusce laoreet, enim gravida vehicula aliquam, tellus sem iaculis lorem, rutrum congue ex lectus ut quam. Cras sollicitudin turpis magna, et tempor elit dignissim eu. Etiam sed auctor leo. Sed semper consectetur purus, nec vehicula tellus tristique ac. Cras a quam et magna posuere varius vitae posuere sapien. Morbi tincidunt tellus eu metus laoreet, quis pulvinar sapien consectetur. Fusce nec viverra lectus, sit amet ullamcorper elit. Vestibulum vestibulum augue sem, vitae egestas ipsum fringilla sit amet. Nulla eget ante sit amet velit placerat gravida.",
		"Duis odio tortor, consequat egestas neque dictum, porttitor laoreet felis. Sed sed odio non orci dignissim vulputate. Praesent a scelerisque lectus. Phasellus sit amet vestibulum felis. Integer blandit, nulla eget tempor vestibulum, nisl dolor interdum eros, sed feugiat est augue sit amet eros. Suspendisse maximus tortor sodales dolor sagittis, vitae mattis est cursus. Etiam lobortis nunc non mi posuere, vel elementum massa congue. Aenean ut lectus vitae nisl scelerisque semper.",
		"Quisque pellentesque mauris ut tortor ultrices, eget posuere metus rhoncus. Aenean maximus ultricies mauris vel facilisis. Pellentesque habitant morbi tristique senectus et netus et malesuada fames ac turpis egestas. Curabitur hendrerit, ligula a sagittis condimentum, metus nibh sodales elit, sed rhoncus felis ipsum sit amet sem. Phasellus nec massa non felis suscipit dictum. Aenean dictum iaculis metus quis aliquam. Aenean suscipit mi vel nisi finibus rhoncus. Donec eleifend, massa in convallis mattis, justo eros euismod dui, sollicitudin imperdiet nibh lacus sit amet diam. Praesent eu mollis ligula. In quis nisi egestas, scelerisque ante vitae, dignissim nisi. Curabitur vel est eget purus porta malesuada.",
		"Duis tincidunt eros ligula, et tincidunt lacus scelerisque ac. Cras aliquam ultrices egestas. Orci varius natoque penatibus et magnis dis parturient montes, nascetur ridiculous mus. Nunc sapien est, imperdiet in cursus id, suscipit ac orci. Integer magna augue, accumsan quis massa rutrum, dictum posuere odio. Vivamus vitae efficitur enim. Donec posuere sapien sit amet turpis lacinia rutrum. Nulla porttitor lacinia justo quis consequat.",
		"Quisque blandit ultricies nisi eu dignissim. Mauris venenatis enim et posuere ornare. Phasellus facilisis libero ut elit consequat scelerisque. Vivamus facilisis, nibh eget hendrerit malesuada, velit tellus vehicula justo, id ultrices justo orci nec dui. Sed hendrerit fermentum pulvinar. Aenean at magna gravida, finibus ligula non, cursus felis. Quisque consectetur malesuada magna ut cursus. Nam aliquet felis aliquet lobortis pulvinar. Fusce vel ipsum felis. Maecenas sapien magna, feugiat vitae tristique sit amet, vehicula ac quam. Donec a consectetur lorem, id euismod augue. Suspendisse metus ipsum, bibendum efficitur tortor vitae, molestie suscipit nulla. Proin vel felis eget libero auctor pulvinar eget ac diam. Vivamus malesuada elementum lobortis. Mauris nibh enim, pharetra eu elit vel, sagittis pulvinar ante. Ut efficitur nunc at odio elementum, sed pretium ante porttitor.",
		"Nulla convallis a lectus quis efficitur. Aenean quis vestibulum enim. Nunc in mattis tortor. Nullam sit amet placerat ipsum. Aene an sagittis, elit non bibendum posuere, sapien libero eleifend nisl, quis iaculis urna tortor ut nibh. Fusce fringilla elit in pellentesque laoreet. Duis ornare semper sagittis. Curabitur efficitur quam ac erat venenatis bibendum. Curabitur et luctus nunc, eu euismod augue. Mauris magna enim, vulputate eget gravida a, vestibulum non massa. Pellentesque eget pulvinar nisl.",
		"Pellentesque habitant morbi tristique senectus et netus et malesuada fames ac turpis egestas. Proin a venenatis felis, a posuere augue. Cras ultrices libero in lorem congue ultrices. Integer porttitor, urna non vehicula maximus, est tellus volutpat erat, id volutpat eros erat sit amet mi. Quisque faucibus maximus risus, in placerat mauris venenatis vitae. Ut placerat, risus eu suscipit cursus, velit magna rhoncus dui, eu condimentum mauris nisi in ligula. Interdum et malesuada fames ac ante ipsum primis in faucibus. Aliquam sed dictum lectus. Quisque malesuada sapien mattis, consectetur augue non, sodales arcu. Vivamus imperdiet leo et lacus bibendum, eu venenatis odio auctor. Donec vitae massa vitae nisi tristique faucibus. Curabitur nec pretium ex. Quisque at sapien ornare, mollis nulla eget, tristique ex.",
		"Fusce faucibus id nulla et ornare. Nunc a diam urna. Ut tortor urna, fringilla eu pellentesque in, consectetur vel neque. Suspendisse at eros nisi. Phasellus dui libero, maximus ut orci sit amet, accumsan semper velit. Aenean quis interdum dolor. Sed molestie urna vitae turpis lacinia commodo. Fusce ipsum massa, blandit et nunc at, vestibulum tincidunt orci. Donec venenatis lorem sed urna sodales maximus. Class aptent taciti sociosqu ad litora torquent per conubia nostra, per inceptos himenaeos. Class aptent taciti sociosqu ad litora torquent per conubia nostra, per inceptos himenaeos. Maecenas orci lorem, bibendum ullamcorper congue ac, vestibulum vel neque. Nulla ut venenatis ex. Nunc pellentesque eros at metus dapibus, ut ullamcorper elit maximus.",
		"Quisque dapibus diam arcu, mollis accumsan dui convallis sed. Pellentesque laoreet nibh eget diam varius rhoncus. Vestibulum luctus, magna rhoncus sollicitudin condimentum, nisi augue faucibus lacus, at eleifend turpis mi eu purus. Vivamus non nisl magna. Praesent bibendum suscipit felis. Sed mi lorem, fringilla at commodo ut, accumsan sed velit. Vestibulum interdum quis leo sed aliquam. In ut velit quis quam pretium mollis vitae non nunc. Praesent ut dolor mi. Nunc scelerisque mi id elit dignissim, id sodales ipsum tincidunt. Duis sit amet risus mi. Morbi ornare neque nunc, semper ornare orci dignissim in. Donec ipsum diam, scelerisque tempus ante et, scelerisque convallis lorem. Aliquam facilisis imperdiet viverra. Pellentesque interdum, elit in consectetur euismod, metus odio pretium lorem, sed imperdiet neque est eu orci. Nunc nec massa et quam porta dictum.",
		"Mauris finibus tempor tempus. Suspendisse imperdiet in tortor ac condimentum. Nullam elementum est eget massa ullamcorper elementum eu quis velit. Nullam sed ipsum id velit consequat commodo. Quisque cursus eget mi nec elementum. Donec vel hendrerit nunc. Nunc egestas felis quam, et tristique nulla congue eu. Fusce quis ex bibendum, luctus urna id, vehicula ipsum. Nunc blandit, nibh a commodo congue, orci eros feugiat tellus, sed euismod lectus mi dapibus lacus. Etiam ac metus vel neque imperdiet efficitur. Suspendisse mattis quam ut turpis posuere faucibus. Sed eleifend justo ultricies odio facilisis bibendum.",
		"Donec in arcu sed justo lobortis ornare eu vitae nulla. Lorem ipsum dolor sit amet, consectetur adipiscing elit. Curabitur pellentesque urna ipsum, non hendrerit urna laoreet dignissim. Integer pellentesque lorem velit, vitae vulputate libero scelerisque ac. Curabitur accumsan cursus leo, at mattis elit mattis et. Phasellus consequat justo et dui faucibus sodales. Cras facilisis vehicula dignissim. Donec consequat tincidunt mi ut faucibus. Ut a massa ullamcorper, finibus diam sed, accumsan erat. Aenean vitae dolor eu ipsum cursus faucibus et condimentum metus. Pellentesque non est id nunc finibus porta et at eros. Cras sodales congue sollicitudin. Nunc ullamcorper tortor vitae tortor aliquam, vitae ultricies neque lobortis. In hac habitasse platea dictumst. Aenean sed fermentum neque, ut pulvinar sem.",
		"Maecenas dapibus semper turpis, vitae laoreet sem facilisis eget. Curabitur sollicitudin purus id congue tincidunt. Nulla vitae nisl eu orci vehicula molestie. Duis ut eros ac nisl finibus molestie. Duis sit amet tempus ipsum, quis consequat metus. Curabitur sed tortor suscipit, consequat erat eget, dapibus tortor. In tristique augue lacus, in ultrices ex scelerisque a. Suspendisse potenti.",
		"Vestibulum efficitur ullamcorper diam non accumsan. Suspendisse ac nisi sit amet orci laoreet imperdiet. Integer tempor sapien nec sollicitudin sodales. Proin euismod, lectus quis lobortis gravida, tellus ligula semper ex, at vehicula ante dolor eget mi. Quisque metus libero, fermentum sodales venenatis in, tristique ac lacus. Aenean sodales nibh a sem rutrum, vel elementum velit interdum. Proin vel lectus ut neque gravida eleifend. Fusce maximus ante ligula, vestibulum congue nulla molestie et.",
		"Donec varius nibh sed ligula feugiat placerat. Fusce dolor ex, malesuada nec convallis id, maximus ac est. Sed eu ex ullamcorper, sagittis velit vel, congue enim. Maecenas eu posuere lectus. Proin eu nisl consequat mi euismod laoreet. Donec quis neque dolor. Donec in nulla gravida, imperdiet mi et, viverra elit.",
		"Nam et risus leo. Pellentesque ut pretium sem. Mauris ac orci sit amet ex placerat commodo. Suspendisse potenti. Vestibulum eleifend convallis sapien, nec semper libero convallis eget. Vivamus vitae ligula et lectus gravida consectetur ut eget quam. In hac habitasse platea dictumst.",
		"Vestibulum pretium tellus ac ipsum fringilla iaculis. Curabitur volutpat sapien nunc, in luctus lacus ullamcorper non. Vestibulum auctor, dui et semper sodales, augue tellus rutrum tortor, eu iaculis leo arcu ornare nulla. Nullam a urna efficitur, blandit nunc sed, tincidunt odio. Morbi cursus eros eget mattis porta. Mauris ac pellentesque metus. Morbi sagittis lacus id euismod tempor. Mauris ultrices risus vel tellus consequat, et tincidunt ipsum volutpat.",
		"Donec pulvinar risus non tellus faucibus, quis tempor elit vestibulum. Pellentesque aliquet lorem sed eros fringilla luctus. Nulla et lacus eget lorem feugiat dignissim. Pellentesque ac velit lacus. Vestibulum a justo tristique, cursus nisl in, bibendum nulla. Nam eu tempor purus, et dapibus ante. Lorem ipsum dolor sit amet, consectetur adipiscing elit.",
		"Donec pellentesque tellus quis arcu semper, ut dignissim mauris gravida. Etiam vel lacus sagittis, rhoncus orci ut, semper velit. Curabitur felis massa, aliquam eu dolor vel, ornare efficitur turpis. Nulla id elit nec orci maximus aliquam ut laoreet ipsum. Sed quis posuere massa. Aliquam lobortis, est quis sagittis interdum, eros risus maximus elit, nec facilisis mi tortor eu nisi. Suspendisse malesuada eleifend sodales. Maecenas mollis mi tortor, sit amet rutrum dolor tincidunt ut. Morbi finibus dignissim porta.",
		"Sed hendrerit massa id molestie ultricies. Sed pretium vel risus dictum ullamcorper. Etiam molestie orci feugiat quam aliquam, in maximus ipsum sodales. Nunc bibendum est dolor, vel rhoncus orci feugiat sit amet. Integer dignissim risus ut mauris volutpat, ac hendrerit erat sodales. Sed accumsan ex ex. Ut varius augue vitae mauris aliquam elementum. Nulla id volutpat magna, in bibendum enim. Aliquam iaculis nunc et augue dapibus, sit amet dictum enim feugiat. Mauris nisl quam, viverra ac massa ac, suscipit porttitor risus.",
		"Donec elementum scelerisque leo, vitae interdum neque fringilla vel. Etiam eu leo rutrum, mollis sapien quis, bibendum mi. Quisque sem ex, tincidunt nec tincidunt molestie, varius eu tellus. Donec viverra sit amet purus eget tincidunt. Cras pulvinar porttitor tellus eu faucibus. In sit amet rhoncus sem. Mauris faucibus tortor urna, at faucibus quam volutpat ac. Vivamus ut molestie velit, quis tristique dolor. Duis molestie semper nisi ac feugiat. Nunc a nisi convallis, commodo dui et, ultrices velit.",
		"Nullam semper suscipit mi, ut consequat velit suscipit nec. Curabitur finibus pharetra diam sed condimentum. Aliquam dolor ligula, hendrerit nec pretium vitae, convallis sit amet leo. Vestibulum magna tortor, blandit eget fermentum in, porttitor et orci. In aliquam urna eu mollis lacinia. Nullam semper interdum orci. Cras semper mauris nec elementum mattis. Donec porta luctus ultricies. Pellentesque in luctus ligula. In ante ex, lacinia at dui vel, bibendum ornare lacus. Quisque porta eleifend dignissim. Nunc sed placerat risus. Etiam fermentum nec enim in dignissim.",
		"Nunc sodales tellus a urna cursus, ac posuere felis ultrices. Suspendisse maximus massa sem, in laoreet eros molestie ut. Aliquam suscipit vel orci at vestibulum. Fusce hendrerit, felis eu posuere sollicitudin, est velit dictum turpis, vitae faucibus mauris mauris ut velit. Praesent bibendum lectus ut vestibulum maximus. Donec blandit ligula libero, ut tempor nulla scelerisque posuere. Nulla egestas elit ex. Maecenas pretium semper quam in rhoncus. Fusce a viverra nunc, sed placerat libero. Sed venenatis convallis risus sed condimentum. Vivamus euismod tellus eu sagittis facilisis. Aliquam ac massa lacus. Interdum et malesuada fames ac ante ipsum primis in faucibus. Ut turpis ligula, euismod ac enim sed, porta tristique magna.",
		"Pellentesque venenatis elementum enim, nec consequat felis. Nulla facilities. Suspendisse tempus erat non ipsum vulputate, in lacinia elit feugiat. Maecenas et velit eget tortor congue luctus nec eu urna. Duis congue tellus vel purus convallis tristique. Aenean dapibus tincidunt leo, vel aliquam turpis pretium sit amet. Pellentesque mollis in massa eu mattis. Suspendisse potenti. Cras facilisis at purus ut elementum. Sed volutpat eget nisl id lobortis. Etiam vestibulum lectus id justo vulputate lacinia quis ut lectus. Phasellus tincidunt dolor id nisl placerat, a consequat est volutpat. Phasellus venenatis finibus ante, et eleifend justo pulvinar id. Cras eu ligula quis libero tempor condimentum. Praesent accumsan enim in sodales vulputate. Curabitur rhoncus ante a luctus interdum.",
		"Maecenas vel rhoncus turpis, sit amet varius lacus. Vivamus venenatis sapien vel mi euismod, eget commodo tortor auctor. Cras sit amet dictum quam, non fermentum massa. Aliquam gravida est sed gravida suscipit. Praesent finibus tempor magna, ut dapibus dolor. Proin quis pulvinar arcu. Suspendisse tempor sem justo, at dignissim justo elementum vel. Aenean vitae dolor varius lacus rutrum eleifend.",
		"Etiam ut varius enim. Quisque ligula neque, accumsan et neque eget, pretium lacinia nisi. Etiam aliquet id quam a ullamcorper. Fusce eleifend, mauris vitae placerat egestas, orci erat euismod enim, ut posuere nisl justo placerat libero. Nam ac dui ac lorem laoreet maximus. Curabitur risus leo, feugiat et ligula ac, pellentesque ullamcorper lorem. Vestibulum ante ipsum primis in faucibus orci luctus et ultrices posuere cubilia Curae; Donec dignissim non turpis tristique hendrerit. Donec libero odio, ullamcorper condimentum tincidunt ac, hendrerit sed metus. Maecenas venenatis sodales ex. Vestibulum sit amet finibus urna, eu pellentesque velit. Donec accumsan lectus sit amet purus lacinia, et aliquam quam imperdiet. Nunc quis sem fermentum, consectetur urna quis, tristique eros. Sed at tortor a velit blandit aliquam in semper odio. Etiam laoreet sapien lacus, at convallis felis feugiat vitae. Integer et facilisis nibh.",
		"Nullam consequat vehicula euismod. Donec non metus sed nulla bibendum facilisis sed vitae orci. Donec at sapien elit. Sed luctus id augue a gravida. Quisque bibendum nisl sed imperdiet congue. Nam tristique diam diam, ut finibus ante laoreet sit amet. Fusce eget condimentum sem, eget imperdiet massa. Sed orci velit, aliquet a malesuada ac, convallis vitae elit. Aliquam molestie tellus vitae tellus accumsan, quis dapibus purus placerat. Cras commodo, ligula quis commodo congue, ipsum enim placerat nisi, eu congue ante dolor sed ante. Nunc luctus est id metus eleifend, sed consequat leo gravida. Phasellus mattis enim sit amet placerat vehicula. Suspendisse vestibulum lacus sit amet nunc placerat, et ultricies elit fermentum. In et est ac turpis vestibulum bibendum.",
		"Cras in est efficitur, volutpat nisi ut, faucibus nisl. Proin vehicula quam vel ante cursus, vel ultricies erat rutrum. Aliquam pharetra fringilla mauris eget condimentum. Sed placerat turpis eu turpis semper, nec vestibulum sem ornare. Suspendisse potenti. Ut at elit porta, luctus lorem in, tincidunt nunc. Vivamus in justo a lectus sollicitudin tempor non non tellus. Phasellus nec iaculis lacus. Cras consequat orci nec feugiat rutrum. Praesent vulputate, lorem a vulputate ornare, nisi ante tempus elit, sit amet interdum eros nunc vitae erat. Donec at neque in orci ultricies pulvinar eu sit amet quam. Duis tempus vitae sapien vel malesuada.",
		"In auctor condimentum diam sed auctor. Vivamus id rutrum arcu. Proin venenatis, neque malesuada eleifend posuere, urna est pellentesque turpis, vitae scelerisque nisi nunc sit amet ligula. Vestibulum viverra consequat est in lobortis. Nam efficitur arcu at neque lacinia, ut condimentum leo gravida. Vestibulum vitae nisi leo. Nulla odio dolor, malesuada a magna non, accumsan efficitur nisi. Proin vitae lacus facilisis erat interdum pulvinar. Praesent in egestas tortor.", "Integer in massa odio. Duis ex mi, varius nec nibh eget, sodales pellentesque sapien. Aenean pharetra eros vitae tellus volutpat euismod. Vestibulum mattis condimentum metus eget ullamcorper. Integer ut iaculis lorem. Fusce nec lobortis massa. Aliquam pellentesque placerat lorem in efficitur. In pharetra et ex sed mattis. Praesent eget nisi sit amet metus ullamcorper fermentum. Integer ante dolor, vestibulum id massa in, dictum porta nisi. Duis lacinia est vitae enim dignissim blandit. Integer non dapibus velit. Cras auctor eu felis sodales tempor.",
		"Fusce convallis facilisis lacus id lacinia. Nunc rhoncus vulputate consectetur. Ut malesuada malesuada mi, molestie luctus purus feugiat at. Integer eros mauris, porttitor eu tristique quis, gravida id enim. Nullam vitae lectus nulla. Etiam consequat sapien vitae risus volutpat, vitae condimentum nisl ultrices. Donec et magna sed justo suscipit iaculis. Vestibulum id mauris eu eros tempus interdum. Morbi porta libero quis leo consectetur maximus. Class aptent taciti sociosqu ad litora torquent per conubia nostra, per inceptos himenaeos. Proin ut dui pharetra, faucibus augue blandit, tempor massa. Duis urna odio, luctus eget justo sed, consectetur faucibus mauris. Phasellus vel sapien justo. Donec sed lectus eget lectus ultrices ornare. Curabitur ultrices sapien libero, sit amet fermentum mi bibendum nec.",
		"Phasellus sed quam mi. Aenean eget sodales neque. Nam convallis lacus non justo blandit, non ornare mauris imperdiet. Nam mattis commodo turpis et lacinia. Duis ac maximus lorem. Suspendisse euismod dui at sodales accumsan. Suspendisse vulputate lobortis sapien, viverra vehicula felis blandit vitae. Fusce viverra ultrices felis sed egestas. Duis ex diam, rutrum nec maximus sit amet, imperdiet ac ex. Orci varius natoque penatibus et magnis dis parturient montes, nascetur ridiculous mus. Mauris scelerisque facilisis eros, sed maximus erat elementum et.",
		"In ultricies, felis eu aliquet tempor, velit ante finibus ipsum, ut sagittis lacus urna sit amet erat. Vestibulum porttitor gravida scelerisque. Fusce semper faucibus est placerat varius. Curabitur nec quam pellentesque, pharetra magna nec, pellentesque nunc. Lorem ipsum dolor sit amet, consectetur adipiscing elit. Donec sagittis tincidunt justo, nec sagittis turpis vehicula eu. Duis ultricies nulla urna, vel efficitur purus feugiat a. Etiam molestie pharetra ex. Cras ut est faucibus, maximus orci eget, accumsan sapien.",
		"Nam tempor, libero eget iaculis commodo, mauris neque feugiat diam, vel lacinia ipsum velit ac lorem. In eget fringilla velit. Nullam mattis elit sem, tincidunt ultricies quam euismod ac. Sed sagittis mauris id orci consequat, a accumsan ligula commodo. Quisque a faucibus ligula, eget porttitor sem. Quisque sit amet sollicitudin sem. Vestibulum dignissim erat lacus, id molestie metus malesuada eget. Vivamus ac metus non dui sagittis rutrum. Curabitur quam quam, luctus at hendrerit quis, vehicula in libero. Fusce et sapien lorem.",
		"Integer ornare ex tempus libero hendrerit, ac maximus purus imperdiet. Nullam tincidunt ex et ipsum vulputate luctus. Nunc ac sodales mi. Integer nec velit leo. Fusce quis ante tortor. Donec congue purus sem. Sed eget velit vel lacus semper suscipit. Praesent id lacinia enim. Fusce luctus in leo sit amet porttitor. Nam non lorem in enim fermentum auctor. Morbi non pellentesque ex.",
		"Integer viverra leo at erat ultrices bibendum. Quisque sollicitudin mauris sapien, non ornare neque congue eget. Nulla tempus, est sed lobortis facilisis, sapien justo placerat mauris, a tincidunt risus turpis eu massa. Ut volutpat ut justo non aliquam. Quisque dignissim dapibus nunc, eget sollicitudin ipsum dignissim sit amet. Mauris sed magna ac nulla scelerisque sodales quis ultricies sapien. Maecenas semper tristique ex, et congue metus.",
		"Ut luctus ligula id tellus faucibus lobortis. Donec vel turpis purus. Lorem ipsum dolor sit amet, consectetur adipiscing elit. Nullam malesuada, leo id condimentum luctus, ante quam auctor velit, at laoreet justo augue ac mi. Maecenas pharetra elit eu dui accumsan tempor. Sed turpis orci, laoreet at mauris non, sollicitudin pharetra ante. Ut gravida rutrum porttitor. Sed velit diam, consequat eu tempus in, ullamcorper non erat. Integer vel dui felis. Donec pharetra ipsum turpis, a pharetra tortor ullamcorper a. Aliquam sed sapien gravida massa tincidunt mollis eget vitae est. Sed dapibus hendrerit tortor eu sollicitudin. Sed ac eleifend neque. Quisque blandit mi turpis, eget faucibus arcu porttitor non. Integer scelerisque metus felis, et porta ligula pretium sit amet.",
		"Sed dictum rutrum velit eleifend tristique. Suspendisse porta vestibulum ultrices. Etiam ac sagittis magna, eget porta ipsum. Praesent non tempus metus. Sed sollicitudin velit quis efficitur placerat. Cras congue nulla dolor, vitae fermentum augue convallis tempor. Aliquam dui erat, sollicitudin quis eros fermentum, ultrices sagittis arcu. Curabitur pharetra porta tortor non consequat. Proin gravida eget leo lacinia mollis. Morbi dui arcu, ornare scelerisque sapien a, feugiat efficitur erat. Vestibulum ac leo porta nisl consequat molestie.",
		"Curabitur auctor faucibus ornare. Aenean sit amet sollicitudin quam, non condimentum dui. In sagittis neque at elit porttitor posuere. Fusce et ex vel nisi mattis ultricies id a eros. Fusce suscipit, nisl et dignissim consectetur, ligula risus mattis ante, at dictum dolor augue sit amet nulla. Aliquam laoreet, libero at laoreet efficitur, justo dolor mollis ex, non venenatis lorem tortor nec sapien. Suspendisse vestibulum dolor id maximus volutpat. Pellentesque mollis orci dolor, nec tempor turpis dapibus ut. Maecenas commodo nisi sapien, nec iaculis nisl hendrerit et. Integer non purus interdum, ultricies nunc vitae, mollis est.",
		"Cras elementum magna in elit cursus ultrices. Aliquam id massa fringilla, tincidunt augue nec, hendrerit tellus. Vestibulum ante ipsum primis in faucibus orci luctus et ultrices posuere cubilia Curae; Cras porttitor nibh a tortor consectetur tincidunt non quis tortor. Mauris in mollis ipsum. Mauris in mauris quam. Mauris ut nibh nisi.",
		"Sed rutrum quam at est finibus, a commodo neque tempor. Etiam bibendum mauris scelerisque tortor faucibus, in lacinia sapien bibendum. Morbi facilisis lorem vitae magna scelerisque, at egestas ante dignissim. Phasellus sit amet nunc sapien. Vivamus sit amet purus finibus, bibendum lorem sit amet, euismod eros. Vivamus consequat non nisi a tristique. Vestibulum ac nulla eu sapien euismod aliquam. Praesent porttitor urna mi.", "Cras porttitor eros vel varius iaculis. Mauris id nulla felis. Aenean scelerisque, ligula et interdum pretium, erat libero fringilla purus, eleifend dapibus elit nunc fermentum nulla. Pellentesque quis mauris hendrerit, luctus sem sed, ullamcorper sapien. Cras nec iaculis velit, at tempus dui. Nullam a eros in orci egestas pellentesque. Vestibulum ante ipsum primis in faucibus orci luctus et ultrices posuere cubilia Curae;",
		"Cras quis vulputate diam. Donec sed placerat felis. Quisque sed auctor elit, id lacinia justo. Proin aliquam orci nec efficitur auctor. Pellentesque laoreet, metus nec ultricies hendrerit, est quam consectetur sem, in fermentum elit neque at velit. Ut pharetra sem congue, malesuada sapien in, consequat mi. Suspendisse magna lectus, pellentesque in nunc non, congue tristique nisi. Praesent id nibh vulputate ante tempor porttitor. Nunc sed lorem non dolor dignissim iaculis at vitae nulla. In at bibendum arcu. Phasellus vel mauris sed lorem pellentesque tristique.",
		"Phasellus sit amet lectus mi. Donec sit amet magna non arcu posuere pulvinar nec a erat. Suspendisse tellus mi, dictum ac accumsan vel, aliquet eget felis. Nulla porta vitae nunc et malesuada. Curabitur tempus porttitor magna sit amet mattis. Praesent pretium nisl maximus, pulvinar mi dignissim, rhoncus lacus. Maecenas in suscipit nisi. Sed non nisl eu nibh elementum imperdiet nec a ex.",
		"Aenean lorem tellus, tempor vitae ornare et, tempor non urna. Suspendisse finibus lectus gravida justo eleifend, ac feugiat augue ultrices. Integer semper ex nisl, ac tempor risus maximus eget. Pellentesque a tortor dignissim erat volutpat convallis ac vel mi. Nam ultricies, diam nec interdum bibendum, urna ipsum bibendum elit, vitae euismod est dolor et lacus. Fusce consectetur iaculis mauris eget mattis. Quisque bibendum nibh quis orci sagittis consectetur. Donec maximus consectetur ex sed fringilla. Interdum et malesuada fames ac ante ipsum primis in faucibus. Duis congue elit nisi, sit amet vehicula purus bibendum ut.",
		"Ut aliquet commodo mauris, quis suscipit urna mollis at. Fusce gravida nibh risus, et sollicitudin leo placerat eget. Maecenas tincidunt hendrerit justo, ac mattis massa commodo vitae. Vivamus neque quam, tincidunt et venenatis nec, convallis et ex. Etiam blandit tellus vitae mi pretium dapibus. Proin ac nibh ullamcorper, tristique nibh eu, venenatis nunc. Mauris non leo gravida, aliquam lacus non, venenatis lacus. Curabitur quis dolor risus. Nullam eros arcu, vehicula a orci nec, pretium pellentesque enim. Sed vestibulum lectus ex, imperdiet eleifend metus facilisis eget.",
		"Etiam nulla lectus, volutpat quis iaculis et, consectetur sit amet sapien. Mauris dictum posuere nibh et ultrices. Curabitur finibus, urna eget cursus volutpat, lectus arcu scelerisque turpis, in pretium lorem nulla quis purus. Quisque in pharetra tortor. Aenean malesuada imperdiet ex. Nulla maximus felis diam, a accumsan odio tristique ac. Quisque id metus quis justo vulputate convallis. In rutrum tortor eget justo molestie, vitae condimentum odio ultrices. Sed sodales mi at commodo commodo. Donec euismod, lectus eu euismod aliquet, lorem libero vehicula urna, id auctor leo tortor quis velit. Curabitur odio magna, iaculis vel viverra id, egestas id augue. Donec enim augue, congue nec maximus sit amet, auctor non mauris. Pellentesque fermentum, lectus vel auctor pulvinar, velit odio aliquam turpis, vel imperdiet est neque at turpis. Fusce ornare auctor justo, id malesuada mauris dictum sed. Pellentesque convallis, turpis eget vulputate egestas, elit nisi cursus lectus, vel tincidunt quam ipsum sit amet sapien.",
		"Pellentesque posuere ullamcorper velit eu interdum. Vivamus posuere aliquam velit a rhoncus. Fusce viverra fermentum justo id rhoncus. Quisque at dui dapibus, bibendum eros fermentum, laoreet ligula. Curabitur porta, diam at fringilla congue, dui enim bibendum elit, non scelerisque lorem risus eget lacus. Proin risus nibh, scelerisque nec feugiat vel, finibus interdum lorem. Morbi elit purus, fringilla at odio at, cursus condimentum erat. In tristique nisi eu ex tristique, sed bibendum est eleifend. Mauris interdum quis velit sit amet sodales. Integer semper tempus magna, vel vehicula eros pharetra sed. Sed tincidunt porttitor tellus. Suspendisse potenti. Nulla et scelerisque nisi. Aenean pellentesque fermentum ipsum vel pretium. Pellentesque habitant morbi tristique senectus et netus et malesuada fames ac turpis egestas.",
		"Curabitur tempus lectus nisl, ut tempus nunc efficitur et. Maecenas vel nulla et ipsum sollicitudin consectetur ac a dolor. Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed vulputate a tellus sit amet pellentesque. Cras sapien nunc, efficitur eu mi nec, tincidunt finibus dolor. Nullam volutpat accumsan elit et efficitur. Nullam felis enim, consequat quis semper nec, gravida id dui. Mauris et lorem ac odio eleifend tempor id vitae dolor. Quisque ante leo, vestibulum id leo sed, bibendum venenatis mauris. Fusce bibendum placerat nibh, in posuere magna placerat ac. Duis imperdiet mauris at risus aliquam, in congue lectus hendrerit. Phasellus porttitor metus eu purus rhoncus, ut iaculis dolor condimentum.",
		"Sed eu ante lorem. Donec diam odio, suscipit eu euismod eget, sagittis non dui. Mauris consectetur volutpat viverra. Curabitur lacinia, dui vel aliquam lacinia, felis turpis bibendum odio, non ultricies augue augue nec diam. Etiam posuere risus sodales nisi gravida, eu venenatis turpis fermentum. Nulla vestibulum nec purus ut imperdiet. Vivamus vestibulum elementum nisi non molestie. Integer sit amet dignissim erat, sed lacinia massa. Sed ac nulla leo. Nulla ex purus, consequat at lectus vel, mollis tempor justo.",
		"Class aptent taciti sociosqu ad litora torquent per conubia nostra, per inceptos himenaeos. Vivamus convallis nulla facilisis odio viverra, quis malesuada leo viverra. Etiam neque purus, interdum eu dui et, facilisis congue felis. Nunc tempor risus ut sapien porta, non tincidunt magna ornare. Vivamus consectetur, ligula in facilisis consequat, turpis quam fermentum ipsum, eget elementum odio velit vel risus. Etiam maximus eleifend lacus et cursus. Sed feugiat dolor et velit suscipit, vel mattis orci auctor. Nam tempor nunc vitae turpis interdum mollis. Interdum et malesuada fames ac ante ipsum primis in faucibus. Vestibulum sit amet mauris a nulla consequat blandit. Aenean fringilla orci fringilla lectus pulvinar, eu pellentesque erat egestas. Vivamus venenatis, tortor at tincidunt fermentum, metus ligula blandit massa, id congue metus eros sit amet nisi. Etiam ac vehicula ex, sit amet mollis mauris. Suspendisse potenti. Praesent ut eleifend ante.",
	}
)

func randInt(min int, max int) int {
	rand.Seed(time.Now().UTC().UnixNano())
	return min + rand.Intn(max-min)
}

const tempDBPath = "test.db"

func removeDB(dbPath string) {
	if err := os.Remove(dbPath); err != nil {
		if !strings.Contains(err.Error(), "no such file or directory") {
			panic(err)
		}
	}
}
