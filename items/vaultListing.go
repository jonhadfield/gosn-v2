package items

import (
	"fmt"
	"slices"
	"time"

	"github.com/jonhadfield/gosn-v2/common"
)

func parseVaultListing(i DecryptedItem) Item {
	c := VaultListing{}

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

		c.Content = *content.(*VaultListingContent)
	}

	return &c
}

// VaultListingContent represents a shared vault listing
type VaultListingContent struct {
	SystemIdentifier string         `json:"systemIdentifier"`
	RootKeyParams    interface{}    `json:"rootKeyParams"`    // KeySystemRootKeyParamsInterface
	KeyStorageMode   string         `json:"keyStorageMode"`   // KeySystemRootKeyStorageMode
	Name             string         `json:"name"`
	Description      string         `json:"description,omitempty"`
	IconString       string         `json:"iconString"`
	Sharing          interface{}    `json:"sharing,omitempty"` // VaultListingSharingInfo
	ItemReferences   ItemReferences `json:"references"`
	AppData          AppDataContent `json:"appData"`
}

type VaultListing struct {
	ItemCommon
	Content VaultListingContent
}

func (c VaultListing) IsDefault() bool {
	return false
}

func (i Items) VaultListings() (c VaultListings) {
	for _, x := range i {
		if x.GetContentType() == common.SNItemTypeVaultListing {
			vaultListing := x.(*VaultListing)
			c = append(c, *vaultListing)
		}
	}

	return c
}

func (c *VaultListings) DeDupe() {
	var encountered []string

	var deDuped VaultListings

	for _, i := range *c {
		if !slices.Contains(encountered, i.UUID) {
			deDuped = append(deDuped, i)
		}

		encountered = append(encountered, i.UUID)
	}

	*c = deDuped
}

// NewVaultListing returns an Item of type VaultListing without content.
func NewVaultListing() VaultListing {
	now := time.Now().UTC().Format(common.TimeLayout)

	var c VaultListing

	c.ContentType = common.SNItemTypeVaultListing
	c.CreatedAt = now
	c.CreatedAtTimestamp = time.Now().UTC().UnixMicro()
	c.UUID = GenUUID()

	return c
}

// NewVaultListingContent returns an empty VaultListing content instance.
func NewVaultListingContent() *VaultListingContent {
	c := &VaultListingContent{}
	c.SetUpdateTime(time.Now().UTC())

	return c
}

type VaultListings []VaultListing

func (c VaultListings) Validate() error {
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
				err = fmt.Errorf("failed to create \"%s\" due to missing name: \"%s\"",
					item.ContentType, item.UUID)
			case item.Content.SystemIdentifier == "":
				err = fmt.Errorf("failed to create \"%s\" due to missing systemIdentifier: \"%s\"",
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

func (c VaultListing) IsDeleted() bool {
	return c.Deleted
}

func (c *VaultListing) SetDeleted(d bool) {
	c.Deleted = d
}

func (c VaultListing) GetContent() Content {
	return &c.Content
}

func (c *VaultListing) SetContent(cc Content) {
	c.Content = *cc.(*VaultListingContent)
}

func (c VaultListing) GetItemsKeyID() string {
	return c.ItemsKeyID
}

func (c VaultListing) GetUUID() string {
	return c.UUID
}

func (c VaultListing) GetDuplicateOf() string {
	return c.DuplicateOf
}

func (c *VaultListing) SetUUID(u string) {
	c.UUID = u
}

func (c VaultListing) GetContentType() string {
	return c.ContentType
}

func (c VaultListing) GetCreatedAt() string {
	return c.CreatedAt
}

func (c *VaultListing) SetCreatedAt(ca string) {
	c.CreatedAt = ca
}

func (c VaultListing) GetUpdatedAt() string {
	return c.UpdatedAt
}

func (c *VaultListing) SetUpdatedAt(ca string) {
	c.UpdatedAt = ca
}

func (c VaultListing) GetCreatedAtTimestamp() int64 {
	return c.CreatedAtTimestamp
}

func (c *VaultListing) SetCreatedAtTimestamp(ca int64) {
	c.CreatedAtTimestamp = ca
}

func (c VaultListing) GetUpdatedAtTimestamp() int64 {
	return c.UpdatedAtTimestamp
}

func (c *VaultListing) SetUpdatedAtTimestamp(ca int64) {
	c.UpdatedAtTimestamp = ca
}

func (c *VaultListing) SetContentType(ct string) {
	c.ContentType = ct
}

func (c VaultListing) GetContentSize() int {
	return c.ContentSize
}

func (c *VaultListing) SetContentSize(s int) {
	c.ContentSize = s
}

func (cc *VaultListingContent) GetUpdateTime() (time.Time, error) {
	if cc.AppData.OrgStandardNotesSN.ClientUpdatedAt == "" {
		return time.Time{}, fmt.Errorf("ClientUpdatedAt not set")
	}

	return time.Parse(common.TimeLayout, cc.AppData.OrgStandardNotesSN.ClientUpdatedAt)
}

func (cc *VaultListingContent) SetUpdateTime(uTime time.Time) {
	cc.AppData.OrgStandardNotesSN.ClientUpdatedAt = uTime.Format(common.TimeLayout)
}

func (cc VaultListingContent) GetTitle() string {
	return cc.Name
}

func (cc *VaultListingContent) SetTitle(title string) {
	cc.Name = title
}

func (cc *VaultListingContent) GetAppData() AppDataContent {
	return cc.AppData
}

func (cc *VaultListingContent) SetAppData(data AppDataContent) {
	cc.AppData = data
}

func (cc VaultListingContent) References() ItemReferences {
	return cc.ItemReferences
}

func (cc *VaultListingContent) SetReferences(input ItemReferences) {
	cc.ItemReferences = input
}

// GetVaultName returns the vault's display name
func (cc VaultListingContent) GetVaultName() string {
	return cc.Name
}

// SetVaultName sets the vault's display name
func (cc *VaultListingContent) SetVaultName(name string) {
	cc.Name = name
}

// GetDescription returns the vault description
func (cc VaultListingContent) GetDescription() string {
	return cc.Description
}

// SetDescription sets the vault description
func (cc *VaultListingContent) SetDescription(description string) {
	cc.Description = description
}

// GetSystemIdentifier returns the key system identifier
func (cc VaultListingContent) GetSystemIdentifier() string {
	return cc.SystemIdentifier
}

// SetSystemIdentifier sets the key system identifier
func (cc *VaultListingContent) SetSystemIdentifier(identifier string) {
	cc.SystemIdentifier = identifier
}

// GetRootKeyParams returns the root key parameters
func (cc VaultListingContent) GetRootKeyParams() interface{} {
	return cc.RootKeyParams
}

// SetRootKeyParams sets the root key parameters
func (cc *VaultListingContent) SetRootKeyParams(params interface{}) {
	cc.RootKeyParams = params
}

// GetKeyStorageMode returns the key storage mode
func (cc VaultListingContent) GetKeyStorageMode() string {
	return cc.KeyStorageMode
}

// SetKeyStorageMode sets the key storage mode
func (cc *VaultListingContent) SetKeyStorageMode(mode string) {
	cc.KeyStorageMode = mode
}

// GetIconString returns the vault icon
func (cc VaultListingContent) GetIconString() string {
	return cc.IconString
}

// SetIconString sets the vault icon
func (cc *VaultListingContent) SetIconString(icon string) {
	cc.IconString = icon
}

// GetSharing returns the sharing configuration
func (cc VaultListingContent) GetSharing() interface{} {
	return cc.Sharing
}

// SetSharing sets the sharing configuration
func (cc *VaultListingContent) SetSharing(sharing interface{}) {
	cc.Sharing = sharing
}