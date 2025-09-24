package items

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/jonhadfield/gosn-v2/common"
	"github.com/jonhadfield/gosn-v2/log"
	"github.com/jonhadfield/gosn-v2/session"
	"github.com/matryer/try"
)

// SyncInput defines the input for retrieving items.
type SyncInput struct {
	Session              *session.Session
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
	Type        string        `json:"type"`
}

// ConflictType represents different types of sync conflicts
const (
	ConflictTypeSync        = "sync_conflict"
	ConflictTypeUUID        = "uuid_conflict"
	ConflictTypeContentType = "content_type_error"
	ConflictTypeContent     = "content_error"
	ConflictTypeReadOnly    = "readonly_error"
	ConflictTypeUUIDError   = "uuid_error"
	ConflictTypeInvalidItem = "invalid_server_item"
)

func syncItems(i SyncInput) (so SyncOutput, err error) {
	giStart := time.Now()
	defer func() {
		log.DebugPrint(i.Session.Debug, fmt.Sprintf("Sync | duration %v", time.Since(giStart)), common.MaxDebugChars)
	}()

	if !i.Session.Valid() {
		err = fmt.Errorf("session is invalid")
		return
	}

	var sResp syncResponse

	// check if we need to add a post sync request delay
	var ok bool
	i.PostSyncRequestDelay, ok, err = common.ParseEnvInt64(common.EnvPostSyncRequestDelay)
	if err != nil {
		return
	}
	if ok {
		log.DebugPrint(i.Session.Debug, fmt.Sprintf("syncItemsViaAPI | sleeping %d milliseconds post each sync request",
			i.PostSyncRequestDelay), common.MaxDebugChars)
	}
	// retry logic is to handle responses that are too large
	// so we can reduce number we retrieve with each sync request
	start := time.Now()
	rErr := try.Do(func(attempt int) (bool, error) {
		// Implement exponential backoff similar to Standard Notes
		if attempt > 1 {
			backoffDuration := time.Duration(1000*(1<<uint(attempt-2))) * time.Millisecond
			log.DebugPrint(i.Session.Debug, fmt.Sprintf("Sync | backing off for %v before attempt %d", backoffDuration, attempt), common.MaxDebugChars)
			time.Sleep(backoffDuration)
		}
		ps := common.PageSize
		if i.PageSize > 0 {
			ps = i.PageSize
		}
		log.DebugPrint(i.Session.Debug, fmt.Sprintf("Sync | attempt %d with page size %d", attempt, ps), common.MaxDebugChars)
		var rErr error
		sResp, rErr = syncItemsViaAPI(i)
		if rErr != nil {
			log.DebugPrint(i.Session.Debug, fmt.Sprintf("Sync | %s", rErr.Error()), common.MaxDebugChars)
			switch {
			case strings.Contains(strings.ToLower(rErr.Error()), "session token") &&
				strings.Contains(strings.ToLower(rErr.Error()), "expired"):
				fmt.Printf("\nerr: %s\n\nplease log in again", rErr)
				os.Exit(1)
			case strings.Contains(strings.ToLower(rErr.Error()), "too large"):
				i.NextItem = sResp.Data.LastItemPut
				resizeForRetry(&i)
				log.DebugPrint(i.Session.Debug, fmt.Sprintf("Sync | failed to retrieve %d items "+
					"at a time as the request was too large so reducing to page size %d",
					sResp.Data.PutLimitUsed, i.PageSize), common.MaxDebugChars)
			case strings.Contains(strings.ToLower(rErr.Error()), "timeout"):
				log.DebugPrint(i.Session.Debug, fmt.Sprintf("Sync | request timed out, retrying with smaller page size"), common.MaxDebugChars)
				i.NextItem = sResp.Data.LastItemPut
				resizeForRetry(&i)
				log.DebugPrint(i.Session.Debug, fmt.Sprintf("Sync | failed to retrieve %d items "+
					"at a time due to timeout so reducing to page size %d", sResp.Data.PutLimitUsed, i.PageSize), common.MaxDebugChars)
			case strings.Contains(strings.ToLower(rErr.Error()), "unauthorized"):
				i.NextItem = sResp.Data.LastItemPut
				// logging.DebugPrint(i.Session.Debug, "Sync | failed with '401 Unauthorized' which is most likely due to throttling or password change since session created")
				return false, fmt.Errorf("sync failed due to either password change since session created, or server throttling. try re-adding session.")
				// panic("sync failed due to either password change since session created, or server throttling. try re-adding session.")
			case strings.Contains(strings.ToLower(rErr.Error()), "EOF"):
				log.DebugPrint(i.Session.Debug, fmt.Sprintf("Sync | got EOF error, retrying with smaller page size"), common.MaxDebugChars)
				i.NextItem = sResp.Data.LastItemPut
				resizeForRetry(&i)
				log.DebugPrint(i.Session.Debug, fmt.Sprintf("Sync | failed to retrieve %d items "+
					"at a time due to EOF so reducing to page size %d", sResp.Data.PutLimitUsed, i.PageSize), common.MaxDebugChars)
			case strings.Contains(strings.ToLower(rErr.Error()), "giving up"):
				log.DebugPrint(i.Session.Debug, fmt.Sprintf("Sync | retry client gave up after multiple attempts"), common.MaxDebugChars)
				return false, fmt.Errorf("sync failed: %+v", rErr)
			case strings.Contains(strings.ToLower(rErr.Error()), "500") || strings.Contains(strings.ToLower(rErr.Error()), "internal server error"):
				log.DebugPrint(i.Session.Debug, fmt.Sprintf("Sync | got HTTP 500 Internal Server Error, likely due to rate limiting - retrying with delay"), common.MaxDebugChars)
				// Add a small delay to prevent overwhelming the server
				time.Sleep(time.Duration(attempt) * time.Second)
				return attempt < 4, rErr
			default:
				panic(fmt.Sprintf("sync returned unhandled error: %+v", rErr))
			}
		}

		return attempt < 4, rErr
	})
	if rErr != nil {
		return so, fmt.Errorf("sync | %w", rErr)
	}

	elapsed := time.Since(start)

	log.DebugPrint(i.Session.Debug, fmt.Sprintf("Sync | took %v to get all items", elapsed), common.MaxDebugChars)

	so.Items = sResp.Data.Items
	so.Items.DeDupe()
	so.Items.RemoveUnsupported()
	so.Unsaved = sResp.Data.Unsaved
	so.Unsaved.DeDupe()
	so.Unsaved.RemoveUnsupported()
	so.SavedItems = sResp.Data.SavedItems
	so.SavedItems.DeDupe()
	so.SavedItems.RemoveUnsupported()
	so.Conflicts = sResp.Data.Conflicts
	so.Conflicts.DeDupe()
	so.Cursor = sResp.Data.CursorToken
	so.SyncToken = sResp.Data.SyncToken

	// update timestamps on saved items
	so.SavedItems = updateTimestampsOnSavedItems(i.Items, so.SavedItems)

	log.DebugPrint(i.Session.Debug,
		fmt.Sprintf("Sync | SN returned %d items, %d saved items, and %d conflicts, with syncToken %s",
			len(so.Items), len(so.SavedItems), len(so.Conflicts), so.SyncToken), common.MaxDebugChars)

	return
}

func updateTimestampsOnSavedItems(orig, synced EncryptedItems) (updatedSaved EncryptedItems) {
	// for each saved item, update the times on the input items	}
	for x := range synced {
		for y := range orig {
			if synced[x].UUID == orig[y].UUID {
				updated := orig[y]
				updated.Content = orig[y].Content
				updated.ItemsKeyID = orig[y].ItemsKeyID
				updated.EncItemKey = orig[y].EncItemKey
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
	// defer func() { input.Session.ImporterItemsKeys = ItemsKeys{} }()

	// CRITICAL SAFEGUARD: Filter out any attempts to delete or modify protected item types
	var filteredItems EncryptedItems
	for _, item := range input.Items {
		// Protect SN|ItemsKey items
		if item.ContentType == common.SNItemTypeItemsKey {
			if item.Deleted {
				log.DebugPrint(input.Session.Debug, fmt.Sprintf("Sync | WARNING: Blocking attempt to delete SN|ItemsKey %s", item.UUID), common.MaxDebugChars)
				continue // Skip this item entirely
			}
			// ItemsKeys should only be synced if they're new (no UUID yet) or being retrieved
			// Never allow modification of existing ItemsKeys
			if item.UUID != "" && item.UpdatedAt != "" {
				log.DebugPrint(input.Session.Debug, fmt.Sprintf("Sync | WARNING: Blocking attempt to modify existing SN|ItemsKey %s", item.UUID), common.MaxDebugChars)
				continue // Skip this item entirely
			}
		}

		// Protect SN|UserPreferences items from deletion
		if item.ContentType == common.SNItemTypeUserPreferences && item.Deleted {
			log.DebugPrint(input.Session.Debug, fmt.Sprintf("Sync | WARNING: Blocking attempt to delete SN|UserPreferences %s", item.UUID), common.MaxDebugChars)
			continue // Skip this item entirely
		}

		filteredItems = append(filteredItems, item)
	}
	input.Items = filteredItems

	log.DebugPrint(input.Session.Debug, fmt.Sprintf("Sync | called with %d items and syncToken %s", len(input.Items), input.SyncToken), common.MaxDebugChars)
	log.DebugPrint(input.Session.Debug, fmt.Sprintf("Sync | pre-sync default items key: %s", input.Session.DefaultItemsKey.UUID), common.MaxDebugChars)
	// if items have been passed but no default items key exists then return error
	if len(input.Items) > 0 && input.Session.DefaultItemsKey.ItemsKey == "" {
		err = fmt.Errorf("missing default items key in session")
	}

	// duplicate items to be pushed so we can update their updated_at_timestamp if saved
	clonedItems := slices.Clone(input.Items)
	// perform initial sync
	output, err = syncItems(input)
	if err != nil {
		return output, err
	}
	processSessionItemsKeysInSavedItems(input.Session, output, err)

	var resolvedConflictsToSync EncryptedItems

	var processedOutput SyncOutput

	resolvedConflictsToSync, processedOutput, err = processSyncOutput(input, output)
	if err != nil {
		return SyncOutput{}, err
	}

	// if no conflicts to sync, then return
	log.DebugPrint(input.Session.Debug, fmt.Sprintf("Sync | resolvedConflictsToSync: %d", len(resolvedConflictsToSync)), common.MaxDebugChars)

	if len(resolvedConflictsToSync) == 0 {
		processSessionItemsKeysInSavedItems(input.Session, processedOutput, err)

		items := append(processedOutput.Items, processedOutput.SavedItems...)
		items.DeDupe()

		processedOutput.Items = items

		log.DebugPrint(input.Session.Debug, fmt.Sprintf("Sync | post-sync default items key: %s", input.Session.DefaultItemsKey.UUID), common.MaxDebugChars)

		return processedOutput, err
	}

	// if we have conflicts to sync, then call sync again
	if len(resolvedConflictsToSync) > 0 {
		// Call Sync Again and add the syncOutput to the syncOutput we've already got
		input.Items = resolvedConflictsToSync

		var resyncOutput SyncOutput

		resyncOutput, err = syncItems(input)
		if err != nil {
			return SyncOutput{}, err
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

	if len(processedOutput.SavedItems) > 0 {
		updatedSaved := updateTimestampsOnSavedItems(clonedItems, processedOutput.SavedItems)
		processedOutput.SavedItems = updatedSaved
	}

	processSessionItemsKeysInSavedItems(input.Session, processedOutput, err)

	items := append(processedOutput.Items, processedOutput.SavedItems...)
	items.DeDupe()

	processedOutput.Items = items

	return processedOutput, err
}

// if sync'd items includes a new items key that's been saved, then set as default.
func processSessionItemsKeysInSavedItems(s *session.Session, output SyncOutput, err error) {
	var iks []session.SessionItemsKey

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

func processSyncConflict(s *session.Session, items EncryptedItems, conflict ConflictedItem, refReMap map[string]string) (conflictedItem EncryptedItem, err error) {
	debug := s.Debug

	// Special handling for ItemsKey conflicts - always keep server version
	if conflict.ServerItem.ContentType == common.SNItemTypeItemsKey {
		log.DebugPrint(debug, "Sync | ItemsKey conflict detected, keeping server version", common.MaxDebugChars)
		return conflict.ServerItem, nil
	}

	switch {
	case conflict.ServerItem.Deleted:
		// if server item is deleted then we will give unsaved item a new uuid and sync it
		log.DebugPrint(debug, fmt.Sprintf("Sync | server item uuid %s type %s is deleted so keeping local",
			conflict.ServerItem.UUID, conflict.ServerItem.ContentType), common.MaxDebugChars)

		var found bool

		for _, item := range items {
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
		// if unsaved item is newer than server item, then unsaved wins but we update timestamp
		log.DebugPrint(debug, fmt.Sprintf("Sync | local is more recent so keeping it and updating timestamp to: %d", conflict.ServerItem.UpdatedAtTimestamp), common.MaxDebugChars)

		conflictedItem = conflict.UnsavedItem
		conflictedItem.UpdatedAtTimestamp = conflict.ServerItem.UpdatedAtTimestamp

	case conflict.UnsavedItem.ContentType != conflict.ServerItem.ContentType:
		// Content type mismatch - this is a serious conflict, duplicate the local item
		log.DebugPrint(debug, "Sync | content type mismatch, duplicating local item", common.MaxDebugChars)
		// Fall through to default duplication logic
		fallthrough

	default:
		log.DebugPrint(debug, "Sync | server item most recent, so set new UUID on the item that conflicted and set it as 'duplicate_of' original", common.MaxDebugChars)

		var found bool

		for _, item := range items {
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

				di, err = DecryptAndParseItem(item, s)
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

				k := s.DefaultItemsKey
				// if the conflict is during import, then we need to re-encrypt with Importer Key
				// if len(s.ImporterItemsKeys) > 0 {
				// 	logging.DebugPrint(s.Debug, fmt.Sprintf("Sync | setting ImportersItemsKey to: %s", k.UUID), common.MaxDebugChars)
				// 	k = s.ImporterItemsKeys.Latest()
				// }

				newis, err = newdis.Encrypt(s, k)
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

	return conflictedItem, err
}

func processUUIDConflict(input SyncInput, conflict ConflictedItem, refReMap map[string]string) (conflictedItem EncryptedItem, err error) {
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
			// if input.Session.ImporterItemsKeys.Latest().Content.ItemsKey != "" {
			// 	k = input.Session.ImporterItemsKeys.Latest()
			// 	logging.DebugPrint(input.Session.Debug, fmt.Sprintf("Sync | setting ImportersItemsKey to: %s", k.UUID), common.MaxDebugChars)
			// }

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

	return
}

func processConflict(input SyncInput, conflict ConflictedItem, refReMap map[string]string) (conflictsToSync EncryptedItems, err error) {
	debug := input.Session.Debug
	var conflictedItem EncryptedItem

	switch conflict.Type {
	case ConflictTypeSync:
		conflictedItem, err = processSyncConflict(input.Session, input.Items, conflict, refReMap)

	case ConflictTypeUUID:
		conflictedItem, err = processUUIDConflict(input, conflict, refReMap)

	case ConflictTypeContentType, ConflictTypeContent:
		// For content errors, we typically want to keep the server version
		// and create a duplicate of the local version for safety
		log.DebugPrint(debug, fmt.Sprintf("Sync | %s detected, keeping server version", conflict.Type), common.MaxDebugChars)
		if conflict.ServerItem.UUID != "" {
			conflictedItem = conflict.ServerItem
		} else {
			// If no server item, this is a validation error on our item
			log.DebugPrint(debug, "Sync | Validation error on local item, skipping", common.MaxDebugChars)
			return nil, nil
		}

	case ConflictTypeReadOnly:
		// Read-only error means we tried to modify something we shouldn't
		// Keep the server version
		log.DebugPrint(debug, "Sync | Read-only conflict, keeping server version", common.MaxDebugChars)
		conflictedItem = conflict.ServerItem

	case ConflictTypeUUIDError, ConflictTypeInvalidItem:
		// These are serious errors, log and skip
		log.DebugPrint(debug, fmt.Sprintf("Sync | Serious conflict type %s, skipping item", conflict.Type), common.MaxDebugChars)
		return nil, nil

	default:
		err = fmt.Errorf("unhandled conflict type: %s", conflict.Type)
	}

	if err == nil && conflictedItem.UUID != "" {
		conflictsToSync = append(conflictsToSync, conflictedItem)
	}

	return conflictsToSync, err
}

func processConflicts(input SyncInput, syncOutput SyncOutput) (conflictsToSync EncryptedItems, err error) {
	// Store any references that need to be remapped due to conflicts
	var refReMap map[string]string
	// create store for old and new uuids in case we need to remap any references to existing items with new uuids
	refReMap = make(map[string]string)

	for _, conflict := range syncOutput.Conflicts {
		var resolvedConflictedItems EncryptedItems

		resolvedConflictedItems, err = processConflict(input, conflict, refReMap)
		if err != nil {
			return
		}

		conflictsToSync = append(conflictsToSync, resolvedConflictedItems...)
	}

	// handle uuid reference remaps
	conflictsToSync, err = updateEncryptedItemRefs(input.Session, conflictsToSync, refReMap)
	if err != nil {
		return
	}

	return
}

func processSyncOutput(input SyncInput, syncOutput SyncOutput) (resolvedConflictsToSync EncryptedItems, so SyncOutput, err error) {
	debug := input.Session.Debug

	// strip any duplicates (https://github.com/standardfile/rails-engine/issues/5)
	// postElapsed := time.Since(postStart)
	// logging.DebugPrint(debug, fmt.Sprintf("Sync | post processing took %v", postElapsed))
	// logging.DebugPrint(debug, fmt.Sprintf("Sync | sync token: %+v", stripLineBreak(syncOutput.SyncToken)))

	if err = syncOutput.Items.Validate(); err != nil {
		panic(err)
	}

	if err = syncOutput.Conflicts.Validate(debug); err != nil {
		panic(err)
	}

	if len(syncOutput.Conflicts) == 0 {
		return nil, syncOutput, err
	}

	log.DebugPrint(debug, fmt.Sprintf("Sync | found %d conflicts", len(syncOutput.Conflicts)), common.MaxDebugChars)
	// Resync any conflicts
	conflictsToSync, err := processConflicts(input, syncOutput)
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

func updateEncryptedItemRefs(s *session.Session, items EncryptedItems, refMap map[string]string) (EncryptedItems, error) {
	var result EncryptedItems

	for _, ei := range items {
		if ei.Deleted || IsEncryptedWithMasterKey(ei.ContentType) {
			result = append(result, ei)
			continue
		}

		if ei.ItemsKeyID != s.DefaultItemsKey.UUID {
			return nil, fmt.Errorf("item %s not encrypted with default key", ei.UUID)
		}

		di, err := DecryptAndParseItem(ei, s)
		if err != nil {
			return nil, err
		}

		content := di.GetContent()
		refs := content.References()

		updated := false
		for i := range refs {
			if newID, ok := refMap[refs[i].UUID]; ok {
				refs[i].UUID = newID
				updated = true
			}
		}

		if !updated {
			result = append(result, ei)
			continue
		}

		content.SetReferences(refs)
		di.SetContent(content)

		disNew := Items{di}
		encrypted, err := disNew.Encrypt(s, s.DefaultItemsKey)
		if err != nil {
			return nil, err
		}

		result = append(result, encrypted[0])
	}

	return result, nil
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
			if !slices.Contains(seenServerItems, ci.ServerItem.UUID) {
				deDuped = append(deDuped, ci)
				seenServerItems = append(seenServerItems, ci.ServerItem.UUID)
			}
		// check if it's an encountered unsaved item
		case ci.UnsavedItem.UUID != "":
			if !slices.Contains(seenUnsavedItems, ci.UnsavedItem.UUID) {
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
		case ConflictTypeSync:
			log.DebugPrint(debug, fmt.Sprintf("Sync | sync conflict of: \"%s\" with uuid: \"%s\"", ci.ServerItem.ContentType, ci.ServerItem.UUID), common.MaxDebugChars)
			continue
		case ConflictTypeUUID:
			log.DebugPrint(debug, fmt.Sprintf("Sync | uuid conflict of: \"%s\" with uuid: \"%s\"", ci.UnsavedItem.ContentType, ci.UnsavedItem.UUID), common.MaxDebugChars)
			continue
		case ConflictTypeUUIDError:
			log.DebugPrint(debug, "Sync | client is attempting to sync an item without uuid", common.MaxDebugChars)
			// Don't panic, just log and continue
			continue
		case ConflictTypeContent:
			log.DebugPrint(debug, "Sync | client is attempting to sync an item with invalid content", common.MaxDebugChars)
			// Don't panic, just log and continue
			continue
		case ConflictTypeContentType:
			log.DebugPrint(debug, fmt.Sprintf("Sync | content type error for item: %s", ci.ServerItem.UUID), common.MaxDebugChars)
			continue
		case ConflictTypeReadOnly:
			log.DebugPrint(debug, fmt.Sprintf("Sync | read-only conflict for item: %s", ci.ServerItem.UUID), common.MaxDebugChars)
			continue
		case ConflictTypeInvalidItem:
			log.DebugPrint(debug, fmt.Sprintf("Sync | invalid server item: %s", ci.ServerItem.UUID), common.MaxDebugChars)
			continue
		default:
			// Don't fail on unknown conflict types, just log them
			log.DebugPrint(debug, fmt.Sprintf("Sync | WARNING: Unknown conflict type %s, will attempt to handle", ci.Type), common.MaxDebugChars)
		}
	}

	return nil
}

// determineLimit returns the page size to use for the sync request.
func determineLimit(pageSize int, debug bool) int {
	if pageSize > 0 {
		log.DebugPrint(debug, fmt.Sprintf("syncItemsViaAPI | input.PageSize: %d", pageSize), common.MaxDebugChars)
		return pageSize
	}

	log.DebugPrint(debug, fmt.Sprintf("syncItemsViaAPI | using default limit: %d", common.PageSize), common.MaxDebugChars)

	return common.PageSize
}

// encodeItems prepares a subset of items to be sent and returns the JSON
// representation and the final item index.
func encodeItems(items EncryptedItems, start, limit int, debug bool) ([]byte, int, error) {
	if len(items) == 0 {
		return []byte("[]"), 0, nil
	}

	finalItem := min(len(items)-1, start+limit-1)
	log.DebugPrint(debug, fmt.Sprintf("syncItemsViaAPI | going to put items: %d to %d", start+1, finalItem+1), common.MaxDebugChars)

	encItemJSON, err := json.Marshal(items[start : finalItem+1])
	if err != nil {
		return nil, 0, err
	}

	log.DebugPrint(debug, fmt.Sprintf("syncItemsViaAPI | request size: %d bytes", len(encItemJSON)), common.MaxDebugChars)

	return encItemJSON, finalItem, nil
}

// buildRequestBody constructs the sync request body.
func buildRequestBody(input SyncInput, limit int, encItemJSON []byte) []byte {
	newST := stripLineBreak(input.SyncToken)

	switch {
	case input.CursorToken == "":
		if len(input.Items) == 0 {
			if input.SyncToken == "" {
				return []byte(fmt.Sprintf(`{"api":"%s","items":[],"limit":%d}`, common.APIVersion, limit))
			}

			return []byte(fmt.Sprintf(`{"api":"%s","items":[],"limit":%d,"sync_token":"%s"}`, common.APIVersion, limit, newST))
		}

		if input.SyncToken == "" {
			return []byte(fmt.Sprintf(`{"api":"%s","limit":%d,"items":%s}`, common.APIVersion, limit, encItemJSON))
		}

		return []byte(fmt.Sprintf(`{"api":"%s","limit":%d,"items":%s,"sync_token":"%s"}`, common.APIVersion, limit, encItemJSON, newST))
	case input.CursorToken == "null":
		if input.SyncToken == "" {
			return []byte(fmt.Sprintf(`{"api":"%s","items":%s,"limit":%d,"cursor_token":null}`, common.APIVersion, encItemJSON, limit))
		}

		return []byte(fmt.Sprintf(`{"api":"%s","items":%s,"limit":%d,"sync_token":"%s","cursor_token":null}`, common.APIVersion, encItemJSON, limit, newST))
	default:
		rawST := input.SyncToken
		input.SyncToken = stripLineBreak(rawST)

		return []byte(fmt.Sprintf(`{"api":"%s","limit":%d,"items":%s,"sync_token":"%s","cursor_token":"%s"}`,
			common.APIVersion, limit, encItemJSON, newST, stripLineBreak(input.CursorToken)))
	}
}

func parseSyncResponse(data []byte) (syncResponse, error) {
	return unmarshallSyncResponse(data)
}

func syncItemsViaAPI(input SyncInput) (out syncResponse, err error) {
	debug := input.Session.Debug
	// log.DebugPrint(debug, fmt.Sprintf("syncItemsViaAPI | input.FinalItem: %d", lesserOf(len(input.Items)-1, input.NextItem+150-1)+1), common.MaxDebugChars)

	limit := determineLimit(input.PageSize, debug)

	out.Data.PutLimitUsed = limit
	// fmt.Printf("[syncItemsViaAPI] Starting sync at %s with %d items, limit %d\n", time.Now().Format("15:04:05.000"), len(input.Items), limit)
	encItemJSON, finalItem, err := encodeItems(input.Items, input.NextItem, limit, debug)
	if err != nil {
		// fmt.Printf("[syncItemsViaAPI] encodeItems failed: %v\n", err)
		return
	}
	// fmt.Printf("[syncItemsViaAPI] Encoded %d bytes at %s\n", len(encItemJSON), time.Now().Format("15:04:05.000"))
	requestBody := buildRequestBody(input, limit, encItemJSON)
	// fmt.Printf("[syncItemsViaAPI] Built request body (%d bytes) at %s\n", len(requestBody), time.Now().Format("15:04:05.000"))
	responseBody, status, err := makeSyncRequest(input.Session, requestBody)
	if input.PostSyncRequestDelay > 0 {
		// fmt.Printf("[syncItemsViaAPI] Sleeping for %dms post-sync\n", input.PostSyncRequestDelay)
		time.Sleep(time.Duration(input.PostSyncRequestDelay) * time.Millisecond)
	}
	if err != nil {
		// fmt.Printf("[syncItemsViaAPI] makeSyncRequest failed with status %d: %v\n", status, err)
		return
	}
	// fmt.Printf("[syncItemsViaAPI] Got response (%d bytes) at %s\n", len(responseBody), time.Now().Format("15:04:05.000"))
	if status != http.StatusOK {
		err = fmt.Errorf("syncItemsViaAPI | unexpected status code: %d, response: %s", status, string(responseBody))
		log.DebugPrint(debug, err.Error(), common.MaxDebugChars)
		return out, err
	}

	// get encrypted items from API response
	var bodyContent syncResponse
	// fmt.Println(string(responseBody))
	bodyContent, err = parseSyncResponse(responseBody)
	if err != nil {
		return
	}

	// fff, _ := json.MarshalIndent(bodyContent, "", "  ")
	// fmt.Println("bodyContent", string(fff))

	out.Data.Items = bodyContent.Data.Items
	out.Data.SavedItems = bodyContent.Data.SavedItems
	out.Data.Unsaved = bodyContent.Data.Unsaved
	out.Data.SyncToken = bodyContent.Data.SyncToken
	out.Data.CursorToken = bodyContent.Data.CursorToken
	out.Data.Conflicts = bodyContent.Data.Conflicts
	out.Data.LastItemPut = finalItem

	if (finalItem > 0 && finalItem < len(input.Items)-1) || (bodyContent.Data.CursorToken != "" && bodyContent.Data.CursorToken != "null") {
		var newOutput syncResponse

		input.SyncToken = out.Data.SyncToken
		log.DebugPrint(debug, fmt.Sprintf("syncItemsViaAPI | setting input sync token: %s", stripLineBreak(input.SyncToken)), common.MaxDebugChars)

		input.CursorToken = out.Data.CursorToken
		log.DebugPrint(debug, fmt.Sprintf("syncItemsViaAPI | setting input cursor token: %s", stripLineBreak(input.CursorToken)), common.MaxDebugChars)

		input.PageSize = limit
		// sync was successful so set new item
		if finalItem > 0 {
			log.DebugPrint(debug, fmt.Sprintf("syncItemsViaAPI | sync successful so setting new item to finalItem+1: %d", finalItem+1), common.MaxDebugChars)
			input.NextItem = finalItem + 1
		}

		newOutput, err = syncItemsViaAPI(input)

		if err != nil {
			return
		}

		out.Data.Items = append(out.Data.Items, newOutput.Data.Items...)
		out.Data.SavedItems = append(out.Data.SavedItems, newOutput.Data.SavedItems...)
		out.Data.Unsaved = append(out.Data.Unsaved, newOutput.Data.Unsaved...)
		out.Data.Conflicts = append(out.Data.Conflicts, newOutput.Data.Conflicts...)
		out.Data.SyncToken = newOutput.Data.SyncToken

		out.Data.LastItemPut = finalItem
	} else {
		return out, err
	}

	out.Data.CursorToken = ""

	return out, err
}

func resizeForRetry(in *SyncInput) {
	if in.PageSize != 0 {
		in.PageSize = int(math.Ceil(float64(in.PageSize) * retryScaleFactor))
	} else {
		in.PageSize = int(math.Ceil(float64(common.PageSize) * retryScaleFactor))
	}
}

func stripLineBreak(input string) string {
	if strings.HasSuffix(input, "\n") {
		return input[:len(input)-1]
	}

	return input
}

// DeleteContent will remove all Notes, Tags, and Components from SN.
func DeleteContent(session *session.Session, everything bool) (deleted int, err error) {
	si := SyncInput{
		Session: session,
	}

	var so SyncOutput

	so, err = Sync(si)
	if err != nil {
		return
	}

	var itemsToPut EncryptedItems

	typesToDelete := []string{
		common.SNItemTypeNote,
		common.SNItemTypeTag,
	}
	if everything {
		log.DebugPrint(session.Debug, "Not deleting config.", common.MaxDebugChars)
		//typesToDelete = append(typesToDelete, []string{
		//	common.SNItemTypeComponent,
		//	"SN|FileSafe|FileMetaData",
		//	common.SNItemTypeFileSafeCredentials,
		//	common.SNItemTypeFileSafeIntegration,
		//	common.SNItemTypeTheme,
		//	common.SNItemTypeExtensionRepo,
		//	common.SNItemTypePrivileges,
		//	common.SNItemTypeExtension,
		//	// Note: Removing UserPreferences from deletion list as it should never be deleted
		//	common.SNItemTypeFile,
		//}...)
	}

	for x := range so.Items {
		// CRITICAL: Never delete SN|ItemsKey items as they are needed to decrypt all other items
		if so.Items[x].ContentType == common.SNItemTypeItemsKey {
			continue
		}
		// CRITICAL: Never delete SN|UserPreferences as it contains important user settings
		if so.Items[x].ContentType == common.SNItemTypeUserPreferences {
			continue
		}
		if !so.Items[x].Deleted && slices.Contains(typesToDelete, so.Items[x].ContentType) {
			so.Items[x].Deleted = true
			itemsToPut = append(itemsToPut, so.Items[x])
		}
	}

	if len(itemsToPut) > 0 {
		log.DebugPrint(session.Debug, fmt.Sprintf("DeleteContent | removing %d items", len(itemsToPut)), common.MaxDebugChars)
	}

	si.Items = itemsToPut

	so, err = Sync(si)

	return len(so.SavedItems), err
}

func unmarshallSyncResponse(input []byte) (output syncResponse, err error) {
	// TODO: There should be an IsValid method on each item that includes this check if SN|ItemsKey
	// fmt.Printf("unmarshallSyncResponse | input: %s\n", string(input))
	err = json.Unmarshal(input, &output)
	if err != nil {
		return
	}

	// check no items keys have an items key
	for _, item := range output.Data.Items {
		if item.ContentType == common.SNItemTypeItemsKey && item.ItemsKeyID != "" {
			err = fmt.Errorf("SN|ItemsKey %s has an ItemsKeyID set", item.UUID)
			return
		}
	}

	return
}
