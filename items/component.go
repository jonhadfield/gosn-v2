package items

import (
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"time"

	"github.com/jonhadfield/gosn-v2/common"
)

// FlexibleBool is a custom type that can unmarshal both boolean and string values
type FlexibleBool bool

// UnmarshalJSON implements custom JSON unmarshaling for FlexibleBool
func (fb *FlexibleBool) UnmarshalJSON(data []byte) error {
	// Try to unmarshal as boolean first
	var b bool
	if err := json.Unmarshal(data, &b); err == nil {
		*fb = FlexibleBool(b)
		return nil
	}

	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}

	parsedBool, err := strconv.ParseBool(s)
	if err != nil {
		parsedBool = false
	}

	*fb = FlexibleBool(parsedBool)
	return nil
}

// MarshalJSON implements custom JSON marshaling for FlexibleBool
func (fb FlexibleBool) MarshalJSON() ([]byte, error) {
	return json.Marshal(bool(fb))
}

// Bool returns the boolean value
func (fb FlexibleBool) Bool() bool {
	return bool(fb)
}

func parseComponent(i DecryptedItem) Item {
	c := Component{}

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

		c.Content = *content.(*ComponentContent)
	}

	return &c
}

type ComponentContent struct {
	Identifier         string         `json:"identifier"`
	LegacyURL          string         `json:"legacy_url,omitempty"`
	HostedURL          string         `json:"hosted_url,omitempty"`
	LocalURL           string         `json:"local_url,omitempty"`
	URL                string         `json:"url,omitempty"`
	ValidUntil         string         `json:"valid_until,omitempty"`
	OfflineOnly        FlexibleBool   `json:"offlineOnly,omitempty"`
	Name               string         `json:"name"`
	Area               string         `json:"area"`
	PackageInfo        interface{}    `json:"package_info,omitempty"`
	Permissions        []interface{}  `json:"permissions,omitempty"` // Should be array
	Active             interface{}    `json:"active,omitempty"`
	AutoUpdateDisabled FlexibleBool   `json:"autoupdateDisabled,omitempty"` // Handles both bool and string values
	ComponentData      interface{}    `json:"componentData,omitempty"`      // Legacy component data
	DissociatedItemIds []string       `json:"disassociatedItemIds,omitempty"`
	AssociatedItemIds  []string       `json:"associatedItemIds,omitempty"`
	ItemReferences     ItemReferences `json:"references"`
	AppData            AppDataContent `json:"appData"`
	// Missing attributes from official Standard Notes
	IsDeprecated       bool           `json:"isDeprecated,omitempty"` // Component deprecation flag
}

type Component struct {
	ItemCommon
	Content ComponentContent
}

func (c Component) IsDefault() bool {
	return false
}

func (i Items) Components() (c Components) {
	for _, x := range i {
		if x.GetContentType() == common.SNItemTypeComponent {
			component := x.(*Component)
			c = append(c, *component)
		}
	}

	return c
}

func (c *Components) DeDupe() {
	var encountered []string

	var deDuped Components

	for _, i := range *c {
		if !slices.Contains(encountered, i.UUID) {
			deDuped = append(deDuped, i)
		}

		encountered = append(encountered, i.UUID)
	}

	*c = deDuped
}

// NewComponent returns an Item of type Component without content.
func NewComponent() Component {
	now := time.Now().UTC().Format(common.TimeLayout)

	var c Component

	c.ContentType = common.SNItemTypeComponent
	c.CreatedAtTimestamp = time.Now().UTC().UnixMicro()
	c.CreatedAt = now
	c.UUID = GenUUID()

	return c
}

// NewComponentContent returns an empty Tag content instance.
func NewComponentContent() *ComponentContent {
	c := &ComponentContent{}
	c.SetUpdateTime(time.Now().UTC())

	return c
}

type Components []Component

func (c Components) Validate() error {
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
	c.Content = *cc.(*ComponentContent)
}

func (c Component) GetItemsKeyID() string {
	return c.ItemsKeyID
}

func (c Component) GetUUID() string {
	return c.UUID
}

func (c Component) GetDuplicateOf() string {
	return c.DuplicateOf
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

func (c Component) GetCreatedAtTimestamp() int64 {
	return c.CreatedAtTimestamp
}

func (c *Component) SetCreatedAtTimestamp(ca int64) {
	c.CreatedAtTimestamp = ca
}

func (c Component) GetUpdatedAtTimestamp() int64 {
	return c.UpdatedAtTimestamp
}

func (c *Component) SetUpdatedAtTimestamp(ca int64) {
	c.UpdatedAtTimestamp = ca
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

func (cc *ComponentContent) DisassociateItems(itemsToRemove []string) {
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

func (cc *ComponentContent) GetUpdateTime() (time.Time, error) {
	if cc.AppData.OrgStandardNotesSN.ClientUpdatedAt == "" {
		return time.Time{}, fmt.Errorf("ClientUpdatedAt not set")
	}

	return time.Parse(common.TimeLayout, cc.AppData.OrgStandardNotesSN.ClientUpdatedAt)
}

func (cc *ComponentContent) SetUpdateTime(uTime time.Time) {
	cc.AppData.OrgStandardNotesSN.ClientUpdatedAt = uTime.Format(common.TimeLayout)
}

func (cc ComponentContent) GetTitle() string {
	return ""
}

func (cc *ComponentContent) GetName() string {
	return cc.Name
}

func (cc *ComponentContent) GetActive() bool {
	return cc.Active.(bool)
}

func (cc *ComponentContent) SetTitle(title string) {
}

func (cc *ComponentContent) GetAppData() AppDataContent {
	return cc.AppData
}

func (cc *ComponentContent) SetAppData(data AppDataContent) {
	cc.AppData = data
}

func (cc ComponentContent) References() ItemReferences {
	return cc.ItemReferences
}

func (cc *ComponentContent) UpsertReferences(input ItemReferences) {
	panic("implement me")
}

func (cc *ComponentContent) SetReferences(input ItemReferences) {
	cc.ItemReferences = input
}
