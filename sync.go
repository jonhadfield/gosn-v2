package gosn

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/matryer/try"
)

// SyncInput defines the input for retrieving items.
type SyncInput struct {
	Session              *Session
	SyncToken            string
	CursorToken          string
	Items                EncryptedItems
	NextItem             int // the next item to put
	OutType              string
	PageSize             int   // override default number of items to request with each sync call
	PostSyncRequestDelay int64 // milliseconds to sleep after sync request
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
	ServerItem  EncryptedItem `json:"server_item"`
	UnsavedItem EncryptedItem `json:"unsaved_item"`
	Type        string
}

// Sync retrieves items from the API using optional filters and updates the provided
// session with the items keys required to encrypt and decrypt items.
func Sync(input SyncInput) (output SyncOutput, err error) {
	// a different items key may be provided in case the items being synced are encrypted with a non-default items key
	// we need to reset on completion it to avoid it being used in future
	defer func() { input.Session.ImporterItemsKey = ItemsKey{} }()

	debug := input.Session.Debug
	// if items have been passed but no default items key exists then return error
	if len(input.Items) > 0 && input.Session.DefaultItemsKey.ItemsKey == "" {
		err = fmt.Errorf("missing default items key in session")
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

	// check if we need to add a post sync request delay
	psrd := os.Getenv("SN_POST_SYNC_REQUEST_DELAY")
	if psrd != "" {
		input.PostSyncRequestDelay, err = strconv.ParseInt(psrd, 10, 64)
		if err != nil {
			err = fmt.Errorf("invalid SN_POST_SYNC_REQUEST_DELAY value: %v", err)
			return
		}

		debugPrint(debug, fmt.Sprintf("syncItemsViaAPI | sleeping %d milliseconds post each sync request",
			input.PostSyncRequestDelay))
	}

	// retry logic is to handle responses that are too large
	// so we can reduce number we retrieve with each sync request
	start := time.Now()
	rErr := try.Do(func(attempt int) (bool, error) {
		ps := PageSize
		if input.PageSize > 0 {
			ps = input.PageSize
		}
		debugPrint(debug, fmt.Sprintf("Sync | attempt %d with page size %d", attempt, ps))
		var rErr error

		sResp, rErr = syncItemsViaAPI(input)
		if rErr != nil {
			debugPrint(debug, fmt.Sprintf("Sync | %s", rErr.Error()))
			switch {
			case strings.Contains(strings.ToLower(rErr.Error()), "too large"):
				input.NextItem = sResp.LastItemPut
				resizeForRetry(&input)
				debugPrint(debug, fmt.Sprintf("Sync | failed to retrieve %d items "+
					"at a time as the request was too large so reducing to page size %d", sResp.PutLimitUsed, input.PageSize))
			case strings.Contains(strings.ToLower(rErr.Error()), "timeout"):
				input.NextItem = sResp.LastItemPut
				resizeForRetry(&input)
				debugPrint(debug, fmt.Sprintf("Sync | failed to retrieve %d items "+
					"at a time due to timeout so reducing to page size %d", sResp.PutLimitUsed, input.PageSize))
			case strings.Contains(strings.ToLower(rErr.Error()), "unauthorized"):
				input.NextItem = sResp.LastItemPut
				debugPrint(debug, "Sync | failed with '401 Unauthorized' which is most likely due to throttling")
				panic("failed to complete sync due to server throttling. please wait five minutes before retrying.")
			case strings.Contains(strings.ToLower(rErr.Error()), "EOF"):
				input.NextItem = sResp.LastItemPut
				resizeForRetry(&input)
				debugPrint(debug, fmt.Sprintf("Sync | failed to retrieve %d items "+
					"at a time due to EOF so reducing to page size %d", sResp.PutLimitUsed, input.PageSize))
			default:
				panic(fmt.Sprintf("sync returned unhandled error: %+v", rErr))
			}
		}

		return attempt < 3, rErr
	})

	if rErr != nil {
		return output, rErr
	}

	elapsed := time.Since(start)

	debugPrint(debug, fmt.Sprintf("Sync | took %v to get all items", elapsed))

	// postStart := time.Now()
	output.Items = sResp.Items
	output.Items.DeDupe()
	// output.Items.RemoveUnsupported()
	output.Unsaved = sResp.Unsaved
	output.Unsaved.DeDupe()
	output.SavedItems = sResp.SavedItems
	output.SavedItems.DeDupe()

	//for _, si := range output.SavedItems {
	//	debugPrint(debug, fmt.Sprintf("Sync | saved item: %s type: %s updated at timestamp: %d", si.UUID, si.ContentType, si.UpdatedAtTimestamp))
	//}

	output.Conflicts = sResp.Conflicts
	output.Conflicts.DeDupe()
	output.Cursor = sResp.CursorToken
	output.SyncToken = sResp.SyncToken
	// strip any duplicates (https://github.com/standardfile/rails-engine/issues/5)
	// postElapsed := time.Since(postStart)
	// debugPrint(debug, fmt.Sprintf("Sync | post processing took %v", postElapsed))
	// debugPrint(debug, fmt.Sprintf("Sync | sync token: %+v", stripLineBreak(output.SyncToken)))

	if err = output.Conflicts.Validate(debug); err != nil {
		panic(err)
	}

	if err = output.Items.Validate(); err != nil {
		panic(err)
	}

	// Resync any conflicts
	var conflictsToSync EncryptedItems

	if len(output.Conflicts) > 0 {
		debugPrint(debug, fmt.Sprintf("Sync | found %d conflicts", len(output.Conflicts)))

		for _, conflict := range output.Conflicts {
			var conflictedItem EncryptedItem

			if conflict.Type == "sync_conflict" {
				switch {
				case conflict.ServerItem.Deleted:
					// if server item is deleted then we will give unsaved item a new uuid and sync it
					debugPrint(debug, fmt.Sprintf("Sync | server item uuid %s type %s is deleted so replace", conflict.ServerItem.UUID, conflict.ServerItem.ContentType))

					var found bool

					for _, item := range input.Items {
						if item.UUID == conflict.ServerItem.UUID {
							found = true
							item.UpdatedAtTimestamp = conflict.ServerItem.UpdatedAtTimestamp
							conflictedItem = item

							break
						}
					}

					if !found {
						panic("could not find item that failed to sync")
					}

				case conflict.UnsavedItem.UpdatedAtTimestamp > conflict.ServerItem.UpdatedAtTimestamp:
					// if unsaved item is newer than that our server item, then unsaved wins
					debugPrint(debug, fmt.Sprintf("Sync | unsaved is most recent so updating its updated_at_timestamp to servers: %d", conflict.ServerItem.UpdatedAtTimestamp))

					conflictedItem = conflict.UnsavedItem
					conflictedItem.UpdatedAtTimestamp = conflict.ServerItem.UpdatedAtTimestamp
				default:
					debugPrint(debug, "Sync | server item most recent, so set new UUID on the item that conflicted and set it as 'duplicate_of' original")

					var found bool

					for _, item := range input.Items {
						if item.UUID == conflict.ServerItem.UUID {
							if item.Deleted {
								item = conflict.ServerItem
								item.Deleted = true
								item.Content = ""
								conflictedItem = item
								found = true

								break
							}

							conflictedItem = item
							// decrypt server item
							is := EncryptedItems{item}
							var dis Items

							dis, err = is.DecryptAndParse(input.Session)
							if err != nil {
								return
							}

							di := dis[0]
							// set new id
							di.SetUUID(GenUUID())
							// re-encrypt to update auth data
							newdis := Items{di}

							var newis EncryptedItems

							k := input.Session.DefaultItemsKey
							// if the conflict is during import, then we need to re-encrypt with Importer Key
							if input.Session.ImporterItemsKey.ItemsKey != "" {
								k = input.Session.ImporterItemsKey
							}

							newis, err = newdis.Encrypt(k, input.Session.MasterKey, input.Session.Debug)
							if err != nil {
								return
							}

							newi := newis[0]
							newis[0].DuplicateOf = &conflict.ServerItem.UUID
							conflictedItem = newi

							found = true

							break
						}
					}

					if !found {
						panic("could not find item that failed to sync")
					}
				}

				conflictsToSync = append(conflictsToSync, conflictedItem)
			}

			if conflict.Type == "uuid_conflict" {
				// give the item a new UUID
				conflictedItem = conflict.UnsavedItem

				debugPrint(debug, "Sync | item has uuid_conflict, so setting item's UUID to a new one")

				conflictedItem.UUID = GenUUID()

				conflictsToSync = append(conflictsToSync, conflictedItem)
			}
		}
	}

	if len(conflictsToSync) > 0 {
		// Call Sync Again and add the output to the output we've already got
		input.Items = conflictsToSync

		var resyncOutput SyncOutput

		resyncOutput, err = Sync(input)
		if err != nil {
			panic(err)
		}

		// we only expect to get saved items back from the new sync as these are conflicts being resolved
		if len(resyncOutput.Conflicts) > 0 {
			panic(fmt.Sprintf("we didn't expect to get any conflicts now, but got: %d", len(resyncOutput.Conflicts)))
		}
		// zero the conflicts as we've resolved them
		output.Conflicts = nil

		output.Items = append(output.Items, resyncOutput.Items...)
		output.SavedItems = append(output.SavedItems, resyncOutput.SavedItems...)
		output.Items.DeDupe()
		output.SavedItems.DeDupe()
	}

	items := append(output.Items, output.SavedItems...)
	items.DeDupe()

	var iks ItemsKeys

	if len(output.SavedItems) > 0 {
		// checking if we've saved a new items key, in which case it should be new default
		iks, err = output.SavedItems.DecryptAndParseItemsKeys(input.Session.MasterKey, input.Session.Debug)
	} else {
		// existing items key would be returned on first sync
		iks, err = output.Items.DecryptAndParseItemsKeys(input.Session.MasterKey, input.Session.Debug)
	}

	if err != nil {
		return
	}

	switch len(iks) {
	case 0:
		break
	default:
		input.Session.DefaultItemsKey = iks[0]
		input.Session.ItemsKeys = iks
	}

	return output, err
}

type ItemsKeys []ItemsKey

func (iks ItemsKeys) Valid() bool {
	seen := make(map[string]int)
	for x := range iks {
		seen[iks[x].UUID]++
		if seen[iks[x].UUID] > 1 {
			return false
		}
	}

	return true
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

func (cis ConflictedItems) Validate(debug bool) error {
	for _, ci := range cis {
		switch ci.Type {
		case "sync_conflict":
			debugPrint(debug, fmt.Sprintf("Sync | sync conflict of: \"%s\" with uuid: \"%s\"", ci.ServerItem.ContentType, ci.ServerItem.UUID))
			continue
		case "uuid_conflict":
			debugPrint(debug, fmt.Sprintf("Sync | uuid conflict of: \"%s\" with uuid: \"%s\"", ci.ServerItem.ContentType, ci.ServerItem.UUID))
			continue
		case "uuid_error":
			debugPrint(debug, "Sync | client is attempting to sync an item without uuid")
			panic("Sync | client is attempting to sync an item without a uuid")
		default:
			return fmt.Errorf("%s conflict type is currently unhandled\nplease raise an issue at https://github.com/jonhadfield/gosn-v2\nConflicted Item: %+v", ci.Type, ci)
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

	// determine how many items to retrieve with each call
	var limit int

	switch {
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
	newST := stripLineBreak(input.SyncToken) + `\n`

	switch {
	case input.CursorToken == "":
		if len(input.Items) == 0 {
			if input.SyncToken == "" {
				requestBody = []byte(`{"api":"20200115","items":[],"compute_integrity":false,"limit":` + strconv.Itoa(limit) + `}`)
			} else {
				requestBody = []byte(`{"api":"20200115","items":[],"compute_integrity":false,"limit":` + strconv.Itoa(limit) + `,"sync_token":"` + newST + `"}`)
			}
		} else {
			if input.SyncToken == "" {
				requestBody = []byte(`{"api":"20200115","compute_integrity":false,"limit":` + strconv.Itoa(limit) + `,"items":` + string(encItemJSON) + `}`)
			} else {
				requestBody = []byte(`{"api":"20200115","compute_integrity":false,"limit":` + strconv.Itoa(limit) + `,"items":` + string(encItemJSON) +
					`,"sync_token":"` + newST + `"}`)
			}
		}

	case input.CursorToken == "null":
		if input.SyncToken == "" {
			requestBody = []byte(`{"api":"20200115","items":[],"compute_integrity":false,"limit":` + strconv.Itoa(limit) +
				`,"items":[],"cursor_token":null}`)
		} else {
			requestBody = []byte(`{"api":"20200115","items":[],"compute_integrity":false,"limit":` + strconv.Itoa(limit) +
				`,"items":[],"sync_token":"` + newST + `","cursor_token":null}`)
		}

	case input.CursorToken != "":
		rawST := input.SyncToken

		input.SyncToken = stripLineBreak(rawST)

		requestBody = []byte(`{"limit":` + strconv.Itoa(limit) +
			`,"items":[],"compute_integrity":false,"sync_token":"` + newST + `","cursor_token":"` + stripLineBreak(input.CursorToken) + `\n"}`)
	}

	var responseBody []byte
	// fmt.Printf("requestBody: %s\n", string(requestBody))
	responseBody, err = makeSyncRequest(*input.Session, requestBody)
	// fmt.Printf("responseBody: %s\n", string(responseBody))
	if input.PostSyncRequestDelay > 0 {
		time.Sleep(time.Duration(input.PostSyncRequestDelay) * time.Millisecond)
	}

	if err != nil {
		return
	}

	// get encrypted items from API response
	var bodyContent syncResponse

	bodyContent, err = unmarshallSyncResponse(responseBody)
	if err != nil {
		return
	}

	out.Items = bodyContent.Items
	out.SavedItems = bodyContent.SavedItems
	out.Unsaved = bodyContent.Unsaved
	out.SyncToken = bodyContent.SyncToken
	out.CursorToken = bodyContent.CursorToken
	out.Conflicts = bodyContent.Conflicts
	out.LastItemPut = finalItem

	if len(input.Items) > 0 {
		debugPrint(debug, fmt.Sprintf("syncItemsViaAPI | final item put: %d total items to put: %d", finalItem, len(input.Items)))
	}

	if (finalItem > 0 && finalItem < len(input.Items)-1) || (bodyContent.CursorToken != "" && bodyContent.CursorToken != "null") {
		var newOutput syncResponse

		input.SyncToken = out.SyncToken
		// debugPrint(debug, fmt.Sprintf("syncItemsViaAPI | setting input sync token: %s", stripLineBreak(input.SyncToken)))

		input.CursorToken = out.CursorToken
		// debugPrint(debug, fmt.Sprintf("syncItemsViaAPI | setting input cursor token: %s", stripLineBreak(input.CursorToken)))

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
		out.SyncToken = newOutput.SyncToken

		out.LastItemPut = finalItem
	} else {
		return out, err
	}

	out.CursorToken = ""

	return out, err
}

func resizeForRetry(in *SyncInput) {
	if in.PageSize != 0 {
		in.PageSize = int(math.Ceil(float64(in.PageSize) * retryScaleFactor))
	} else {
		in.PageSize = int(math.Ceil(float64(PageSize) * retryScaleFactor))
	}
}
