package items

import (
	"fmt"
	"slices"
	"time"

	"github.com/jonhadfield/gosn-v2/common"
)

func parseSFExtension(i DecryptedItem) Item {
	c := SFExtension{}

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

		if content != nil {
			c.Content = *content.(*SFExtensionContent)
		}
	}

	return &c
}

type SFExtensionContent struct {
	ItemReferences     ItemReferences `json:"references"`
	AppData            AppDataContent `json:"appData"`
	Name               string         `json:"name"`
	DissociatedItemIds []string       `json:"disassociatedItemIds"`
	AssociatedItemIds  []string       `json:"associatedItemIds"`
	Active             interface{}    `json:"active"`
}

type SFExtension struct {
	ItemCommon
	Content SFExtensionContent
}

func (c SFExtension) IsDefault() bool {
	return false
}

func (i Items) SFExtension() (c SFExtensions) {
	for _, x := range i {
		if x.GetContentType() == common.SNItemTypeSFExtension {
			component := x.(*SFExtension)
			c = append(c, *component)
		}
	}

	return c
}

func (c *SFExtensions) DeDupe() {
	var encountered []string

	var deDuped SFExtensions

	for _, i := range *c {
		if !slices.Contains(encountered, i.UUID) {
			deDuped = append(deDuped, i)
		}

		encountered = append(encountered, i.UUID)
	}

	*c = deDuped
}

// NewSFExtension returns an Item of type SFExtension without content.
func NewSFExtension() SFExtension {
	now := time.Now().UTC().Format(common.TimeLayout)

	var c SFExtension

	c.ContentType = common.SNItemTypeSFExtension
	c.CreatedAt = now
	c.CreatedAtTimestamp = time.Now().UTC().UnixMicro()
	c.UUID = GenUUID()

	return c
}

// NewSFExtensionContent returns an empty Tag content instance.
func NewSFExtensionContent() *SFExtensionContent {
	c := &SFExtensionContent{}
	c.SetUpdateTime(time.Now().UTC())

	return c
}

type SFExtensions []SFExtension

func (c SFExtensions) Validate() error {
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

func (c SFExtension) IsDeleted() bool {
	return c.Deleted
}

func (c *SFExtension) SetDeleted(d bool) {
	c.Deleted = d
}

func (c SFExtension) GetContent() Content {
	return &c.Content
}

func (c *SFExtension) SetContent(cc Content) {
	c.Content = *cc.(*SFExtensionContent)
}

func (c SFExtension) GetItemsKeyID() string {
	return c.ItemsKeyID
}

func (c SFExtension) GetUUID() string {
	return c.UUID
}

func (c SFExtension) GetDuplicateOf() string {
	return c.DuplicateOf
}

func (c *SFExtension) SetUUID(u string) {
	c.UUID = u
}

func (c SFExtension) GetContentType() string {
	return c.ContentType
}

func (c SFExtension) GetCreatedAt() string {
	return c.CreatedAt
}

func (c *SFExtension) SetCreatedAt(ca string) {
	c.CreatedAt = ca
}

func (c SFExtension) GetUpdatedAt() string {
	return c.UpdatedAt
}

func (c *SFExtension) SetUpdatedAt(ca string) {
	c.UpdatedAt = ca
}

func (c SFExtension) GetCreatedAtTimestamp() int64 {
	return c.CreatedAtTimestamp
}

func (c *SFExtension) SetCreatedAtTimestamp(ca int64) {
	c.CreatedAtTimestamp = ca
}

func (c SFExtension) GetUpdatedAtTimestamp() int64 {
	return c.UpdatedAtTimestamp
}

func (c *SFExtension) SetUpdatedAtTimestamp(ca int64) {
	c.UpdatedAtTimestamp = ca
}

func (c *SFExtension) SetContentType(ct string) {
	c.ContentType = ct
}

func (c SFExtension) GetContentSize() int {
	return c.ContentSize
}

func (c *SFExtension) SetContentSize(s int) {
	c.ContentSize = s
}

func (cc *SFExtensionContent) AssociateItems(newItems []string) {
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

func (cc *SFExtensionContent) GetItemAssociations() []string {
	return cc.AssociatedItemIds
}

func (cc *SFExtensionContent) GetItemDisassociations() []string {
	return cc.DissociatedItemIds
}

func (cc *SFExtensionContent) DisassociateItems(itemsToRemove []string) {
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

func (cc *SFExtensionContent) GetUpdateTime() (time.Time, error) {
	if cc.AppData.OrgStandardNotesSN.ClientUpdatedAt == "" {
		return time.Time{}, fmt.Errorf("ClientUpdatedAt not set")
	}

	return time.Parse(common.TimeLayout, cc.AppData.OrgStandardNotesSN.ClientUpdatedAt)
}

func (cc *SFExtensionContent) SetUpdateTime(uTime time.Time) {
	cc.AppData.OrgStandardNotesSN.ClientUpdatedAt = uTime.Format(common.TimeLayout)
}

func (cc SFExtensionContent) GetTitle() string {
	return ""
}

func (cc *SFExtensionContent) GetName() string {
	return cc.Name
}

func (cc *SFExtensionContent) GetActive() bool {
	return cc.Active.(bool)
}

func (cc *SFExtensionContent) SetTitle(title string) {
}

func (cc *SFExtensionContent) GetAppData() AppDataContent {
	return cc.AppData
}

func (cc *SFExtensionContent) SetAppData(data AppDataContent) {
	cc.AppData = data
}

func (cc SFExtensionContent) References() ItemReferences {
	return cc.ItemReferences
}

func (cc *SFExtensionContent) UpsertReferences(input ItemReferences) {
	cc.SetReferences(UpsertReferences(cc.ItemReferences, input))
}

func (cc *SFExtensionContent) SetReferences(input ItemReferences) {
	cc.ItemReferences = input
}
