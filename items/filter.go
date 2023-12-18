package items

import (
	"github.com/jonhadfield/gosn-v2/common"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

type ItemFilters struct {
	MatchAny bool
	Filters  []Filter
}

type Filter struct {
	Type       string
	Key        string
	Comparison string
	Value      string
}

func (i *Items) Filter(f ItemFilters) {
	var filtered Items

	var tags Tags

	// produce list of tags to be used in filters
	ix := *i
	for x := range ix {
		switch t := ix[x].(type) {
		case *Tag:
			tag := t
			tags = append(tags, *tag)
		}
	}

	for x := range ix {
		switch t := ix[x].(type) {
		case *Note:
			note := ix[x].(*Note)
			if found := applyNoteFilters(*note, f, tags); found {
				filtered = append(filtered, note)
			}
		case *Tag:
			tag := ix[x].(*Tag)
			if found := applyTagFilters(*t, f); found {
				filtered = append(filtered, tag)
			}
		case *Component:
			component := ix[x].(*Component)
			if found := applyComponentFilters(*component, f); found {
				filtered = append(filtered, component)
			}
		default:
			if found := applyAnyTypeFilters(ix[x], f); found {
				filtered = append(filtered, ix[x])
			}
		}
	}

	*i = filtered
}

func (i *Items) FilterAllTypes(f ItemFilters) {
	var filtered Items

	ix := *i

	for x := range ix {
		if found := applyAnyTypeFilters(ix[x], f); found {
			filtered = append(filtered, ix[x])
		}
	}

	*i = filtered
}

func applyNoteEditorFilter(f Filter, i Note, matchAny bool) (result, matchedAll, done bool) {
	content := i.GetContent().(*NoteContent)
	if content.EditorIdentifier == "" {
		matchedAll = false
	} else {
		switch f.Comparison {
		case "~":
			// TODO: Don't compile every time
			r := regexp.MustCompile(f.Value)
			editorIdentifier := content.EditorIdentifier
			if r.MatchString(editorIdentifier) {
				if matchAny {
					result = true
					done = true
					return
				}
				matchedAll = true
			} else {
				if !matchAny {
					result = false
					done = true
					return
				}
				matchedAll = false
			}
		case "==":
			if content.EditorIdentifier == f.Value {
				if matchAny {
					result = true
					done = true

					return
				}

				matchedAll = true
			} else {
				if !matchAny {
					result = false
					done = true
					return
				}
				matchedAll = false
			}
		case "!=":
			if content.EditorIdentifier != f.Value {
				if matchAny {
					result = true
					done = true
					return
				}
				matchedAll = true
			} else {
				if !matchAny {
					result = false
					done = true
					return
				}
				matchedAll = false
			}
		case "contains":
			if strings.Contains(content.EditorIdentifier, f.Value) {
				if matchAny {
					result = true
					done = true
					return
				}
				matchedAll = true
			} else {
				if !matchAny {
					result = false
					done = true
					return
				}
				matchedAll = false
			}
		}
	}

	return result, matchedAll, done
}

func applyNoteTextFilter(f Filter, i Note, matchAny bool) (result, matchedAll, done bool) {
	content := i.GetContent().(*NoteContent)
	if content.Title == "" {
		matchedAll = false
	} else {
		switch f.Comparison {
		case "~":
			// TODO: Don't compile every time
			r := regexp.MustCompile(f.Value)
			text := content.GetText()
			if r.MatchString(text) {
				if matchAny {
					result = true
					done = true
					return
				}
				matchedAll = true
			} else {
				if !matchAny {
					result = false
					done = true
					return
				}
				matchedAll = false
			}
		case "==":
			if content.GetText() == f.Value {
				if matchAny {
					result = true
					done = true
					return
				}
				matchedAll = true
			} else {
				if !matchAny {
					result = false
					done = true
					return
				}
				matchedAll = false
			}
		case "!=":
			if content.GetText() != f.Value {
				if matchAny {
					result = true
					done = true
					return
				}
				matchedAll = true
			} else {
				if !matchAny {
					result = false
					done = true
					return
				}
				matchedAll = false
			}
		case "contains":
			if strings.Contains(content.GetText(), f.Value) {
				if matchAny {
					result = true
					done = true
					return
				}
				matchedAll = true
			} else {
				if !matchAny {
					result = false
					done = true
					return
				}
				matchedAll = false
			}
		}
	}

	return result, matchedAll, done
}

func applyNoteTrashedFilter(f Filter, i Note, matchAny bool) (result, matchedAll, done bool) {
	content := i.GetContent().(*NoteContent)
	tp := content.Trashed

	var isTrashed bool

	if tp != nil && *tp {
		isTrashed = true
	}

	switch f.Comparison {
	case "==":
		if isTrashed {
			if matchAny {
				result = true
				done = true

				return
			}
			matchedAll = true
		} else {
			if !matchAny {
				result = false
				done = true
				return
			}
			matchedAll = false
		}
	case "!=":
		if !isTrashed {
			if matchAny {
				result = true
				done = true
				return
			}
			matchedAll = true
		} else {
			if !matchAny {
				result = false
				done = true
				return
			}
			matchedAll = false
		}
	}

	return result, matchedAll, done
}

func applyNoteTagTitleFilter(f Filter, i Note, tags Tags, matchAny bool) (result, matchedAll, done bool) {
	var matchesTag bool

	for _, tag := range tags {
		if tag.Content.Title == "" {
			matchedAll = false
		} else {
			switch f.Comparison {
			case "~":
				r := regexp.MustCompile(f.Value)
				if r.MatchString(tag.Content.Title) {
					for _, ref := range tag.Content.References() {
						if i.UUID == ref.UUID {
							matchesTag = true
						}
					}
				}
				if matchesTag {
					if matchAny {
						result = true
						done = true
						return
					}
					matchedAll = true
				} else {
					if !matchAny {
						result = false
						done = true
						return
					}
					matchedAll = false
				}
			case "==":
				if tag.Content.Title == f.Value {
					for _, ref := range tag.Content.References() {
						if i.UUID == ref.UUID {
							matchesTag = true
						}
					}
				}
				if matchesTag {
					if matchAny {
						result = true
						done = true
						return
					}
					matchedAll = true
				} else {
					if !matchAny {
						result = false
						done = true
						return
					}
					matchedAll = false
				}
			}
		}
	}

	return result, matchedAll, done
}

func applyNoteTagUUIDFilter(f Filter, i Note, tags Tags, matchAny bool) (result, matchedAll, done bool) {
	var matchesTag bool

	for _, tag := range tags {
		if tag.UUID == f.Value {
			for _, ref := range tag.Content.References() {
				if i.UUID == ref.UUID {
					matchesTag = true
				}
			}
			// after checking all references in the matching ID we can move on
			break
		}
	}

	switch f.Comparison {
	case "==":
		if matchesTag {
			if matchAny {
				result = true
				done = true

				return
			}

			matchedAll = true
		} else {
			if !matchAny {
				result = false
				done = true

				return
			}
			matchedAll = false
		}
	case "!=":
		if matchesTag {
			if matchAny {
				result = false
				done = true

				return
			}

			matchedAll = false
		} else {
			if !matchAny {
				result = true
				done = true

				return
			}
			matchedAll = true
		}
	}

	return result, matchedAll, done
}

func applyNoteFilters(item Note, itemFilters ItemFilters, tags Tags) bool {
	var matchedAll, result, done bool

	for i, filter := range itemFilters.Filters {
		if !slices.Contains([]string{common.SNItemTypeNote, "Item"}, filter.Type) {
			continue
		}

		switch strings.ToLower(filter.Key) {
		case "title": // GetTitle
			result, matchedAll, done = applyNoteTitleFilter(filter, item, itemFilters.MatchAny)
			if done {
				return result
			}
		case "text": // Text
			result, matchedAll, done = applyNoteTextFilter(filter, item, itemFilters.MatchAny)
			if done {
				return result
			}
		case "editor": // GetEditor
			result, matchedAll, done = applyNoteEditorFilter(filter, item, itemFilters.MatchAny)
			if done {
				return result
			}
		case "trash": // trash
			result, matchedAll, done = applyNoteTrashedFilter(filter, item, itemFilters.MatchAny)
			if done {
				return result
			}
		case "tagtitle": // Tag Title
			result, matchedAll, done = applyNoteTagTitleFilter(filter, item, tags, itemFilters.MatchAny)
			if done {
				return result
			}
		case "taguuid": // Tag UUID
			result, matchedAll, done = applyNoteTagUUIDFilter(filter, item, tags, itemFilters.MatchAny)
			if done {
				return result
			}
		case "uuid": // UUID
			if item.UUID == filter.Value {
				if itemFilters.MatchAny {
					return true
				}

				matchedAll = true
			} else {
				if !itemFilters.MatchAny {
					return false
				}
				matchedAll = false
			}
		case "deleted": // Deleted
			isDel, _ := strconv.ParseBool(filter.Value)
			if item.Deleted == isDel {
				if itemFilters.MatchAny {
					return true
				}

				matchedAll = true
			} else {
				if !itemFilters.MatchAny {
					return false
				}

				matchedAll = false
			}
		default:
			matchedAll = true // if no criteria specified then filter applies to type only
		}
		// if last filter and matchedAll is true, then return true
		if matchedAll && i == len(itemFilters.Filters)-1 {
			return true
		}
	}

	return matchedAll
}

func applyNoteTitleFilter(f Filter, i Note, matchAny bool) (result, matchedAll, done bool) {
	if i.Content.Title == "" {
		matchedAll = false
	} else {
		switch f.Comparison {
		case "~":
			r := regexp.MustCompile(f.Value)
			if r.MatchString(i.Content.GetTitle()) {
				if matchAny {
					result = true
					done = true
					return
				}
				matchedAll = true
			} else {
				if !matchAny {
					result = false
					done = true
					return
				}
				matchedAll = false
			}
		case "==":
			if i.Content.GetTitle() == f.Value {
				if matchAny {
					result = true
					done = true
					return
				}
				matchedAll = true
			} else {
				if !matchAny {
					result = false
					done = true
					return
				}
				matchedAll = false
			}
		case "!=":
			if i.Content.GetTitle() != f.Value {
				if matchAny {
					result = true
					done = true
					return
				}
				matchedAll = true
			} else {
				if matchAny {
					result = false
					done = true
					return
				}
				matchedAll = false
			}
		case "contains":
			if strings.Contains(i.Content.GetTitle(), f.Value) {
				if matchAny {
					result = true
					done = true
					return
				}
				matchedAll = true
			} else {
				if !matchAny {
					result = false
					done = true
					return
				}
				matchedAll = false
			}
		}
	}

	return result, matchedAll, done
}

func applyTagFilters(item Tag, itemFilters ItemFilters) bool {
	var matchedAll bool

	for _, filter := range itemFilters.Filters {
		if !slices.Contains([]string{common.SNItemTypeTag, "Item"}, filter.Type) {
			continue
		}

		switch strings.ToLower(filter.Key) {
		case "title":
			if item.Content.Title == "" {
				matchedAll = false
			} else {
				switch filter.Comparison {
				case "~":
					r := regexp.MustCompile(filter.Value)
					if r.MatchString(item.Content.GetTitle()) {
						if itemFilters.MatchAny {
							return true
						}
						matchedAll = true
					} else {
						if !itemFilters.MatchAny {
							return false
						}
						matchedAll = false
					}
				case "==":
					if item.Content.GetTitle() == filter.Value {
						if itemFilters.MatchAny {
							return true
						}
						matchedAll = true
					} else {
						if !itemFilters.MatchAny {
							return false
						}
						matchedAll = false
					}
				case "!=":
					if item.Content.GetTitle() != filter.Value {
						if itemFilters.MatchAny {
							return true
						}
						matchedAll = true
					} else {
						if !itemFilters.MatchAny {
							return false
						}
						matchedAll = false
					}
				case "contains":
					if strings.Contains(item.Content.GetTitle(), filter.Value) {
						if itemFilters.MatchAny {
							return true
						}
						matchedAll = true
					} else {
						if !itemFilters.MatchAny {
							return false
						}
						matchedAll = false
					}
				}
			}
		case "uuid":
			if item.UUID == filter.Value {
				if itemFilters.MatchAny {
					return true
				}

				matchedAll = true
			} else {
				if !itemFilters.MatchAny {
					return false
				}

				matchedAll = false
			}
		default:
			matchedAll = true // if no criteria specified then filter applies to type only, so true
		}
	}

	return matchedAll
}

func applyComponentFilters(item Component, itemFilters ItemFilters) bool {
	var matchedAll bool

	for _, filter := range itemFilters.Filters {
		if !slices.Contains([]string{common.SNItemTypeComponent, "Item"}, filter.Type) {
			continue
		}

		switch strings.ToLower(filter.Key) {
		case "name":
			if item.Content.Name == "" {
				matchedAll = false
			} else {
				switch filter.Comparison {
				case "~":
					r := regexp.MustCompile(filter.Value)
					if r.MatchString(item.Content.GetName()) {
						if itemFilters.MatchAny {
							return true
						}
						matchedAll = true
					} else {
						if !itemFilters.MatchAny {
							return false
						}
						matchedAll = false
					}
				case "==":
					if item.Content.GetName() == filter.Value {
						if itemFilters.MatchAny {
							return true
						}
						matchedAll = true
					} else {
						if !itemFilters.MatchAny {
							return false
						}
						matchedAll = false
					}
				case "!=":
					if item.Content.GetName() != filter.Value {
						if itemFilters.MatchAny {
							return true
						}
						matchedAll = true
					} else {
						if !itemFilters.MatchAny {
							return false
						}
						matchedAll = false
					}
				case "contains":
					if strings.Contains(item.Content.GetName(), filter.Value) {
						if itemFilters.MatchAny {
							return true
						}
						matchedAll = true
					} else {
						if !itemFilters.MatchAny {
							return false
						}
						matchedAll = false
					}
				}
			}
		case "uuid":
			if item.UUID == filter.Value {
				if itemFilters.MatchAny {
					return true
				}

				matchedAll = true
			} else {
				if !itemFilters.MatchAny {
					return false
				}

				matchedAll = false
			}
		case "active":
			filterActive, _ := strconv.ParseBool(filter.Value)
			if item.Content.GetActive() == filterActive {
				if itemFilters.MatchAny {
					return true
				}

				matchedAll = true
			} else {
				if !itemFilters.MatchAny {
					return false
				}

				matchedAll = false
			}
		default:
			matchedAll = true // if no criteria specified then filter applies to type only, so true
		}
	}

	return matchedAll
}

func applyAnyTypeFilters(item Item, itemFilters ItemFilters) bool {
	var matchedAll bool

	for _, filter := range itemFilters.Filters {
		switch strings.ToLower(filter.Key) {
		case "uuid":
			if item.GetUUID() == filter.Value {
				if itemFilters.MatchAny {
					return true
				}

				matchedAll = true
			} else {
				if !itemFilters.MatchAny {
					return false
				}

				matchedAll = false
			}
		default:
			matchedAll = false
		}
	}

	return matchedAll
}
