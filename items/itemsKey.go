package items

import (
	crand "crypto/rand"
	"encoding/hex"
	"encoding/json"
	"github.com/jonhadfield/gosn-v2/common"
	"time"
)

// Encrypted ItemsKey has content
// Encrypted ItemsKey content can be unmarshalled onto an ItemsKey, hence it needs no content
// ItemsKey is a merge of the Item (UUID, type, etc.) and the decrypted content
// To decrypt, we decrypt the EncryptedItem as normal
// Get encrypted content and unmarshall onto ItemsKey

type ItemsKey struct {
	// Following attributes set from:
	// - unmarshalling of the EncryptedItem
	UUID               string `json:"uuid"`
	EncryptedItemKey   EIT    `json:"enc_item_key"`
	ContentType        string `json:"content_type"`
	Deleted            bool   `json:"deleted"`
	CreatedAt          string `json:"created_at"`
	UpdatedAt          string `json:"updated_at"`
	CreatedAtTimestamp int64  `json:"created_at_timestamp"`
	UpdatedAtTimestamp int64  `json:"updated_at_timestamp"`
	// Following attributes set from:
	// - the unmarshalled content, post decryption
	// - creation of a new ItemsKey
	ItemsKey       string         `json:"itemsKey"`
	Version        string         `json:"version"`
	ItemReferences ItemReferences `json:"references"`
	AppData        AppDataContent `json:"appData"`
	Default        bool           `json:"isDefault"`
	// Following attibute set only for the purpose of marshaling a new ItemsKey when encrypting
	Content     ItemsKeyContent `json:"content"`
	ContentSize int
}

// ItemsKeyContent is only used when marshaling the ItemsKey, before encryption
// For decryption, we unmarshall the decrypted content string onto the ItemsKey instance
// split enc_item_key - nonce: xxx, cipherText: xxx, authenticatedData: eyJ1IjoiMDg5ODQzN2YtZDViOC00MTNkLWEwNTctODRiODVhNGQzNzRlIiwidiI6IjAwNCJ9

type ItemsKeyEncKey struct {
	Version         string `json:"version"`
	Protocol        string `json:"protocol"`
	EncryptionNonce string `json:"encryption_nonce"`
}

type ItemsKeyContent struct {
	ItemsKey       string         `json:"itemsKey"`
	Version        string         `json:"version"`
	ItemReferences ItemReferences `json:"references"`
	AppData        AppDataContent `json:"appData"`
	Default        bool           `json:"isDefault"`
}

func (i ItemsKeyContent) References() ItemReferences {
	// TODO implement me
	panic("implement me")
}

func (i ItemsKeyContent) AuthData() AppDataContent {
	// TODO implement me
	panic("implement me")
}

type EIT struct {
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

// {UUID:6b43b454-414e-4ac5-b9ca-db41d8cde75f EncryptedItemKey:{Kp:{Identifier: PwNonce: Version: Origination: Created:} U: V:} ContentType:SN|ItemsKey
//  Deleted:false CreatedAt:2022-01-08T17:49:10.190Z UpdatedAt: CreatedAtTimestamp:1641664150190277 UpdatedAtTimestamp:0 ItemsKey:9835761f3c4d3a9db97593564f766790e1bdada329e9c578e491f85c2b2686ab
// Version: ItemReferences:[] AppData:{OrgStandardNotesSN:{ClientUpdatedAt: PrefersPlainEditor:false}} Default:true
// Content:{ItemsKey:9835761f3c4d3a9db97593564f766790e1bdada329e9c578e491f85c2b2686ab Version:004 ItemReferences:[] AppData:{OrgStandardNotesSN:{ClientUpdatedAt: PrefersPlainEditor:false}} Default:true} ContentSize:0}

// NewItemsKey returns an Item of type ItemsKey without content.
func NewItemsKey() ItemsKey {
	now := time.Now().UTC().Format(common.TimeLayout)

	var c ItemsKey

	c.ContentType = "SN|ItemsKey"
	c.CreatedAt = now
	c.CreatedAtTimestamp = time.Now().UTC().UnixMicro()
	c.UUID = GenUUID()

	// TODO: generate items key content
	itemKeyBytes := make([]byte, 64)

	_, err := crand.Read(itemKeyBytes)
	if err != nil {
		panic(err)
	}

	itemKey := hex.EncodeToString(itemKeyBytes)
	// get Item Encryption Key
	val := itemKey[:len(itemKey)/2]

	c.ItemsKey = val

	content := NewItemsKeyContent()
	content.ItemsKey = val
	c.Content = *content

	// ItemsKey       string         `json:"itemsKey"`
	//	Version        string         `json:"version"`
	//	ItemReferences ItemReferences `json:"references"`
	//	AppData        AppDataContent `json:"appData"`
	//	Default        bool           `json:"isDefault"`

	return c
}

func (i ItemsKeyContent) MarshalJSON() ([]byte, error) {
	type Alias ItemsKeyContent

	a := struct {
		Alias
	}{
		Alias: (Alias)(i),
	}

	if a.ItemReferences == nil {
		a.ItemReferences = ItemReferences{}
	}

	return json.Marshal(a)
}

// NewItemsKeyContent returns an empty ItemsKey content instance.
func NewItemsKeyContent() *ItemsKeyContent {
	c := &ItemsKeyContent{}
	c.Version = common.DefaultSNVersion
	// we only create default keys as the only time we generate is:
	// - during registration (no pre-existing keys, therefore this is default)
	// - during export (we re-encrypt everything, so this is not only the default, but also the only one)
	c.Default = true

	itemKeyBytes := make([]byte, 64)

	_, err := crand.Read(itemKeyBytes)
	if err != nil {
		panic(err)
	}

	itemKey := hex.EncodeToString(itemKeyBytes)

	c.ItemsKey = itemKey[:len(itemKey)/2]

	// not setting references or app data as we don't currently need them

	return c
}

type EncItemKey struct {
	ProtocolVersion string
	EncryptionNonce string
	CipherText      string
	AuthData        AuthData
}

// content := fmt.Sprintf("004:%s:%s:%s", nonce, encryptedContent, b64AuthData)
