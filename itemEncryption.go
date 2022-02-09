package gosn

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

func encryptItems(s *Session, decItems *Items, ik ItemsKey) (encryptedItems EncryptedItems, err error) {
	debugPrint(s.Debug, fmt.Sprintf("encryptItems | encrypting %d items", len(*decItems)))
	d := *decItems

	for _, decItem := range d {
		var e EncryptedItem
		e, err = EncryptItem(decItem, ik, s)
		encryptedItems = append(encryptedItems, e)
	}

	return
}

func (ik ItemsKey) Encrypt(session *Session, new bool) (encryptedItem EncryptedItem, err error) {
	encryptedItem.UUID = ik.UUID

	encryptedItem.ContentType = ik.ContentType

	// updatedat is set by SN so will be zero for a new key
	if !new {
		encryptedItem.UpdatedAt = ik.UpdatedAt
		encryptedItem.UpdatedAtTimestamp = ik.UpdatedAtTimestamp
	}

	encryptedItem.CreatedAt = ik.CreatedAt
	encryptedItem.Deleted = ik.Deleted

	if ik.CreatedAtTimestamp == 0 {
		panic("ik.CreatedAtTimeStamp is 0")
	}

	encryptedItem.CreatedAtTimestamp = ik.CreatedAtTimestamp

	itemEncryptionKey := generateItemKey()

	var encryptedContent string

	if ik.ItemsKey == "" {
		panic("attempting to encrypt empty items key")
	}

	// Marshall the ItemsKey plaintext content
	mContent, err := json.Marshal(ik.Content)
	if err != nil {
		return
	}

	// Create the auth data that will be used to authenticate the encrypted content
	authData := generateAuthData(ik.ContentType, ik.UUID, session.KeyParams)

	b64AuthData := base64.StdEncoding.EncodeToString([]byte(authData))
	// Generate nonce
	nonce := hex.EncodeToString(generateNonce())

	encryptedContent, err = encryptString(string(mContent), itemEncryptionKey, nonce, b64AuthData)
	if err != nil {
		return
	}

	// Create the Encrypted Items Key content element
	content := fmt.Sprintf("004:%s:%s:%s", nonce, encryptedContent, b64AuthData)

	encryptedItem.Content = content
	nonce = hex.EncodeToString(generateNonce())

	// Encrypt the Encrypted Items Key content element with the master key
	var encryptedContentKey string
	encryptedContentKey, err = encryptString(itemEncryptionKey, session.MasterKey, nonce, b64AuthData)
	encItemKey := fmt.Sprintf("004:%s:%s:%s", nonce, encryptedContentKey, b64AuthData)
	encryptedItem.EncItemKey = encItemKey

	switch {
	case encryptedItem.EncItemKey == "":
		panic("produced encrypted ItemsKey with empty enc_item_key")
	case encryptedItem.UUID == "":
		panic("produced encrypted ItemsKey with empty uuid")
	case encryptedItem.Content == "":
		panic("produced encrypted ItemsKey with empty content")
	case encryptedItem.ItemsKeyID != nil:
		panic("produced encrypted ItemsKey non nil ItemsKeyID")
	case encryptedItem.CreatedAtTimestamp == 0:
		panic("encrypted items key has CreatedAtTimestamp set to 0")
	}

	return encryptedItem, err
}

func EncryptItem(item Item, ik ItemsKey, session *Session) (encryptedItem EncryptedItem, err error) {
	var contentEncryptionKey string

	if ik.UUID == "" {
		panic("in EncryptItem with invalid items key (missing UUID)")
	}

	ikid := ik.UUID

	encryptedItem.ItemsKeyID = &ikid
	contentEncryptionKey = ik.ItemsKey
	encryptedItem.UUID = item.GetUUID()
	encryptedItem.ContentType = item.GetContentType()
	encryptedItem.UpdatedAt = item.GetUpdatedAt()
	encryptedItem.CreatedAt = item.GetCreatedAt()
	encryptedItem.Deleted = item.IsDeleted()
	encryptedItem.UpdatedAtTimestamp = item.GetUpdatedAtTimestamp()
	encryptedItem.CreatedAtTimestamp = item.GetCreatedAtTimestamp()
	// Generate Item Key
	itemKey := generateItemKey()

	// get Item Encryption Key
	itemEncryptionKey := itemKey[:len(itemKey)/2]
	// encrypt Item content
	var encryptedContent string

	mContent, _ := json.Marshal(item.GetContent())
	authData := generateAuthData(item.GetContentType(), item.GetUUID(), session.KeyParams)

	b64AuthData := base64.StdEncoding.EncodeToString([]byte(authData))
	nonce := hex.EncodeToString(generateNonce())

	encryptedContent, err = encryptString(string(mContent), itemEncryptionKey, nonce, b64AuthData)
	if err != nil {
		return
	}

	content := fmt.Sprintf("004:%s:%s:%s", nonce, encryptedContent, b64AuthData)
	encryptedItem.Content = content
	// encrypt content encryption key
	var encryptedContentKey string
	encryptedContentKey, err = encryptString(itemEncryptionKey, contentEncryptionKey, nonce, b64AuthData)
	encItemKey := fmt.Sprintf("004:%s:%s:%s", nonce, encryptedContentKey, b64AuthData)
	encryptedItem.EncItemKey = encItemKey

	return encryptedItem, err
}

type AuthData struct {
	Kp struct {
		Identifier  string `json:"identifier"`
		PwNonce     string `json:"pw_nonce"`
		Version     string `json:"version"`
		Origination string `json:"origination"`
		Created     string `json:"created"`
	} `json:"kp"`

	U string `json:"u"`
	V string `json:"v"`
}

func (di DecryptedItem) Encrypt(ik ItemsKey, session *Session) (encryptedItem EncryptedItem, err error) {
	var contentEncryptionKey string

	if ik.UUID == "" {
		panic("in EncryptItem with invalid items key (missing UUID)")
	}

	ikid := ik.UUID

	encryptedItem.ItemsKeyID = &ikid
	contentEncryptionKey = ik.ItemsKey
	encryptedItem.UUID = di.UUID
	encryptedItem.ContentType = di.ContentType
	encryptedItem.UpdatedAt = di.UpdatedAt
	encryptedItem.CreatedAt = di.CreatedAt
	encryptedItem.Deleted = di.Deleted
	encryptedItem.UpdatedAtTimestamp = di.UpdatedAtTimestamp
	encryptedItem.CreatedAtTimestamp = di.CreatedAtTimestamp
	// Generate Item Key
	itemEncryptionKey := generateItemKey()

	mContent := []byte(di.Content)

	authData := generateAuthData(di.ContentType, di.UUID, session.KeyParams)

	b64AuthData := base64.StdEncoding.EncodeToString([]byte(authData))
	nonce := hex.EncodeToString(generateNonce())

	encryptedContent, err := encryptString(string(mContent), itemEncryptionKey, nonce, b64AuthData)
	if err != nil {
		return
	}

	encryptedItem.Content = fmt.Sprintf("004:%s:%s:%s", nonce, encryptedContent, b64AuthData)

	// generate nonce
	nonce = hex.EncodeToString(generateNonce())
	// encrypt content encryption key
	var encryptedContentKey string
	encryptedContentKey, err = encryptString(itemEncryptionKey, contentEncryptionKey, nonce, b64AuthData)
	encItemKey := fmt.Sprintf("004:%s:%s:%s", nonce, encryptedContentKey, b64AuthData)
	encryptedItem.EncItemKey = encItemKey

	return encryptedItem, err
}
