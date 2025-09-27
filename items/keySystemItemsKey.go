package items

import (
	"fmt"
	"slices"
	"time"

	"github.com/jonhadfield/gosn-v2/common"
)

func parseKeySystemItemsKey(i DecryptedItem) Item {
	c := KeySystemItemsKey{}

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

		c.Content = *content.(*KeySystemItemsKeyContent)
	}

	return &c
}

// KeySystemItemsKeyContent represents a key system items key for advanced encryption
type KeySystemItemsKeyContent struct {
	Version           string         `json:"version"`           // ProtocolVersion
	CreationTimestamp int64          `json:"creationTimestamp"`
	ItemsKey          string         `json:"itemsKey"`
	RootKeyToken      string         `json:"rootKeyToken"`
	ItemReferences    ItemReferences `json:"references"`
	AppData           AppDataContent `json:"appData"`
}

type KeySystemItemsKey struct {
	ItemCommon
	Content KeySystemItemsKeyContent
}

func (c KeySystemItemsKey) IsDefault() bool {
	return false
}

func (i Items) KeySystemItemsKeys() (c KeySystemItemsKeys) {
	for _, x := range i {
		if x.GetContentType() == common.SNItemTypeKeySystemItemsKey {
			keySystemItemsKey := x.(*KeySystemItemsKey)
			c = append(c, *keySystemItemsKey)
		}
	}

	return c
}

func (c *KeySystemItemsKeys) DeDupe() {
	var encountered []string

	var deDuped KeySystemItemsKeys

	for _, i := range *c {
		if !slices.Contains(encountered, i.UUID) {
			deDuped = append(deDuped, i)
		}

		encountered = append(encountered, i.UUID)
	}

	*c = deDuped
}

// NewKeySystemItemsKey returns an Item of type KeySystemItemsKey without content.
func NewKeySystemItemsKey() KeySystemItemsKey {
	now := time.Now().UTC().Format(common.TimeLayout)

	var c KeySystemItemsKey

	c.ContentType = common.SNItemTypeKeySystemItemsKey
	c.CreatedAt = now
	c.CreatedAtTimestamp = time.Now().UTC().UnixMicro()
	c.UUID = GenUUID()

	return c
}

// NewKeySystemItemsKeyContent returns an empty KeySystemItemsKey content instance.
func NewKeySystemItemsKeyContent() *KeySystemItemsKeyContent {
	c := &KeySystemItemsKeyContent{}
	c.SetUpdateTime(time.Now().UTC())

	return c
}

type KeySystemItemsKeys []KeySystemItemsKey

func (c KeySystemItemsKeys) Validate() error {
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
			case item.Content.Version == "":
				err = fmt.Errorf("failed to create \"%s\" due to missing version: \"%s\"",
					item.ContentType, item.UUID)
			case item.Content.ItemsKey == "":
				err = fmt.Errorf("failed to create \"%s\" due to missing itemsKey: \"%s\"",
					item.ContentType, item.UUID)
			case item.Content.RootKeyToken == "":
				err = fmt.Errorf("failed to create \"%s\" due to missing rootKeyToken: \"%s\"",
					item.ContentType, item.UUID)
			case item.Content.CreationTimestamp == 0:
				err = fmt.Errorf("failed to create \"%s\" due to missing creationTimestamp: \"%s\"",
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

func (c KeySystemItemsKey) IsDeleted() bool {
	return c.Deleted
}

func (c *KeySystemItemsKey) SetDeleted(d bool) {
	c.Deleted = d
}

func (c KeySystemItemsKey) GetContent() Content {
	return &c.Content
}

func (c *KeySystemItemsKey) SetContent(cc Content) {
	c.Content = *cc.(*KeySystemItemsKeyContent)
}

func (c KeySystemItemsKey) GetItemsKeyID() string {
	return c.ItemsKeyID
}

func (c KeySystemItemsKey) GetUUID() string {
	return c.UUID
}

func (c KeySystemItemsKey) GetDuplicateOf() string {
	return c.DuplicateOf
}

func (c *KeySystemItemsKey) SetUUID(u string) {
	c.UUID = u
}

func (c KeySystemItemsKey) GetContentType() string {
	return c.ContentType
}

func (c KeySystemItemsKey) GetCreatedAt() string {
	return c.CreatedAt
}

func (c *KeySystemItemsKey) SetCreatedAt(ca string) {
	c.CreatedAt = ca
}

func (c KeySystemItemsKey) GetUpdatedAt() string {
	return c.UpdatedAt
}

func (c *KeySystemItemsKey) SetUpdatedAt(ca string) {
	c.UpdatedAt = ca
}

func (c KeySystemItemsKey) GetCreatedAtTimestamp() int64 {
	return c.CreatedAtTimestamp
}

func (c *KeySystemItemsKey) SetCreatedAtTimestamp(ca int64) {
	c.CreatedAtTimestamp = ca
}

func (c KeySystemItemsKey) GetUpdatedAtTimestamp() int64 {
	return c.UpdatedAtTimestamp
}

func (c *KeySystemItemsKey) SetUpdatedAtTimestamp(ca int64) {
	c.UpdatedAtTimestamp = ca
}

func (c *KeySystemItemsKey) SetContentType(ct string) {
	c.ContentType = ct
}

func (c KeySystemItemsKey) GetContentSize() int {
	return c.ContentSize
}

func (c *KeySystemItemsKey) SetContentSize(s int) {
	c.ContentSize = s
}

func (cc *KeySystemItemsKeyContent) GetUpdateTime() (time.Time, error) {
	if cc.AppData.OrgStandardNotesSN.ClientUpdatedAt == "" {
		return time.Time{}, fmt.Errorf("ClientUpdatedAt not set")
	}

	return time.Parse(common.TimeLayout, cc.AppData.OrgStandardNotesSN.ClientUpdatedAt)
}

func (cc *KeySystemItemsKeyContent) SetUpdateTime(uTime time.Time) {
	cc.AppData.OrgStandardNotesSN.ClientUpdatedAt = uTime.Format(common.TimeLayout)
}

func (cc KeySystemItemsKeyContent) GetTitle() string {
	return cc.Version
}

func (cc *KeySystemItemsKeyContent) SetTitle(title string) {
	cc.Version = title
}

func (cc *KeySystemItemsKeyContent) GetAppData() AppDataContent {
	return cc.AppData
}

func (cc *KeySystemItemsKeyContent) SetAppData(data AppDataContent) {
	cc.AppData = data
}

func (cc KeySystemItemsKeyContent) References() ItemReferences {
	return cc.ItemReferences
}

func (cc *KeySystemItemsKeyContent) SetReferences(input ItemReferences) {
	cc.ItemReferences = input
}

// GetVersion returns the protocol version
func (cc KeySystemItemsKeyContent) GetVersion() string {
	return cc.Version
}

// SetVersion sets the protocol version
func (cc *KeySystemItemsKeyContent) SetVersion(version string) {
	cc.Version = version
}

// GetCreationTimestamp returns the creation timestamp
func (cc KeySystemItemsKeyContent) GetCreationTimestamp() int64 {
	return cc.CreationTimestamp
}

// SetCreationTimestamp sets the creation timestamp
func (cc *KeySystemItemsKeyContent) SetCreationTimestamp(timestamp int64) {
	cc.CreationTimestamp = timestamp
}

// GetItemsKey returns the encrypted items key
func (cc KeySystemItemsKeyContent) GetItemsKey() string {
	return cc.ItemsKey
}

// SetItemsKey sets the encrypted items key
func (cc *KeySystemItemsKeyContent) SetItemsKey(key string) {
	cc.ItemsKey = key
}

// GetRootKeyToken returns the root key reference token
func (cc KeySystemItemsKeyContent) GetRootKeyToken() string {
	return cc.RootKeyToken
}

// SetRootKeyToken sets the root key reference token
func (cc *KeySystemItemsKeyContent) SetRootKeyToken(token string) {
	cc.RootKeyToken = token
}