package gosn

import (
	"fmt"
	"slices"
	"time"
)

func parseUserPreferences(i DecryptedItem) Item {
	c := UserPreferences{}
	c.UUID = i.UUID
	c.ItemsKeyID = i.ItemsKeyID
	c.ContentType = i.ContentType
	c.Deleted = i.Deleted
	c.UpdatedAt = i.UpdatedAt
	c.CreatedAt = i.CreatedAt
	c.UpdatedAtTimestamp = i.UpdatedAtTimestamp
	c.CreatedAtTimestamp = i.CreatedAtTimestamp

	var err error

	if !c.Deleted {
		var content Content

		content, err = processContentModel(i.ContentType, i.Content)
		if err != nil {
			panic(err)
		}

		c.Content = *content.(*UserPreferencesContent)
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

func parseSNTime(s string) (t time.Time, err error) {
	// if no time specified, then return zero time
	if s == "" {
		return
	}

	t, err = time.Parse(timeLayout, s)
	if err == nil {
		return
	}

	return time.Parse(timeLayout2, s)
}

type UserPreferencesContent struct {
	ItemReferences     ItemReferences `json:"references"`
	AppData            AppDataContent `json:"appData"`
	Name               string         `json:"name"`
	DissociatedItemIds []string       `json:"disassociatedItemIds"`
	AssociatedItemIds  []string       `json:"associatedItemIds"`
	Active             interface{}    `json:"active"`
}

type UserPreferences struct {
	ItemCommon
	Content UserPreferencesContent
}

func (c UserPreferences) IsDefault() bool {
	return false
}

func (i Items) UserPreferences() (c UserPreferencess) {
	for _, x := range i {
		if x.GetContentType() == "UserPreferences" {
			component := x.(*UserPreferences)
			c = append(c, *component)
		}
	}

	return c
}

func (c *UserPreferencess) DeDupe() {
	var encountered []string

	var deDuped UserPreferencess

	for _, i := range *c {
		if !slices.Contains(encountered, i.UUID) {
			deDuped = append(deDuped, i)
		}

		encountered = append(encountered, i.UUID)
	}

	*c = deDuped
}

// NewUserPreferences returns an Item of type UserPreferences without content.
func NewUserPreferences() UserPreferences {
	now := time.Now().UTC().Format(timeLayout)

	var c UserPreferences

	c.ContentType = "UserPreferences"
	c.CreatedAt = now
	c.CreatedAtTimestamp = time.Now().UTC().UnixMicro()
	c.UUID = GenUUID()

	return c
}

// NewUserPreferencesContent returns an empty Tag content instance.
func NewUserPreferencesContent() *UserPreferencesContent {
	c := &UserPreferencesContent{}
	c.SetUpdateTime(time.Now().UTC())

	return c
}

type UserPreferencess []UserPreferences

func (c UserPreferencess) Validate() error {
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

func (c UserPreferences) IsDeleted() bool {
	return c.Deleted
}

func (c *UserPreferences) SetDeleted(d bool) {
	c.Deleted = d
}

func (c UserPreferences) GetContent() Content {
	return &c.Content
}

func (c *UserPreferences) SetContent(cc Content) {
	c.Content = *cc.(*UserPreferencesContent)
}

func (c UserPreferences) GetItemsKeyID() string {
	return c.ItemsKeyID
}

func (c UserPreferences) GetUUID() string {
	return c.UUID
}

func (c *UserPreferences) SetUUID(u string) {
	c.UUID = u
}

func (c UserPreferences) GetContentType() string {
	return c.ContentType
}

func (c UserPreferences) GetCreatedAt() string {
	return c.CreatedAt
}

func (c *UserPreferences) SetCreatedAt(ca string) {
	c.CreatedAt = ca
}

func (c UserPreferences) GetUpdatedAt() string {
	return c.UpdatedAt
}

func (c *UserPreferences) SetUpdatedAt(ca string) {
	c.UpdatedAt = ca
}

func (c UserPreferences) GetCreatedAtTimestamp() int64 {
	return c.CreatedAtTimestamp
}

func (c *UserPreferences) SetCreatedAtTimestamp(ca int64) {
	c.CreatedAtTimestamp = ca
}

func (c UserPreferences) GetUpdatedAtTimestamp() int64 {
	return c.UpdatedAtTimestamp
}

func (c *UserPreferences) SetUpdatedAtTimestamp(ca int64) {
	c.UpdatedAtTimestamp = ca
}

func (c *UserPreferences) SetContentType(ct string) {
	c.ContentType = ct
}

func (c UserPreferences) GetContentSize() int {
	return c.ContentSize
}

func (c *UserPreferences) SetContentSize(s int) {
	c.ContentSize = s
}

func (cc *UserPreferencesContent) AssociateItems(newItems []string) {
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

func (cc *UserPreferencesContent) GetItemAssociations() []string {
	return cc.AssociatedItemIds
}

func (cc *UserPreferencesContent) GetItemDisassociations() []string {
	return cc.DissociatedItemIds
}

func (cc *UserPreferencesContent) DisassociateItems(itemsToRemove []string) {
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

func (cc *UserPreferencesContent) GetUpdateTime() (time.Time, error) {
	if cc.AppData.OrgStandardNotesSN.ClientUpdatedAt == "" {
		return time.Time{}, fmt.Errorf("ClientUpdatedAt not set")
	}

	return time.Parse(timeLayout, cc.AppData.OrgStandardNotesSN.ClientUpdatedAt)
}

func (cc *UserPreferencesContent) SetUpdateTime(uTime time.Time) {
	cc.AppData.OrgStandardNotesSN.ClientUpdatedAt = uTime.Format(timeLayout)
}

func (cc UserPreferencesContent) GetTitle() string {
	return ""
}

func (cc *UserPreferencesContent) GetName() string {
	return cc.Name
}

func (cc *UserPreferencesContent) GetActive() bool {
	return cc.Active.(bool)
}

func (cc *UserPreferencesContent) SetTitle(title string) {
}

func (cc *UserPreferencesContent) GetAppData() AppDataContent {
	return cc.AppData
}

func (cc *UserPreferencesContent) SetAppData(data AppDataContent) {
	cc.AppData = data
}

func (cc UserPreferencesContent) References() ItemReferences {
	return cc.ItemReferences
}

func (cc *UserPreferencesContent) UpsertReferences(input ItemReferences) {
	panic("implement me")
}

func (cc *UserPreferencesContent) SetReferences(input ItemReferences) {
	panic("implement me")
}
