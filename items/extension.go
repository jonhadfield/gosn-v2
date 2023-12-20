package items

import (
	"fmt"
	"slices"
	"time"

	"github.com/jonhadfield/gosn-v2/common"
)

func parseExtension(i DecryptedItem) Item {
	c := Extension{}
	c.UUID = i.UUID
	c.ItemsKeyID = i.ItemsKeyID
	c.ContentType = i.ContentType
	c.Deleted = i.Deleted
	c.UpdatedAt = i.UpdatedAt
	c.CreatedAt = i.CreatedAt
	c.UpdatedAtTimestamp = i.UpdatedAtTimestamp
	c.CreatedAtTimestamp = i.CreatedAtTimestamp
	c.ContentSize = len(i.Content)

	var err error

	if !c.Deleted {
		var content Content

		content, err = processContentModel(i.ContentType, i.Content)
		if err != nil {
			panic(err)
		}

		c.Content = *content.(*ExtensionContent)
	}

	var cAt, uAt time.Time

	cAt, err = parseSNTime(i.CreatedAt)
	if err != nil {
		panic(err)
	}

	c.CreatedAt = cAt.Format(common.TimeLayout)

	uAt, err = parseSNTime(i.UpdatedAt)
	if err != nil {
		panic(err)
	}

	c.UpdatedAt = uAt.Format(common.TimeLayout)

	return &c
}

type ExtensionContent struct {
	ItemReferences     ItemReferences `json:"references"`
	AppData            AppDataContent `json:"appData"`
	Name               string         `json:"name"`
	DissociatedItemIds []string       `json:"disassociatedItemIds"`
	AssociatedItemIds  []string       `json:"associatedItemIds"`
	Active             interface{}    `json:"active"`
}

type Extension struct {
	ItemCommon
	Content ExtensionContent
}

func (c Extension) IsDefault() bool {
	return false
}

func (i Items) Extension() (c Extensions) {
	for _, x := range i {
		if x.GetContentType() == common.SNItemTypeExtension {
			component := x.(*Extension)
			c = append(c, *component)
		}
	}

	return c
}

func (c *Extensions) DeDupe() {
	var encountered []string

	var deDuped Extensions

	for _, i := range *c {
		if !slices.Contains(encountered, i.UUID) {
			deDuped = append(deDuped, i)
		}

		encountered = append(encountered, i.UUID)
	}

	*c = deDuped
}

// NewExtension returns an Item of type Extension without content.
func NewExtension() Extension {
	now := time.Now().UTC().Format(common.TimeLayout)

	var c Extension

	c.ContentType = common.SNItemTypeExtension
	c.CreatedAt = now
	c.UUID = GenUUID()

	return c
}

// NewExtensionContent returns an empty Tag content instance.
func NewExtensionContent() *ExtensionContent {
	c := &ExtensionContent{}
	c.SetUpdateTime(time.Now().UTC())

	return c
}

type Extensions []Extension

func (c Extensions) Validate() error {
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

func (c Extension) IsDeleted() bool {
	return c.Deleted
}

func (c *Extension) SetDeleted(d bool) {
	c.Deleted = d
}

func (c Extension) GetContent() Content {
	return &c.Content
}

func (c *Extension) SetContent(cc Content) {
	c.Content = *cc.(*ExtensionContent)
}

func (c Extension) GetItemsKeyID() string {
	return c.ItemsKeyID
}

func (c Extension) GetUUID() string {
	return c.UUID
}

func (c Extension) GetDuplicateOf() string {
	return c.DuplicateOf
}

func (c *Extension) SetUUID(u string) {
	c.UUID = u
}

func (c Extension) GetContentType() string {
	return c.ContentType
}

func (c Extension) GetCreatedAt() string {
	return c.CreatedAt
}

func (c *Extension) SetCreatedAt(ca string) {
	c.CreatedAt = ca
}

func (c Extension) GetUpdatedAt() string {
	return c.UpdatedAt
}

func (c *Extension) SetUpdatedAt(ca string) {
	c.UpdatedAt = ca
}

func (c Extension) GetCreatedAtTimestamp() int64 {
	return c.CreatedAtTimestamp
}

func (c *Extension) SetCreatedAtTimestamp(ca int64) {
	c.CreatedAtTimestamp = ca
}

func (c Extension) GetUpdatedAtTimestamp() int64 {
	return c.UpdatedAtTimestamp
}

func (c *Extension) SetUpdatedAtTimestamp(ca int64) {
	c.UpdatedAtTimestamp = ca
}

func (c *Extension) SetContentType(ct string) {
	c.ContentType = ct
}

func (c Extension) GetContentSize() int {
	return c.ContentSize
}

func (c *Extension) SetContentSize(s int) {
	c.ContentSize = s
}

func (cc *ExtensionContent) AssociateItems(newItems []string) {
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

func (cc *ExtensionContent) GetItemAssociations() []string {
	return cc.AssociatedItemIds
}

func (cc *ExtensionContent) GetItemDisassociations() []string {
	return cc.DissociatedItemIds
}

func (cc *ExtensionContent) DisassociateItems(itemsToRemove []string) {
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

func (cc *ExtensionContent) GetUpdateTime() (time.Time, error) {
	if cc.AppData.OrgStandardNotesSN.ClientUpdatedAt == "" {
		return time.Time{}, fmt.Errorf("ClientUpdatedAt not set")
	}

	return time.Parse(common.TimeLayout, cc.AppData.OrgStandardNotesSN.ClientUpdatedAt)
}

func (cc *ExtensionContent) SetUpdateTime(uTime time.Time) {
	cc.AppData.OrgStandardNotesSN.ClientUpdatedAt = uTime.Format(common.TimeLayout)
}

func (cc ExtensionContent) GetTitle() string {
	return ""
}

func (cc *ExtensionContent) GetName() string {
	return cc.Name
}

func (cc *ExtensionContent) GetActive() bool {
	return cc.Active.(bool)
}

func (cc *ExtensionContent) SetTitle(title string) {
}

func (cc *ExtensionContent) GetAppData() AppDataContent {
	return cc.AppData
}

func (cc *ExtensionContent) SetAppData(data AppDataContent) {
	cc.AppData = data
}

func (cc ExtensionContent) References() ItemReferences {
	return cc.ItemReferences
}

func (cc *ExtensionContent) UpsertReferences(input ItemReferences) {
	panic("implement me")
}

func (cc *ExtensionContent) SetReferences(input ItemReferences) {
	cc.ItemReferences = input
}
