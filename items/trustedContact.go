package items

import (
	"fmt"
	"slices"
	"time"

	"github.com/jonhadfield/gosn-v2/common"
)

func parseTrustedContact(i DecryptedItem) Item {
	c := TrustedContact{}

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

		c.Content = *content.(*TrustedContactContent)
	}

	return &c
}

// TrustedContactContent represents a trusted contact for collaboration
type TrustedContactContent struct {
	Name         string         `json:"name"`
	ContactUUID  string         `json:"contactUuid"`
	PublicKeySet interface{}    `json:"publicKeySet"` // ContactPublicKeySetJsonInterface
	IsMe         bool           `json:"isMe"`
	ItemReferences ItemReferences `json:"references"`
	AppData      AppDataContent `json:"appData"`
}

type TrustedContact struct {
	ItemCommon
	Content TrustedContactContent
}

func (c TrustedContact) IsDefault() bool {
	return false
}

func (i Items) TrustedContacts() (c TrustedContacts) {
	for _, x := range i {
		if x.GetContentType() == common.SNItemTypeTrustedContact {
			trustedContact := x.(*TrustedContact)
			c = append(c, *trustedContact)
		}
	}

	return c
}

func (c *TrustedContacts) DeDupe() {
	var encountered []string

	var deDuped TrustedContacts

	for _, i := range *c {
		if !slices.Contains(encountered, i.UUID) {
			deDuped = append(deDuped, i)
		}

		encountered = append(encountered, i.UUID)
	}

	*c = deDuped
}

// NewTrustedContact returns an Item of type TrustedContact without content.
func NewTrustedContact() TrustedContact {
	now := time.Now().UTC().Format(common.TimeLayout)

	var c TrustedContact

	c.ContentType = common.SNItemTypeTrustedContact
	c.CreatedAt = now
	c.CreatedAtTimestamp = time.Now().UTC().UnixMicro()
	c.UUID = GenUUID()

	return c
}

// NewTrustedContactContent returns an empty TrustedContact content instance.
func NewTrustedContactContent() *TrustedContactContent {
	c := &TrustedContactContent{}
	c.SetUpdateTime(time.Now().UTC())

	return c
}

type TrustedContacts []TrustedContact

func (c TrustedContacts) Validate() error {
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
			case item.Content.ContactUUID == "":
				err = fmt.Errorf("failed to create \"%s\" due to missing contactUuid: \"%s\"",
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

func (c TrustedContact) IsDeleted() bool {
	return c.Deleted
}

func (c *TrustedContact) SetDeleted(d bool) {
	c.Deleted = d
}

func (c TrustedContact) GetContent() Content {
	return &c.Content
}

func (c *TrustedContact) SetContent(cc Content) {
	c.Content = *cc.(*TrustedContactContent)
}

func (c TrustedContact) GetItemsKeyID() string {
	return c.ItemsKeyID
}

func (c TrustedContact) GetUUID() string {
	return c.UUID
}

func (c TrustedContact) GetDuplicateOf() string {
	return c.DuplicateOf
}

func (c *TrustedContact) SetUUID(u string) {
	c.UUID = u
}

func (c TrustedContact) GetContentType() string {
	return c.ContentType
}

func (c TrustedContact) GetCreatedAt() string {
	return c.CreatedAt
}

func (c *TrustedContact) SetCreatedAt(ca string) {
	c.CreatedAt = ca
}

func (c TrustedContact) GetUpdatedAt() string {
	return c.UpdatedAt
}

func (c *TrustedContact) SetUpdatedAt(ca string) {
	c.UpdatedAt = ca
}

func (c TrustedContact) GetCreatedAtTimestamp() int64 {
	return c.CreatedAtTimestamp
}

func (c *TrustedContact) SetCreatedAtTimestamp(ca int64) {
	c.CreatedAtTimestamp = ca
}

func (c TrustedContact) GetUpdatedAtTimestamp() int64 {
	return c.UpdatedAtTimestamp
}

func (c *TrustedContact) SetUpdatedAtTimestamp(ca int64) {
	c.UpdatedAtTimestamp = ca
}

func (c *TrustedContact) SetContentType(ct string) {
	c.ContentType = ct
}

func (c TrustedContact) GetContentSize() int {
	return c.ContentSize
}

func (c *TrustedContact) SetContentSize(s int) {
	c.ContentSize = s
}

func (cc *TrustedContactContent) GetUpdateTime() (time.Time, error) {
	if cc.AppData.OrgStandardNotesSN.ClientUpdatedAt == "" {
		return time.Time{}, fmt.Errorf("ClientUpdatedAt not set")
	}

	return time.Parse(common.TimeLayout, cc.AppData.OrgStandardNotesSN.ClientUpdatedAt)
}

func (cc *TrustedContactContent) SetUpdateTime(uTime time.Time) {
	cc.AppData.OrgStandardNotesSN.ClientUpdatedAt = uTime.Format(common.TimeLayout)
}

func (cc TrustedContactContent) GetTitle() string {
	return cc.Name
}

func (cc *TrustedContactContent) SetTitle(title string) {
	cc.Name = title
}

func (cc *TrustedContactContent) GetAppData() AppDataContent {
	return cc.AppData
}

func (cc *TrustedContactContent) SetAppData(data AppDataContent) {
	cc.AppData = data
}

func (cc TrustedContactContent) References() ItemReferences {
	return cc.ItemReferences
}

func (cc *TrustedContactContent) SetReferences(input ItemReferences) {
	cc.ItemReferences = input
}

// GetContactName returns the contact's display name
func (cc TrustedContactContent) GetContactName() string {
	return cc.Name
}

// SetContactName sets the contact's display name
func (cc *TrustedContactContent) SetContactName(name string) {
	cc.Name = name
}

// GetContactUUID returns the unique contact identifier
func (cc TrustedContactContent) GetContactUUID() string {
	return cc.ContactUUID
}

// SetContactUUID sets the unique contact identifier
func (cc *TrustedContactContent) SetContactUUID(uuid string) {
	cc.ContactUUID = uuid
}

// GetPublicKeySet returns the contact's public key set
func (cc TrustedContactContent) GetPublicKeySet() interface{} {
	return cc.PublicKeySet
}

// SetPublicKeySet sets the contact's public key set
func (cc *TrustedContactContent) SetPublicKeySet(keySet interface{}) {
	cc.PublicKeySet = keySet
}

// GetIsMe returns whether this contact represents the current user
func (cc TrustedContactContent) GetIsMe() bool {
	return cc.IsMe
}

// SetIsMe sets whether this contact represents the current user
func (cc *TrustedContactContent) SetIsMe(isMe bool) {
	cc.IsMe = isMe
}