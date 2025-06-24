package items

import (
	"testing"

	"slices"

	"github.com/stretchr/testify/require"
)

type stubItem struct{ uuid string }

func (s *stubItem) GetItemsKeyID() string        { return "" }
func (s *stubItem) GetUUID() string              { return s.uuid }
func (s *stubItem) SetUUID(u string)             { s.uuid = u }
func (s *stubItem) GetContentSize() int          { return 0 }
func (s *stubItem) SetContentSize(int)           {}
func (s *stubItem) GetContentType() string       { return "" }
func (s *stubItem) SetContentType(string)        {}
func (s *stubItem) IsDeleted() bool              { return false }
func (s *stubItem) SetDeleted(bool)              {}
func (s *stubItem) GetCreatedAt() string         { return "" }
func (s *stubItem) SetCreatedAt(string)          {}
func (s *stubItem) SetUpdatedAt(string)          {}
func (s *stubItem) GetUpdatedAt() string         { return "" }
func (s *stubItem) GetCreatedAtTimestamp() int64 { return 0 }
func (s *stubItem) SetCreatedAtTimestamp(int64)  {}
func (s *stubItem) SetUpdatedAtTimestamp(int64)  {}
func (s *stubItem) GetUpdatedAtTimestamp() int64 { return 0 }
func (s *stubItem) GetContent() Content          { return nil }
func (s *stubItem) SetContent(Content)           {}
func (s *stubItem) IsDefault() bool              { return false }
func (s *stubItem) GetDuplicateOf() string       { return "" }

func TestRemoveStringFromSlice(t *testing.T) {
	t.Parallel()
	in := []string{"a", "b", "c", "b"}
	out := removeStringFromSlice("b", in)
	require.Equal(t, []string{"a", "c"}, out)
}

func TestRemoveStringFromSliceMissing(t *testing.T) {
	t.Parallel()
	in := []string{"a", "b"}
	out := removeStringFromSlice("x", in)
	require.Equal(t, in, out)
}

func TestItemsDeDupe(t *testing.T) {
	t.Parallel()
	items := Items{
		&stubItem{uuid: "a"},
		&stubItem{uuid: "b"},
		&stubItem{uuid: "a"},
		&stubItem{uuid: "b"},
	}
	items.DeDupe()
	require.Len(t, items, 2)
	uuids := []string{items[0].GetUUID(), items[1].GetUUID()}
	slices.Sort(uuids)
	require.Equal(t, []string{"a", "b"}, uuids)
}
