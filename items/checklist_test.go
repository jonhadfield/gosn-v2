package items

import (
	"encoding/json"
	"sort"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

func (c Checklist) Sort() {
	// sort groups
	sort.Slice(c.Groups, func(i, j int) bool {
		return c.Groups[i].LastActive.Unix() > c.Groups[j].LastActive.Unix()
	})

	for x := range c.Groups {
		// sort sections
		sort.Slice(c.Groups[x].Sections, func(i, j int) bool {
			return c.Groups[x].Sections[i].Name < c.Groups[x].Sections[j].Name
		})

		sort.Slice(c.Groups[x].Tasks, func(i, j int) bool {
			return !c.Groups[x].Tasks[i].Completed && c.Groups[x].Tasks[j].Completed
		})
	}
}

func TestActivityChecklistParsing(t *testing.T) {
	exampleChecklist := `"{\n  \"schemaVersion\": \"1.0.0\",\n  \"groups\": [\n    {\n      \"name\": \"Group 1\",\n      \"tasks\": [\n        {\n          \"id\": \"0ff7fc6c-c196-4b02-b70e-ad7b3d01fe48\",\n          \"description\": \"Task 2\",\n          \"completed\": true,\n          \"createdAt\": \"2023-12-18T17:30:24.290Z\",\n          \"updatedAt\": \"2023-12-18T17:30:57.207Z\",\n          \"completedAt\": \"2023-12-18T17:30:57.207Z\"\n        },\n        {\n          \"id\": \"05cfab69-5047-44da-aa6a-adbd3d45c5db\",\n          \"description\": \"Task 1\",\n          \"completed\": false,\n          \"createdAt\": \"2023-12-18T17:30:21.886Z\"\n        }\n      ],\n      \"lastActive\": \"2023-12-18T17:30:57.207Z\",\n      \"sections\": [\n        {\n          \"id\": \"open-tasks\",\n          \"name\": \"Open\",\n          \"collapsed\": false\n        },\n        {\n          \"id\": \"completed-tasks\",\n          \"name\": \"Completed\",\n          \"collapsed\": false\n        }\n      ],\n      \"collapsed\": false\n    },\n    {\n      \"name\": \"Group 2\",\n      \"tasks\": [\n        {\n          \"id\": \"6e87a727-a761-48a4-90ca-f86bb962f294\",\n          \"description\": \"Task 5\",\n          \"completed\": false,\n          \"createdAt\": \"2023-12-18T17:44:22.646Z\"\n        },\n        {\n          \"id\": \"d55e6eda-3683-4226-8e4a-89d0eb4efa3a\",\n          \"description\": \"Task 3\",\n          \"completed\": true,\n          \"createdAt\": \"2023-12-18T17:30:36.184Z\",\n          \"updatedAt\": \"2023-12-18T17:30:52.523Z\",\n          \"completedAt\": \"2023-12-18T17:30:52.523Z\"\n        },\n        {\n          \"id\": \"02877033-e4d7-4cf6-9364-f2694acfa916\",\n          \"description\": \"Task 4\",\n          \"completed\": false,\n          \"createdAt\": \"2023-12-18T17:30:38.479Z\"\n        }\n      ],\n      \"lastActive\": \"2023-12-18T17:44:22.647Z\",\n      \"sections\": [\n        {\n          \"id\": \"open-tasks\",\n          \"name\": \"Open\",\n          \"collapsed\": false\n        },\n        {\n          \"id\": \"completed-tasks\",\n          \"name\": \"Completed\",\n          \"collapsed\": false\n        }\n      ],\n      \"collapsed\": false\n    }\n  ],\n  \"defaultSections\": [\n    {\n      \"id\": \"open-tasks\",\n      \"name\": \"Open\"\n    },\n    {\n      \"id\": \"completed-tasks\",\n      \"name\": \"Completed\"\n    }\n  ]\n}"`
	unquoted, err := strconv.Unquote(exampleChecklist)
	require.NoError(t, err)

	var checklist Checklist
	require.NoError(t, json.Unmarshal([]byte(unquoted), &checklist))

	checklist.Sort()

	require.NotNil(t, checklist.Groups)
	require.Len(t, checklist.Groups, 2)
	require.Equal(t, "1.0.0", checklist.SchemaVersion)
	require.Equal(t, "Group 2", checklist.Groups[0].Name)
	require.Equal(t, "Group 1", checklist.Groups[1].Name)
	require.Equal(t, "Task 2", checklist.Groups[1].Tasks[1].Description)
	require.True(t, checklist.Groups[1].Tasks[1].Completed)
	require.Equal(t, "Task 3", checklist.Groups[0].Tasks[2].Description)
	require.True(t, checklist.Groups[0].Tasks[2].Completed)

	// jc, _ := json.MarshalIndent(checklist, "", "  ")
	// fmt.Println(string(jc))
}
