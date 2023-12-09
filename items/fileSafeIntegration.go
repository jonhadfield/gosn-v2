package items

import (
	"fmt"
	"github.com/jonhadfield/gosn-v2/common"
	"slices"
	"time"
)

func parseFileSafeIntegration(i DecryptedItem) Item {
	c := FileSafeIntegration{}
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

		c.Content = *content.(*FileSafeIntegrationContent)
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

type FileSafeIntegrationContent struct {
	Source                string         `json:"source"`
	Authorization         string         `json:"authorization"`
	RelayURL              string         `json:"relayUrl"`
	RawCode               string         `json:"rawCode"`
	IsDefaultUploadSource bool           `json:"isDefaultUploadSource"`
	ItemReferences        ItemReferences `json:"references"`
	AppData               AppDataContent `json:"appData"`
	Name                  string         `json:"name"`
	DissociatedItemIds    []string       `json:"disassociatedItemIds"`
	AssociatedItemIds     []string       `json:"associatedItemIds"`
	Active                interface{}    `json:"active"`
}

type FileSafeIntegration struct {
	ItemCommon
	Content FileSafeIntegrationContent
}

func (c FileSafeIntegration) IsDefault() bool {
	return false
}

func (i Items) FileSafeIntegration() (c FileSafeIntegrations) {
	for _, x := range i {
		if slices.Contains([]string{"FileSafeIntegration", "SN|FileSafe|Integration"}, x.GetContentType()) {
			component := x.(*FileSafeIntegration)
			c = append(c, *component)
		}
	}

	return c
}

func (c *FileSafeIntegrations) DeDupe() {
	var encountered []string

	var deDuped FileSafeIntegrations

	for _, i := range *c {
		if !slices.Contains(encountered, i.UUID) {
			deDuped = append(deDuped, i)
		}

		encountered = append(encountered, i.UUID)
	}

	*c = deDuped
}

// NewFileSafeIntegration returns an Item of type FileSafeIntegration without content.
func NewFileSafeIntegration() FileSafeIntegration {
	now := time.Now().UTC().Format(common.TimeLayout)

	var c FileSafeIntegration

	c.ContentType = "FileSafeIntegration"
	c.CreatedAt = now
	c.UUID = GenUUID()

	return c
}

// NewTagContent returns an empty Tag content instance.
func NewFileSafeIntegrationContent() *FileSafeIntegrationContent {
	c := &FileSafeIntegrationContent{}
	c.SetUpdateTime(time.Now().UTC())

	return c
}

type FileSafeIntegrations []FileSafeIntegration

func (c FileSafeIntegrations) Validate() error {
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

func (c FileSafeIntegration) IsDeleted() bool {
	return c.Deleted
}

func (c *FileSafeIntegration) SetDeleted(d bool) {
	c.Deleted = d
}

func (c FileSafeIntegration) GetContent() Content {
	return &c.Content
}

func (c *FileSafeIntegration) SetContent(cc Content) {
	c.Content = *cc.(*FileSafeIntegrationContent)
}

func (c FileSafeIntegration) GetItemsKeyID() string {
	return c.ItemsKeyID
}

func (c FileSafeIntegration) GetUUID() string {
	return c.UUID
}

func (c *FileSafeIntegration) SetUUID(u string) {
	c.UUID = u
}

func (c FileSafeIntegration) GetContentType() string {
	return c.ContentType
}

func (c FileSafeIntegration) GetCreatedAt() string {
	return c.CreatedAt
}

func (c *FileSafeIntegration) SetCreatedAt(ca string) {
	c.CreatedAt = ca
}

func (c FileSafeIntegration) GetUpdatedAt() string {
	return c.UpdatedAt
}

func (c *FileSafeIntegration) SetUpdatedAt(ca string) {
	c.UpdatedAt = ca
}

func (c FileSafeIntegration) GetCreatedAtTimestamp() int64 {
	return c.CreatedAtTimestamp
}

func (c *FileSafeIntegration) SetCreatedAtTimestamp(ca int64) {
	c.CreatedAtTimestamp = ca
}

func (c FileSafeIntegration) GetUpdatedAtTimestamp() int64 {
	return c.UpdatedAtTimestamp
}

func (c *FileSafeIntegration) SetUpdatedAtTimestamp(ca int64) {
	c.UpdatedAtTimestamp = ca
}

func (c *FileSafeIntegration) SetContentType(ct string) {
	c.ContentType = ct
}

func (c FileSafeIntegration) GetContentSize() int {
	return c.ContentSize
}

func (c *FileSafeIntegration) SetContentSize(s int) {
	c.ContentSize = s
}

func (cc *FileSafeIntegrationContent) AssociateItems(newItems []string) {
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

func (cc *FileSafeIntegrationContent) GetItemAssociations() []string {
	return cc.AssociatedItemIds
}

func (cc *FileSafeIntegrationContent) GetItemDisassociations() []string {
	return cc.DissociatedItemIds
}

func (cc *FileSafeIntegrationContent) DisassociateItems(itemsToRemove []string) {
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

func (cc *FileSafeIntegrationContent) GetUpdateTime() (time.Time, error) {
	if cc.AppData.OrgStandardNotesSN.ClientUpdatedAt == "" {
		return time.Time{}, fmt.Errorf("ClientUpdatedAt not set")
	}

	return time.Parse(common.TimeLayout, cc.AppData.OrgStandardNotesSN.ClientUpdatedAt)
}

func (cc *FileSafeIntegrationContent) SetUpdateTime(uTime time.Time) {
	cc.AppData.OrgStandardNotesSN.ClientUpdatedAt = uTime.Format(common.TimeLayout)
}

func (cc FileSafeIntegrationContent) GetTitle() string {
	return ""
}

func (cc *FileSafeIntegrationContent) GetName() string {
	return cc.Name
}

func (cc *FileSafeIntegrationContent) GetActive() bool {
	return cc.Active.(bool)
}

func (cc *FileSafeIntegrationContent) SetTitle(title string) {
}

func (cc *FileSafeIntegrationContent) GetAppData() AppDataContent {
	return cc.AppData
}

func (cc *FileSafeIntegrationContent) SetAppData(data AppDataContent) {
	cc.AppData = data
}

func (cc FileSafeIntegrationContent) References() ItemReferences {
	return cc.ItemReferences
}

func (cc *FileSafeIntegrationContent) UpsertReferences(input ItemReferences) {
	panic("implement me")
}

func (cc *FileSafeIntegrationContent) SetReferences(input ItemReferences) {
	cc.ItemReferences = input
}
