package items

import (
	"encoding/json"
	"fmt"

	"github.com/jonhadfield/gosn-v2/common"
	"github.com/jonhadfield/gosn-v2/log"
	"github.com/jonhadfield/gosn-v2/session"
)

func DecryptItem(e EncryptedItem, s *session.Session, iks []session.SessionItemsKey) (o DecryptedItem, err error) {
	if e.Deleted {
		err = fmt.Errorf(fmt.Sprintf("cannot decrypt deleted item: %s %s", e.ContentType, e.UUID))

		return
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
func DecryptItems(s *session.Session, ei EncryptedItems, iks []session.SessionItemsKey) (o DecryptedItems, err error) {
	for _, e := range ei {
		if e.Deleted {
			continue
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

		var content []byte

		content, err = e.DecryptItemOnly(key)
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

		o = append(o, di)
	}

	return
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
