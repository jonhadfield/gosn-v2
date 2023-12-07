package gosn

import (
	"encoding/json"
	"testing"

	"testing"

	"github.com/stretchr/testify/require"
)

func TestSchemaIsLoaded(t *testing.T) {
	t.Parallel()

	require.NotEmpty(t, testSession.Schemas)
	require.NotNil(t, testSession.Schemas[noteContentSchemaName])
}

func TestSchemaValidation(t *testing.T) {
	t.Parallel()

	// succeed with valid instance
	noteSchema := testSession.Schemas[noteContentSchemaName]
	instance := `{"text":"text-area","title":"title","noteType":"markdown","references":[],"appData":{"org.standardnotes.sn":{"client_updated_at":"2023-02-19T11:24:13.255Z","pinned":true},"org.standardnotes.sn.components":{"536bdf3e-1968-45cc-8bb1-4529873efc33":{}}},"preview_plain":"preview-plain-area","editorIdentifier":"org.standardnotes.markdown-visual-editor","spellcheck":true}`
	var v interface{}
	require.NoError(t, json.Unmarshal([]byte(instance), &v))
	require.NoError(t, noteSchema.Validate(v))

	// remove necessary text attribute
	instance = `{"title":"title","noteType":"markdown","references":[],"appData":{"org.standardnotes.sn":{"client_updated_at":"2023-02-19T11:24:13.255Z","pinned":true},"org.standardnotes.sn.components":{"536bdf3e-1968-45cc-8bb1-4529873efc33":{}}},"preview_plain":"preview-plain-area","editorIdentifier":"org.standardnotes.markdown-visual-editor","spellcheck":"true"}`
	require.NoError(t, json.Unmarshal([]byte(instance), &v))
	err := noteSchema.Validate(v)
	require.Error(t, err)
	require.ErrorContains(t, err, "missing")

	// add unexpected new attribute
	instance = `{"this":"is new attribute","text":"text-area","title":"title","noteType":"markdown","references":[],"appData":{"org.standardnotes.sn":{"client_updated_at":"2023-02-19T11:24:13.255Z","pinned":true},"org.standardnotes.sn.components":{"536bdf3e-1968-45cc-8bb1-4529873efc33":{}}},"preview_plain":"preview-plain-area","editorIdentifier":"org.standardnotes.markdown-visual-editor","spellcheck":true}`
	require.NoError(t, json.Unmarshal([]byte(instance), &v))
	err = noteSchema.Validate(v)
	require.Error(t, err)
	require.ErrorContains(t, err, "additionalProperties")
}
