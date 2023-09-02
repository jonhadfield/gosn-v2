package gosn

import (
	"encoding/json"
	"fmt"
	"github.com/matryer/try"
	"math"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"
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

func syncItems(i SyncInput) (so SyncOutput, err error) {
	fmt.Printf("syncItems called with %d items\n", len(i.Items))
	for _, x := range i.Items {
		fmt.Printf("----- %s %s %s\n", x.UUID, x.ItemsKeyID, x.EncItemKey)
	}

	giStart := time.Now()
	defer func() {
		debugPrint(i.Session.Debug, fmt.Sprintf("Sync | duration %v", time.Since(giStart)))
	}()

	if !i.Session.Valid() {
		err = fmt.Errorf("session is invalid")
		return
	}

	var sResp syncResponse

	// check if we need to add a post sync request delay
	psrd := os.Getenv("SN_POST_SYNC_REQUEST_DELAY")
	if psrd != "" {
		i.PostSyncRequestDelay, err = strconv.ParseInt(psrd, 10, 64)
		if err != nil {
			err = fmt.Errorf("invalid SN_POST_SYNC_REQUEST_DELAY value: %v", err)
			return
		}

		debugPrint(i.Session.Debug, fmt.Sprintf("syncItemsViaAPI | sleeping %d milliseconds post each sync request",
			i.PostSyncRequestDelay))
	}

	// retry logic is to handle responses that are too large
	// so we can reduce number we retrieve with each sync request
	start := time.Now()
	rErr := try.Do(func(attempt int) (bool, error) {
		ps := PageSize
		if i.PageSize > 0 {
			ps = i.PageSize
		}
		debugPrint(i.Session.Debug, fmt.Sprintf("Sync | attempt %d with page size %d", attempt, ps))
		var rErr error

		sResp, rErr = syncItemsViaAPI(i)
		if rErr != nil {
			debugPrint(i.Session.Debug, fmt.Sprintf("Sync | %s", rErr.Error()))
			switch {
			case strings.Contains(strings.ToLower(rErr.Error()), "session token") &&
				strings.Contains(strings.ToLower(rErr.Error()), "expired"):
				fmt.Printf("\nerr: %s\n\nplease log in again", rErr)
				os.Exit(1)
			case strings.Contains(strings.ToLower(rErr.Error()), "too large"):
				i.NextItem = sResp.LastItemPut
				resizeForRetry(&i)
				debugPrint(i.Session.Debug, fmt.Sprintf("Sync | failed to retrieve %d items "+
					"at a time as the request was too large so reducing to page size %d",
					sResp.PutLimitUsed, i.PageSize))
			case strings.Contains(strings.ToLower(rErr.Error()), "timeout"):
				i.NextItem = sResp.LastItemPut
				resizeForRetry(&i)
				debugPrint(i.Session.Debug, fmt.Sprintf("Sync | failed to retrieve %d items "+
					"at a time due to timeout so reducing to page size %d", sResp.PutLimitUsed, i.PageSize))
			case strings.Contains(strings.ToLower(rErr.Error()), "unauthorized"):
				i.NextItem = sResp.LastItemPut
				debugPrint(i.Session.Debug, "Sync | failed with '401 Unauthorized' which is most likely due to throttling")
				panic("failed to complete sync due to server throttling. please wait five minutes before retrying.")
			case strings.Contains(strings.ToLower(rErr.Error()), "EOF"):
				i.NextItem = sResp.LastItemPut
				resizeForRetry(&i)
				debugPrint(i.Session.Debug, fmt.Sprintf("Sync | failed to retrieve %d items "+
					"at a time due to EOF so reducing to page size %d", sResp.PutLimitUsed, i.PageSize))
			default:
				panic(fmt.Sprintf("sync returned unhandled error: %+v", rErr))
			}
		}

		return attempt < 3, rErr
	})

	if rErr != nil {
		return so, fmt.Errorf("sync | %w", rErr)
	}

	elapsed := time.Since(start)

	debugPrint(i.Session.Debug, fmt.Sprintf("Sync | took %v to get all items", elapsed))

	so.Items = sResp.Items
	so.Items.DeDupe()
	so.Unsaved = sResp.Unsaved
	so.Unsaved.DeDupe()
	so.SavedItems = sResp.SavedItems
	so.SavedItems.DeDupe()
	so.Conflicts = sResp.Conflicts
	so.Conflicts.DeDupe()
	so.Cursor = sResp.CursorToken
	so.SyncToken = sResp.SyncToken

	// update timestamps on saved items
	so.SavedItems = updateTimestampsOnSavedItems(i.Items, so.SavedItems)

	debugPrint(i.Session.Debug,
		fmt.Sprintf("Sync | SN returned %d items, %d saved items, and %d conflicts, with syncToken %s",
			len(so.Items), len(so.SavedItems), len(so.Conflicts), so.SyncToken))

	return
}

func updateTimestampsOnSavedItems(orig, synced EncryptedItems) (updatedSaved EncryptedItems) {
	// for each saved item, update the times on the input items	}

	for x := range synced {
		// fmt.Printf("SAVED ***** %#+v\n", syncOutput.SavedItems[x])
		for y := range orig {
			if synced[x].UUID == orig[y].UUID {

				updated := orig[y]
				updated.Content = orig[x].Content
				updated.ItemsKeyID = orig[x].ItemsKeyID
				updated.EncItemKey = orig[x].EncItemKey
				updated.UpdatedAtTimestamp = synced[x].UpdatedAtTimestamp
				updated.UpdatedAt = synced[x].UpdatedAt
				updatedSaved = append(updatedSaved, updated)
			}
		}
	}

	return updatedSaved
}

// Sync retrieves items from the API using optional filters and updates the provided
// session with the items keys required to encrypt and decrypt items.
func Sync(input SyncInput) (output SyncOutput, err error) {

	// sync until all conflicts have been resolved
	// a different items key may be provided in case the items being synced are encrypted with a non-default items key
	// we need to reset on completion it to avoid it being used in future
	defer func() { input.Session.ImporterItemsKeys = ItemsKeys{} }()

	debugPrint(input.Session.Debug, fmt.Sprintf("Sync | called with %d items and syncToken %s", len(input.Items), input.SyncToken))
	debugPrint(input.Session.Debug, fmt.Sprintf("Sync | pre-sync default items key: %s", input.Session.DefaultItemsKey.UUID))
	// if items have been passed but no default items key exists then return error
	if len(input.Items) > 0 && input.Session.DefaultItemsKey.ItemsKey == "" {
		err = fmt.Errorf("missing default items key in session")
	}

	// duplicate items to be pushed so we can update their updated_at_timestamp if saved
	clonedItems := slices.Clone(input.Items)

	// perform initial sync
	output, err = syncItems(input)

	debugPrint(true, "INITIAL SYNC COMPLETE")
	for x := range output.SavedItems {
		fmt.Printf("SAVED (before processing) ***** %#+v\n", output.SavedItems[x])
	}
	processSessionItemsKeysInSavedItems(input.Session, output, err)

	var resolvedConflictsToSync EncryptedItems
	var processedOutput SyncOutput

	resolvedConflictsToSync, processedOutput, err = processSyncOutput(input, output)
	if err != nil {
		return SyncOutput{}, err
	}

	for x := range processedOutput.SavedItems {
		fmt.Printf("SAVED (post processing) ***** %#+v\n", processedOutput.SavedItems[x])
	}

	// if no conflicts to sync, then return
	debugPrint(input.Session.Debug, fmt.Sprintf("Sync | resolvedConflictsToSync: %d", len(resolvedConflictsToSync)))
	if len(resolvedConflictsToSync) == 0 {
		processSessionItemsKeysInSavedItems(input.Session, processedOutput, err)

		items := append(processedOutput.Items, processedOutput.SavedItems...)
		items.DeDupe()

		processedOutput.Items = items
		debugPrint(input.Session.Debug, fmt.Sprintf("Sync | post-sync default items key: %s", input.Session.DefaultItemsKey.UUID))
		return processedOutput, err
	}

	// if we have conflicts to sync, then call sync again
	if len(resolvedConflictsToSync) > 0 {
		// Call Sync Again and add the syncOutput to the syncOutput we've already got
		input.Items = resolvedConflictsToSync

		var resyncOutput SyncOutput

		resyncOutput, err = syncItems(input)
		if err != nil {
			panic(err)
		}

		// we only expect to get saved items back from the new sync as these are conflicts being resolved
		if len(resyncOutput.Conflicts) > 0 {
			panic(fmt.Sprintf("we didn't expect to get any conflicts now, but got: %d", len(resyncOutput.Conflicts)))
		}

		// zero the conflicts as we've resolved them
		processedOutput.Conflicts = nil

		processedOutput.Items = append(processedOutput.Items, resyncOutput.Items...)
		processedOutput.SavedItems = append(processedOutput.SavedItems, resyncOutput.SavedItems...)
		processedOutput.Items.DeDupe()
		processedOutput.SavedItems.DeDupe()
	}

	// for each saved item, update the times on the input items
	var updatedSaved EncryptedItems
	// TODO: update original items (if saved) updated timestamps before adding back to db
	for x := range processedOutput.SavedItems {
		// fmt.Printf("SAVED ***** %#+v\n", syncOutput.SavedItems[x])
		for y := range clonedItems {
			if processedOutput.SavedItems[x].UUID == clonedItems[y].UUID {

				updated := clonedItems[y]
				updated.Content = clonedItems[x].Content
				updated.ItemsKeyID = clonedItems[x].ItemsKeyID
				updated.EncItemKey = clonedItems[x].EncItemKey
				updated.UpdatedAtTimestamp = processedOutput.SavedItems[x].UpdatedAtTimestamp
				updated.UpdatedAt = processedOutput.SavedItems[x].UpdatedAt
				updatedSaved = append(updatedSaved, updated)
			}
			// fmt.Printf("ORIGINAL ***** %#+v\n", clonedItems[y])
		}
	}

	// fmt.Printf("POST UPDATEs - orig saved: %d", len(syncOutput.SavedItems))
	// fmt.Printf("POST UPDATEs - updated saved: %d", len(updatedSaved))

	// items := append(syncOutput.Items, syncOutput.SavedItems...)

	// instead of saving the saved items returned from the syncing option (minus content), we should save the originals with any updates
	processedOutput.SavedItems = updatedSaved

	processSessionItemsKeysInSavedItems(input.Session, processedOutput, err)

	items := append(processedOutput.Items, processedOutput.SavedItems...)
	items.DeDupe()

	processedOutput.Items = items

	return processedOutput, err
}

// if sync'd items includes a new items key that's been saved, then set as default
func processSessionItemsKeysInSavedItems(s *Session, output SyncOutput, err error) {
	var iks ItemsKeys

	if len(output.SavedItems) > 0 {
		// checking if we've saved a new items key, in which case it should be new default
		iks, err = output.SavedItems.DecryptAndParseItemsKeys(s.MasterKey, s.Debug)
	} else {
		// existing items key would be returned on first sync
		iks, err = output.Items.DecryptAndParseItemsKeys(s.MasterKey, s.Debug)
	}

	if err != nil {
		return
	}

	switch len(iks) {
	case 0:
		break
	default:
		s.DefaultItemsKey = iks[0]
		s.ItemsKeys = iks
	}
}

func processSyncOutput(input SyncInput, syncOutput SyncOutput) (resolvedConflictsToSync EncryptedItems, so SyncOutput, err error) {
	debug := input.Session.Debug

	// strip any duplicates (https://github.com/standardfile/rails-engine/issues/5)
	// postElapsed := time.Since(postStart)
	// debugPrint(debug, fmt.Sprintf("Sync | post processing took %v", postElapsed))
	// debugPrint(debug, fmt.Sprintf("Sync | sync token: %+v", stripLineBreak(syncOutput.SyncToken)))

	if err = syncOutput.Items.Validate(); err != nil {
		panic(err)
	}

	if err = syncOutput.Conflicts.Validate(debug); err != nil {
		panic(err)
	}

	// Resync any conflicts
	var conflictsToSync EncryptedItems

	// Store any references that need to be remapped due to conflicts
	var refReMap map[string]string

	if len(syncOutput.Conflicts) == 0 {
		return nil, syncOutput, err
	}

	// if len(syncOutput.Conflicts) > 0 {
	debugPrint(debug, fmt.Sprintf("Sync | found %d conflicts", len(syncOutput.Conflicts)))
	// create store for old and new uuids in case we need to remap any references to existing items with new uuids
	refReMap = make(map[string]string)

	for _, conflict := range syncOutput.Conflicts {
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

						var di Item

						di, err = DecryptAndParseItem(item, input.Session)
						if err != nil {
							return
						}

						// generate new uuid
						newUUID := GenUUID()
						// create remap reference for later
						refReMap[di.GetUUID()] = newUUID
						// set new uuid
						di.SetUUID(newUUID)
						// re-encrypt to update auth data
						newdis := Items{di}

						var newis EncryptedItems

						k := input.Session.DefaultItemsKey
						// if the conflict is during import, then we need to re-encrypt with Importer Key
						if len(input.Session.ImporterItemsKeys) > 0 {
							debugPrint(input.Session.Debug, fmt.Sprintf("Sync | setting ImportersItemsKey to: %s", k.UUID))
							k = input.Session.ImporterItemsKeys.Latest()
						}

						newis, err = newdis.Encrypt(input.Session, k)
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
			var found bool

			for _, item := range input.Items {
				if item.UUID == conflict.UnsavedItem.UUID {
					if item.Deleted {
						item = conflict.UnsavedItem
						item.Deleted = true
						item.Content = ""
						conflictedItem = item
						found = true

						break
					}

					conflictedItem = item
					// decrypt server item
					var di Item

					di, err = DecryptAndParseItem(item, input.Session)
					if err != nil {
						return
					}

					// generate new uuid
					newUUID := GenUUID()
					// create remap reference for later
					refReMap[di.GetUUID()] = newUUID
					// set new uuid
					di.SetUUID(newUUID)
					// re-encrypt to update auth data
					newdis := Items{di}

					var newis EncryptedItems

					k := input.Session.DefaultItemsKey
					// if the conflict is during import, then we need to re-encrypt with Importer Key
					if input.Session.ImporterItemsKeys.Latest().Content.ItemsKey != "" {
						k = input.Session.ImporterItemsKeys.Latest()
						debugPrint(input.Session.Debug, fmt.Sprintf("Sync | setting ImportersItemsKey to: %s", k.UUID))
					}

					newis, err = newdis.Encrypt(input.Session, k)
					if err != nil {
						return
					}

					newi := newis[0]
					newis[0].DuplicateOf = &conflict.UnsavedItem.UUID
					conflictedItem = newi

					found = true

					break
				}
			}

			if !found {
				panic("could not find item that failed to sync")
			}

			conflictsToSync = append(conflictsToSync, conflictedItem)
		}
	}
	// }

	// handle uuid reference remaps
	conflictsToSync, err = updateEncryptedItemRefs(input.Session, conflictsToSync, refReMap)
	if err != nil {
		return
	}

	// if we had conflicts to sync, then we need to return them for processing
	if len(conflictsToSync) > 0 {
		return conflictsToSync, syncOutput, err
	}

	// if len(conflictsToSync) > 0 {
	// 	// Call Sync Again and add the syncOutput to the syncOutput we've already got
	// 	input.Items = conflictsToSync
	//
	// 	var resyncOutput SyncOutput
	//
	// 	resyncOutput, err = Sync(input)
	// 	if err != nil {
	// 		panic(err)
	// 	}
	//
	// 	// we only expect to get saved items back from the new sync as these are conflicts being resolved
	// 	if len(resyncOutput.Conflicts) > 0 {
	// 		panic(fmt.Sprintf("we didn't expect to get any conflicts now, but got: %d", len(resyncOutput.Conflicts)))
	// 	}
	//
	// 	// zero the conflicts as we've resolved them
	// 	syncOutput.Conflicts = nil
	//
	// 	syncOutput.Items = append(syncOutput.Items, resyncOutput.Items...)
	// 	syncOutput.SavedItems = append(syncOutput.SavedItems, resyncOutput.SavedItems...)
	// 	syncOutput.Items.DeDupe()
	// 	syncOutput.SavedItems.DeDupe()
	// }

	return
}

func updateEncryptedItemRefs(s *Session, eis EncryptedItems, refReMap map[string]string) (result EncryptedItems, err error) {
	for _, ei := range eis {
		if ei.Deleted {
			continue
		}

		if isEncryptedWithMasterKey(ei.ContentType) {
			result = append(result, ei)

			continue
		}

		// decrypt
		if ei.ItemsKeyID != s.DefaultItemsKey.UUID {
			panic("not default key")
		}

		var di Item

		di, err = DecryptAndParseItem(ei, s)
		if err != nil {
			return
		}

		// update refs
		var updatedRefs ItemReferences

		var itemUpdated bool

		diContent := di.GetContent()
		itemsReferences := diContent.References()

		for _, ref := range itemsReferences {
			var found bool

			for k, v := range refReMap {
				if ref.UUID == k {
					updatedRefs = append(updatedRefs, ItemReference{
						UUID:        v,
						ContentType: ref.ContentType,
					})
					itemUpdated = true
					found = true

					break
				}
			}

			if !found {
				updatedRefs = append(updatedRefs, ref)
			}
		}

		if !itemUpdated {
			// if we haven't amended the item, just add encrypted item to list to return
			result = append(result, ei)

			continue
		}
		// reencrypt
		itemsReferences = updatedRefs
		diContent.SetReferences(itemsReferences)
		di.SetContent(diContent)
		disNew := Items{di}

		var eisNew EncryptedItems

		eisNew, err = disNew.Encrypt(s, s.DefaultItemsKey)
		if err != nil {
			return
		}

		result = append(result, eisNew[0])
	}

	return
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
	var seenServerItems []string

	var seenUnsavedItems []string

	var deDuped ConflictedItems

	for _, ci := range *cis {
		switch {
		// check if it's an encountered server item
		case ci.ServerItem.UUID != "":
			if !stringInSlice(ci.ServerItem.UUID, seenServerItems, true) {
				deDuped = append(deDuped, ci)
				seenServerItems = append(seenServerItems, ci.ServerItem.UUID)
			}
		// check if it's an encountered unsaved item
		case ci.UnsavedItem.UUID != "":
			if !stringInSlice(ci.UnsavedItem.UUID, seenUnsavedItems, true) {
				deDuped = append(deDuped, ci)
				seenUnsavedItems = append(seenUnsavedItems, ci.UnsavedItem.UUID)
			}
		default:
			panic("unexpected conflict")
		}
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
			debugPrint(debug, fmt.Sprintf("Sync | uuid conflict of: \"%s\" with uuid: \"%s\"", ci.UnsavedItem.ContentType, ci.UnsavedItem.UUID))
			continue
		case "uuid_error":
			debugPrint(debug, "Sync | client is attempting to sync an item without uuid")
			panic("Sync | client is attempting to sync an item without a uuid")
		case "content_error":
			debugPrint(debug, "Sync | client is attempting to sync an item with invalid content")
			panic("Sync | client is attempting to sync an item without a uuid")
		default:
			return fmt.Errorf("%s conflict type is currently unhandled\nplease raise an issue at https://github.com/jonhadfield/gosn-v2\nConflicted Item: %+v", ci.Type, ci)
		}
	}

	return nil
}

//
// func lesserOf(first, second int) int {
//	if first < second {
//		if first < 0 {
//			return 0
//		}
//
//		return first
//	}
//
//	if second < 0 {
//		return 0
//	}
//
//	return second
// }

func syncItemsViaAPI(input SyncInput) (out syncResponse, err error) {
	debug := input.Session.Debug
	debugPrint(debug, fmt.Sprintf("syncItemsViaAPI | input.FinalItem: %d", lesserOf(len(input.Items)-1, input.NextItem+150-1)+1))

	// fmt.Printf("syncItemsViaAPI START\n")
	// for x := range input.Items {
	// 	fmt.Printf("syncItemsViaAPI----- %s %s\n", input.Items[x].ItemsKeyID, input.Items[x].EncItemKey)
	// }
	// fmt.Printf("syncItemsViaAPI\n")
	// fmt.Printf("syncItemsViaAPI END\n")

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

	encItemJSON := []byte("[]")

	itemsToPut := input.Items

	var finalItem int

	if len(input.Items) > 0 {
		finalItem = lesserOf(len(input.Items)-1, input.NextItem+limit-1)
		debugPrint(debug, fmt.Sprintf("syncItemsViaAPI | going to put items: %d to %d", input.NextItem+1, finalItem+1))
		// fmt.Printf("syncItemsViaAPI | going to put items: %d to %d\n", input.NextItem, finalItem)

		encItemJSON, err = json.Marshal(itemsToPut[input.NextItem : finalItem+1])
		// fmt.Printf("syncItemsViaAPI | encItemJSON: %s\n", encItemJSON)
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
				`,"items":` + string(encItemJSON) +
				`,"cursor_token":null}`)
		} else {
			requestBody = []byte(`{"api":"20200115","items":[],"compute_integrity":false,"limit":` + strconv.Itoa(limit) +
				`,"items":` + string(encItemJSON) +
				`,"sync_token":"` + newST + `","cursor_token":null}`)
		}

	case input.CursorToken != "":
		rawST := input.SyncToken

		input.SyncToken = stripLineBreak(rawST)

		requestBody = []byte(`{"api":"20200115", "limit":` + strconv.Itoa(limit) +
			`,"items":` + string(encItemJSON) +
			`,"compute_integrity":false,"sync_token":"` + newST + `","cursor_token":"` + stripLineBreak(input.CursorToken) + `\n"}`)
	}

	var responseBody []byte
	responseBody, err = makeSyncRequest(*input.Session, requestBody)

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
		debugPrint(debug, fmt.Sprintf("syncItemsViaAPI | final item put: %d total items to put: %d", finalItem+1, len(input.Items)))
		// fmt.Printf("syncItemsViaAPI | final item put: %d total items to put: %d", finalItem, len(input.Items))

	}

	// fmt.Printf("finalItem: %d len(input.Items)-1): %d CursorToken: %s\n", finalItem, len(input.Items)-1, bodyContent.CursorToken)

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
