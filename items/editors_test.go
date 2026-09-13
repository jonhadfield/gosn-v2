package items

import (
	"testing"

	"github.com/jonhadfield/gosn-v2/common"
	"github.com/stretchr/testify/require"
)

func newTestEditorComponent(identifier, name, area string) *Component {
	c := NewComponent()
	c.ContentType = common.SNItemTypeComponent
	c.Content = *NewComponentContent()
	c.Content.Identifier = identifier
	c.Content.Name = name
	c.Content.Area = area

	return &c
}

func TestBuiltInEditorsIsACopy(t *testing.T) {
	t.Parallel()

	first := BuiltInEditors()
	require.NotEmpty(t, first)

	first[0].Name = "mutated"

	require.NotEqual(t, "mutated", BuiltInEditors()[0].Name, "callers must not be able to mutate the package list")
}

func TestBuiltInEditorsIdentifiersAreUnique(t *testing.T) {
	t.Parallel()

	seen := make(map[string]bool)
	for _, e := range BuiltInEditors() {
		require.NotEmpty(t, e.Identifier, "editor %q has no identifier", e.Name)
		require.NotEmpty(t, e.Name, "editor %q has no name", e.Identifier)
		require.False(t, seen[e.Identifier], "duplicate identifier %q", e.Identifier)
		seen[e.Identifier] = true
	}

	require.True(t, seen[EditorPlainText], "plain text must be present as the fallback editor")
}

func TestInstalledEditorsFiltersByArea(t *testing.T) {
	t.Parallel()

	its := Items{
		newTestEditorComponent("com.example.my-editor", "My Editor", ComponentAreaEditor),
		newTestEditorComponent("com.example.my-theme", "My Theme", "themes"),
	}

	editors := its.InstalledEditors()

	require.Len(t, editors, 1)
	require.Equal(t, "com.example.my-editor", editors[0].Identifier)
	require.Equal(t, "My Editor", editors[0].Name)
	require.False(t, editors[0].BuiltIn, "a component is not a built-in editor")
}

func TestInstalledEditorsFallsBackToIdentifierWhenUnnamed(t *testing.T) {
	t.Parallel()

	its := Items{newTestEditorComponent("com.example.unnamed", "", ComponentAreaEditor)}

	editors := its.InstalledEditors()

	require.Len(t, editors, 1)
	require.Equal(t, "com.example.unnamed", editors[0].Name, "an unnamed editor should still be selectable")
}

func TestAvailableEditorsMergesAndSorts(t *testing.T) {
	t.Parallel()

	its := Items{newTestEditorComponent("com.example.aaa-editor", "AAA Editor", ComponentAreaEditor)}

	editors := AvailableEditors(its)

	require.Greater(t, len(editors), len(BuiltInEditors()), "installed editors should be added to the built-in list")
	require.Equal(t, "AAA Editor", editors[0].Name, "results should be sorted by name")

	seen := make(map[string]bool)
	for _, e := range editors {
		require.False(t, seen[e.Identifier], "duplicate identifier %q in merged list", e.Identifier)
		seen[e.Identifier] = true
	}
}

// TestAvailableEditorsPrefersInstalledOverBuiltIn covers an account that has
// installed a component sharing an identifier with a built-in editor: the
// account's own copy describes what is actually there.
func TestAvailableEditorsPrefersInstalledOverBuiltIn(t *testing.T) {
	t.Parallel()

	its := Items{newTestEditorComponent(EditorPlainText, "Plain Text (custom build)", ComponentAreaEditor)}

	editors := AvailableEditors(its)

	found, ok := FindEditor(editors, EditorPlainText)
	require.True(t, ok)
	require.Equal(t, "Plain Text (custom build)", found.Name)
	require.False(t, found.BuiltIn)

	count := 0
	for _, e := range editors {
		if e.Identifier == EditorPlainText {
			count++
		}
	}
	require.Equal(t, 1, count, "the identifier must not appear twice")
}

func TestFindEditor(t *testing.T) {
	t.Parallel()

	editors := AvailableEditors(nil)

	testCases := []struct {
		name           string
		input          string
		wantIdentifier string
		wantFound      bool
	}{
		{name: "by identifier", input: "com.standardnotes.super-editor", wantIdentifier: "com.standardnotes.super-editor", wantFound: true},
		{name: "by display name", input: "Super", wantIdentifier: "com.standardnotes.super-editor", wantFound: true},
		{name: "name is case insensitive", input: "plain text", wantIdentifier: EditorPlainText, wantFound: true},
		{name: "identifier is case insensitive", input: "COM.STANDARDNOTES.PLAIN-TEXT", wantIdentifier: EditorPlainText, wantFound: true},
		{name: "surrounding space is ignored", input: "  Super  ", wantIdentifier: "com.standardnotes.super-editor", wantFound: true},
		{name: "unknown editor", input: "com.example.nope", wantFound: false},
		{name: "empty input", input: "", wantFound: false},
		{name: "whitespace only", input: "   ", wantFound: false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, ok := FindEditor(editors, tc.input)
			require.Equal(t, tc.wantFound, ok)

			if tc.wantFound {
				require.Equal(t, tc.wantIdentifier, got.Identifier)
			}
		})
	}
}

// TestFindEditorPrefersIdentifierOverName guards the case where one editor's
// display name is another's identifier; the identifier match must win.
func TestFindEditorPrefersIdentifierOverName(t *testing.T) {
	t.Parallel()

	editors := []Editor{
		{Identifier: "com.example.b", Name: "com.example.a"},
		{Identifier: "com.example.a", Name: "Editor A"},
	}

	got, ok := FindEditor(editors, "com.example.a")
	require.True(t, ok)
	require.Equal(t, "com.example.a", got.Identifier, "an exact identifier match must take precedence over a name match")
}

func TestDefaultEditorIdentifierRoundTrip(t *testing.T) {
	t.Parallel()

	content := NewUserPreferencesContent()

	_, ok := content.GetDefaultEditorIdentifier()
	require.False(t, ok, "an unset preference should report as unset rather than empty")

	content.SetDefaultEditorIdentifier("com.standardnotes.super-editor")

	identifier, ok := content.GetDefaultEditorIdentifier()
	require.True(t, ok)
	require.Equal(t, "com.standardnotes.super-editor", identifier)
	require.Equal(t, "com.standardnotes.super-editor", content.Preferences[PrefKeyDefaultEditorIdentifier],
		"the value must be written under the key Standard Notes reads")
}

func TestGetDefaultEditorIdentifierIgnoresUnusableValues(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name  string
		value interface{}
	}{
		{name: "empty string", value: ""},
		{name: "wrong type", value: 42},
		{name: "nil", value: nil},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			content := NewUserPreferencesContent()
			content.SetPref(PrefKeyDefaultEditorIdentifier, tc.value)

			_, ok := content.GetDefaultEditorIdentifier()
			require.False(t, ok, "an unusable value should report as unset")
		})
	}
}
