package items

import (
	"fmt"
	"slices"
	"time"

	"github.com/jonhadfield/gosn-v2/common"
)

func parseKeySystemRootKey(i DecryptedItem) Item {
	c := KeySystemRootKey{}

	if err := populateItemCommon(&c.ItemCommon, i); err != nil {
		panic(err)
	}

	var err error

	if !c.Deleted {
		var content Content

		content, err = processContentModel(i.ContentType, i.Content)
		if err != nil {
			panic(err)
		}

		c.Content = *content.(*KeySystemRootKeyContent)
	}

	return &c
}

// KeySystemRootKeyContent represents a key system root key for advanced encryption
type KeySystemRootKeyContent struct {
	KeyParams        interface{}    `json:"keyParams"`        // KeySystemRootKeyParamsInterface
	SystemIdentifier string         `json:"systemIdentifier"` // KeySystemIdentifier
	Key              string         `json:"key"`
	KeyVersion       string         `json:"keyVersion"`       // ProtocolVersion
	Token            string         `json:"token"`
	ItemReferences   ItemReferences `json:"references"`
	AppData          AppDataContent `json:"appData"`
}

type KeySystemRootKey struct {
	ItemCommon
	Content KeySystemRootKeyContent
}

func (c KeySystemRootKey) IsDefault() bool {
	return false
}

func (i Items) KeySystemRootKeys() (c KeySystemRootKeys) {
	for _, x := range i {
		if x.GetContentType() == common.SNItemTypeKeySystemRootKey {
			keySystemRootKey := x.(*KeySystemRootKey)
			c = append(c, *keySystemRootKey)
		}
	}

	return c
}

func (c *KeySystemRootKeys) DeDupe() {
	var encountered []string

	var deDuped KeySystemRootKeys

	for _, i := range *c {
		if !slices.Contains(encountered, i.UUID) {
			deDuped = append(deDuped, i)
		}

		encountered = append(encountered, i.UUID)
	}

	*c = deDuped
}

// NewKeySystemRootKey returns an Item of type KeySystemRootKey without content.
func NewKeySystemRootKey() KeySystemRootKey {
	now := time.Now().UTC().Format(common.TimeLayout)

	var c KeySystemRootKey

	c.ContentType = common.SNItemTypeKeySystemRootKey
	c.CreatedAt = now
	c.CreatedAtTimestamp = time.Now().UTC().UnixMicro()
	c.UUID = GenUUID()

	return c
}

// NewKeySystemRootKeyContent returns an empty KeySystemRootKey content instance.
func NewKeySystemRootKeyContent() *KeySystemRootKeyContent {
	c := &KeySystemRootKeyContent{}
	c.SetUpdateTime(time.Now().UTC())

	return c
}

type KeySystemRootKeys []KeySystemRootKey

func (c KeySystemRootKeys) Validate() error {
	var updatedTime time.Time

	var err error

	for _, item := range c {
		// validate content if being added
		if !item.Deleted {
			updatedTime, err = item.Content.GetUpdateTime()
			if err != nil {
				return err
			}

			switch {
			case item.Content.SystemIdentifier == "":
				err = fmt.Errorf("failed to create \"%s\" due to missing systemIdentifier: \"%s\"",
					item.ContentType, item.UUID)
			case item.Content.Key == "":
				err = fmt.Errorf("failed to create \"%s\" due to missing key: \"%s\"",
					item.ContentType, item.UUID)
			case item.Content.Token == "":
				err = fmt.Errorf("failed to create \"%s\" due to missing token: \"%s\"",
					item.ContentType, item.UUID)
			case updatedTime.IsZero():
				err = fmt.Errorf("failed to create \"%s\" due to missing content updated time: \"%s\"",
					item.ContentType, item.Content.GetTitle())
			case item.CreatedAt == "":
				err = fmt.Errorf("failed to create \"%s\" due to missing created at date: \"%s\"",
					item.ContentType, item.Content.GetTitle())
			}

			if err != nil {
				return err
			}
		}
	}

	return err
}

func (c KeySystemRootKey) IsDeleted() bool {
	return c.Deleted
}

func (c *KeySystemRootKey) SetDeleted(d bool) {
	c.Deleted = d
}

func (c KeySystemRootKey) GetContent() Content {
	return &c.Content
}

func (c *KeySystemRootKey) SetContent(cc Content) {
	c.Content = *cc.(*KeySystemRootKeyContent)
}

func (c KeySystemRootKey) GetItemsKeyID() string {
	return c.ItemsKeyID
}

func (c KeySystemRootKey) GetUUID() string {
	return c.UUID
}

func (c KeySystemRootKey) GetDuplicateOf() string {
	return c.DuplicateOf
}

func (c *KeySystemRootKey) SetUUID(u string) {
	c.UUID = u
}

func (c KeySystemRootKey) GetContentType() string {
	return c.ContentType
}

func (c KeySystemRootKey) GetCreatedAt() string {
	return c.CreatedAt
}

func (c *KeySystemRootKey) SetCreatedAt(ca string) {
	c.CreatedAt = ca
}

func (c KeySystemRootKey) GetUpdatedAt() string {
	return c.UpdatedAt
}

func (c *KeySystemRootKey) SetUpdatedAt(ca string) {
	c.UpdatedAt = ca
}

func (c KeySystemRootKey) GetCreatedAtTimestamp() int64 {
	return c.CreatedAtTimestamp
}

func (c *KeySystemRootKey) SetCreatedAtTimestamp(ca int64) {
	c.CreatedAtTimestamp = ca
}

func (c KeySystemRootKey) GetUpdatedAtTimestamp() int64 {
	return c.UpdatedAtTimestamp
}

func (c *KeySystemRootKey) SetUpdatedAtTimestamp(ca int64) {
	c.UpdatedAtTimestamp = ca
}

func (c *KeySystemRootKey) SetContentType(ct string) {
	c.ContentType = ct
}

func (c KeySystemRootKey) GetContentSize() int {
	return c.ContentSize
}

func (c *KeySystemRootKey) SetContentSize(s int) {
	c.ContentSize = s
}

func (cc *KeySystemRootKeyContent) GetUpdateTime() (time.Time, error) {
	if cc.AppData.OrgStandardNotesSN.ClientUpdatedAt == "" {
		return time.Time{}, fmt.Errorf("ClientUpdatedAt not set")
	}

	return time.Parse(common.TimeLayout, cc.AppData.OrgStandardNotesSN.ClientUpdatedAt)
}

func (cc *KeySystemRootKeyContent) SetUpdateTime(uTime time.Time) {
	cc.AppData.OrgStandardNotesSN.ClientUpdatedAt = uTime.Format(common.TimeLayout)
}

func (cc KeySystemRootKeyContent) GetTitle() string {
	return cc.SystemIdentifier
}

func (cc *KeySystemRootKeyContent) SetTitle(title string) {
	cc.SystemIdentifier = title
}

func (cc *KeySystemRootKeyContent) GetAppData() AppDataContent {
	return cc.AppData
}

func (cc *KeySystemRootKeyContent) SetAppData(data AppDataContent) {
	cc.AppData = data
}

func (cc KeySystemRootKeyContent) References() ItemReferences {
	return cc.ItemReferences
}

func (cc *KeySystemRootKeyContent) SetReferences(input ItemReferences) {
	cc.ItemReferences = input
}

// GetKeyParams returns the key derivation parameters
func (cc KeySystemRootKeyContent) GetKeyParams() interface{} {
	return cc.KeyParams
}

// SetKeyParams sets the key derivation parameters
func (cc *KeySystemRootKeyContent) SetKeyParams(params interface{}) {
	cc.KeyParams = params
}

// GetSystemIdentifier returns the system identifier
func (cc KeySystemRootKeyContent) GetSystemIdentifier() string {
	return cc.SystemIdentifier
}

// SetSystemIdentifier sets the system identifier
func (cc *KeySystemRootKeyContent) SetSystemIdentifier(identifier string) {
	cc.SystemIdentifier = identifier
}

// GetKey returns the encrypted root key
func (cc KeySystemRootKeyContent) GetKey() string {
	return cc.Key
}

// SetKey sets the encrypted root key
func (cc *KeySystemRootKeyContent) SetKey(key string) {
	cc.Key = key
}

// GetKeyVersion returns the protocol version
func (cc KeySystemRootKeyContent) GetKeyVersion() string {
	return cc.KeyVersion
}

// SetKeyVersion sets the protocol version
func (cc *KeySystemRootKeyContent) SetKeyVersion(version string) {
	cc.KeyVersion = version
}

// GetToken returns the authentication token
func (cc KeySystemRootKeyContent) GetToken() string {
	return cc.Token
}

// SetToken sets the authentication token
func (cc *KeySystemRootKeyContent) SetToken(token string) {
	cc.Token = token
}