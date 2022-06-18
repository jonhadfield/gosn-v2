package gosn

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type Note struct {
	ItemCommon
	Content NoteContent
}

func (n Note) IsDefault() bool {
	return false
}

var _ Item = &Note{}

func parseNote(i DecryptedItem) Item {
	n := Note{}
	n.UUID = i.UUID
	n.ItemsKeyID = i.ItemsKeyID
	n.ContentType = i.ContentType
	n.Deleted = i.Deleted
	n.UpdatedAt = i.UpdatedAt
	n.CreatedAt = i.CreatedAt
	n.UpdatedAtTimestamp = i.UpdatedAtTimestamp
	n.CreatedAtTimestamp = i.CreatedAtTimestamp
	n.ContentSize = len(i.Content)

	var err error

	if !n.Deleted {
		var content Content

		content, err = processContentModel(i.ContentType, i.Content)
		if err != nil {
			panic(err)
		}

		n.Content = *content.(*NoteContent)
	}

	var cAt, uAt time.Time

	cAt, err = parseSNTime(i.CreatedAt)
	if err != nil {
		panic(err)
	}

	n.CreatedAt = cAt.Format(timeLayout)

	uAt, err = parseSNTime(i.UpdatedAt)
	if err != nil {
		panic(err)
	}

	n.UpdatedAt = uAt.Format(timeLayout)

	return &n
}

func (i Items) Notes() (n Notes) {
	for _, x := range i {
		if x.GetContentType() == "Note" {
			note := x.(*Note)
			n = append(n, *note)
		}
	}

	return n
}

func (n *Notes) DeDupe() {
	var encountered []string

	var deDuped Notes

	for _, i := range *n {
		if !stringInSlice(i.UUID, encountered, true) {
			deDuped = append(deDuped, i)
		}

		encountered = append(encountered, i.UUID)
	}

	*n = deDuped
}

func (n *Notes) Encrypt(s Session) (e EncryptedItems, err error) {
	var ite Items

	na := *n
	for x := range na {
		g := na[x]
		ite = append(ite, &g)
	}

	e, err = encryptItems(&s, &ite, s.DefaultItemsKey)

	return
}

// NewNote returns an Item of type Note.
func NewNote(title string, text string, references ItemReferences) (note Note, err error) {
	now := time.Now().UTC().Format(timeLayout)

	note.UUID = GenUUID()
	note.ContentType = "Note"

	if strings.TrimSpace(title) == "" {
		return note, fmt.Errorf("title cannot be empty")
	}

	note.Content = *NewNoteContent()
	note.Content.SetUpdateTime(time.Now().UTC())
	note.Content.Title = title
	note.Content.Text = text
	note.Content.ItemReferences = references
	note.CreatedAt = now
	note.CreatedAtTimestamp = time.Now().UTC().UnixMicro()
	return note, err
}

// NewNoteContent returns an empty Note content instance.
func NewNoteContent() *NoteContent {
	c := &NoteContent{}
	c.SetUpdateTime(time.Now().UTC())

	return c
}

type Notes []Note

func (n Notes) Validate() error {
	var updatedTime time.Time

	var err error

	for _, item := range n {
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

func (n Note) GetItemsKeyID() string {
	return n.ItemsKeyID
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
	n.Content = *c.(*NoteContent)
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

func (n Note) GetCreatedAtTimestamp() int64 {
	return n.CreatedAtTimestamp
}

func (n *Note) SetCreatedAtTimestamp(ca int64) {
	n.CreatedAtTimestamp = ca
}

func (n Note) GetUpdatedAtTimestamp() int64 {
	return n.UpdatedAtTimestamp
}

func (n *Note) SetUpdatedAtTimestamp(ca int64) {
	n.UpdatedAtTimestamp = ca
}

func (n Note) GetContentSize() int {
	return n.ContentSize
}

func (n *Note) SetContentSize(s int) {
	n.ContentSize = s
}

func (noteContent NoteContent) MarshalJSON() ([]byte, error) {
	type Alias NoteContent

	a := struct {
		Alias
	}{
		Alias: (Alias)(noteContent),
	}

	if a.ItemReferences == nil {
		a.ItemReferences = ItemReferences{}
	}

	return json.Marshal(a)
}

type NoteContent struct {
	Title          string             `json:"title"`
	Text           string             `json:"text"`
	ItemReferences ItemReferences     `json:"references"`
	AppData        NoteAppDataContent `json:"appData"`
	PreviewPlain   string             `json:"preview_plain"`
	Spellcheck     bool               `json:"spellcheck"`
	PreviewHtml    string             `json:"preview_html"`
	Trashed        *bool              `json:"trashed,omitempty"`
}

func (noteContent *NoteContent) GetUpdateTime() (time.Time, error) {
	if noteContent.AppData.OrgStandardNotesSN.ClientUpdatedAt == "" {
		return time.Time{}, fmt.Errorf("ClientUpdatedAt not set")
	}

	return time.Parse(timeLayout, noteContent.AppData.OrgStandardNotesSN.ClientUpdatedAt)
}

func (noteContent *NoteContent) SetUpdateTime(uTime time.Time) {
	noteContent.AppData.OrgStandardNotesSN.ClientUpdatedAt = uTime.Format(timeLayout)
}

func (noteContent *NoteContent) SetPrefersPlainEditor(p bool) {
	noteContent.AppData.OrgStandardNotesSN.PrefersPlainEditor = p
}

func (noteContent NoteContent) GetPrefersPlainEditor() bool {
	return noteContent.AppData.OrgStandardNotesSN.PrefersPlainEditor
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

func (noteContent *NoteContent) GetAppData() NoteAppDataContent {
	return noteContent.AppData
}

func (noteContent *NoteContent) SetAppData(data NoteAppDataContent) {
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

	if n.Content.Trashed != e.Content.Trashed {
		return false
	}

	return true
}

func (noteContent NoteContent) Copy() NoteContent {
	res := NoteContent{
		Title:          noteContent.Title,
		Text:           noteContent.Text,
		ItemReferences: noteContent.ItemReferences,
		AppData:        noteContent.AppData,
	}

	return res
}

func (n Note) Copy() Note {
	c, _ := NewNote("", "", nil)
	tContent := n.Content
	c.Content = tContent.Copy()
	c.UpdatedAt = n.UpdatedAt
	c.CreatedAt = n.CreatedAt
	c.ContentSize = n.ContentSize
	c.ContentType = n.ContentType
	c.UUID = n.UUID

	return c
}

func (noteContent *NoteContent) GetTrashed() bool {
	return *noteContent.Trashed
}

func (noteContent *NoteContent) SetTrashed(t bool) {
	noteContent.Trashed = &t
}

func (noteContent *NoteContent) UpsertReferences(newRefs ItemReferences) {
	noteContent.SetReferences(UpsertReferences(noteContent.ItemReferences, newRefs))
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
