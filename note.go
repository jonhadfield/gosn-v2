package gosn

import (
	"fmt"
	"time"
)

type Note struct {
	ItemCommon
	Content NoteContent
}


// NewNote returns an Item of type Note without content
func NewNote() Note {
	now := time.Now().UTC().Format(timeLayout)
	var note Note
	note.ContentType = "Note"
	note.CreatedAt = now
	note.UpdatedAt = now
	note.UUID = GenUUID()

	return note
}

// NewNoteContent returns an empty Note content instance
func NewNoteContent() *NoteContent {
	c := &NoteContent{}
	c.SetUpdateTime(time.Now().UTC())

	return c
}

type Notes []Note

func (i Notes) Validate() error {
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
			case item.Content.GetTitle() == "":
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

func (n Note) IsDeleted() bool {
	return n.Deleted
}

func (n *Note) SetDeleted(d bool) {
	n.Deleted = d
}

func (n Note) GetContent() Content {
	return &n.Content
}

func (n Note) GetUUID() string {
	return n.UUID
}

func (n *Note) SetUUID(u string) {
	n.UUID = u
}

func (n Note) GetContentType() string {
	return n.ContentType
}

func (n *Note) SetContentType(ct string) {
	n.ContentType = ct
}

func (n *Note) SetContent(c Content) {
	n.Content = c.(NoteContent)
}

func (n Note) GetCreatedAt() string {
	return n.CreatedAt
}

func (n *Note) SetCreatedAt(ca string) {
	n.CreatedAt = ca
}

func (n Note) GetUpdatedAt() string {
	return n.UpdatedAt
}

func (n *Note) SetUpdatedAt(ca string) {
	n.UpdatedAt = ca
}

func (n Note) GetContentSize() int {
	return n.ContentSize
}

func (n *Note) SetContentSize(s int) {
	n.ContentSize = s
}

type NoteContent struct {
	Title          string         `json:"title"`
	Text           string         `json:"text"`
	ItemReferences ItemReferences `json:"references"`
	AppData        AppDataContent `json:"appData"`
}

func (noteContent *NoteContent) GetUpdateTime() (time.Time, error) {
	if noteContent.AppData.OrgStandardNotesSN.ClientUpdatedAt == "" {
		return time.Time{}, fmt.Errorf("notset")
	}

	return time.Parse(timeLayout, noteContent.AppData.OrgStandardNotesSN.ClientUpdatedAt)
}


func (noteContent *NoteContent) SetUpdateTime(uTime time.Time) {
	noteContent.AppData.OrgStandardNotesSN.ClientUpdatedAt = uTime.Format(timeLayout)
}


func (noteContent NoteContent) GetTitle() string {
	return noteContent.Title
}

func (noteContent *NoteContent) SetTitle(title string) {
	noteContent.Title = title
}

func (tagContent *TagContent) SetTitle(title string) {
	tagContent.Title = title
}

func (noteContent NoteContent) GetText() string {
	return noteContent.Text
}

func (noteContent *NoteContent) SetText(text string) {
	noteContent.Text = text
}


func (noteContent *NoteContent) GetAppData() AppDataContent {
	return noteContent.AppData
}

func (noteContent *NoteContent) SetAppData(data AppDataContent) {
	noteContent.AppData = data
}

func (noteContent NoteContent) References() ItemReferences {
	return noteContent.ItemReferences
}


func (noteContent *NoteContent) GetActive() bool {
	// not implemented
	return false
}

func (noteContent *NoteContent) GetName() string {
	return "not implemented"
}

func (noteContent *NoteContent) AddItemAssociations() string {
	return "not implemented"
}


func (noteContent *NoteContent) GetItemAssociations() []string {
	panic("not implemented")
}

func (noteContent *NoteContent) GetItemDisassociations() []string {
	panic("not implemented")
}

func (noteContent *NoteContent) AssociateItems(newItems []string) {

}

func (tagContent *TagContent) AssociateItems(newItems []string) {

}

func (noteContent *NoteContent) DisassociateItems(newItems []string) {

}

func (tagContent *TagContent) DisassociateItems(newItems []string) {

}


func (n Note) Equals(e Note) bool {
	if n.UUID != e.UUID {
		return false
	}

	if n.ContentType != e.ContentType {
		return false
	}

	if n.Deleted != e.Deleted {
		return false
	}

	if n.Content.Title != e.Content.Title {
		return false
	}

	if n.Content.Text != e.Content.Text {
		return false
	}

	return true
}

func (noteContent NoteContent) Copy() NoteContent {
	res := *new(NoteContent)
	res.Title = noteContent.Title
	res.Text = noteContent.Text
	res.AppData = noteContent.AppData
	res.ItemReferences = noteContent.ItemReferences

	return res
}

func (n Note) Copy() Note {
	c := NewNote()
	tContent := n.Content
	c.Content = tContent.Copy()
	c.UpdatedAt = n.UpdatedAt
	c.CreatedAt = n.CreatedAt
	c.ContentSize = n.ContentSize
	c.ContentType = n.ContentType
	c.UUID = n.UUID

	return c
}


func (noteContent *NoteContent) UpsertReferences(newRefs ItemReferences) {
	for _, newRef := range newRefs {
		var found bool

		for _, existingRef := range noteContent.ItemReferences {
			if existingRef.UUID == newRef.UUID {
				found = true
			}
		}

		if !found {
			noteContent.ItemReferences = append(noteContent.ItemReferences, newRef)
		}
	}
}

func (noteContent *NoteContent) SetReferences(newRefs ItemReferences) {
	noteContent.ItemReferences = newRefs
}

func (n *Notes) RemoveDeleted() {
	var clean Notes

	for _, j := range *n {
		if !j.IsDeleted() {
			clean = append(clean, j)
		}
	}

	*n = clean
}