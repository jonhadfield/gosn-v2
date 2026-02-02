package items

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/jonhadfield/gosn-v2/auth"
	"github.com/jonhadfield/gosn-v2/common"
	"github.com/jonhadfield/gosn-v2/crypto"
	"github.com/jonhadfield/gosn-v2/log"
	"github.com/jonhadfield/gosn-v2/session"
)

func encryptItems(s *session.Session, decItems *Items, ik session.SessionItemsKey) (encryptedItems EncryptedItems, err error) {
	log.DebugPrint(s.Debug, fmt.Sprintf("encryptItems | encrypting %d items", len(*decItems)), common.MaxDebugChars)
	d := *decItems

	// fmt.Printf("Encrypt | encrypting %d items\n", len(*decItems))
	// for _, x := range *decItems {
	// 	fmt.Printf("----- %s %s\n", x.GetContentType(), x.GetUUID())
	// }

	for _, decItem := range d {
		var e EncryptedItem
		e, err = EncryptItem(decItem, ik, s)
		// fmt.Printf("Encrypt22 | encrypted item: %+v\n", e)
		encryptedItems = append(encryptedItems, e)
	}

	return
}

func EncryptItemsKey(ik session.SessionItemsKey, s *session.Session, new bool) (encryptedItem EncryptedItem, err error) {
	encryptedItem.UUID = ik.UUID
	encryptedItem.ContentType = common.SNItemTypeItemsKey

	// Set timestamps - updated_at is set by SN server for new keys
	if !new {
		encryptedItem.UpdatedAt = ik.UpdatedAt
		encryptedItem.UpdatedAtTimestamp = ik.UpdatedAtTimestamp
	}

	encryptedItem.CreatedAt = ik.CreatedAt
	encryptedItem.Deleted = ik.Deleted

	if ik.CreatedAtTimestamp == 0 {
		return encryptedItem, fmt.Errorf("encryptItemsKey: items key has zero CreatedAtTimestamp")
	}

	encryptedItem.CreatedAtTimestamp = ik.CreatedAtTimestamp

	// Generate random item encryption key
	itemEncryptionKey, err := crypto.GenerateItemKey(64)
	if err != nil {
		return encryptedItem, fmt.Errorf("encryptItemsKey: %w", err)
	}

	var encryptedContent string

	if ik.ItemsKey == "" {
		return encryptedItem, fmt.Errorf("encryptItemsKey: attempting to encrypt empty items key")
	}

	// Construct ItemsKeyContent from SessionItemsKey fields
	content := ItemsKeyContent{
		ItemsKey: ik.ItemsKey,
		Version:  ik.Version,
		Default:  ik.Default,
		// ItemReferences and AppData typically empty for ItemsKeys
		ItemReferences: ItemReferences{},
		AppData:        AppDataContent{},
	}

	// Marshall the ItemsKey plaintext content
	mContent, err := json.Marshal(content)
	if err != nil {
		return
	}

	// Create the auth data that will be used to authenticate the encrypted content
	authData := auth.GenerateAuthData(common.SNItemTypeItemsKey, ik.UUID, s.KeyParams)

	b64AuthData := base64.StdEncoding.EncodeToString([]byte(authData))
	// Generate nonce
	nonceBytes, err := crypto.GenerateNonce()
	if err != nil {
		return encryptedItem, fmt.Errorf("encryptItemsKey: %w", err)
	}
	nonce := hex.EncodeToString(nonceBytes)

	encryptedContent, err = crypto.EncryptString(string(mContent), itemEncryptionKey, nonce, b64AuthData, 32)
	if err != nil {
		return encryptedItem, fmt.Errorf("encryptItemsKey: failed to encrypt content: %w", err)
	}

	// Create the Encrypted Items Key content element
	contentStr := fmt.Sprintf("004:%s:%s:%s", nonce, encryptedContent, b64AuthData)

	encryptedItem.Content = contentStr
	nonceBytes, err = crypto.GenerateNonce()
	if err != nil {
		return encryptedItem, fmt.Errorf("encryptItemsKey: %w", err)
	}
	nonce = hex.EncodeToString(nonceBytes)

	// Encrypt the item encryption key with the master key
	var encryptedContentKey string
	encryptedContentKey, err = crypto.EncryptString(itemEncryptionKey, s.MasterKey, nonce, b64AuthData, 32)
	if err != nil {
		return encryptedItem, fmt.Errorf("encryptItemsKey: failed to encrypt content key: %w", err)
	}
	encItemKey := fmt.Sprintf("004:%s:%s:%s", nonce, encryptedContentKey, b64AuthData)
	encryptedItem.EncItemKey = encItemKey

	switch {
	case encryptedItem.EncItemKey == "":
		return encryptedItem, fmt.Errorf("encryptItemsKey: produced encrypted ItemsKey with empty enc_item_key")
	case encryptedItem.UUID == "":
		return encryptedItem, fmt.Errorf("encryptItemsKey: produced encrypted ItemsKey with empty uuid")
	case encryptedItem.Content == "":
		return encryptedItem, fmt.Errorf("encryptItemsKey: produced encrypted ItemsKey with empty content")
	case encryptedItem.ItemsKeyID != "":
		return encryptedItem, fmt.Errorf("encryptItemsKey: produced encrypted ItemsKey with non-nil ItemsKeyID")
	case encryptedItem.CreatedAtTimestamp == 0:
		return encryptedItem, fmt.Errorf("encryptItemsKey: encrypted items key has CreatedAtTimestamp set to 0")
	}

	return encryptedItem, err
}

func EncryptItem(item Item, ik session.SessionItemsKey, session *session.Session) (encryptedItem EncryptedItem, err error) {
	var contentEncryptionKey string

	if ik.UUID == "" {
		return encryptedItem, fmt.Errorf("encryptItem: invalid items key (missing UUID)")
	}

	ikid := ik.UUID
	// fmt.Println("ikid: ", ikid)

	encryptedItem.ItemsKeyID = ikid
	contentEncryptionKey = ik.ItemsKey
	encryptedItem.UUID = item.GetUUID()
	encryptedItem.ContentType = item.GetContentType()
	encryptedItem.UpdatedAt = item.GetUpdatedAt()
	encryptedItem.CreatedAt = item.GetCreatedAt()
	encryptedItem.Deleted = item.IsDeleted()
	encryptedItem.UpdatedAtTimestamp = item.GetUpdatedAtTimestamp()
	encryptedItem.CreatedAtTimestamp = item.GetCreatedAtTimestamp()
	// Generate Item Key
	itemKey, err := crypto.GenerateItemKey(64)
	if err != nil {
		return encryptedItem, fmt.Errorf("encryptItem: %w", err)
	}
	// fmt.Printf("GENERATED ITEM KEY: %s\n", itemKey)
	// get Item Encryption Key
	itemEncryptionKey := itemKey
	// encrypt Item content
	var encryptedContent string

	mContent, _ := json.Marshal(item.GetContent())
	authData := auth.GenerateAuthData(item.GetContentType(), item.GetUUID(), session.KeyParams)
	b64AuthData := base64.StdEncoding.EncodeToString([]byte(authData))
	nonceBytes, err := crypto.GenerateNonce()
	if err != nil {
		return encryptedItem, fmt.Errorf("encryptItem: %w", err)
	}
	nonce := hex.EncodeToString(nonceBytes)

	encryptedContent, err = crypto.EncryptString(string(mContent), itemEncryptionKey, nonce, b64AuthData, 32)
	if err != nil {
		return encryptedItem, fmt.Errorf("encryptItem: failed to encrypt content: %w", err)
	}

	content := fmt.Sprintf("004:%s:%s:%s", nonce, encryptedContent, b64AuthData)
	encryptedItem.Content = content
	// encrypt content encryption key
	var encryptedContentKey string
	encryptedContentKey, err = crypto.EncryptString(itemEncryptionKey, contentEncryptionKey, nonce, b64AuthData, 32)
	if err != nil {
		return encryptedItem, fmt.Errorf("encryptItem: failed to encrypt content key: %w", err)
	}
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

func (di DecryptedItem) Encrypt(ik ItemsKey, session *session.Session) (encryptedItem EncryptedItem, err error) {
	var contentEncryptionKey string

	if ik.UUID == "" {
		return encryptedItem, fmt.Errorf("encrypt: invalid items key (missing UUID)")
	}

	ikid := ik.UUID

	encryptedItem.ItemsKeyID = ikid
	contentEncryptionKey = ik.ItemsKey
	encryptedItem.UUID = di.UUID
	encryptedItem.ContentType = di.ContentType
	encryptedItem.UpdatedAt = di.UpdatedAt
	encryptedItem.CreatedAt = di.CreatedAt
	encryptedItem.Deleted = di.Deleted
	encryptedItem.UpdatedAtTimestamp = di.UpdatedAtTimestamp
	encryptedItem.CreatedAtTimestamp = di.CreatedAtTimestamp
	// Generate Item Key
	itemEncryptionKey, err := crypto.GenerateItemKey(32)
	if err != nil {
		return encryptedItem, fmt.Errorf("encrypt: %w", err)
	}

	mContent := []byte(di.Content)

	authData := auth.GenerateAuthData(di.ContentType, di.UUID, session.KeyParams)

	b64AuthData := base64.StdEncoding.EncodeToString([]byte(authData))
	nonceBytes, err := crypto.GenerateNonce()
	if err != nil {
		return encryptedItem, fmt.Errorf("encrypt: %w", err)
	}
	nonce := hex.EncodeToString(nonceBytes)

	encryptedContent, err := crypto.EncryptString(string(mContent), itemEncryptionKey, nonce, b64AuthData, 32)
	if err != nil {
		return encryptedItem, fmt.Errorf("encrypt: failed to encrypt content: %w", err)
	}

	encryptedItem.Content = fmt.Sprintf("004:%s:%s:%s", nonce, encryptedContent, b64AuthData)
	// generate nonce
	nonceBytes, err = crypto.GenerateNonce()
	if err != nil {
		return encryptedItem, fmt.Errorf("encrypt: %w", err)
	}
	nonce = hex.EncodeToString(nonceBytes)
	// encrypt content encryption key
	var encryptedContentKey string
	encryptedContentKey, err = crypto.EncryptString(itemEncryptionKey, contentEncryptionKey, nonce, b64AuthData, 32)
	if err != nil {
		return encryptedItem, fmt.Errorf("encrypt: failed to encrypt content key: %w", err)
	}
	encItemKey := fmt.Sprintf("004:%s:%s:%s", nonce, encryptedContentKey, b64AuthData)
	encryptedItem.EncItemKey = encItemKey

	return encryptedItem, err
}
