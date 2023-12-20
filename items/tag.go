package items

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/jonhadfield/gosn-v2/common"
	"github.com/jonhadfield/gosn-v2/log"
	"github.com/jonhadfield/gosn-v2/session"
)

type Tag struct {
	ItemCommon
	Content TagContent
}

func (t Tag) IsDefault() bool {
	return false
}

func parseTag(i DecryptedItem) Item {
	t := Tag{}
	t.UUID = i.UUID
	t.ItemsKeyID = i.ItemsKeyID
	t.ContentType = i.ContentType
	t.Deleted = i.Deleted
	t.UpdatedAt = i.UpdatedAt
	t.CreatedAt = i.CreatedAt
	t.UpdatedAtTimestamp = i.UpdatedAtTimestamp
	t.CreatedAtTimestamp = i.CreatedAtTimestamp
	t.ContentSize = len(i.Content)

	var err error

	if !t.Deleted {
		var content Content

		content, err = processContentModel(i.ContentType, i.Content)
		if err != nil {
			log.Println("failed to decrypt item", t.GetContentType(), t.GetUUID())
		}

		t.Content = *content.(*TagContent)
	}

	var cAt, uAt time.Time

	cAt, err = parseSNTime(i.CreatedAt)
	if err != nil {
		panic(err)
	}

	t.CreatedAt = cAt.Format(common.TimeLayout)

	uAt, err = parseSNTime(i.UpdatedAt)
	if err != nil {
		panic(err)
	}

	t.UpdatedAt = uAt.Format(common.TimeLayout)

	return &t
}

func (i Items) Tags() (t Tags) {
	for _, x := range i {
		if x.GetContentType() == common.SNItemTypeTag {
			tag := x.(*Tag)
			t = append(t, *tag)
		}
	}

	return t
}

func (t *Tags) DeDupe() {
	var encountered []string

	var deDuped Tags

	for _, i := range *t {
		if !slices.Contains(encountered, i.UUID) {
			deDuped = append(deDuped, i)
		}

		encountered = append(encountered, i.UUID)
	}

	*t = deDuped
}

func (t *Tags) Encrypt(s session.Session) (e EncryptedItems, err error) {
	var ite Items

	ta := *t
	for x := range ta {
		g := ta[x]
		ite = append(ite, &g)
	}

	e, err = encryptItems(&s, &ite, s.DefaultItemsKey)

	return
}

// NewTag returns an Item of type Tag without content.
func NewTag(title string, refs ItemReferences) (tag Tag, err error) {
	now := time.Now().UTC().Format(common.TimeLayout)

	if strings.TrimSpace(title) == "" {
		return tag, fmt.Errorf("title cannot be empty")
	}

	c := NewTagContent()
	c.Title = title
	c.ItemReferences = refs
	tag.Content = *c
	tag.ContentType = common.SNItemTypeTag
	tag.CreatedAt = now
	tag.CreatedAtTimestamp = time.Now().UTC().UnixMicro()
	tag.UUID = GenUUID()

	return tag, err
}

func (tagContent TagContent) MarshalJSON() ([]byte, error) {
	type Alias TagContent

	a := struct {
		Alias
	}{
		Alias: (Alias)(tagContent),
	}

	if a.ItemReferences == nil {
		a.ItemReferences = ItemReferences{}
	}

	return json.Marshal(a)
}

// NewTagContent returns an empty Tag content instance.
func NewTagContent() *TagContent {
	c := &TagContent{}
	c.SetUpdateTime(time.Now().UTC())

	return c
}

type Tags []Tag

func (t Tag) Equals(e Tag) bool {
	if t.UUID != e.UUID {
		return false
	}

	if t.ContentType != e.ContentType {
		return false
	}

	if t.Deleted != e.Deleted {
		return false
	}

	if t.Content.Title != e.Content.Title {
		return false
	}

	return true
}

func (t Tags) Validate() error {
	var updatedTime time.Time

	var err error

	for _, item := range t {
		// validate content if being added
		if !item.Deleted {
			updatedTime, err = item.Content.GetUpdateTime()
			if err != nil {
				return err
			}

			switch {
			case item.Content.Title == "":
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

func (t Tag) IsDeleted() bool {
	return t.Deleted
}

func (t *Tag) SetDeleted(d bool) {
	t.Deleted = d
}

func (t Tag) GetContent() Content {
	return &t.Content
}

func (t Tag) GetItemsKeyID() string {
	return t.ItemsKeyID
}

func (t Tag) GetUUID() string {
	return t.UUID
}

func (c Tag) GetDuplicateOf() string {
	return c.DuplicateOf
}

func (t *Tag) SetUUID(u string) {
	t.UUID = u
}

func (t Tag) GetContentType() string {
	return t.ContentType
}

func (t Tag) GetCreatedAt() string {
	return t.CreatedAt
}

func (t Tag) GetCreatedAtTimestamp() int64 {
	return t.CreatedAtTimestamp
}

func (t *Tag) SetCreatedAt(ca string) {
	t.CreatedAt = ca
}

func (t *Tag) SetCreatedAtTimestamp(ca int64) {
	t.CreatedAtTimestamp = ca
}

func (t Tag) GetUpdatedAt() string {
	return t.UpdatedAt
}

func (t Tag) GetUpdatedAtTimestamp() int64 {
	return t.UpdatedAtTimestamp
}

func (t Tag) GetContentSize() int {
	return t.ContentSize
}

func (t *Tag) SetContentSize(s int) {
	t.ContentSize = s
}

func (t *Tag) SetUpdatedAt(ca string) {
	t.UpdatedAt = ca
}

func (t *Tag) SetUpdatedAtTimestamp(ca int64) {
	t.UpdatedAtTimestamp = ca
}

func (t *Tag) SetContentType(ct string) {
	t.ContentType = ct
}

func (t *Tag) SetContent(c Content) {
	t.Content = *c.(*TagContent)
}

func (tagContent *TagContent) GetItemAssociations() []string {
	panic("not implemented")
}

func (tagContent *TagContent) GetItemDisassociations() []string {
	panic("not implemented")
}

func (tagContent *TagContent) SetAppData(data AppDataContent) {
	tagContent.AppData = data
}

func (tagContent TagContent) GetText() string {
	// Tags only have titles, so empty string
	return ""
}

func (tagContent *TagContent) SetText(text string) {
	// not implemented
}

func (tagContent *TagContent) GetName() string {
	return "not implemented"
}

func (tagContent *TagContent) GetActive() bool {
	// not implemented
	return false
}

func (tagContent *TagContent) TextContains(findString string, matchCase bool) bool {
	// Tags only have titles, so always false
	return false
}

func (tagContent TagContent) GetTitle() string {
	return tagContent.Title
}

func (tagContent TagContent) References() ItemReferences {
	var output ItemReferences
	return append(output, tagContent.ItemReferences...)
}

func (tagContent *TagContent) GetAppData() AppDataContent {
	return tagContent.AppData
}

func (tagContent *TagContent) SetUpdateTime(uTime time.Time) {
	tagContent.AppData.OrgStandardNotesSN.ClientUpdatedAt = uTime.Format(common.TimeLayout)
}

func (tagContent *TagContent) GetUpdateTime() (time.Time, error) {
	if tagContent.AppData.OrgStandardNotesSN.ClientUpdatedAt == "" {
		return time.Time{}, fmt.Errorf("ClientUpdatedAt not set")
	}

	return time.Parse(common.TimeLayout, tagContent.AppData.OrgStandardNotesSN.ClientUpdatedAt)
}

func (tagContent TagContent) Equals(e TagContent) bool {
	// TODO: compare references
	return tagContent.Title == e.Title
}

func (tagContent TagContent) Copy() TagContent {
	return TagContent{
		Title:          tagContent.Title,
		AppData:        tagContent.AppData,
		ItemReferences: tagContent.ItemReferences,
	}
}

func (t Tag) Copy() Tag {
	c, _ := NewTag("", nil)
	tContent := t.Content
	c.Content = tContent.Copy()
	c.UpdatedAt = t.UpdatedAt
	c.CreatedAt = t.CreatedAt
	c.ContentSize = t.ContentSize
	c.ContentType = t.ContentType
	c.UUID = t.UUID

	return c
}

func (tagContent *TagContent) SetReferences(newRefs ItemReferences) {
	tagContent.ItemReferences = newRefs
}

func (tagContent *TagContent) UpsertReferences(newRefs ItemReferences) {
	tagContent.SetReferences(UpsertReferences(tagContent.ItemReferences, newRefs))
}
