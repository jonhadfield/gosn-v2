package gosn

import (
	"fmt"
	"time"
)

func parseSmartTag(i DecryptedItem) Item {
	c := SmartTag{}
	c.UUID = i.UUID
	c.ItemsKeyID = i.ItemsKeyID
	c.ContentType = i.ContentType
	c.Deleted = i.Deleted
	c.UpdatedAt = i.UpdatedAt
	c.CreatedAt = i.CreatedAt
	c.ContentSize = len(i.Content)

	var err error

	if !c.Deleted {
		var content Content

		content, err = processContentModel(i.ContentType, i.Content)
		if err != nil {
			panic(err)
		}

		c.Content = content.(SmartTagContent)
	}

	var cAt, uAt time.Time

	cAt, err = parseSNTime(i.CreatedAt)
	if err != nil {
		panic(err)
	}

	c.CreatedAt = cAt.Format(timeLayout)

	uAt, err = parseSNTime(i.UpdatedAt)
	if err != nil {
		panic(err)
	}

	c.UpdatedAt = uAt.Format(timeLayout)

	return &c
}

type SmartTagContent struct {
	ItemReferences     ItemReferences `json:"references"`
	AppData            AppDataContent `json:"appData"`
	Name               string         `json:"name"`
	DissociatedItemIds []string       `json:"disassociatedItemIds"`
	AssociatedItemIds  []string       `json:"associatedItemIds"`
	Active             interface{}    `json:"active"`
}

type SmartTag struct {
	ItemCommon
	Content SmartTagContent
}

func (i Items) SmartTag() (c SmartTags) {
	for _, x := range i {
		if x.GetContentType() == "SmartTag" {
			component := x.(*SmartTag)
			c = append(c, *component)
		}
	}

	return c
}

func (c *SmartTags) DeDupe() {
	var encountered []string

	var deDuped SmartTags

	for _, i := range *c {
		if !stringInSlice(i.UUID, encountered, true) {
			deDuped = append(deDuped, i)
		}

		encountered = append(encountered, i.UUID)
	}

	*c = deDuped
}

// NewSmartTag returns an Item of type SmartTag without content.
func NewSmartTag() SmartTag {
	now := time.Now().UTC().Format(timeLayout)

	var c SmartTag

	c.ContentType = "SmartTag"
	c.CreatedAt = now
	c.UpdatedAt = now
	c.UUID = GenUUID()

	return c
}

// NewTagContent returns an empty Tag content instance.
func NewSmartTagContent() *SmartTagContent {
	c := &SmartTagContent{}
	c.SetUpdateTime(time.Now().UTC())

	return c
}

type SmartTags []SmartTag

func (c SmartTags) Validate() error {
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

func (c SmartTag) IsDeleted() bool {
	return c.Deleted
}

func (c *SmartTag) SetDeleted(d bool) {
	c.Deleted = d
}

func (c SmartTag) GetContent() Content {
	return &c.Content
}

func (c *SmartTag) SetContent(cc Content) {
	c.Content = cc.(SmartTagContent)
}

func (c SmartTag) GetItemsKeyID() string {
	return c.ItemsKeyID
}

func (c SmartTag) GetUUID() string {
	return c.UUID
}

func (c *SmartTag) SetUUID(u string) {
	c.UUID = u
}

func (c SmartTag) GetContentType() string {
	return c.ContentType
}

func (c SmartTag) GetCreatedAt() string {
	return c.CreatedAt
}

func (c *SmartTag) SetCreatedAt(ca string) {
	c.CreatedAt = ca
}

func (c SmartTag) GetUpdatedAt() string {
	return c.UpdatedAt
}

func (c *SmartTag) SetUpdatedAt(ca string) {
	c.UpdatedAt = ca
}

func (c SmartTag) GetCreatedAtTimestamp() int64 {
	return c.CreatedAtTimestamp
}

func (c *SmartTag) SetCreatedAtTimestamp(ca int64) {
	c.CreatedAtTimestamp = ca
}

func (c SmartTag) GetUpdatedAtTimestamp() int64 {
	return c.UpdatedAtTimestamp
}

func (c *SmartTag) SetUpdatedAtTimestamp(ca int64) {
	c.UpdatedAtTimestamp = ca
}

func (c *SmartTag) SetContentType(ct string) {
	c.ContentType = ct
}

func (c SmartTag) GetContentSize() int {
	return c.ContentSize
}

func (c *SmartTag) SetContentSize(s int) {
	c.ContentSize = s
}

func (cc *SmartTagContent) AssociateItems(newItems []string) {
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

func (cc *SmartTagContent) GetItemAssociations() []string {
	return cc.AssociatedItemIds
}

func (cc *SmartTagContent) GetItemDisassociations() []string {
	return cc.DissociatedItemIds
}

func (cc *SmartTagContent) DisassociateItems(itemsToRemove []string) {
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

func (cc *SmartTagContent) GetUpdateTime() (time.Time, error) {
	if cc.AppData.OrgStandardNotesSN.ClientUpdatedAt == "" {
		return time.Time{}, fmt.Errorf("ClientUpdatedAt not set")
	}

	return time.Parse(timeLayout, cc.AppData.OrgStandardNotesSN.ClientUpdatedAt)
}

func (cc *SmartTagContent) SetUpdateTime(uTime time.Time) {
	cc.AppData.OrgStandardNotesSN.ClientUpdatedAt = uTime.Format(timeLayout)
}

func (cc SmartTagContent) GetTitle() string {
	return ""
}

func (cc *SmartTagContent) GetName() string {
	return cc.Name
}

func (cc *SmartTagContent) GetActive() bool {
	return cc.Active.(bool)
}

func (cc *SmartTagContent) SetTitle(title string) {
}

func (cc *SmartTagContent) GetAppData() AppDataContent {
	return cc.AppData
}

func (cc *SmartTagContent) SetAppData(data AppDataContent) {
	cc.AppData = data
}

func (cc SmartTagContent) References() ItemReferences {
	return cc.ItemReferences
}

func (cc *SmartTagContent) UpsertReferences(input ItemReferences) {
	panic("implement me")
}

func (cc *SmartTagContent) SetReferences(input ItemReferences) {
	panic("implement me")
}
