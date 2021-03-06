package gosn

// Item defines all types of SN item, e.g. Note, Tag, and Component
type Item interface {
	GetItemsKeyID() string
	GetUUID() string
	SetUUID(string)
	GetContentSize() int
	SetContentSize(int)
	GetContentType() string
	SetContentType(string)
	IsDeleted() bool
	SetDeleted(bool)
	GetCreatedAt() string
	SetCreatedAt(string)
	SetUpdatedAt(string)
	GetUpdatedAt() string
	GetContent() Content
	SetContent(Content)
}

type Content interface {
	References() ItemReferences
}

// ItemCommon contains the fields common to all SN Items
type ItemCommon struct {
	UUID             string
	ItemsKeyID       string
	EncryptedItemKey string
	ContentType      string
	Deleted          bool
	CreatedAt        string
	UpdatedAt        string
	ContentSize      int
}

func (i Items) Validate() error {
	var err error

	for _, item := range i {
		switch item.(type) {
		case *Tag:
			t := item.(*Tag)
			err = Tags{*t}.Validate()
		case *Note:
			n := item.(*Note)
			err = Notes{*n}.Validate()
		case *Component:
			c := item.(*Component)
			err = Components{*c}.Validate()
		}

		if err != nil {
			return err
		}
	}

	return nil
}
