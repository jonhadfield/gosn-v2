package gosn

import (
	"fmt"
	"time"
)

type Component struct {
	ItemCommon
	Content ComponentContent
}

// NewComponent returns an Item of type Component without content
func NewComponent() Component {
	now := time.Now().UTC().Format(timeLayout)
	var c Component
	c.ContentType = "SN|Component"
	c.CreatedAt = now
	c.UpdatedAt = now
	c.UUID = GenUUID()
	return c
}

// NewTagContent returns an empty Tag content instance
func NewComponentContent() *ComponentContent {
	c := &ComponentContent{}
	c.SetUpdateTime(time.Now().UTC())

	return c
}

type Components []Component

func (i Components) Validate() error {
	var updatedTime time.Time

	var err error
	for _, item := range i {
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

func (c Component) IsDeleted() bool {
	return c.Deleted
}

func (c *Component) SetDeleted(d bool) {
	c.Deleted = d
}

func (c Component) GetContent() Content {
	return &c.Content
}

func (c *Component) SetContent(cc Content) {
	c.Content = cc.(ComponentContent)
}

func (c Component) GetUUID() string {
	return c.UUID
}

func (c *Component) SetUUID(u string) {
	c.UUID = u
}

func (c Component) GetContentType() string {
	return c.ContentType
}

func (c Component) GetCreatedAt() string {
	return c.CreatedAt
}

func (c *Component) SetCreatedAt(ca string) {
	c.CreatedAt = ca
}

func (c Component) GetUpdatedAt() string {
	return c.UpdatedAt
}

func (c *Component) SetUpdatedAt(ca string) {
	c.UpdatedAt = ca
}

func (c *Component) SetContentType(ct string) {
	c.ContentType = ct
}

func (c Component) GetContentSize() int {
	return c.ContentSize
}

func (c *Component) SetContentSize(s int) {
	c.ContentSize = s
}

func (cc *ComponentContent) AssociateItems(newItems []string) {
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

func (cc *ComponentContent) GetItemAssociations() []string {
	return cc.AssociatedItemIds
}

func (cc *ComponentContent) GetItemDisassociations() []string {
	return cc.DissociatedItemIds
}
