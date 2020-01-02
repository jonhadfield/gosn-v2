package gosn

import (
	"fmt"
	"time"
)

type Tag struct {
	ItemCommon
	Content TagContent
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

func (i Tags) Validate() error {
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
