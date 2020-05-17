package gosn

import (
	"fmt"
	"time"
)

type Tag struct {
	ItemCommon
	Content TagContent
}


func parseTag(i DecryptedItem) Item {
	t := Tag{}
	t.UUID = i.UUID
	t.ContentType = i.ContentType
	t.Deleted = i.Deleted
	t.UpdatedAt = i.UpdatedAt
	t.CreatedAt = i.CreatedAt
	t.ContentSize = len(i.Content)

	var err error

	if !t.Deleted {
		var content Content
		content, err = processContentModel(i.ContentType, i.Content)

		if err != nil {
			panic(err)
		}

		t.Content = content.(TagContent)
	}

	var cAt, uAt time.Time

	cAt, err = time.Parse(timeLayout, i.CreatedAt)
	if err != nil {
		panic(err)
	}

	t.CreatedAt = cAt.Format(timeLayout)

	uAt, err = time.Parse(timeLayout, i.UpdatedAt)
	if err != nil {
		panic(err)
	}

	t.UpdatedAt = uAt.Format(timeLayout)

	return &t
}

func (i Items) Tags() (t Tags) {
	for _, x := range i {
		if x.GetContentType() == "Tag" {
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
		if !stringInSlice(i.UUID, encountered, true) {
			deDuped = append(deDuped, i)
		}

		encountered = append(encountered, i.UUID)
	}

	*t = deDuped
}

func (t *Tags) Encrypt(mk, ak string, debug bool) (e EncryptedItems, err error) {
	var ite Items
	ta := *t
	for x := range ta {
		g := ta[x]
		ite = append(ite, &g)
	}
	e, err = encryptItems(&ite, mk, ak, debug)
	return
}

// NewTag returns an Item of type Tag without content
func NewTag() Tag {
	now := time.Now().UTC().Format(timeLayout)

	var tag Tag
	tag.ContentType = "Tag"
	tag.CreatedAt = now
	tag.UpdatedAt = now
	tag.UUID = GenUUID()

	return tag
}

// NewTagContent returns an empty Tag content instance
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

func (t Tag) GetUUID() string {
	return t.UUID
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

func (t *Tag) SetCreatedAt(ca string) {
	t.CreatedAt = ca
}

func (t Tag) GetUpdatedAt() string {
	return t.UpdatedAt
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

func (t *Tag) SetContentType(ct string) {
	t.ContentType = ct
}

func (t *Tag) SetContent(c Content) {
	t.Content = c.(TagContent)
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
	tagContent.AppData.OrgStandardNotesSN.ClientUpdatedAt = uTime.Format(timeLayout)
}

func (tagContent *TagContent) GetUpdateTime() (time.Time, error) {
	if tagContent.AppData.OrgStandardNotesSN.ClientUpdatedAt == "" {
		return time.Time{}, fmt.Errorf("notset")
	}

	return time.Parse(timeLayout, tagContent.AppData.OrgStandardNotesSN.ClientUpdatedAt)
}

func (tagContent TagContent) Equals(e TagContent) bool {
	// TODO: compare references
	return tagContent.Title == e.Title
}

func (tagContent TagContent) Copy() TagContent {
	res := *new(TagContent)
	res.Title = tagContent.Title
	res.AppData = tagContent.AppData
	res.ItemReferences = tagContent.ItemReferences

	return res
}

func (t Tag) Copy() Tag {
	c := NewTag()
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
	for _, newRef := range newRefs {
		var found bool

		for _, existingRef := range tagContent.ItemReferences {
			if existingRef.UUID == newRef.UUID {
				found = true
			}
		}

		if !found {
			tagContent.ItemReferences = append(tagContent.ItemReferences, newRef)
		}
	}
}
