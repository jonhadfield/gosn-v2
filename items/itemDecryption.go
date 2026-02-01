package items

import (
	"encoding/json"
	"fmt"
	"runtime"
	"sync"

	"github.com/jonhadfield/gosn-v2/common"
	"github.com/jonhadfield/gosn-v2/log"
	"github.com/jonhadfield/gosn-v2/session"
)

func DecryptItem(e EncryptedItem, s *session.Session, iks []session.SessionItemsKey) (o DecryptedItem, err error) {
	if e.Deleted {
		return o, fmt.Errorf("cannot decrypt deleted item: %s %s", e.ContentType, e.UUID)
	}

	var key string

	ik := GetMatchingItem(e.GetItemsKeyID(), iks)

	switch {
	case ik.ItemsKey != "":
		key = ik.ItemsKey
	case IsEncryptedWithMasterKey(e.ContentType):
		key = s.MasterKey
	default:
		if e.ItemsKeyID == "" {
			log.DebugPrint(s.Debug, fmt.Sprintf("decryptItems | missing ItemsKeyID for content type: %s", e.ContentType), common.MaxDebugChars)
			err = fmt.Errorf("encountered deleted: %t item %s of type %s without ItemsKeyID",
				e.Deleted,
				e.UUID,
				e.ContentType)

			return
		}

		key = GetMatchingItem(e.ItemsKeyID, s.ItemsKeys).ItemsKey
		if key == "" {
			err = fmt.Errorf("deleted: %t item %s of type %s cannot be decrypted as we're missing ItemsKey %s",
				e.Deleted,
				e.UUID,
				e.ContentType,
				e.ItemsKeyID)

			return
		}
	}

	content, err := e.DecryptItemOnly(key)
	if err != nil {
		return
	}

	var di DecryptedItem
	di.UUID = e.UUID
	di.ContentType = e.ContentType
	di.Deleted = e.Deleted

	if e.ItemsKeyID != "" {
		di.ItemsKeyID = e.ItemsKeyID
	}

	di.UpdatedAt = e.UpdatedAt
	di.CreatedAt = e.CreatedAt
	di.CreatedAtTimestamp = e.CreatedAtTimestamp
	di.UpdatedAtTimestamp = e.UpdatedAtTimestamp

	if e.DuplicateOf != nil {
		di.DuplicateOf = *e.DuplicateOf
	}

	di.AuthHash = e.AuthHash
	di.UpdatedWithSession = e.UpdatedWithSession
	di.KeySystemIdentifier = e.KeySystemIdentifier
	di.SharedVaultUUID = e.SharedVaultUUID
	di.UserUUID = e.UserUUID
	di.LastEditedByUUID = e.LastEditedByUUID
	di.Content = string(content)

	return di, err
}

// DecryptAndParseItemKeys takes the master key and a list of EncryptedItemKeys
// and returns a list of items keys.
func DecryptAndParseItemKeys(mk string, eiks EncryptedItems) (iks []ItemsKey, err error) {
	for x := range eiks {
		if eiks[x].ContentType != common.SNItemTypeItemsKey {
			continue
		}

		var content []byte

		content, err = eiks[x].DecryptItemOnly(mk)
		if err != nil {
			return
		}

		var f ItemsKey

		err = json.Unmarshal(content, &f)
		if err != nil {
			return iks, fmt.Errorf("DecryptAndParseItemsKeys | failed to unmarshall %w", err)
		}

		f.UUID = eiks[x].UUID
		f.ContentType = eiks[x].ContentType
		f.UpdatedAt = eiks[x].UpdatedAt
		f.UpdatedAtTimestamp = eiks[x].UpdatedAtTimestamp
		f.CreatedAtTimestamp = eiks[x].CreatedAtTimestamp
		f.CreatedAt = eiks[x].CreatedAt

		if f.ItemsKey == "" {
			continue
		}

		iks = append(iks, f)
	}

	return iks, err
}

// DecryptItems.
const (
	// DecryptionBatchThreshold determines when to use parallel vs sequential decryption
	DecryptionBatchThreshold = 50
)

// decryptJob represents a single decryption task
type decryptJob struct {
	item  EncryptedItem
	index int
}

// decryptResult holds the result of a decryption operation
type decryptResult struct {
	item  DecryptedItem
	index int
	err   error
}

// DecryptItems decrypts multiple items, using parallel processing for large batches
func DecryptItems(s *session.Session, ei EncryptedItems, iks []session.SessionItemsKey) (o DecryptedItems, err error) {
	// Count non-deleted items
	nonDeletedCount := 0
	for _, e := range ei {
		if !e.Deleted {
			nonDeletedCount++
		}
	}

	if nonDeletedCount == 0 {
		return o, nil
	}

	// For small batches, sequential is faster (avoids goroutine overhead)
	if nonDeletedCount < DecryptionBatchThreshold {
		return decryptItemsSequential(s, ei, iks)
	}

	// Parallel decryption for large batches
	return decryptItemsParallel(s, ei, iks, nonDeletedCount)
}

// decryptItemsSequential processes items one at a time (for small batches)
func decryptItemsSequential(s *session.Session, ei EncryptedItems, iks []session.SessionItemsKey) (o DecryptedItems, err error) {
	for _, e := range ei {
		if e.Deleted {
			continue
		}

		var di DecryptedItem
		di, err = DecryptItem(e, s, iks)
		if err != nil {
			return
		}

		o = append(o, di)
	}

	return o, nil
}

// decryptItemsParallel processes items concurrently using a worker pool
func decryptItemsParallel(s *session.Session, ei EncryptedItems, iks []session.SessionItemsKey, nonDeletedCount int) (o DecryptedItems, err error) {
	// Use number of CPUs as worker count
	workers := runtime.NumCPU()
	if workers > nonDeletedCount {
		workers = nonDeletedCount
	}

	// Create channels for jobs and results
	jobs := make(chan decryptJob, nonDeletedCount)
	results := make(chan decryptResult, nonDeletedCount)

	// Start worker goroutines
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				di, decryptErr := DecryptItem(job.item, s, iks)
				results <- decryptResult{
					item:  di,
					index: job.index,
					err:   decryptErr,
				}
			}
		}()
	}

	// Queue all decryption jobs
	jobIndex := 0
	for _, e := range ei {
		if e.Deleted {
			continue
		}
		jobs <- decryptJob{item: e, index: jobIndex}
		jobIndex++
	}
	close(jobs)

	// Wait for all workers to finish and close results channel
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results in order
	decrypted := make([]DecryptedItem, nonDeletedCount)
	for result := range results {
		if result.err != nil {
			return nil, result.err
		}
		decrypted[result.index] = result.item
	}

	return decrypted, nil
}

const (
	noteContentSchemaName = "note"
)

func (ei EncryptedItem) DecryptItemOnly(key string) (content []byte, err error) {
	var itemKey []byte

	itemKey, err = DecryptEncryptedItemKey(ei, key)
	if err != nil {
		return
	}

	return DecryptContent(ei, string(itemKey))
}

func (ei *EncryptedItem) Decrypt(mk string) (ik ItemsKey, err error) {
	if ei.ContentType != common.SNItemTypeItemsKey {
		return ik, fmt.Errorf("item passed to decrypt is of type %s, expected SN|ItemsKey", ik.ContentType)
	}

	content, err := ei.DecryptItemOnly(mk)
	if err != nil {
		return
	}

	var f ItemsKey

	err = json.Unmarshal(content, &f)
	if err != nil {
		return
	}

	f.UUID = ei.UUID
	f.ContentType = ei.ContentType
	f.UpdatedAt = ei.UpdatedAt
	f.UpdatedAtTimestamp = ei.UpdatedAtTimestamp
	f.CreatedAtTimestamp = ei.CreatedAtTimestamp
	f.CreatedAt = ei.CreatedAt

	ik = f

	return
}
