package items

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/jonhadfield/gosn-v2/common"
)

type FileSafeFileMetaDataContent struct {
	ServerMetadata     json.RawMessage `json:"serverMetadata"`
	ItemReferences     ItemReferences  `json:"references"`
	AppData            AppDataContent  `json:"appData"`
	Name               string          `json:"name"`
	DissociatedItemIds []string        `json:"disassociatedItemIds"`
	AssociatedItemIds  []string        `json:"associatedItemIds"`
	Active             interface{}     `json:"active"`
}

func parseFileSafeFileMetadata(i DecryptedItem) Item {
	c := FileSafeFileMetaData{}

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

		c.Content = *content.(*FileSafeFileMetaDataContent)
	}

	return &c
}

type FileSafeFileMetaData struct {
	ItemCommon
	Content FileSafeFileMetaDataContent
}

func (c FileSafeFileMetaData) IsDefault() bool {
	return false
}

func (i Items) FileSafeFileMetaData() (c FileSafeFileMetaDatas) {
	for _, x := range i {
		if x.GetContentType() == "FileSafeFileMetaData" {
			component := x.(*FileSafeFileMetaData)
			c = append(c, *component)
		}
	}

	return c
}

func (c *FileSafeFileMetaDatas) DeDupe() {
	*c = DeDupeByUUID(*c)
}

// NewFileSafeFileMetaData returns an Item of type FileSafeFileMetaData without content.
func NewFileSafeFileMetaData() FileSafeFileMetaData {
	now := time.Now().UTC().Format(common.TimeLayout)

	var c FileSafeFileMetaData

	c.ContentType = "FileSafeFileMetaData"
	c.CreatedAt = now
	c.UUID = GenUUID()

	return c
}

// NewTagContent returns an empty Tag content instance.
func NewFileSafeFileMetaDataContent() *FileSafeFileMetaDataContent {
	c := &FileSafeFileMetaDataContent{}
	c.SetUpdateTime(time.Now().UTC())

	return c
}

type FileSafeFileMetaDatas []FileSafeFileMetaData

func (c FileSafeFileMetaDatas) Validate() error {
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
			case item.Content.Name == "":
				err = fmt.Errorf("failed to create \"%s\" due to missing title: \"%s\"",
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

func (c FileSafeFileMetaData) IsDeleted() bool {
	return c.Deleted
}

func (c *FileSafeFileMetaData) SetDeleted(d bool) {
	c.Deleted = d
}

func (c FileSafeFileMetaData) GetContent() Content {
	return &c.Content
}

func (c *FileSafeFileMetaData) SetContent(cc Content) {
	c.Content = *cc.(*FileSafeFileMetaDataContent)
}

func (c FileSafeFileMetaData) GetItemsKeyID() string {
	return c.ItemsKeyID
}

func (c FileSafeFileMetaData) GetUUID() string {
	return c.UUID
}

func (c FileSafeFileMetaData) GetDuplicateOf() string {
	return c.DuplicateOf
}

func (c *FileSafeFileMetaData) SetUUID(u string) {
	c.UUID = u
}

func (c FileSafeFileMetaData) GetContentType() string {
	return c.ContentType
}

func (c FileSafeFileMetaData) GetCreatedAt() string {
	return c.CreatedAt
}

func (c *FileSafeFileMetaData) SetCreatedAt(ca string) {
	c.CreatedAt = ca
}

func (c FileSafeFileMetaData) GetUpdatedAt() string {
	return c.UpdatedAt
}

func (c *FileSafeFileMetaData) SetUpdatedAt(ca string) {
	c.UpdatedAt = ca
}

func (c FileSafeFileMetaData) GetCreatedAtTimestamp() int64 {
	return c.CreatedAtTimestamp
}

func (c *FileSafeFileMetaData) SetCreatedAtTimestamp(ca int64) {
	c.CreatedAtTimestamp = ca
}

func (c FileSafeFileMetaData) GetUpdatedAtTimestamp() int64 {
	return c.UpdatedAtTimestamp
}

func (c *FileSafeFileMetaData) SetUpdatedAtTimestamp(ca int64) {
	c.UpdatedAtTimestamp = ca
}

func (c *FileSafeFileMetaData) SetContentType(ct string) {
	c.ContentType = ct
}

func (c FileSafeFileMetaData) GetContentSize() int {
	return c.ContentSize
}

func (c *FileSafeFileMetaData) SetContentSize(s int) {
	c.ContentSize = s
}

func (cc *FileSafeFileMetaDataContent) AssociateItems(newItems []string) {
	// add to associated item ids
	for _, newRef := range newItems {
		var existingFound bool

		var existingDFound bool

		for _, existingRef := range cc.AssociatedItemIds {
			if existingRef == newRef {
				existingFound = true
			}
		}

		for _, existingDRef := range cc.DissociatedItemIds {
			if existingDRef == newRef {
				existingDFound = true
			}
		}

		// add reference if it doesn't exist
		if !existingFound {
			cc.AssociatedItemIds = append(cc.AssociatedItemIds, newRef)
		}

		// remove reference (from disassociated) if it does exist in that list
		if existingDFound {
			cc.DissociatedItemIds = removeStringFromSlice(newRef, cc.DissociatedItemIds)
		}
	}
}

func (cc *FileSafeFileMetaDataContent) GetItemAssociations() []string {
	return cc.AssociatedItemIds
}

func (cc *FileSafeFileMetaDataContent) GetItemDisassociations() []string {
	return cc.DissociatedItemIds
}

func (cc *FileSafeFileMetaDataContent) DisassociateItems(itemsToRemove []string) {
	// remove from associated item ids
	for _, delRef := range itemsToRemove {
		var existingFound bool

		for _, existingRef := range cc.AssociatedItemIds {
			if existingRef == delRef {
				existingFound = true
			}
		}

		// remove reference (from disassociated) if it does exist in that list
		if existingFound {
			cc.AssociatedItemIds = removeStringFromSlice(delRef, cc.AssociatedItemIds)
		}
	}
}

func (cc *FileSafeFileMetaDataContent) GetUpdateTime() (time.Time, error) {
	if cc.AppData.OrgStandardNotesSN.ClientUpdatedAt == "" {
		return time.Time{}, fmt.Errorf("ClientUpdatedAt not set")
	}

	return time.Parse(common.TimeLayout, cc.AppData.OrgStandardNotesSN.ClientUpdatedAt)
}

func (cc *FileSafeFileMetaDataContent) SetUpdateTime(uTime time.Time) {
	cc.AppData.OrgStandardNotesSN.ClientUpdatedAt = uTime.Format(common.TimeLayout)
}

func (cc FileSafeFileMetaDataContent) GetTitle() string {
	return ""
}

func (cc *FileSafeFileMetaDataContent) GetName() string {
	return cc.Name
}

func (cc *FileSafeFileMetaDataContent) GetActive() bool {
	return cc.Active.(bool)
}

func (cc *FileSafeFileMetaDataContent) SetTitle(title string) {
}

func (cc *FileSafeFileMetaDataContent) GetAppData() AppDataContent {
	return cc.AppData
}

func (cc *FileSafeFileMetaDataContent) SetAppData(data AppDataContent) {
	cc.AppData = data
}

func (cc FileSafeFileMetaDataContent) References() ItemReferences {
	return cc.ItemReferences
}

func (cc *FileSafeFileMetaDataContent) UpsertReferences(input ItemReferences) {
	panic("implement me")
}

func (cc *FileSafeFileMetaDataContent) SetReferences(input ItemReferences) {
	cc.ItemReferences = input
}
