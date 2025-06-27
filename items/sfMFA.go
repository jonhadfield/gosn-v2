package items

import (
	"fmt"
	"time"

	"github.com/jonhadfield/gosn-v2/common"
)

func parseSFMFA(i DecryptedItem) Item {
	c := SFMFA{}

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
			c.Content = *content.(*SFMFAContent)
		}
	}

	return &c
}

type SFMFAContent struct {
	ItemReferences     ItemReferences `json:"references"`
	AppData            AppDataContent `json:"appData"`
	Name               string         `json:"name"`
	DissociatedItemIds []string       `json:"disassociatedItemIds"`
	AssociatedItemIds  []string       `json:"associatedItemIds"`
	Active             interface{}    `json:"active"`
}

type SFMFA struct {
	ItemCommon
	Content SFMFAContent
}

func (c SFMFA) IsDefault() bool {
	return false
}

func (i Items) SFMFA() (c SFMFAs) {
	for _, x := range i {
		if x.GetContentType() == "SFMFA" {
			component := x.(*SFMFA)
			c = append(c, *component)
		}
	}

	return c
}

func (c *SFMFAs) DeDupe() {
	*c = DeDupeByUUID(*c)
}

// NewSFMFA returns an Item of type SFMFA without content.
func NewSFMFA() SFMFA {
	now := time.Now().UTC().Format(common.TimeLayout)

	var c SFMFA

	c.ContentType = "SFMFA"
	c.CreatedAt = now
	c.UUID = GenUUID()

	return c
}

// NewTagContent returns an empty Tag content instance.
func NewSFMFAContent() *SFMFAContent {
	c := &SFMFAContent{}
	c.SetUpdateTime(time.Now().UTC())

	return c
}

type SFMFAs []SFMFA

func (c SFMFAs) Validate() error {
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

func (c SFMFA) IsDeleted() bool {
	return c.Deleted
}

func (c *SFMFA) SetDeleted(d bool) {
	c.Deleted = d
}

func (c SFMFA) GetContent() Content {
	return &c.Content
}

func (c *SFMFA) SetContent(cc Content) {
	c.Content = *cc.(*SFMFAContent)
}

func (c SFMFA) GetItemsKeyID() string {
	return c.ItemsKeyID
}

func (c SFMFA) GetUUID() string {
	return c.UUID
}

func (c SFMFA) GetDuplicateOf() string {
	return c.DuplicateOf
}

func (c *SFMFA) SetUUID(u string) {
	c.UUID = u
}

func (c SFMFA) GetContentType() string {
	return c.ContentType
}

func (c SFMFA) GetCreatedAt() string {
	return c.CreatedAt
}

func (c *SFMFA) SetCreatedAt(ca string) {
	c.CreatedAt = ca
}

func (c SFMFA) GetUpdatedAt() string {
	return c.UpdatedAt
}

func (c *SFMFA) SetUpdatedAt(ca string) {
	c.UpdatedAt = ca
}

func (c SFMFA) GetCreatedAtTimestamp() int64 {
	return c.CreatedAtTimestamp
}

func (c *SFMFA) SetCreatedAtTimestamp(ca int64) {
	c.CreatedAtTimestamp = ca
}

func (c SFMFA) GetUpdatedAtTimestamp() int64 {
	return c.UpdatedAtTimestamp
}

func (c *SFMFA) SetUpdatedAtTimestamp(ca int64) {
	c.UpdatedAtTimestamp = ca
}

func (c *SFMFA) SetContentType(ct string) {
	c.ContentType = ct
}

func (c SFMFA) GetContentSize() int {
	return c.ContentSize
}

func (c *SFMFA) SetContentSize(s int) {
	c.ContentSize = s
}

func (cc *SFMFAContent) AssociateItems(newItems []string) {
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

func (cc *SFMFAContent) GetItemAssociations() []string {
	return cc.AssociatedItemIds
}

func (cc *SFMFAContent) GetItemDisassociations() []string {
	return cc.DissociatedItemIds
}

func (cc *SFMFAContent) DisassociateItems(itemsToRemove []string) {
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

func (cc *SFMFAContent) GetUpdateTime() (time.Time, error) {
	if cc.AppData.OrgStandardNotesSN.ClientUpdatedAt == "" {
		return time.Time{}, fmt.Errorf("ClientUpdatedAt not set")
	}

	return time.Parse(common.TimeLayout, cc.AppData.OrgStandardNotesSN.ClientUpdatedAt)
}

func (cc *SFMFAContent) SetUpdateTime(uTime time.Time) {
	cc.AppData.OrgStandardNotesSN.ClientUpdatedAt = uTime.Format(common.TimeLayout)
}

func (cc SFMFAContent) GetTitle() string {
	return ""
}

func (cc *SFMFAContent) GetName() string {
	return cc.Name
}

func (cc *SFMFAContent) GetActive() bool {
	return cc.Active.(bool)
}

func (cc *SFMFAContent) SetTitle(title string) {
}

func (cc *SFMFAContent) GetAppData() AppDataContent {
	return cc.AppData
}

func (cc *SFMFAContent) SetAppData(data AppDataContent) {
	cc.AppData = data
}

func (cc SFMFAContent) References() ItemReferences {
	return cc.ItemReferences
}

func (cc *SFMFAContent) UpsertReferences(input ItemReferences) {
	panic("implement me")
}

func (cc *SFMFAContent) SetReferences(input ItemReferences) {
	cc.ItemReferences = input
}
