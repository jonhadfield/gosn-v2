package items

import (
	"sort"
	"strings"

	"github.com/jonhadfield/gosn-v2/common"
)

// PrefKeyDefaultEditorIdentifier is the user preference key holding the editor
// used for new notes. It matches PrefKey.DefaultEditorIdentifier in Standard
// Notes' own models package.
const PrefKeyDefaultEditorIdentifier = "defaultEditorIdentifier"

// EditorPlainText is the identifier Standard Notes falls back to when no
// default editor is set, or when the one that is set cannot be resolved.
const EditorPlainText = "com.standardnotes.plain-text"

// ComponentAreaEditor is the Area value a component carries when it is a note
// editor rather than a theme or other extension.
const ComponentAreaEditor = "editor"

// Editor identifies a note editor that can be set as the default for new
// notes.
type Editor struct {
	// Identifier is the value written to the defaultEditorIdentifier
	// preference, e.g. "com.standardnotes.super-editor".
	Identifier string
	// Name is the human readable name Standard Notes displays.
	Name string
	// BuiltIn reports whether the editor ships with Standard Notes rather than
	// being installed into the account as a component.
	BuiltIn bool
	// Deprecated reports whether Standard Notes has retired the editor. These
	// remain settable because existing accounts still reference them, but they
	// should not be offered as a first choice.
	Deprecated bool
}

// builtInEditors mirrors the native and iframe editor lists in Standard Notes'
// features package. Built-in editors are compile-time constants there and are
// not present as items in an account, so they cannot be discovered by syncing
// and have to be listed here.
//
// Keep in step with:
//
//	packages/features/src/Domain/Feature/NativeFeatureIdentifier.ts
//	packages/features/src/Domain/Lists/{NativeEditors,IframeEditors,DeprecatedFeatures}.ts
var builtInEditors = []Editor{
	{Identifier: EditorPlainText, Name: "Plain Text", BuiltIn: true},
	{Identifier: "com.standardnotes.super-editor", Name: "Super", BuiltIn: true},
	{Identifier: "org.standardnotes.standard-sheets", Name: "Spreadsheet", BuiltIn: true},
	{Identifier: "org.standardnotes.token-vault", Name: "Authenticator", BuiltIn: true},

	{Identifier: "org.standardnotes.code-editor", Name: "Code", BuiltIn: true, Deprecated: true},
	{Identifier: "org.standardnotes.plus-editor", Name: "Rich Text", BuiltIn: true, Deprecated: true},
	{Identifier: "org.standardnotes.advanced-markdown-editor", Name: "Markdown Pro", BuiltIn: true, Deprecated: true},
	{Identifier: "org.standardnotes.markdown-visual-editor", Name: "Markdown Visual", BuiltIn: true, Deprecated: true},
	{Identifier: "org.standardnotes.simple-markdown-editor", Name: "Markdown Basic", BuiltIn: true, Deprecated: true},
	{Identifier: "org.standardnotes.fancy-markdown-editor", Name: "Markdown Math", BuiltIn: true, Deprecated: true},
	{Identifier: "org.standardnotes.minimal-markdown-editor", Name: "Markdown Minimist", BuiltIn: true, Deprecated: true},
	{Identifier: "org.standardnotes.bold-editor", Name: "Bold", BuiltIn: true, Deprecated: true},
	{Identifier: SimpleTaskEditorNoteType, Name: "Checklist", BuiltIn: true, Deprecated: true},
}

// BuiltInEditors returns the editors that ship with Standard Notes.
func BuiltInEditors() []Editor {
	out := make([]Editor, len(builtInEditors))
	copy(out, builtInEditors)

	return out
}

// InstalledEditors returns the editors installed into the account as
// components. Unlike the built-in list these are discovered from synced items,
// so they are always accurate for the account they came from.
func (i Items) InstalledEditors() []Editor {
	var out []Editor

	for _, x := range i {
		if x.GetContentType() != common.SNItemTypeComponent || x.IsDeleted() {
			continue
		}

		component, ok := x.(*Component)
		if !ok {
			continue
		}

		if !strings.EqualFold(component.Content.Area, ComponentAreaEditor) {
			continue
		}

		name := component.Content.Name
		if name == "" {
			name = component.Content.Identifier
		}

		out = append(out, Editor{
			Identifier: component.Content.Identifier,
			Name:       name,
			Deprecated: component.Content.IsDeprecated,
		})
	}

	return out
}

// AvailableEditors returns every editor that can be set as the default for the
// account: those built in to Standard Notes plus any installed as components.
// An installed component takes precedence over a built-in entry sharing its
// identifier, since the account's own copy is the more accurate description.
// The result is sorted by name.
func AvailableEditors(i Items) []Editor {
	installed := i.InstalledEditors()

	seen := make(map[string]bool, len(installed))
	out := make([]Editor, 0, len(installed)+len(builtInEditors))

	for _, e := range installed {
		if e.Identifier == "" || seen[e.Identifier] {
			continue
		}

		seen[e.Identifier] = true

		out = append(out, e)
	}

	for _, e := range builtInEditors {
		if seen[e.Identifier] {
			continue
		}

		seen[e.Identifier] = true

		out = append(out, e)
	}

	sort.Slice(out, func(a, b int) bool {
		return strings.ToLower(out[a].Name) < strings.ToLower(out[b].Name)
	})

	return out
}

// FindEditor resolves a user-supplied string to an editor, matching either the
// identifier or the display name, case insensitively. It exists so callers can
// accept "Super" as readily as "com.standardnotes.super-editor".
func FindEditor(editors []Editor, nameOrIdentifier string) (Editor, bool) {
	needle := strings.TrimSpace(nameOrIdentifier)
	if needle == "" {
		return Editor{}, false
	}

	// Prefer an exact identifier match, so a component whose display name
	// happens to collide with another's identifier cannot shadow it.
	for _, e := range editors {
		if strings.EqualFold(e.Identifier, needle) {
			return e, true
		}
	}

	for _, e := range editors {
		if strings.EqualFold(e.Name, needle) {
			return e, true
		}
	}

	return Editor{}, false
}

// GetDefaultEditorIdentifier returns the editor used for new notes, and
// whether the preference is set at all. An unset preference is not an error:
// Standard Notes falls back to plain text.
func (cc *UserPreferencesContent) GetDefaultEditorIdentifier() (string, bool) {
	value, exists := cc.GetPref(PrefKeyDefaultEditorIdentifier)
	if !exists {
		return "", false
	}

	identifier, ok := value.(string)
	if !ok || identifier == "" {
		return "", false
	}

	return identifier, true
}

// SetDefaultEditorIdentifier sets the editor used for new notes.
//
// Standard Notes does not validate this value when reading it: an identifier
// matching no available editor silently falls back to plain text. Callers
// should resolve the identifier with FindEditor first so that a typo is
// reported rather than quietly ignored.
func (cc *UserPreferencesContent) SetDefaultEditorIdentifier(identifier string) {
	cc.SetPref(PrefKeyDefaultEditorIdentifier, identifier)
}
