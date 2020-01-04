package gosn

import (
	"encoding/json"
	"fmt"
	"github.com/matryer/try"
	"math"
	"strconv"
	"strings"
	"time"
)

// GetItemsInput defines the input for retrieving items
type SyncItemsInput struct {
	Session     Session
	SyncToken   string
	CursorToken string
	Items       EncryptedItems
	OutType     string
	BatchSize   int // number of items to retrieve
	PageSize    int // override default number of items to request with each sync call
	Debug       bool
}

// GetItemsOutput defines the output from retrieving items
// It contains slices of items based on their state
// see: https://standardfile.org/ for state details
type SyncItemsOutput struct {
	Items      EncryptedItems // items new or modified since last sync
	SavedItems EncryptedItems // dirty items needing resolution
	Unsaved    EncryptedItems // items not saved during sync
	SyncToken  string
	Cursor     string
}

// GetItems retrieves items from the API using optional filters
func SyncItems(input SyncItemsInput) (output SyncItemsOutput, err error) {
	giStart := time.Now()

	defer func() {
		debugPrint(input.Debug, fmt.Sprintf("SyncItems | duration %v", time.Since(giStart)))
	}()

	if !input.Session.Valid() {
		err = fmt.Errorf("session is invalid")
		return
	}

	var sResp syncResponse

	debugPrint(input.Debug, fmt.Sprintf("SyncItems | PageSize %d", input.PageSize))
	// retry logic is to handle responses that are too large
	// so we can reduce number we retrieve with each sync request
	start := time.Now()
	rErr := try.Do(func(attempt int) (bool, error) {
		debugPrint(input.Debug, fmt.Sprintf("SyncItems | attempt %d", attempt))
		var rErr error

		sResp, rErr = syncItemsViaAPI(input)
		if rErr != nil && strings.Contains(strings.ToLower(rErr.Error()), "too large") {
			debugPrint(input.Debug, fmt.Sprintf("SyncItems | %s", rErr.Error()))
			initialSize := input.PageSize
			resizeForRetry2(&input)
			debugPrint(input.Debug, fmt.Sprintf("SyncItems | failed to retrieve %d items "+
				"at a time so reducing to %d", initialSize, input.PageSize))
		}
		return attempt < 3, rErr
	})

	if rErr != nil {
		return output, rErr
	}

	elapsed := time.Since(start)

	debugPrint(input.Debug, fmt.Sprintf("SyncItems | took %v to get all items", elapsed))

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
	debugPrint(input.Debug, fmt.Sprintf("SyncItems | post processing took %v", postElapsed))
	debugPrint(input.Debug, fmt.Sprintf("SyncItems | sync token: %+v", stripLineBreak(output.SyncToken)))

	return output, err
}

func syncItemsViaAPI(input SyncItemsInput) (out syncResponse, err error) {
	debugPrint(input.Debug, fmt.Sprintf("syncItemsViaAPI | items: %+v", input.Items))

	// determine how many items to retrieve with each call
	var limit int

	switch {
	case input.BatchSize > 0:
		debugPrint(input.Debug, fmt.Sprintf("syncItemsViaAPI |input.BatchSize: %d", input.BatchSize))
		// batch size must be lower than or equal to page size
		limit = input.BatchSize
	case input.PageSize > 0:
		debugPrint(input.Debug, fmt.Sprintf("syncItemsViaAPI | input.PageSize: %d", input.PageSize))
		limit = input.PageSize
	default:
		debugPrint(input.Debug, fmt.Sprintf("syncItemsViaAPI | default - limit: %d", PageSize))
		limit = PageSize
	}

	debugPrint(input.Debug, fmt.Sprintf("syncItemsViaAPI | using limit: %d", limit))

	var encItemJSON []byte
	itemsToPut := input.Items
	encItemJSON, err = json.Marshal(itemsToPut)
	if err != nil {
		panic(err)
	}
	var requestBody []byte
	// generate request body
	switch {
	case input.CursorToken == "":
		if len(input.Items) == 0 {
			requestBody = []byte(`{"limit":` + strconv.Itoa(limit) + `}`)
		} else {
			requestBody = []byte(`{"limit":` + strconv.Itoa(limit) + `,"items":` + string(encItemJSON) +
				`,"sync_token":"` + stripLineBreak(input.SyncToken) + `"}`)
		}
	case input.CursorToken == "null":
		debugPrint(input.Debug, "syncItemsViaAPI | cursor is null")

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
	debugPrint(input.Debug, fmt.Sprintf("syncItemsViaAPI | making request: %s", stripLineBreak(string(requestBody))))

	msrStart := time.Now()
	responseBody, err := makeSyncRequest(input.Session, requestBody, input.Debug)
	msrEnd := time.Since(msrStart)
	debugPrint(input.Debug, fmt.Sprintf("syncItemsViaAPI | makeSyncRequest took: %v", msrEnd))

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

		newOutput, err = syncItemsViaAPI(input)

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

func resizeForRetry2(in *SyncItemsInput) {
	switch {
	case in.BatchSize != 0:
		in.BatchSize = int(math.Ceil(float64(in.BatchSize) * retryScaleFactor))
	case in.PageSize != 0:
		in.PageSize = int(math.Ceil(float64(in.PageSize) * retryScaleFactor))
	default:
		in.PageSize = int(math.Ceil(float64(PageSize) * retryScaleFactor))
	}
}
