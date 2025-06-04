package items

import (
	"encoding/json"
	"fmt"
	"slices"
	"time"

	"github.com/jonhadfield/gosn-v2/common"
)

type FileSafeCredentialsContent struct {
	Keys               json.RawMessage `json:"keys"`
	AuthParams         json.RawMessage `json:"authParams"`
	IsDefault          json.RawMessage `json:"isDefault"`
	ItemReferences     ItemReferences  `json:"references"`
	AppData            AppDataContent  `json:"appData"`
	Name               string          `json:"name"`
	DissociatedItemIds []string        `json:"disassociatedItemIds"`
	AssociatedItemIds  []string        `json:"associatedItemIds"`
	Active             interface{}     `json:"active"`
}

func parseFileSafeCredentials(i DecryptedItem) Item {
	c := FileSafeCredentials{}

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

		c.Content = *content.(*FileSafeCredentialsContent)
	}

	return &c
}

type FileSafeCredentials struct {
	ItemCommon
	Content FileSafeCredentialsContent
}

func (c FileSafeCredentials) IsDefault() bool {
	return false
}

func (i Items) FileSafeCredentials() (c FileSafeCredentialsList) {
	for _, x := range i {
		if x.GetContentType() == common.SNItemTypeFileSafeCredentials {
			component := x.(*FileSafeCredentials)
			c = append(c, *component)
		}
	}

	return c
}

func (c *FileSafeCredentialsList) DeDupe() {
	var encountered []string

	var deDuped FileSafeCredentialsList

	for _, i := range *c {
		if !slices.Contains(encountered, i.UUID) {
			deDuped = append(deDuped, i)
		}

		encountered = append(encountered, i.UUID)
	}

	*c = deDuped
}

// NewFileSafeCredentials returns an Item of type FileSafeCredentials without content.
func NewFileSafeCredentials() FileSafeCredentials {
	now := time.Now().UTC().Format(common.TimeLayout)

	var c FileSafeCredentials

	c.ContentType = common.SNItemTypeFileSafeCredentials
	c.CreatedAt = now
	c.CreatedAtTimestamp = time.Now().UTC().UnixMicro()
	c.UUID = GenUUID()

	return c
}

// NewTagContent returns an empty Tag content instance.
func NewFileSafeCredentialsContent() *FileSafeCredentialsContent {
	c := &FileSafeCredentialsContent{}
	c.SetUpdateTime(time.Now().UTC())

	return c
}

type FileSafeCredentialsList []FileSafeCredentials

func (c FileSafeCredentialsList) Validate() error {
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

func (c FileSafeCredentials) IsDeleted() bool {
	return c.Deleted
}

func (c *FileSafeCredentials) SetDeleted(d bool) {
	c.Deleted = d
}

func (c FileSafeCredentials) GetContent() Content {
	return &c.Content
}

func (c *FileSafeCredentials) SetContent(cc Content) {
	c.Content = *cc.(*FileSafeCredentialsContent)
}

func (c FileSafeCredentials) GetItemsKeyID() string {
	return c.ItemsKeyID
}

func (c FileSafeCredentials) GetUUID() string {
	return c.UUID
}

func (c FileSafeCredentials) GetDuplicateOf() string {
	return c.DuplicateOf
}

func (c *FileSafeCredentials) SetUUID(u string) {
	c.UUID = u
}

func (c FileSafeCredentials) GetContentType() string {
	return c.ContentType
}

func (c FileSafeCredentials) GetCreatedAt() string {
	return c.CreatedAt
}

func (c *FileSafeCredentials) SetCreatedAt(ca string) {
	c.CreatedAt = ca
}

func (c FileSafeCredentials) GetUpdatedAt() string {
	return c.UpdatedAt
}

func (c *FileSafeCredentials) SetUpdatedAt(ca string) {
	c.UpdatedAt = ca
}

func (c FileSafeCredentials) GetCreatedAtTimestamp() int64 {
	return c.CreatedAtTimestamp
}

func (c *FileSafeCredentials) SetCreatedAtTimestamp(ca int64) {
	c.CreatedAtTimestamp = ca
}

func (c FileSafeCredentials) GetUpdatedAtTimestamp() int64 {
	return c.UpdatedAtTimestamp
}

func (c *FileSafeCredentials) SetUpdatedAtTimestamp(ca int64) {
	c.UpdatedAtTimestamp = ca
}

func (c *FileSafeCredentials) SetContentType(ct string) {
	c.ContentType = ct
}

func (c FileSafeCredentials) GetContentSize() int {
	return c.ContentSize
}

func (c *FileSafeCredentials) SetContentSize(s int) {
	c.ContentSize = s
}

func (cc *FileSafeCredentialsContent) AssociateItems(newItems []string) {
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

func (cc *FileSafeCredentialsContent) GetItemAssociations() []string {
	return cc.AssociatedItemIds
}

func (cc *FileSafeCredentialsContent) GetItemDisassociations() []string {
	return cc.DissociatedItemIds
}

func (cc *FileSafeCredentialsContent) DisassociateItems(itemsToRemove []string) {
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

func (cc *FileSafeCredentialsContent) GetUpdateTime() (time.Time, error) {
	if cc.AppData.OrgStandardNotesSN.ClientUpdatedAt == "" {
		return time.Time{}, fmt.Errorf("ClientUpdatedAt not set")
	}

	return time.Parse(common.TimeLayout, cc.AppData.OrgStandardNotesSN.ClientUpdatedAt)
}

func (cc *FileSafeCredentialsContent) SetUpdateTime(uTime time.Time) {
	cc.AppData.OrgStandardNotesSN.ClientUpdatedAt = uTime.Format(common.TimeLayout)
}

func (cc FileSafeCredentialsContent) GetTitle() string {
	return ""
}

func (cc *FileSafeCredentialsContent) GetName() string {
	return cc.Name
}

func (cc *FileSafeCredentialsContent) GetActive() bool {
	return cc.Active.(bool)
}

func (cc *FileSafeCredentialsContent) SetTitle(title string) {
}

func (cc *FileSafeCredentialsContent) GetAppData() AppDataContent {
	return cc.AppData
}

func (cc *FileSafeCredentialsContent) SetAppData(data AppDataContent) {
	cc.AppData = data
}

func (cc FileSafeCredentialsContent) References() ItemReferences {
	return cc.ItemReferences
}

func (cc *FileSafeCredentialsContent) UpsertReferences(input ItemReferences) {
	cc.SetReferences(UpsertReferences(cc.ItemReferences, input))
}

func (cc *FileSafeCredentialsContent) SetReferences(input ItemReferences) {
	cc.ItemReferences = input
}
