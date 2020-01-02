package gosn

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"gopkg.in/matryer/try.v1"
)

// NewNote returns an Item of type Note without content
func NewNote() Note {
	now := time.Now().UTC().Format(timeLayout)
	var note Note
	note.ContentType = "Note"
	note.CreatedAt = now
	note.UpdatedAt = now
	note.UUID = GenUUID()

	return note
}

// NewTag returns an Item of type Tag without content
func NewTag() Tag {
	now := time.Now().UTC().Format(timeLayout)
	var tag Tag
	tag.ContentType = "Tag"
	tag.CreatedAt = now
	tag.UpdatedAt = now
	tag.UUID = GenUUID()
	return tag
}

//
//// ClientStructure defines behaviour of a Component Item's content entry
//type ComponentClientStructure interface {
//	// return name
//	GetName() string
//	// return active status
//	GetActive() bool
//	// get item associations
//	GetItemAssociations() []string
//	// get item disassociations
//	GetItemDisassociations() []string
//	// associate items
//	AssociateItems(items []string)
//	// disassociate items
//	DisassociateItems(items []string)
//}

//// ClientStructure defines behaviour of a Component Item's content entry
//type NoteClientStructure interface {
//	// set text
//	SetText(input string)
//	// return text
//	GetText() string
//	// get last update time
//}
//
//// ClientStructure defines behaviour of an Item's content entry
//type ClientStructure interface {
//	References() ItemReferences
//	// update or insert item references
//	UpsertReferences(input ItemReferences)
//	// set references
//	SetReferences(input ItemReferences)
//	// return title
//	GetTitle() string
//	// set title
//	SetTitle(input string)
//	// get last update time
//	GetUpdateTime() (time.Time, error)
//	// set last update time
//	SetUpdateTime(time.Time)
//	// get appdata
//	GetAppData() AppDataContent
//	// set appdata
//	SetAppData(data AppDataContent)
//	// client structure methods for Note
//	NoteClientStructure
//	// client structure methods for Component
//	ComponentClientStructure
//}

type syncResponse struct {
	Items       EncryptedItems `json:"retrieved_items"`
	SavedItems  EncryptedItems `json:"saved_items"`
	Unsaved     EncryptedItems `json:"unsaved"`
	SyncToken   string         `json:"sync_token"`
	CursorToken string         `json:"cursor_token"`
}

// AppTagConfig defines expected configuration structure for making Tag related operations
type AppTagConfig struct {
	Email    string
	Token    string
	FindText string
	FindTag  string
	NewTags  []string
	Debug    bool
}

// GetItemsInput defines the input for retrieving items
type GetItemsInput struct {
	Session     Session
	SyncToken   string
	CursorToken string
	OutType     string
	BatchSize   int // number of items to retrieve
	PageSize    int // override default number of items to request with each sync call
	Debug       bool
}

// GetItemsOutput defines the output from retrieving items
// It contains slices of items based on their state
// see: https://standardfile.org/ for state details
type GetItemsOutput struct {
	Items      EncryptedItems // items new or modified since last sync
	SavedItems EncryptedItems // dirty items needing resolution
	Unsaved    EncryptedItems // items not saved during sync
	SyncToken  string
	Cursor     string
}

const retryScaleFactor = 0.25

func resizeForRetry(in *GetItemsInput) {
	switch {
	case in.BatchSize != 0:
		in.BatchSize = int(math.Ceil(float64(in.BatchSize) * retryScaleFactor))
	case in.PageSize != 0:
		in.PageSize = int(math.Ceil(float64(in.PageSize) * retryScaleFactor))
	default:
		in.PageSize = int(math.Ceil(float64(PageSize) * retryScaleFactor))
	}
}

type EncryptedItems []EncryptedItem

func (ei EncryptedItems) Decrypt(Mk, Ak string, debug bool) (o DecryptedItems, err error) {
	debugPrint(debug, fmt.Sprintf("Decrypt | decrypting %d items", len(ei)))

	for _, eItem := range ei {
		var item DecryptedItem

		if eItem.EncItemKey != "" {
			var decryptedEncItemKey string

			decryptedEncItemKey, err = decryptString(eItem.EncItemKey, Mk, Ak, eItem.UUID)
			if err != nil {
				return
			}

			itemEncryptionKey := decryptedEncItemKey[:len(decryptedEncItemKey)/2]
			itemAuthKey := decryptedEncItemKey[len(decryptedEncItemKey)/2:]

			var decryptedContent string

			decryptedContent, err = decryptString(eItem.Content, itemEncryptionKey, itemAuthKey, eItem.UUID)
			if err != nil {
				return
			}

			item.Content = decryptedContent
		}

		item.UUID = eItem.UUID
		item.Deleted = eItem.Deleted
		item.ContentType = eItem.ContentType
		item.UpdatedAt = eItem.UpdatedAt
		item.CreatedAt = eItem.CreatedAt

		o = append(o, item)
	}

	return o, err
}

func (ei EncryptedItems) DecryptAndParse(Mk, Ak string, debug bool) (o Items, err error) {
	debugPrint(debug, fmt.Sprintf("DecryptAndParse | items: %d", len(ei)))

	var di DecryptedItems

	di, err = ei.Decrypt(Mk, Ak, debug)
	if err != nil {
		return
	}

	o, err = di.Parse()

	return
}

// GetItems retrieves items from the API using optional filters
func GetItems(input GetItemsInput) (output GetItemsOutput, err error) {
	giStart := time.Now()

	defer func() {
		debugPrint(input.Debug, fmt.Sprintf("GetItems | duration %v", time.Since(giStart)))
	}()

	if !input.Session.Valid() {
		err = fmt.Errorf("session is invalid")
		return
	}

	var sResp syncResponse

	debugPrint(input.Debug, fmt.Sprintf("GetItems | PageSize %d", input.PageSize))
	// retry logic is to handle responses that are too large
	// so we can reduce number we retrieve with each sync request
	start := time.Now()
	rErr := try.Do(func(attempt int) (bool, error) {
		debugPrint(input.Debug, fmt.Sprintf("GetItems | attempt %d", attempt))
		var rErr error
		sResp, rErr = getItemsViaAPI(input)
		if rErr != nil && strings.Contains(strings.ToLower(rErr.Error()), "too large") {
			debugPrint(input.Debug, fmt.Sprintf("GetItems | %s", rErr.Error()))
			initialSize := input.PageSize
			resizeForRetry(&input)
			debugPrint(input.Debug, fmt.Sprintf("GetItems | failed to retrieve %d items "+
				"at a time so reducing to %d", initialSize, input.PageSize))
		}
		return attempt < 3, rErr
	})

	if rErr != nil {
		return output, rErr
	}

	elapsed := time.Since(start)

	debugPrint(input.Debug, fmt.Sprintf("GetItems | took %v to get all items", elapsed))

	postStart := time.Now()
	output.Items = sResp.Items
	output.Items.DeDupe()
	output.Unsaved = sResp.Unsaved
	output.Unsaved.DeDupe()
	output.SavedItems = sResp.SavedItems
	output.SavedItems.DeDupe()
	output.Cursor = sResp.CursorToken
	output.SyncToken = sResp.SyncToken
	// strip any duplicates (https://github.com/standardfile/rails-engine/issues/5)
	postElapsed := time.Since(postStart)
	debugPrint(input.Debug, fmt.Sprintf("GetItems | post processing took %v", postElapsed))
	debugPrint(input.Debug, fmt.Sprintf("GetItems | sync token: %+v", stripLineBreak(output.SyncToken)))

	return output, err
}

// PutItemsInput defines the input used to put items
type PutItemsInput struct {
	Items     EncryptedItems
	SyncToken string
	Session   Session
	Debug     bool
}

// PutItemsOutput defines the output from putting items
type PutItemsOutput struct {
	ResponseBody syncResponse
}

func (i *Items) Encrypt(Mk, Ak string, debug bool) (e EncryptedItems, err error) {
	e, err = encryptItems(i, Mk, Ak, debug)
	return
}

// PutItems validates and then syncs items via API
func PutItems(i PutItemsInput) (output PutItemsOutput, err error) {
	piStart := time.Now()

	defer func() {
		debugPrint(i.Debug, fmt.Sprintf("PutItems | duration %v", time.Since(piStart)))
	}()

	if !i.Session.Valid() {
		err = fmt.Errorf("session is invalid")
		return
	}

	debugPrint(i.Debug, fmt.Sprintf("PutItems | putting %d items", len(i.Items)))

	// for each page size, send to push and get response
	syncToken := stripLineBreak(i.SyncToken)

	var savedItems []EncryptedItem

	// put items in big chunks, default being page size
	for x := 0; x < len(i.Items); x += PageSize {
		var finalChunk bool

		var lastItemInChunkIndex int
		// if current big chunk > num encrypted items then it's the last
		if x+PageSize >= len(i.Items) {
			lastItemInChunkIndex = len(i.Items) - 1
			finalChunk = true
		} else {
			lastItemInChunkIndex = x + PageSize
		}

		debugPrint(i.Debug, fmt.Sprintf("PutItems | putting items: %d to %d", x+1, lastItemInChunkIndex+1))

		bigChunkSize := (lastItemInChunkIndex - x) + 1

		fullChunk := i.Items[x : lastItemInChunkIndex+1]

		var subChunkStart, subChunkEnd int
		subChunkStart = x
		subChunkEnd = lastItemInChunkIndex
		// initialise running total
		totalPut := 0
		// keep trying to push chunk of encrypted items in reducing subChunk sizes until it succeeds
		maxAttempts := 20
		try.MaxRetries = 20

		for {
			rErr := try.Do(func(attempt int) (bool, error) {
				var rErr error
				// if chunk is too big to put then try with smaller chunk
				var encItemJSON []byte
				itemsToPut := i.Items[subChunkStart : subChunkEnd+1]
				encItemJSON, _ = json.Marshal(itemsToPut)
				var s []EncryptedItem
				s, syncToken, rErr = putChunk(i.Session, encItemJSON, i.Debug)
				if rErr != nil && strings.Contains(strings.ToLower(rErr.Error()), "too large") {
					subChunkEnd = resizePutForRetry(subChunkStart, subChunkEnd, len(encItemJSON))
				}
				if rErr == nil {
					savedItems = append(savedItems, s...)
					totalPut += len(itemsToPut)
				}
				debugPrint(i.Debug, fmt.Sprintf("PutItems | attempt: %d of %d", attempt, maxAttempts))
				return attempt < maxAttempts, rErr
			})
			if rErr != nil {
				err = errors.New("failed to put all items")
				return
			}

			// if it's not the last of the chunk then continue with next subChunk
			if totalPut < bigChunkSize {
				subChunkStart = subChunkEnd + 1
				subChunkEnd = lastItemInChunkIndex

				continue
			}

			// if it's last of the full chunk, then break
			if len(fullChunk) == lastItemInChunkIndex {
				break
			}

			if totalPut == len(fullChunk) {
				break
			}
		} // end infinite for loop for subset

		if finalChunk {
			break
		}
	} // end looping through encrypted items

	output.ResponseBody.SyncToken = syncToken
	output.ResponseBody.SavedItems = savedItems

	return output, err
}

func resizePutForRetry(start, end, numBytes int) int {
	preShrink := end
	// reduce to 90%
	multiplier := 0.90
	// if size is over 2M then be more aggressive and half
	if numBytes > 2000000 {
		multiplier = 0.50
	}

	end = int(math.Ceil(float64(end) * multiplier))
	if end <= start {
		end = start + 1
	}

	if preShrink == end && preShrink > 1 {
		end--
	}

	return end
}

func putChunk(session Session, encItemJSON []byte, debug bool) (savedItems []EncryptedItem, syncToken string, err error) {
	reqBody := []byte(`{"items":` + string(encItemJSON) +
		`,"sync_token":"` + stripLineBreak(syncToken) + `"}`)

	var syncRespBodyBytes []byte

	syncRespBodyBytes, err = makeSyncRequest(session, reqBody, debug)
	if err != nil {
		return
	}

	// get item results from API response
	var bodyContent syncResponse

	bodyContent, err = getBodyContent(syncRespBodyBytes)
	if err != nil {
		return
	}
	// Get new items
	syncToken = stripLineBreak(bodyContent.SyncToken)
	savedItems = bodyContent.SavedItems

	return
}

type EncryptedItem struct {
	UUID        string `json:"uuid"`
	Content     string `json:"content"`
	ContentType string `json:"content_type"`
	EncItemKey  string `json:"enc_item_key"`
	Deleted     bool   `json:"deleted"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

type DecryptedItem struct {
	UUID        string `json:"uuid"`
	Content     string `json:"content"`
	ContentType string `json:"content_type"`
	Deleted     bool   `json:"deleted"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
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
			item.SetContent(*ic)
		case *TagContent:
			ic := item.GetContent().(*TagContent)
			ic.UpsertReferences(refs)
			item.SetContent(*ic)
		}
		updated = append(updated, item)
	}

	return UpdateItemRefsOutput{
		Items: updated,
	}
}
func (noteContent *NoteContent) SetReferences(newRefs ItemReferences) {
	noteContent.ItemReferences = newRefs
}
func (tagContent *TagContent) SetReferences(newRefs ItemReferences) {
	tagContent.ItemReferences = newRefs
}

//func (tagContent *TagContent) References() ItemReferences {
//	return tagContent.ItemReferences
//}

func (tagContent *TagContent) UpsertReferences(newRefs ItemReferences) {
	for _, newRef := range newRefs {
		var found bool

		for _, existingRef := range tagContent.ItemReferences {
			if existingRef.UUID == newRef.UUID {
				found = true
			}
		}

		if !found {
			tagContent.ItemReferences = append(tagContent.ItemReferences, newRef)
		}
	}
}

func (noteContent *NoteContent) UpsertReferences(newRefs ItemReferences) {
	for _, newRef := range newRefs {
		var found bool

		for _, existingRef := range noteContent.ItemReferences {
			if existingRef.UUID == newRef.UUID {
				found = true
			}
		}

		if !found {
			noteContent.ItemReferences = append(noteContent.ItemReferences, newRef)
		}
	}
}

func makeSyncRequest(session Session, reqBody []byte, debug bool) (responseBody []byte, err error) {
	var request *http.Request

	request, err = http.NewRequest(http.MethodPost, session.Server+syncPath, bytes.NewBuffer(reqBody))
	if err != nil {
		return
	}
	request.Header.Set("content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+session.Token)

	var response *http.Response

	start := time.Now()
	response, err = httpClient.Do(request)
	elapsed := time.Since(start)

	debugPrint(debug, fmt.Sprintf("makeSyncRequest | request took: %v", elapsed))

	if err != nil {
		return
	}

	defer func() {
		if err := response.Body.Close(); err != nil {
			debugPrint(debug, fmt.Sprintf("makeSyncRequest | failed to close body closed"))
		}
		debugPrint(debug, fmt.Sprintf("makeSyncRequest | response body closed"))
	}()
	switch response.StatusCode {
	case 413:
		err = errors.New("payload too large")
		return
	}

	if response.StatusCode > 400 {
		debugPrint(debug, fmt.Sprintf("makeSyncRequest | sync of %d req bytes failed with: %s", len(reqBody), response.Status))
		return
	}

	if response.StatusCode >= 200 && response.StatusCode < 300 {
		debugPrint(debug, fmt.Sprintf("makeSyncRequest | sync of %d req bytes succeeded with: %s", len(reqBody), response.Status))
	}

	readStart := time.Now()
	responseBody, err = ioutil.ReadAll(response.Body)
	debugPrint(debug, fmt.Sprintf("makeSyncRequest | response read took %+v", time.Since(readStart)))

	if err != nil {
		return
	}

	debugPrint(debug, fmt.Sprintf("makeSyncRequest | response size %d bytes", len(responseBody)))

	return responseBody, err
}

func getItemsViaAPI(input GetItemsInput) (out syncResponse, err error) {
	// determine how many items to retrieve with each call
	var limit int

	switch {
	case input.BatchSize > 0:
		debugPrint(input.Debug, fmt.Sprintf("getItemsViaAPI |input.BatchSize: %d", input.BatchSize))
		// batch size must be lower than or equal to page size
		limit = input.BatchSize
	case input.PageSize > 0:
		debugPrint(input.Debug, fmt.Sprintf("getItemsViaAPI | input.PageSize: %d", input.PageSize))
		limit = input.PageSize
	default:
		debugPrint(input.Debug, fmt.Sprintf("getItemsViaAPI | default - limit: %d", PageSize))
		limit = PageSize
	}

	debugPrint(input.Debug, fmt.Sprintf("getItemsViaAPI | using limit: %d", limit))

	var requestBody []byte
	// generate request body
	switch {
	case input.CursorToken == "":
		requestBody = []byte(`{"limit":` + strconv.Itoa(limit) + `}`)
	case input.CursorToken == "null":
		debugPrint(input.Debug, "getItemsViaAPI | cursor is null")

		requestBody = []byte(`{"limit":` + strconv.Itoa(limit) +
			`,"items":[],"sync_token":"` + input.SyncToken + `\n","cursor_token":null}`)
	case input.CursorToken != "":
		rawST := input.SyncToken
		input.SyncToken = stripLineBreak(rawST)
		newST := stripLineBreak(input.SyncToken)
		requestBody = []byte(`{"limit":` + strconv.Itoa(limit) +
			`,"items":[],"sync_token":"` + newST + `\n","cursor_token":"` + stripLineBreak(input.CursorToken) + `\n"}`)
	}

	// make the request
	debugPrint(input.Debug, fmt.Sprintf("getItemsViaAPI | making request: %s", stripLineBreak(string(requestBody))))

	msrStart := time.Now()
	responseBody, err := makeSyncRequest(input.Session, requestBody, input.Debug)
	msrEnd := time.Since(msrStart)
	debugPrint(input.Debug, fmt.Sprintf("getItemsViaAPI | makeSyncRequest took: %v", msrEnd))

	if err != nil {
		return
	}

	// get encrypted items from API response
	var bodyContent syncResponse

	bodyContent, err = getBodyContent(responseBody)
	if err != nil {
		return
	}

	out.Items = bodyContent.Items
	out.SavedItems = bodyContent.SavedItems
	out.Unsaved = bodyContent.Unsaved
	out.SyncToken = bodyContent.SyncToken
	out.CursorToken = bodyContent.CursorToken

	if input.BatchSize > 0 {
		return
	}

	if bodyContent.CursorToken != "" && bodyContent.CursorToken != "null" {
		var newOutput syncResponse

		input.SyncToken = out.SyncToken
		input.CursorToken = out.CursorToken
		input.PageSize = limit

		newOutput, err = getItemsViaAPI(input)

		if err != nil {
			return
		}

		out.Items = append(out.Items, newOutput.Items...)
		out.SavedItems = append(out.Items, newOutput.SavedItems...)
		out.Unsaved = append(out.Items, newOutput.Unsaved...)
	} else {
		return out, err
	}

	out.CursorToken = ""

	return out, err
}

// ItemReference defines a reference from one item to another
type ItemReference struct {
	// unique identifier of the item being referenced
	UUID string `json:"uuid"`
	// type of item being referenced
	ContentType string `json:"content_type"`
}

type OrgStandardNotesSNDetail struct {
	ClientUpdatedAt string `json:"client_updated_at"`
}
type AppDataContent struct {
	OrgStandardNotesSN OrgStandardNotesSNDetail `json:"org.standardnotes.sn"`
}

type TagContent struct {
	Title          string         `json:"title"`
	ItemReferences ItemReferences `json:"references"`
	AppData        AppDataContent `json:"appData"`
}

type ComponentContent struct {
	LegacyURL          string         `json:"legacy_url"`
	HostedURL          string         `json:"hosted_url"`
	LocalURL           string         `json:"local_url"`
	ValidUntil         string         `json:"valid_until"`
	OfflineOnly        string         `json:"offlineOnly"`
	Name               string         `json:"name"`
	Area               string         `json:"area"`
	PackageInfo        interface{}    `json:"package_info"`
	Permissions        interface{}    `json:"permissions"`
	Active             interface{}    `json:"active"`
	AutoUpdateDisabled string         `json:"autoupdateDisabled"`
	ComponentData      interface{}    `json:"componentData"`
	DissociatedItemIds []string       `json:"disassociatedItemIds"`
	AssociatedItemIds  []string       `json:"associatedItemIds"`
	ItemReferences     ItemReferences `json:"references"`
	AppData            AppDataContent `json:"appData"`
}

func (cc *ComponentContent) UpsertReferences(input ItemReferences) {
	panic("implement me")
}

func (cc *ComponentContent) SetReferences(input ItemReferences) {
	panic("implement me")
}

func removeStringFromSlice(inSt string, inSl []string) (outSl []string) {
	for _, si := range inSl {
		if inSt != si {
			outSl = append(outSl, si)
		}
	}
	return
}

func (cc *ComponentContent) DisassociateItems(itemsToRemove []string) {
	// remove from associated item ids
	for _, delRef := range itemsToRemove {
		var existingFound bool

		for _, existingRef := range cc.AssociatedItemIds {
			if existingRef == delRef {
				existingFound = true
			}
		}

		// remove reference (from disassociated) if it does exist in that list
		if existingFound {
			cc.AssociatedItemIds = removeStringFromSlice(delRef, cc.AssociatedItemIds)
		}
	}
}

func (cc *ComponentContent) SetText(input string) {
	panic("implement me")
}

func (cc ComponentContent) GetText() string {
	return ""
}

func (cc *ComponentContent) GetUpdateTime() (time.Time, error) {
	if cc.AppData.OrgStandardNotesSN.ClientUpdatedAt == "" {
		return time.Time{}, fmt.Errorf("notset")
	}

	return time.Parse(timeLayout, cc.AppData.OrgStandardNotesSN.ClientUpdatedAt)
}

func (cc *ComponentContent) SetUpdateTime(uTime time.Time) {
	cc.AppData.OrgStandardNotesSN.ClientUpdatedAt = uTime.Format(timeLayout)
}

func (cc ComponentContent) GetTitle() string {
	return ""
}

func (cc *ComponentContent) GetName() string {
	return cc.Name
}

func (cc *ComponentContent) GetActive() bool {
	return cc.Active.(bool)
}

func (cc *ComponentContent) SetTitle(title string) {
}

func (cc *ComponentContent) GetAppData() AppDataContent {
	return cc.AppData
}

func (cc *ComponentContent) SetAppData(data AppDataContent) {
	cc.AppData = data
}

func (cc ComponentContent) References() ItemReferences {
	return cc.ItemReferences
}

type ItemReferences []ItemReference

type Items []Item

func parseNote(i DecryptedItem) Item {
	n := Note{}
	n.UUID = i.UUID
	n.ContentType = i.ContentType
	n.Deleted = i.Deleted
	n.UpdatedAt = i.UpdatedAt
	n.CreatedAt = i.CreatedAt
	var err error
	if ! n.Deleted {
		var content Content
		content, err = processContentModel(i.ContentType, i.Content)
		if err != nil {
			panic(err)
		}
		n.Content = content.(NoteContent)
	}

	var cAt, uAt time.Time

	cAt, err = time.Parse(timeLayout, i.CreatedAt)
	if err != nil {
		panic(err)
	}

	n.CreatedAt = cAt.Format(timeLayout)

	uAt, err = time.Parse(timeLayout, i.UpdatedAt)
	if err != nil {
		panic(err)
	}

	n.UpdatedAt = uAt.Format(timeLayout)

	return &n
}

func parseTag(i DecryptedItem) Item {
	t := Tag{}
	t.UUID = i.UUID
	t.ContentType = i.ContentType
	t.Deleted = i.Deleted
	t.UpdatedAt = i.UpdatedAt
	t.CreatedAt = i.CreatedAt
	var err error
	if ! t.Deleted {
		var content Content
		content, err = processContentModel(i.ContentType, i.Content)
		if err != nil {
			panic(err)
		}
		t.Content = content.(TagContent)
	}

	var cAt, uAt time.Time

	cAt, err = time.Parse(timeLayout, i.CreatedAt)
	if err != nil {
		panic(err)
	}

	t.CreatedAt = cAt.Format(timeLayout)

	uAt, err = time.Parse(timeLayout, i.UpdatedAt)
	if err != nil {
		panic(err)
	}

	t.UpdatedAt = uAt.Format(timeLayout)

	return &t
}

func parseComponent(i DecryptedItem) Item {
	c := Component{}
	c.UUID = i.UUID
	c.ContentType = i.ContentType
	c.Deleted = i.Deleted
	c.UpdatedAt = i.UpdatedAt
	c.CreatedAt = i.CreatedAt
	var err error
	if ! c.Deleted {
		var content Content
		content, err = processContentModel(i.ContentType, i.Content)
		if err != nil {
			panic(err)
		}
		c.Content = content.(ComponentContent)
	}

	var cAt, uAt time.Time

	cAt, err = time.Parse(timeLayout, i.CreatedAt)
	if err != nil {
		panic(err)
	}

	c.CreatedAt = cAt.Format(timeLayout)

	uAt, err = time.Parse(timeLayout, i.UpdatedAt)
	if err != nil {
		panic(err)
	}

	c.UpdatedAt = uAt.Format(timeLayout)

	return &c
}

func (di *DecryptedItems) Parse() (p Items, err error) {
	for _, i := range *di {

		var pi Item
		switch i.ContentType {
		case "Note":
			pi = parseNote(i)
		case "Tag":
			pi = parseTag(i)
		case "SN|Component":
			pi = parseComponent(i)
		}

		//var processedItem Item

		p = append(p, pi)
	}

	return p, err
}

func processContentModel(contentType, input string) (output Content, err error) {
	// identify content model
	// try and unmarshall Item

	switch contentType {
	case "Note":
		var nc NoteContent
		err = json.Unmarshal([]byte(input), &nc)
		return nc, err
	case "Tag":
		var tc TagContent
		err = json.Unmarshal([]byte(input), &tc)

		return tc, err
	case "SN|Component":
		var cc ComponentContent
		err = json.Unmarshal([]byte(input), &cc)
		return cc, err
	}

	return
}

func (ei *EncryptedItems) DeDupe() {
	var encountered []string

	var deDuped EncryptedItems

	for _, i := range *ei {
		if !stringInSlice(i.UUID, encountered, true) {
			deDuped = append(deDuped, i)
		}

		encountered = append(encountered, i.UUID)
	}

	*ei = deDuped
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
	var encountered []string

	var deDuped Items

	for _, j := range *i {
		if !stringInSlice(j.GetUUID(), encountered, true) {
			deDuped = append(deDuped, j)
		}

		encountered = append(encountered, j.GetUUID())
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

func (n *Notes) RemoveDeleted() {
	var clean Notes

	for _, j := range *n {
		if !j.IsDeleted() {
			clean = append(clean, j)
		}
	}

	*n = clean
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

func (tagContent TagContent) Equals(e TagContent) bool {
	// TODO: compare references
	return tagContent.Title == e.Title
}

//func (item Item) Equals(e Item) bool {
//	if item.GetUUID() != e.GetUUID() {
//		return false
//	}
//
//	if item.GetContentType() != e.GetContentType() {
//		return false
//	}
//
//	if item.IsDeleted() != e.IsDeleted() {
//		return false
//	}
//
//	if item.GetContent().GetTitle() != e.GetContent().GetTitle() {
//		return false
//	}
//
//	if item.GetContent().GetText() != e.GetContent().GetText() {
//		return false
//	}
//
//	return true
//}

func (n Note) Equals(e Note) bool {
	if n.UUID != e.UUID {
		return false
	}

	if n.ContentType != e.ContentType {
		return false
	}

	if n.Deleted != e.Deleted {
		return false
	}

	if n.Content.Title != e.Content.Title {
		return false
	}

	if n.Content.Text != e.Content.Text {
		return false
	}

	return true
}

func (noteContent NoteContent) Copy() NoteContent {
	res := *new(NoteContent)
	res.Title = noteContent.Title
	res.Text = noteContent.Text
	res.AppData = noteContent.AppData
	res.ItemReferences = noteContent.ItemReferences

	return res
}
func (tagContent TagContent) Copy() TagContent {
	res := *new(TagContent)
	res.Title = tagContent.Title
	res.AppData = tagContent.AppData
	res.ItemReferences = tagContent.ItemReferences

	return res
}

//
//func (n Note) Copy() *Note {
//	res := new(Item)
//
//	switch item.Content.(type) {
//	case *NoteContent:
//		tContent := item.Content.(*NoteContent)
//		res.Content = tContent.Copy()
//	case *TagContent:
//		tContent := item.Content.(*TagContent)
//		res.Content = tContent.Copy()
//	default:
//		fmt.Printf("unable to copy items with content of type: %s", reflect.TypeOf(item.Content))
//	}
//
//	res.UpdatedAt = item.UpdatedAt
//	res.CreatedAt = item.CreatedAt
//	res.ContentSize = item.ContentSize
//	res.ContentType = item.ContentType
//	res.UUID = item.UUID
//
//	return res
//}

func (n Note) Copy() Note {
	c := NewNote()
	tContent := n.Content
	c.Content = tContent.Copy()
	c.UpdatedAt = n.UpdatedAt
	c.CreatedAt = n.CreatedAt
	c.ContentSize = n.ContentSize
	c.ContentType = n.ContentType
	c.UUID = n.UUID

	return c
}

func (t Tag) Copy() Tag {
	c := NewTag()
	tContent := t.Content
	c.Content = tContent.Copy()
	c.UpdatedAt = t.UpdatedAt
	c.CreatedAt = t.CreatedAt
	c.ContentSize = t.ContentSize
	c.ContentType = t.ContentType
	c.UUID = t.UUID

	return c
}
