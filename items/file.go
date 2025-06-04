package items

import (
	"fmt"
	"slices"
	"time"

	"github.com/jonhadfield/gosn-v2/common"
)

func parseFile(i DecryptedItem) Item {
	c := File{}

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

		c.Content = *content.(*FileContent)
	}

	return &c
}

type FileContent struct {
	EncryptionHeader   string         `json:"encryptionHeader"`
	Key                string         `json:"key"`
	MimeType           string         `json:"mimeType"`
	Name               string         `json:"name"`
	RemoteIdentifier   string         `json:"remoteIdentifier"`
	ItemReferences     ItemReferences `json:"references"`
	AppData            AppDataContent `json:"appData"`
	DissociatedItemIds []string       `json:"disassociatedItemIds"`
	AssociatedItemIds  []string       `json:"associatedItemIds"`
	Active             interface{}    `json:"active"`
}

type File struct {
	ItemCommon
	Content FileContent
}

func (c File) IsDefault() bool {
	return false
}

func (i Items) File() (c Files) {
	for _, x := range i {
		if slices.Contains([]string{"File", common.SNItemTypeFile}, x.GetContentType()) {
			component := x.(*File)
			c = append(c, *component)
		}
	}

	return c
}

func (c *Files) DeDupe() {
	var encountered []string

	var deDuped Files

	for _, i := range *c {
		if !slices.Contains(encountered, i.UUID) {
			deDuped = append(deDuped, i)
		}

		encountered = append(encountered, i.UUID)
	}

	*c = deDuped
}

// NewFile returns an Item of type File without content.
func NewFile() File {
	now := time.Now().UTC().Format(common.TimeLayout)

	var c File

	c.ContentType = common.SNItemTypeFile
	c.CreatedAt = now
	c.CreatedAtTimestamp = time.Now().UTC().UnixMicro()
	c.UUID = GenUUID()

	return c
}

// NewTagContent returns an empty Tag content instance.
func NewFileContent() *FileContent {
	c := &FileContent{}
	c.SetUpdateTime(time.Now().UTC())

	return c
}

type Files []File

func (c Files) Validate() error {
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

func (c File) IsDeleted() bool {
	return c.Deleted
}

func (c *File) SetDeleted(d bool) {
	c.Deleted = d
}

func (c File) GetContent() Content {
	return &c.Content
}

func (c *File) SetContent(cc Content) {
	c.Content = *cc.(*FileContent)
}

func (c File) GetItemsKeyID() string {
	return c.ItemsKeyID
}

func (c File) GetUUID() string {
	return c.UUID
}

func (c File) GetDuplicateOf() string {
	return c.DuplicateOf
}

func (c *File) SetUUID(u string) {
	c.UUID = u
}

func (c File) GetContentType() string {
	return c.ContentType
}

func (c File) GetCreatedAt() string {
	return c.CreatedAt
}

func (c *File) SetCreatedAt(ca string) {
	c.CreatedAt = ca
}

func (c File) GetUpdatedAt() string {
	return c.UpdatedAt
}

func (c *File) SetUpdatedAt(ca string) {
	c.UpdatedAt = ca
}

func (c File) GetCreatedAtTimestamp() int64 {
	return c.CreatedAtTimestamp
}

func (c *File) SetCreatedAtTimestamp(ca int64) {
	c.CreatedAtTimestamp = ca
}

func (c File) GetUpdatedAtTimestamp() int64 {
	return c.UpdatedAtTimestamp
}

func (c *File) SetUpdatedAtTimestamp(ca int64) {
	c.UpdatedAtTimestamp = ca
}

func (c *File) SetContentType(ct string) {
	c.ContentType = ct
}

func (c File) GetContentSize() int {
	return c.ContentSize
}

func (c *File) SetContentSize(s int) {
	c.ContentSize = s
}

func (cc *FileContent) AssociateItems(newItems []string) {
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

func (cc *FileContent) GetItemAssociations() []string {
	return cc.AssociatedItemIds
}

func (cc *FileContent) GetItemDisassociations() []string {
	return cc.DissociatedItemIds
}

func (cc *FileContent) DisassociateItems(itemsToRemove []string) {
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

func (cc *FileContent) GetUpdateTime() (time.Time, error) {
	if cc.AppData.OrgStandardNotesSN.ClientUpdatedAt == "" {
		return time.Time{}, fmt.Errorf("ClientUpdatedAt not set")
	}

	return time.Parse(common.TimeLayout, cc.AppData.OrgStandardNotesSN.ClientUpdatedAt)
}

func (cc *FileContent) SetUpdateTime(uTime time.Time) {
	cc.AppData.OrgStandardNotesSN.ClientUpdatedAt = uTime.Format(common.TimeLayout)
}

func (cc FileContent) GetTitle() string {
	return ""
}

func (cc *FileContent) GetName() string {
	return cc.Name
}

func (cc *FileContent) GetActive() bool {
	return cc.Active.(bool)
}

func (cc *FileContent) SetTitle(title string) {
}

func (cc *FileContent) GetAppData() AppDataContent {
	return cc.AppData
}

func (cc *FileContent) SetAppData(data AppDataContent) {
	cc.AppData = data
}

func (cc FileContent) References() ItemReferences {
	return cc.ItemReferences
}

func (cc *FileContent) UpsertReferences(input ItemReferences) {
	panic("implement me")
}

func (cc *FileContent) SetReferences(input ItemReferences) {
	cc.ItemReferences = input
}
