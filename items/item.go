package items

import (
	"github.com/jonhadfield/gosn-v2/common"
	"github.com/jonhadfield/gosn-v2/session"
)

// Item defines all types of SN item, e.g. Note, Tag, and Component.
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
	GetCreatedAtTimestamp() int64
	SetCreatedAtTimestamp(int64)
	SetUpdatedAtTimestamp(int64)
	GetUpdatedAtTimestamp() int64
	GetContent() Content
	SetContent(Content)
	IsDefault() bool
}

type Content interface {
	References() ItemReferences
	SetReferences(ItemReferences)
}

// ItemCommon contains the fields common to all SN Items.
type ItemCommon struct {
	UUID               string
	ItemsKeyID         string
	EncryptedItemKey   string
	ContentType        string
	Deleted            bool
	DuplicateOf        string
	CreatedAt          string
	UpdatedAt          string
	CreatedAtTimestamp int64
	UpdatedAtTimestamp int64
	ContentSize        int
}

func GetMatchingItem(uuid string, iks []session.SessionItemsKey) session.SessionItemsKey {
	for x := range iks {
		if uuid == iks[x].UUID {
			return iks[x]
		}
	}

	return session.SessionItemsKey{}
}

func (i Items) Validate(session *session.Session) error {
	var err error

	for _, item := range i {
		switch v := item.(type) {
		case *Tag:
			t := v
			err = Tags{*t}.Validate()
		case *Note:
			n := v
			err = Notes{*n}.Validate(session)
		case *Component:
			c := v
			err = Components{*c}.Validate()
		}

		if err != nil {
			return err
		}
	}

	return nil
}

func IsEncryptedWithMasterKey(t string) bool {
	return t == common.SNItemTypeItemsKey
}
