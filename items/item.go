package items

import (
	"time"

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
	GetDuplicateOf() string
}

type Content interface {
	References() ItemReferences
	SetReferences(ItemReferences)
}

// ItemCommon contains the fields common to all SN Items.
// Fields are ordered to minimize memory padding: strings, pointers, integers, then bools.
type ItemCommon struct {
	// String fields (16 bytes each on 64-bit)
	UUID             string
	ItemsKeyID       string
	EncryptedItemKey string
	ContentType      string
	DuplicateOf      string
	CreatedAt        string
	UpdatedAt        string

	// Pointer fields (8 bytes each on 64-bit)
	AuthHash            *string
	UpdatedWithSession  *string
	KeySystemIdentifier *string
	SharedVaultUUID     *string
	UserUUID            *string
	LastEditedByUUID    *string
	ConflictOf          *string `json:"conflict_of,omitempty"`

	// Integer fields (8 bytes each)
	CreatedAtTimestamp int64
	UpdatedAtTimestamp int64
	ContentSize        int

	// Boolean fields packed together (1 byte each, minimal padding)
	Deleted   bool
	Protected bool `json:"protected,omitempty"`
	Trashed   bool `json:"trashed,omitempty"`
	Pinned    bool `json:"pinned,omitempty"`
	Archived  bool `json:"archived,omitempty"`
	Starred   bool `json:"starred,omitempty"`
	Locked    bool `json:"locked,omitempty"`
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

// populateItemCommon fills the common fields of an item from a DecryptedItem.
// It also normalises the CreatedAt and UpdatedAt fields using the SN time layout.
func populateItemCommon(c *ItemCommon, di DecryptedItem) error {
	c.UUID = di.UUID
	c.ItemsKeyID = di.ItemsKeyID
	c.ContentType = di.ContentType
	c.Deleted = di.Deleted
	c.DuplicateOf = di.DuplicateOf
	c.ContentSize = len(di.Content)
	c.CreatedAtTimestamp = di.CreatedAtTimestamp
	c.UpdatedAtTimestamp = di.UpdatedAtTimestamp
	c.AuthHash = di.AuthHash
	c.UpdatedWithSession = di.UpdatedWithSession
	c.KeySystemIdentifier = di.KeySystemIdentifier
	c.SharedVaultUUID = di.SharedVaultUUID
	c.UserUUID = di.UserUUID
	c.LastEditedByUUID = di.LastEditedByUUID

	var err error

	var cAt, uAt time.Time

	cAt, err = parseSNTime(di.CreatedAt)
	if err != nil {
		return err
	}

	c.CreatedAt = cAt.Format(common.TimeLayout)

	uAt, err = parseSNTime(di.UpdatedAt)
	if err != nil {
		return err
	}

	c.UpdatedAt = uAt.Format(common.TimeLayout)

	return nil
}
