package gosn

import (
	"fmt"
	"time"
)

type Privileges struct {
	ItemCommon
	Content PrivilegesContent
}

func (i Items) Privileges() (c Privilegess) {
	for _, x := range i {
		if x.GetContentType() == "Privileges" {
			component := x.(*Privileges)
			c = append(c, *component)
		}
	}

	return c
}

func (c *Privilegess) DeDupe() {
	var encountered []string

	var deDuped Privilegess

	for _, i := range *c {
		if !stringInSlice(i.UUID, encountered, true) {
			deDuped = append(deDuped, i)
		}

		encountered = append(encountered, i.UUID)
	}

	*c = deDuped
}

// NewPrivileges returns an Item of type Privileges without content
func NewPrivileges() Privileges {
	now := time.Now().UTC().Format(timeLayout)

	var c Privileges

	c.ContentType = "SN|Privileges"
	c.CreatedAt = now
	c.UpdatedAt = now
	c.UUID = GenUUID()

	return c
}

// NewTagContent returns an empty Tag content instance
func NewPrivilegesContent() *PrivilegesContent {
	c := &PrivilegesContent{}
	c.SetUpdateTime(time.Now().UTC())

	return c
}

type Privilegess []Privileges

func (c Privilegess) Validate() error {
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

func (c Privileges) IsDeleted() bool {
	return c.Deleted
}

func (c *Privileges) SetDeleted(d bool) {
	c.Deleted = d
}

func (c Privileges) GetContent() Content {
	return &c.Content
}

func (c *Privileges) SetContent(cc Content) {
	c.Content = cc.(PrivilegesContent)
}

func (c Privileges) GetUUID() string {
	return c.UUID
}

func (c *Privileges) SetUUID(u string) {
	c.UUID = u
}

func (c Privileges) GetContentType() string {
	return c.ContentType
}

func (c Privileges) GetCreatedAt() string {
	return c.CreatedAt
}

func (c *Privileges) SetCreatedAt(ca string) {
	c.CreatedAt = ca
}

func (c Privileges) GetUpdatedAt() string {
	return c.UpdatedAt
}

func (c *Privileges) SetUpdatedAt(ca string) {
	c.UpdatedAt = ca
}

func (c *Privileges) SetContentType(ct string) {
	c.ContentType = ct
}

func (c Privileges) GetContentSize() int {
	return c.ContentSize
}

func (c *Privileges) SetContentSize(s int) {
	c.ContentSize = s
}

func (cc *PrivilegesContent) AssociateItems(newItems []string) {
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

func (cc *PrivilegesContent) GetItemAssociations() []string {
	return cc.AssociatedItemIds
}

func (cc *PrivilegesContent) GetItemDisassociations() []string {
	return cc.DissociatedItemIds
}

func (cc *PrivilegesContent) DisassociateItems(itemsToRemove []string) {
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

func (cc *PrivilegesContent) GetUpdateTime() (time.Time, error) {
	if cc.AppData.OrgStandardNotesSN.ClientUpdatedAt == "" {
		return time.Time{}, fmt.Errorf("notset")
	}

	return time.Parse(timeLayout, cc.AppData.OrgStandardNotesSN.ClientUpdatedAt)
}

func (cc *PrivilegesContent) SetUpdateTime(uTime time.Time) {
	cc.AppData.OrgStandardNotesSN.ClientUpdatedAt = uTime.Format(timeLayout)
}

func (cc PrivilegesContent) GetTitle() string {
	return ""
}

func (cc *PrivilegesContent) GetName() string {
	return cc.Name
}

func (cc *PrivilegesContent) GetActive() bool {
	return cc.Active.(bool)
}

func (cc *PrivilegesContent) SetTitle(title string) {
}

func (cc *PrivilegesContent) GetAppData() AppDataContent {
	return cc.AppData
}

func (cc *PrivilegesContent) SetAppData(data AppDataContent) {
	cc.AppData = data
}

func (cc PrivilegesContent) References() ItemReferences {
	return cc.ItemReferences
}

func (cc *PrivilegesContent) UpsertReferences(input ItemReferences) {
	panic("implement me")
}

func (cc *PrivilegesContent) SetReferences(input ItemReferences) {
	panic("implement me")
}
