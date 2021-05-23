package gosn

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/matryer/try"
)

// SyncInput defines the input for retrieving items
type SyncInput struct {
	Session     *Session
	SyncToken   string
	CursorToken string
	Items       EncryptedItems
	NextItem    int // the next item to put
	OutType     string
	BatchSize   int // number of items to retrieve
	PageSize    int // override default number of items to request with each sync call
}

// SyncOutput defines the output from retrieving items
// It contains slices of items based on their state
// see: https://standardfile.org/ for state details
type SyncOutput struct {
	Items      EncryptedItems  // items new or modified since last sync
	SavedItems EncryptedItems  // dirty items needing resolution
	Unsaved    EncryptedItems  // items not saved during sync TODO: No longer needed? Replaced by Conflicts?
	Conflicts  ConflictedItems // items not saved during sync due to significant difference in updated_time values. can be triggered by import where the server item has been updated since export.
	SyncToken  string

	Cursor string
}

type ConflictedItem struct {
	ServerItem EncryptedItem `json:"server_item"`
	Type       string
}

// Sync retrieves items from the API using optional filters and updates the provided
// session with the items keys required to encrypt and decrypt items
func Sync(input SyncInput) (output SyncOutput, err error) {
	debug := input.Session.Debug
	// if items have been passed but no default items key exists then return error
	if len(input.Items) > 0 && input.Session.DefaultItemsKey.ItemsKey == "" {
		err = fmt.Errorf("missing default items key in session")
	}

	for _, a := range input.Items {
		if a.ContentType == "SN|ItemsKey" && a.Deleted {
			panic("trying to delete SN|ItemsKey")
		}
	}

	giStart := time.Now()
	defer func() {
		debugPrint(debug, fmt.Sprintf("Sync | duration %v", time.Since(giStart)))
	}()

	if !input.Session.Valid() {
		err = fmt.Errorf("session is invalid")
		return
	}

	var sResp syncResponse

	debugPrint(debug, fmt.Sprintf("Sync | PageSize %d", input.PageSize))
	// retry logic is to handle responses that are too large
	// so we can reduce number we retrieve with each sync request
	start := time.Now()
	rErr := try.Do(func(attempt int) (bool, error) {
		debugPrint(debug, fmt.Sprintf("Sync | attempt %d", attempt))
		var rErr error

		sResp, rErr = syncItemsViaAPI(input)
		if rErr != nil && strings.Contains(strings.ToLower(rErr.Error()), "too large") {
			debugPrint(debug, fmt.Sprintf("Sync | %s", rErr.Error()))
			input.NextItem = sResp.LastItemPut
			resizeForRetry2(&input)
			debugPrint(debug, fmt.Sprintf("Sync | failed to retrieve %d items "+
				"at a time so reducing to %d", sResp.PutLimitUsed, input.PageSize))
		}
		return attempt < 3, rErr
	})

	if rErr != nil {
		return output, rErr
	}

	elapsed := time.Since(start)

	debugPrint(debug, fmt.Sprintf("Sync | took %v to get all items", elapsed))

	postStart := time.Now()
	output.Items = sResp.Items
	output.Items.DeDupe()
	output.Unsaved = sResp.Unsaved
	output.Unsaved.DeDupe()
	output.SavedItems = sResp.SavedItems
	output.SavedItems.DeDupe()
	output.Conflicts = sResp.Conflicts
	output.Conflicts.DeDupe()
	output.Cursor = sResp.CursorToken
	output.SyncToken = sResp.SyncToken
	// strip any duplicates (https://github.com/standardfile/rails-engine/issues/5)
	postElapsed := time.Since(postStart)
	debugPrint(debug, fmt.Sprintf("Sync | post processing took %v", postElapsed))
	debugPrint(debug, fmt.Sprintf("Sync | sync token: %+v", stripLineBreak(output.SyncToken)))
	if err = output.Conflicts.Validate(); err != nil {
		panic(err)
	}

	if err = output.Items.Validate(); err != nil {
		panic(err)
	}

	if len(output.Items) > 0 {
		_, err = output.Items.DecryptAndParseItemsKeys(input.Session)
	}

	return output, err
}

type ConflictedItems []ConflictedItem

func (cis *ConflictedItems) DeDupe() {
	var encountered []string

	var deDuped ConflictedItems

	for _, ci := range *cis {
		if !stringInSlice(ci.ServerItem.UUID, encountered, true) {
			deDuped = append(deDuped, ci)
		}

		encountered = append(encountered, ci.ServerItem.UUID)
	}

	*cis = deDuped
}

func (cis ConflictedItems) Validate() error {
	for _, ci := range cis {
		switch ci.Type {
		case "sync_conflict":
			continue
		case "uuid_conflict":
			return fmt.Errorf("uuid_conflict is currently unhandled\nplease raise an issue at https://github.com/jonhadfield/gosn-v2")
		default:
			return fmt.Errorf("%s conflict type is currently unhandled\nplease raise an issue at https://github.com/jonhadfield/gosn-v2", ci.Type)
		}
	}

	return nil
}

func lesserOf(first, second int) int {
	if first < second {
		if first < 0 {
			return 0
		}

		return first
	}

	if second < 0 {
		return 0
	}

	return second
}

func syncItemsViaAPI(input SyncInput) (out syncResponse, err error) {
	debug := input.Session.Debug
	debugPrint(debug, fmt.Sprintf("syncItemsViaAPI | SyncInput: NextItem: %d", input.NextItem))
	// determine how many items to retrieve with each call
	var limit int

	// DEBUG START
	for _, a := range input.Items {
		if a.ContentType == "SN|ItemsKey" {
			if a.Deleted {
				panic("TRYING TO DELETE ITEMS KEY")
			}
		}
		if a.ItemsKeyID == "" {
			panic("Trying to sync item without itemskeyid")
		}
	}
	// DEBUG END

	switch {
	case input.BatchSize > 0:
		debugPrint(debug, fmt.Sprintf("syncItemsViaAPI | input.BatchSize: %d", input.BatchSize))
		// batch size must be lower than or equal to page size
		limit = input.BatchSize
	case input.PageSize > 0:
		debugPrint(debug, fmt.Sprintf("syncItemsViaAPI | input.PageSize: %d", input.PageSize))
		limit = input.PageSize
	default:
		debugPrint(debug, fmt.Sprintf("syncItemsViaAPI | using default limit: %d", PageSize))
		limit = PageSize
	}

	out.PutLimitUsed = limit

	var encItemJSON []byte

	itemsToPut := input.Items

	var finalItem int

	if len(input.Items) > 0 {
		finalItem = lesserOf(len(input.Items)-1, input.NextItem+limit-1)
		debugPrint(debug, fmt.Sprintf("syncItemsViaAPI | going to put items: %d to %d", input.NextItem, finalItem))

		encItemJSON, err = json.Marshal(itemsToPut[input.NextItem : finalItem+1])
		if err != nil {
			panic(err)
		}
	}

	var requestBody []byte
	// generate request body
	debugPrint(debug, fmt.Sprintf("syncItemsViaAPI | items to put %d", len(input.Items)))
	newST := stripLineBreak(input.SyncToken) + `\n`

	switch {
	case input.CursorToken == "":
		debugPrint(debug, "syncItemsViaAPI | cursor is empty")

		if len(input.Items) == 0 {

			if input.SyncToken == "" {
				requestBody = []byte(`{"api":"20200115","items":[],"compute_integrity":true,"limit":` + strconv.Itoa(limit) + `}`)
			} else {
				requestBody = []byte(`{"api":"20200115","items":[],"compute_integrity":true,"limit":` + strconv.Itoa(limit) + `,"sync_token":"` + newST + `"}`)
			}
		} else {
			if input.SyncToken == "" {
				requestBody = []byte(`{"api":"20200115","compute_integrity":true,"limit":` + strconv.Itoa(limit) + `,"items":` + string(encItemJSON) + `}`)
			} else {
				requestBody = []byte(`{"api":"20200115","compute_integrity":true,"limit":` + strconv.Itoa(limit) + `,"items":` + string(encItemJSON) +
					`,"sync_token":"` + newST + `"}`)
			}
		}

	case input.CursorToken == "null":
		debugPrint(debug, "syncItemsViaAPI | cursor is null")

		if input.SyncToken == "" {
			requestBody = []byte(`{"api":"20200115","items":[],"compute_integrity":true,"limit":` + strconv.Itoa(limit) +
				`,"items":[],"cursor_token":null}`)
		} else {
			requestBody = []byte(`{"api":"20200115","items":[],"compute_integrity":true,"limit":` + strconv.Itoa(limit) +
				`,"items":[],"sync_token":"` + newST + `","cursor_token":null}`)
		}

	case input.CursorToken != "":
		debugPrint(debug, fmt.Sprintf("syncItemsViaAPI | got cursor %s", stripLineBreak(input.CursorToken)))

		rawST := input.SyncToken

		input.SyncToken = stripLineBreak(rawST)

		requestBody = []byte(`{"limit":` + strconv.Itoa(limit) +
			`,"items":[],"compute_integrity":true,"sync_token":"` + newST + `","cursor_token":"` + stripLineBreak(input.CursorToken) + `\n"}`)
	}

	// make the request
	debugPrint(debug, fmt.Sprintf("syncItemsViaAPI | making request: %s", stripLineBreak(string(requestBody))))

	msrStart := time.Now()

	var responseBody []byte
	responseBody, err = makeSyncRequest(*input.Session, requestBody)
	msrEnd := time.Since(msrStart)
	debugPrint(debug, fmt.Sprintf("syncItemsViaAPI | makeSyncRequest took: %v", msrEnd))

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

	debugPrint(debug, fmt.Sprintf("syncItemsViaAPI | Saved %d items", len(out.SavedItems)))
	debugPrint(debug, fmt.Sprintf("syncItemsViaAPI | Retrieved %d items", len(out.Items)))
	debugPrint(debug, fmt.Sprintf("syncItemsViaAPI | Unsaved %d items", len(out.Unsaved)))
	debugPrint(debug, fmt.Sprintf("syncItemsViaAPI | Conflict %d items", len(out.Conflicts)))
	out.Unsaved = bodyContent.Unsaved
	out.SyncToken = bodyContent.SyncToken
	out.CursorToken = bodyContent.CursorToken
	out.Conflicts = bodyContent.Conflicts
	out.LastItemPut = finalItem

	if input.BatchSize > 0 {
		return
	}

	debugPrint(debug, fmt.Sprintf("syncItemsViaAPI | final item put: %d total items to put: %d", finalItem, len(input.Items)))

	if (finalItem > 0 && finalItem < len(input.Items)-1) || (bodyContent.CursorToken != "" && bodyContent.CursorToken != "null") {
		var newOutput syncResponse

		input.SyncToken = out.SyncToken
		debugPrint(debug, fmt.Sprintf("syncItemsViaAPI | setting input sync token: %s", stripLineBreak(input.SyncToken)))

		input.CursorToken = out.CursorToken
		debugPrint(debug, fmt.Sprintf("syncItemsViaAPI | setting input cursor token: %s", stripLineBreak(input.CursorToken)))

		input.PageSize = limit
		// sync was successful so set new item
		if finalItem > 0 {
			debugPrint(debug, fmt.Sprintf("syncItemsViaAPI | sync successful so setting new item to finalItem+1: %d", finalItem+1))
			input.NextItem = finalItem + 1
		}

		newOutput, err = syncItemsViaAPI(input)

		if err != nil {
			return
		}

		out.Items = append(out.Items, newOutput.Items...)
		out.SavedItems = append(out.SavedItems, newOutput.SavedItems...)
		out.Unsaved = append(out.Unsaved, newOutput.Unsaved...)
		out.Conflicts = append(out.Conflicts, newOutput.Conflicts...)

		debugPrint(debug, fmt.Sprintf("syncItemsViaAPI | setting out.LastItemPut to: %d", finalItem))
		out.LastItemPut = finalItem
	} else {
		return out, err
	}

	out.CursorToken = ""

	return out, err
}

func resizeForRetry2(in *SyncInput) {
	switch {
	case in.BatchSize != 0:
		in.BatchSize = int(math.Ceil(float64(in.BatchSize) * retryScaleFactor))
	case in.PageSize != 0:
		in.PageSize = int(math.Ceil(float64(in.PageSize) * retryScaleFactor))
	default:
		in.PageSize = int(math.Ceil(float64(PageSize) * retryScaleFactor))
	}
}
