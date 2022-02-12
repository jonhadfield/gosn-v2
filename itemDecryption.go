package gosn

import (
	"encoding/json"
	"fmt"
)

func DecryptItem(e EncryptedItem, s *Session, iks ItemsKeys) (o DecryptedItem, err error) {
	debugPrint(s.Debug, fmt.Sprintf("Decrypt | decrypting %s %s", e.ContentType, e.UUID))

	if e.Deleted {
		err = fmt.Errorf(fmt.Sprintf("cannot decrypt deleted item: %s %s", e.ContentType, e.UUID))

		return
	}

	var contentEncryptionKey string

	ik := getMatchingItem(e.GetItemsKeyID(), iks)

	switch {
	case ik.ItemsKey != "":
		contentEncryptionKey = ik.ItemsKey
	case isEncryptedWithMasterKey(e.ContentType):
		contentEncryptionKey = s.MasterKey
	default:
		if e.ItemsKeyID == nil {
			debugPrint(s.Debug, fmt.Sprintf("decryptItems | missing ItemsKeyID for content type: %s", e.ContentType))
			err = fmt.Errorf("encountered deleted: %t item %s of type %s without ItemsKeyID", e.Deleted, e.UUID, e.ContentType)

			return
		}

		contentEncryptionKey = getMatchingItem(*e.ItemsKeyID, s.ItemsKeys).ItemsKey
		if contentEncryptionKey == "" {
			err = fmt.Errorf("deleted: %t item %s of type %s cannot be decrypted as we're missing ItemsKey %s", e.Deleted, e.UUID, e.ContentType, *e.ItemsKeyID)
			return
		}
	}

	itemKey, err := decryptEncryptedItemKey(e, contentEncryptionKey)
	if err != nil {
		return
	}

	content, err := decryptContent(e, string(itemKey))
	if err != nil {
		return
	}

	var di DecryptedItem
	di.UUID = e.UUID
	di.ContentType = e.ContentType
	di.Deleted = e.Deleted

	if e.ItemsKeyID != nil {
		di.ItemsKeyID = *e.ItemsKeyID
	}

	di.UpdatedAt = e.UpdatedAt
	di.CreatedAt = e.CreatedAt
	di.CreatedAtTimestamp = e.CreatedAtTimestamp
	di.UpdatedAtTimestamp = e.UpdatedAtTimestamp
	di.Content = string(content)

	return di, err
}

// DecryptAndParseItemKeys takes the master key and a list of EncryptedItemKeys
// and returns a list of items keys.
func DecryptAndParseItemKeys(mk string, eiks EncryptedItems) (iks []ItemsKey, err error) {
	for _, eik := range eiks {
		if eik.ContentType != "SN|ItemsKey" {
			continue
		}

		var itemKey []byte

		itemKey, err = decryptEncryptedItemKey(eik, mk)
		if err != nil {
			return
		}

		var content []byte

		content, err = decryptContent(eik, string(itemKey))
		if err != nil {
			return
		}

		var f ItemsKey

		err = json.Unmarshal(content, &f)
		if err != nil {
			return
		}

		f.UUID = eik.UUID
		f.ContentType = eik.ContentType
		f.UpdatedAt = eik.UpdatedAt
		f.UpdatedAtTimestamp = eik.UpdatedAtTimestamp
		f.CreatedAtTimestamp = eik.CreatedAtTimestamp
		f.CreatedAt = eik.CreatedAt

		if f.ItemsKey == "" {
			continue
		}

		iks = append(iks, f)
	}

	return
}

// Decrypt.
func (ei EncryptedItems) Decrypt(s *Session, iks ItemsKeys) (o DecryptedItems, err error) {
	debugPrint(s.Debug, fmt.Sprintf("Decrypt | decrypting %d items", len(ei)))

	for x := range ei {
		if ei[x].Deleted {
			continue
		}

		var di DecryptedItem

		di, err = DecryptItem(ei[x], s, iks)
		if err != nil {
			return
		}

		o = append(o, di)
	}

	return
}

func (ei EncryptedItem) Decrypt(mk string) (ik ItemsKey, err error) {
	if ei.ContentType != "SN|ItemsKey" {
		return ik, fmt.Errorf("item passed to decrypt is of type %s, expected SN|ItemsKey", ik.ContentType)
	}

	var itemKey []byte

	itemKey, err = decryptEncryptedItemKey(ei, mk)
	if err != nil {
		return
	}

	var content []byte

	content, err = decryptContent(ei, string(itemKey))
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
