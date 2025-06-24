package items

import (
	"testing"

	"github.com/jonhadfield/gosn-v2/common"
	"github.com/stretchr/testify/require"
)

func TestFilterNoteTitle(t *testing.T) {
	t.Parallel()
	gnuNote := createNote("GNU", "Is not Unix", "")
	filter := Filter{
		Type:       common.SNItemTypeNote,
		Key:        "Title",
		Comparison: "==",
		Value:      "GNU",
	}
	itemFilters := ItemFilters{
		Filters:  []Filter{filter},
		MatchAny: true,
	}
	res := applyNoteFilters(*gnuNote, itemFilters, nil)
	require.True(t, res, "failed to match note by title")
}

func TestFilterNoteUUID(t *testing.T) {
	t.Parallel()
	uuid := GenUUID()
	gnuNote := createNote("GNU", "Is not Unix", uuid)
	filter := Filter{
		Type:       common.SNItemTypeNote,
		Key:        "UUID",
		Comparison: "==",
		Value:      uuid,
	}
	itemFilters := ItemFilters{
		Filters:  []Filter{filter},
		MatchAny: true,
	}
	res := applyNoteFilters(*gnuNote, itemFilters, nil)
	require.True(t, res, "failed to match note by uuid")
}

func TestFilterNoteByTagUUID(t *testing.T) {
	t.Parallel()
	gnuNoteUUID := GenUUID()
	animalTagUUID := GenUUID()
	cheeseNoteUUID := GenUUID()
	foodTagUUID := GenUUID()
	sportNoteUUID := GenUUID()

	animalTag, _ := createTag("Animal", animalTagUUID, nil)
	gnuNote := createNote("GNU", "Is not Unix", gnuNoteUUID)
	sportNote := createNote("Sport", "Is dull", sportNoteUUID)

	foodTag, _ := createTag("Food", foodTagUUID, nil)
	cheeseNote := createNote("Cheese", "Is not a vegetable", cheeseNoteUUID)

	gnuRef := ItemReference{
		UUID:        gnuNoteUUID,
		ContentType: common.SNItemTypeNote,
	}
	animalTag.Content.UpsertReferences(ItemReferences{gnuRef})

	cheeseRef := ItemReference{
		UUID:        cheeseNoteUUID,
		ContentType: common.SNItemTypeNote,
	}
	foodTag.Content.UpsertReferences(ItemReferences{cheeseRef})

	animalTagUUIDFilter := Filter{
		Type:       common.SNItemTypeNote,
		Key:        "TagUUID",
		Comparison: "==",
		Value:      animalTagUUID,
	}

	foodTagUUIDFilter := Filter{
		Type:       common.SNItemTypeNote,
		Key:        "TagUUID",
		Comparison: "==",
		Value:      foodTagUUID,
	}

	animalTagUUIDFilterNegative := Filter{
		Type:       common.SNItemTypeNote,
		Key:        "TagUUID",
		Comparison: "!=",
		Value:      animalTagUUID,
	}

	animalItemFiltersNegativeMatchAny := ItemFilters{
		Filters:  []Filter{animalTagUUIDFilterNegative},
		MatchAny: true,
	}

	animalItemFiltersNegativeMatchAll := ItemFilters{
		Filters:  []Filter{animalTagUUIDFilterNegative},
		MatchAny: false,
	}

	animalItemFilters := ItemFilters{
		Filters:  []Filter{animalTagUUIDFilter},
		MatchAny: true,
	}
	animalAndFoodItemFiltersAnyTrue := ItemFilters{
		Filters:  []Filter{foodTagUUIDFilter, animalTagUUIDFilter},
		MatchAny: true,
	}
	animalAndFoodItemFiltersAnyFalse := ItemFilters{
		Filters:  []Filter{foodTagUUIDFilter, animalTagUUIDFilter},
		MatchAny: false,
	}
	// try match single animal (success)
	res := applyNoteFilters(*gnuNote, animalItemFilters, Tags{*animalTag})
	require.True(t, res, "failed to match any note by tag uuid")

	// try match animal note against food tag (failure)
	res = applyNoteFilters(*gnuNote, animalItemFilters, Tags{*foodTag})
	require.False(t, res, "incorrectly matched note by tag uuid")

	// try against any of multiple filters - match any (success)
	res = applyNoteFilters(*cheeseNote, animalAndFoodItemFiltersAnyTrue, Tags{*animalTag, *foodTag})
	require.True(t, res, "failed to match cheese note against any of animal or food tag")

	// try against any of multiple filters - match all (failure)
	res = applyNoteFilters(*cheeseNote, animalAndFoodItemFiltersAnyFalse, Tags{*animalTag, *foodTag})
	require.False(t, res, "incorrectly matched cheese note against both animal and food tag")

	// try against any of multiple filters - match any (failure)
	res = applyNoteFilters(*sportNote, animalAndFoodItemFiltersAnyFalse, Tags{*animalTag, *foodTag})
	require.False(t, res, "incorrectly matched sport note against animal and food tags")

	// try against any of multiple filters - match any (success)
	res = applyNoteFilters(*gnuNote, animalItemFiltersNegativeMatchAny, Tags{*foodTag})
	require.True(t, res, "expected true as gnu note should be negative match for food tag")

	// try against any of multiple filters - match all (failure)
	res = applyNoteFilters(*gnuNote, animalItemFiltersNegativeMatchAll, Tags{*foodTag, *animalTag})
	require.False(t, res, "expected false as gnu note should be negative match for food tag only")

	// try against any of multiple filters - match any (failure)
	res = applyNoteFilters(*gnuNote, animalItemFiltersNegativeMatchAny, Tags{*animalTag})
	require.False(t, res, "expected gnu note not to match negative animal tag")

	// try against any of multiple filters - don't want note to match any of the food nor animal tags (success)
	res = applyNoteFilters(*gnuNote, animalItemFiltersNegativeMatchAny, Tags{*foodTag, *animalTag})
	require.False(t, res, "wanted negative match against animal tag")

	// try against any of multiple filters - match all (failure)
	res = applyNoteFilters(*gnuNote, animalItemFiltersNegativeMatchAll, Tags{*animalTag, *foodTag})
	require.False(t, res, "expected gnu note not to match negative animal tag")

	// try against any of multiple filters - match all (success)
	res = applyNoteFilters(*gnuNote, animalItemFiltersNegativeMatchAll, Tags{*foodTag})
	require.True(t, res, "expected gnu note to negative match food tag")
}

func TestFilterNoteByTagTitle(t *testing.T) {
	t.Parallel()
	gnuNoteUUID := GenUUID()
	animalTagUUID := GenUUID()
	cheeseNoteUUID := GenUUID()
	foodTagUUID := GenUUID()
	sportNoteUUID := GenUUID()

	animalTag, _ := createTag("Animal", animalTagUUID, nil)
	gnuNote := createNote("GNU", "Is not Unix", gnuNoteUUID)
	sportNote := createNote("Sport", "Is dull", sportNoteUUID)

	foodTag, _ := createTag("Food", foodTagUUID, nil)
	cheeseNote := createNote("Cheese", "Is not a vegetable", cheeseNoteUUID)

	gnuRef := ItemReference{
		UUID:        gnuNoteUUID,
		ContentType: common.SNItemTypeNote,
	}

	animalTag.Content.UpsertReferences(ItemReferences{gnuRef})

	cheeseRef := ItemReference{
		UUID:        cheeseNoteUUID,
		ContentType: common.SNItemTypeNote,
	}
	foodTag.Content.UpsertReferences(ItemReferences{cheeseRef})

	animalTagTitleRegexFilter := Filter{
		Type:       common.SNItemTypeNote,
		Key:        "TagTitle",
		Comparison: "~",
		Value:      "^[A-Z]nima.?$",
	}

	animalTagTitleEqualsFilter := Filter{
		Type:       common.SNItemTypeNote,
		Key:        "TagTitle",
		Comparison: "==",
		Value:      "Animal",
	}

	foodTagUUIDFilter := Filter{
		Type:       common.SNItemTypeNote,
		Key:        "TagTitle",
		Comparison: "==",
		Value:      "Food",
	}

	animalTagUUIDFilterNegative := Filter{
		Type:       common.SNItemTypeNote,
		Key:        "TagUUID",
		Comparison: "!=",
		Value:      animalTagUUID,
	}

	animalItemFiltersTagTitleRegex := ItemFilters{
		Filters:  []Filter{animalTagTitleRegexFilter},
		MatchAny: true,
	}

	animalItemFiltersNegativeMatchAny := ItemFilters{
		Filters:  []Filter{animalTagUUIDFilterNegative},
		MatchAny: true,
	}

	animalItemFiltersNegativeMatchAll := ItemFilters{
		Filters:  []Filter{animalTagUUIDFilterNegative},
		MatchAny: false,
	}

	animalItemFilters := ItemFilters{
		Filters:  []Filter{animalTagTitleEqualsFilter},
		MatchAny: true,
	}
	animalAndFoodItemFiltersAnyTrue := ItemFilters{
		Filters:  []Filter{foodTagUUIDFilter, animalTagTitleEqualsFilter},
		MatchAny: true,
	}
	animalAndFoodItemFiltersAnyFalse := ItemFilters{
		Filters:  []Filter{foodTagUUIDFilter, animalTagTitleEqualsFilter},
		MatchAny: false,
	}
	animalAndFoodItemFiltersIncRegexAnyTrue := ItemFilters{
		Filters:  []Filter{foodTagUUIDFilter, animalTagTitleRegexFilter},
		MatchAny: true,
	}
	animalAndFoodItemFiltersIncRegexAnyFalse := ItemFilters{
		Filters:  []Filter{foodTagUUIDFilter, animalTagTitleRegexFilter},
		MatchAny: false,
	}

	// try match single animal by tag title regex (success)
	res := applyNoteFilters(*gnuNote, animalItemFiltersTagTitleRegex, Tags{*animalTag})
	require.True(t, res, "failed to match any note by tag title regex")

	// try match single animal (success)
	res = applyNoteFilters(*gnuNote, animalItemFilters, Tags{*animalTag})
	require.True(t, res, "failed to match any note by tag title")

	// try match animal note against food tag (failure)
	res = applyNoteFilters(*gnuNote, animalItemFilters, Tags{*foodTag})
	require.False(t, res, "incorrectly matched note by tag title")

	// try against any of multiple filters - match any (success)
	res = applyNoteFilters(*cheeseNote, animalAndFoodItemFiltersAnyTrue, Tags{*animalTag, *foodTag})
	require.True(t, res, "failed to match cheese note against any of animal or food tag")

	// try against any of multiple filters - match any (success)
	res = applyNoteFilters(*cheeseNote, animalAndFoodItemFiltersIncRegexAnyTrue, Tags{*animalTag, *foodTag})
	require.True(t, res, "failed to match cheese note against any of animal or food tag")

	// try against any of multiple filters - match any (success)
	res = applyNoteFilters(*cheeseNote, animalAndFoodItemFiltersIncRegexAnyFalse, Tags{*animalTag, *foodTag})
	require.False(t, res, "incorrectly matched cheese note against both animal and food")

	// try against any of multiple filters - match all (failure)
	res = applyNoteFilters(*cheeseNote, animalAndFoodItemFiltersAnyFalse, Tags{*animalTag, *foodTag})
	require.False(t, res, "incorrectly matched cheese note against both animal and food tag")

	// try against any of multiple filters - match any (failure)
	res = applyNoteFilters(*sportNote, animalAndFoodItemFiltersAnyFalse, Tags{*animalTag, *foodTag})
	require.False(t, res, "incorrectly matched sport note against animal and food tags")

	// try against any of multiple filters - match any (success)
	res = applyNoteFilters(*gnuNote, animalItemFiltersNegativeMatchAny, Tags{*foodTag})
	require.True(t, res, "expected true as gnu note should be negative match for food tag")

	// try against any of multiple filters - match all (failure)
	res = applyNoteFilters(*gnuNote, animalItemFiltersNegativeMatchAll, Tags{*foodTag, *animalTag})
	require.False(t, res, "expected false as gnu note should be negative match for food tag only")

	// try against any of multiple filters - match any (failure)
	res = applyNoteFilters(*gnuNote, animalItemFiltersNegativeMatchAny, Tags{*animalTag})
	require.False(t, res, "expected gnu note not to match negative animal tag")

	// try against any of multiple filters - don't want note to match any of the food nor animal tags (success)
	res = applyNoteFilters(*gnuNote, animalItemFiltersNegativeMatchAny, Tags{*foodTag, *animalTag})
	require.False(t, res, "wanted negative match against animal tag")

	// try against any of multiple filters - match all (failure)
	res = applyNoteFilters(*gnuNote, animalItemFiltersNegativeMatchAll, Tags{*animalTag, *foodTag})
	require.False(t, res, "expected gnu note not to match negative animal tag")

	// try against any of multiple filters - match all (success)
	res = applyNoteFilters(*gnuNote, animalItemFiltersNegativeMatchAll, Tags{*foodTag})
	require.True(t, res, "expected gnu note to negative match food tag")
}

func TestFilterNoteTitleContains(t *testing.T) {
	t.Parallel()
	gnuNote := createNote("GNU", "Is not Unix", "")
	filter := Filter{
		Type:       common.SNItemTypeNote,
		Key:        "Title",
		Comparison: "contains",
		Value:      "N",
	}
	itemFilters := ItemFilters{
		Filters:  []Filter{filter},
		MatchAny: true,
	}
	res := applyNoteFilters(*gnuNote, itemFilters, nil)
	require.True(t, res, "failed to match note by title contains")
}

func TestFilterNoteText(t *testing.T) {
	t.Parallel()
	gnuNote := createNote("GNU", "Is not Unix", "")
	filter := Filter{
		Type:       common.SNItemTypeNote,
		Key:        "Text",
		Comparison: "==",
		Value:      "Is not Unix",
	}
	itemFilters := ItemFilters{
		Filters:  []Filter{filter},
		MatchAny: true,
	}
	res := applyNoteFilters(*gnuNote, itemFilters, nil)
	require.True(t, res, "failed to match note by text")
}

func TestFilterNoteTextContains(t *testing.T) {
	t.Parallel()
	gnuNote := createNote("GNU", "Is not Unix", "")
	filter := Filter{
		Type:       common.SNItemTypeNote,
		Key:        "Text",
		Comparison: "contains",
		Value:      "Unix",
	}
	itemFilters := ItemFilters{
		Filters:  []Filter{filter},
		MatchAny: true,
	}
	res := applyNoteFilters(*gnuNote, itemFilters, nil)
	require.True(t, res, "failed to match note by title contains")
}

func TestFilterNoteTitleNotEqualTo(t *testing.T) {
	t.Parallel()
	gnuNote := createNote("GNU", "Is not Unix", "")
	filter := Filter{
		Type:       common.SNItemTypeNote,
		Key:        "Title",
		Comparison: "!=",
		Value:      "Potato",
	}
	itemFilters := ItemFilters{
		Filters:  []Filter{filter},
		MatchAny: true,
	}
	res := applyNoteFilters(*gnuNote, itemFilters, nil)
	require.True(t, res, "failed to match note by negative title match")
}

func TestFilterNoteTextNotEqualTo(t *testing.T) {
	t.Parallel()
	gnuNote := createNote("GNU", "Is not Unix", "")
	filter := Filter{
		Type:       common.SNItemTypeNote,
		Key:        "Text",
		Comparison: "!=",
		Value:      "Potato",
	}
	itemFilters := ItemFilters{
		Filters:  []Filter{filter},
		MatchAny: true,
	}
	res := applyNoteFilters(*gnuNote, itemFilters, nil)
	require.True(t, res, "failed to match note by negative text match")
}

func TestFilterNoteTextByRegex(t *testing.T) {
	t.Parallel()
	gnuNote := createNote("GNU", "Is not Unix", "")
	filter := Filter{
		Type:       common.SNItemTypeNote,
		Key:        "Text",
		Comparison: "~",
		Value:      "^.*Unix",
	}
	itemFilters := ItemFilters{
		Filters:  []Filter{filter},
		MatchAny: true,
	}
	res := applyNoteFilters(*gnuNote, itemFilters, nil)
	require.True(t, res, "failed to match note by text regex")
}

func TestFilterNoteTitleByRegex(t *testing.T) {
	t.Parallel()
	gnuNote := createNote("GNU", "Is not Unix", "")
	filter := Filter{
		Type:       common.SNItemTypeNote,
		Key:        "Title",
		Comparison: "~",
		Value:      "^.N.$",
	}
	itemFilters := ItemFilters{
		Filters:  []Filter{filter},
		MatchAny: true,
	}
	res := applyNoteFilters(*gnuNote, itemFilters, nil)
	require.True(t, res, "failed to match note by title text regex")
}

func TestFilterTagTitle(t *testing.T) {
	t.Parallel()
	gnuTag, _ := createTag("GNU", GenUUID(), nil)
	filter := Filter{
		Type:       common.SNItemTypeTag,
		Key:        "Title",
		Comparison: "==",
		Value:      "GNU",
	}
	itemFilters := ItemFilters{
		Filters:  []Filter{filter},
		MatchAny: true,
	}
	res := applyTagFilters(*gnuTag, itemFilters)
	require.True(t, res, "failed to match tag by title")
}

func TestFilterTagUUID(t *testing.T) {
	t.Parallel()
	uuid := GenUUID()
	gnuTag, _ := createTag("GNU", uuid, nil)
	filter := Filter{
		Type:       common.SNItemTypeTag,
		Key:        "UUID",
		Comparison: "==",
		Value:      uuid,
	}
	itemFilters := ItemFilters{
		Filters:  []Filter{filter},
		MatchAny: true,
	}
	res := applyTagFilters(*gnuTag, itemFilters)
	require.True(t, res, "failed to match tag by uuid")
}

func TestFilterTagTitleByRegex(t *testing.T) {
	t.Parallel()
	gnuTag, _ := createTag("GNU", GenUUID(), nil)
	filter := Filter{
		Type:       common.SNItemTypeTag,
		Key:        "Title",
		Comparison: "~",
		Value:      "^.*U$",
	}
	itemFilters := ItemFilters{
		Filters:  []Filter{filter},
		MatchAny: true,
	}
	res := applyTagFilters(*gnuTag, itemFilters)
	require.True(t, res, "failed to match tag by title regex")
}

func TestFilterTagTitleByNotEqualTo(t *testing.T) {
	t.Parallel()
	gnuTag, _ := createTag("GNU", GenUUID(), nil)
	filter := Filter{
		Type:       common.SNItemTypeTag,
		Key:        "Title",
		Comparison: "!=",
		Value:      "potato",
	}
	itemFilters := ItemFilters{
		Filters:  []Filter{filter},
		MatchAny: true,
	}
	res := applyTagFilters(*gnuTag, itemFilters)
	require.True(t, res, "failed to match tag by title negative title match")
}

func TestFilterNoteByTitleAndDeletion(t *testing.T) {
	t.Parallel()
	scotlandNote := createNote("Scotland", "example", "")
	englandNote := createNote("England", "example", "")
	englandNote.Deleted = true

	itemFilters := ItemFilters{
		Filters: []Filter{
			{
				Type:       common.SNItemTypeNote,
				Key:        "Title",
				Comparison: "Contans",
				Value:      "land",
			},
			{
				Type:       common.SNItemTypeNote,
				Key:        "Deleted",
				Comparison: "==",
				Value:      "False",
			},
		},
		MatchAny: true,
	}
	res := applyNoteFilters(*scotlandNote, itemFilters, nil)
	require.True(t, res, "failed to match note by title and deletion status")
	res = applyNoteFilters(*englandNote, itemFilters, nil)
	require.False(t, res, "failed to match note by title and deletion status")
}
