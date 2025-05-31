package items

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestActivityAdvancedChecklistParsing(t *testing.T) {
	exampleChecklist := `"{\n  \"schemaVersion\": \"1.0.0\",\n  \"groups\": [\n    {\n      \"name\": \"Group 1\",\n      \"tasks\": [\n        {\n          \"id\": \"0ff7fc6c-c196-4b02-b70e-ad7b3d01fe48\",\n          \"description\": \"AdvancedChecklistTask 2\",\n          \"completed\": true,\n          \"createdAt\": \"2023-12-18T17:30:24.290Z\",\n          \"updatedAt\": \"2023-12-18T17:30:57.207Z\",\n          \"completedAt\": \"2023-12-18T17:30:57.207Z\"\n        },\n        {\n          \"id\": \"05cfab69-5047-44da-aa6a-adbd3d45c5db\",\n          \"description\": \"AdvancedChecklistTask 1\",\n          \"completed\": false,\n          \"createdAt\": \"2023-12-18T17:30:21.886Z\"\n        }\n      ],\n      \"lastActive\": \"2023-12-18T17:30:57.207Z\",\n      \"sections\": [\n        {\n          \"id\": \"open-tasks\",\n          \"name\": \"Open\",\n          \"collapsed\": false\n        },\n        {\n          \"id\": \"completed-tasks\",\n          \"name\": \"Completed\",\n          \"collapsed\": false\n        }\n      ],\n      \"collapsed\": false\n    },\n    {\n      \"name\": \"Group 2\",\n      \"tasks\": [\n        {\n          \"id\": \"6e87a727-a761-48a4-90ca-f86bb962f294\",\n          \"description\": \"AdvancedChecklistTask 5\",\n          \"completed\": false,\n          \"createdAt\": \"2023-12-18T17:44:22.646Z\"\n        },\n        {\n          \"id\": \"d55e6eda-3683-4226-8e4a-89d0eb4efa3a\",\n          \"description\": \"AdvancedChecklistTask 3\",\n          \"completed\": true,\n          \"createdAt\": \"2023-12-18T17:30:36.184Z\",\n          \"updatedAt\": \"2023-12-18T17:30:52.523Z\",\n          \"completedAt\": \"2023-12-18T17:30:52.523Z\"\n        },\n        {\n          \"id\": \"02877033-e4d7-4cf6-9364-f2694acfa916\",\n          \"description\": \"AdvancedChecklistTask 4\",\n          \"completed\": false,\n          \"createdAt\": \"2023-12-18T17:30:38.479Z\"\n        }\n      ],\n      \"lastActive\": \"2023-12-18T17:44:22.647Z\",\n      \"sections\": [\n        {\n          \"id\": \"open-tasks\",\n          \"name\": \"Open\",\n          \"collapsed\": false\n        },\n        {\n          \"id\": \"completed-tasks\",\n          \"name\": \"Completed\",\n          \"collapsed\": false\n        }\n      ],\n      \"collapsed\": false\n    }\n  ],\n  \"defaultSections\": [\n    {\n      \"id\": \"open-tasks\",\n      \"name\": \"Open\"\n    },\n    {\n      \"id\": \"completed-tasks\",\n      \"name\": \"Completed\"\n    }\n  ]\n}"`

	checklist, err := NoteTextToAdvancedChecklist(exampleChecklist, true)
	require.NoError(t, err)

	require.NotNil(t, checklist.Groups)
	require.Len(t, checklist.Groups, 2)
	require.Equal(t, "1.0.0", checklist.SchemaVersion)
	require.Equal(t, "Group 1", checklist.Groups[0].Name)
	require.Equal(t, "Group 2", checklist.Groups[1].Name)
	require.Equal(t, "AdvancedChecklistTask 3", checklist.Groups[1].Tasks[1].Description)
	require.Equal(t, "d55e6eda-3683-4226-8e4a-89d0eb4efa3a", checklist.Groups[1].Tasks[1].Id)
	require.True(t, checklist.Groups[1].Tasks[1].Completed)
	require.Equal(t, "AdvancedChecklistTask 4", checklist.Groups[1].Tasks[2].Description)
	require.False(t, checklist.Groups[1].Tasks[2].Completed)
}

func TestSortAdvancedChecklist(t *testing.T) {
	var checklist AdvancedChecklist
	checklist.SchemaVersion = "1.0.0"
	oneDayAgo := time.Now().Add(-time.Hour * 24 * 1)
	twoDaysAgo := time.Now().Add(-time.Hour * 24 * 2)
	threeDaysAgo := time.Now().Add(-time.Hour * 24 * 3)
	fourDaysAgo := time.Now().Add(-time.Hour * 24 * 4)

	checklist.Groups = []AdvancedChecklistGroup{
		{
			Name:       "Group 1",
			LastActive: twoDaysAgo,
			Tasks: []AdvancedChecklistTask{
				{
					Id:          "0ff7fc6c-c196-4b02-b70e-ad7b3d01fe48",
					Description: "AdvancedChecklistTask 1",
					UpdatedAt:   twoDaysAgo,
					Completed:   true,
				},
				{
					Id:          "05cfab69-5047-44da-aa6a-adbd3d45c5db",
					Description: "AdvancedChecklistTask 2",
					UpdatedAt:   threeDaysAgo,
					Completed:   false,
				},
			},
		},
		{
			Name:       "Group 2",
			LastActive: oneDayAgo,
			Tasks: []AdvancedChecklistTask{
				{
					Id:          "6e87a727-a761-48a4-90ca-f86bb962f294",
					Description: "AdvancedChecklistTask 3",
					UpdatedAt:   fourDaysAgo,
					Completed:   true,
				},
				{
					Id:          "d55e6eda-3683-4226-8e4a-89d0eb4efa3a",
					Description: "AdvancedChecklistTask 4",
					UpdatedAt:   oneDayAgo,
					Completed:   false,
				},
			},
		},
	}

	checklist.Sort()

	require.Equal(t, "Group 2", checklist.Groups[0].Name)
	require.Equal(t, "AdvancedChecklistTask 4", checklist.Groups[0].Tasks[0].Description)
	require.Equal(t, "AdvancedChecklistTask 3", checklist.Groups[0].Tasks[1].Description)
	require.Equal(t, "Group 1", checklist.Groups[1].Name)
	require.Equal(t, "AdvancedChecklistTask 1", checklist.Groups[1].Tasks[0].Description)
	require.Equal(t, "AdvancedChecklistTask 2", checklist.Groups[1].Tasks[1].Description)
}

func TestSortAdvancedChecklistTasks(t *testing.T) {
	var checklist AdvancedChecklist
	checklist.SchemaVersion = "1.0.0"
	oneDayAgo := time.Now().Add(-time.Hour * 24 * 1)
	twoDaysAgo := time.Now().Add(-time.Hour * 24 * 2)
	threeDaysAgo := time.Now().Add(-time.Hour * 24 * 3)
	fourDaysAgo := time.Now().Add(-time.Hour * 24 * 4)

	tasks := AdvancedChecklistTasks{
		{
			Id:          "0ff7fc6c-c196-4b02-b70e-ad7b3d01fe48",
			Description: "AdvancedChecklistTask 1",
			UpdatedAt:   twoDaysAgo,
			Completed:   true,
		},
		{
			Id:          "05cfab69-5047-44da-aa6a-adbd3d45c5db",
			Description: "AdvancedChecklistTask 2",
			UpdatedAt:   threeDaysAgo,
			Completed:   false,
		},
		{
			Id:          "6e87a727-a761-48a4-90ca-f86bb962f294",
			Description: "AdvancedChecklistTask 3",
			UpdatedAt:   fourDaysAgo,
			Completed:   true,
		},
		{
			Id:          "d55e6eda-3683-4226-8e4a-89d0eb4efa3a",
			Description: "AdvancedChecklistTask 4",
			UpdatedAt:   oneDayAgo,
			Completed:   false,
		},
	}

	tasks.Sort()

	require.Equal(t, "AdvancedChecklistTask 4", tasks[0].Description)
	require.Equal(t, "AdvancedChecklistTask 1", tasks[1].Description)
	require.Equal(t, "AdvancedChecklistTask 2", tasks[2].Description)
	require.Equal(t, "AdvancedChecklistTask 3", tasks[3].Description)
}

func TestAdvancedChecklistToNoteText(t *testing.T) {
	var checklist AdvancedChecklist
	checklist.SchemaVersion = "1.0.0"
	checklist.DefaultSections = []DefaultSection{
		{
			Id:   openTasksSectionID,
			Name: openTasksSectionName,
		},
		{
			Id:   completedTasksSectionID,
			Name: completedTasksSectionName,
		},
	}
	checklist.Groups = []AdvancedChecklistGroup{
		{
			Name:       "Group 1",
			LastActive: time.Now(),
			Sections: []AdvancedChecklistSection{
				{
					Id:        openTasksSectionID,
					Name:      openTasksSectionName,
					Collapsed: false,
				},
				{
					Id:        completedTasksSectionID,
					Name:      completedTasksSectionName,
					Collapsed: false,
				},
			},
			Tasks: []AdvancedChecklistTask{
				{
					Id:          "0ff7fc6c-c196-4b02-b70e-ad7b3d01fe48",
					Description: "AdvancedChecklistTask 1",
					Completed:   true,
				},
				{
					Id:          "05cfab69-5047-44da-aa6a-adbd3d45c5db",
					Description: "AdvancedChecklistTask 2",
					Completed:   false,
				},
			},
			Collapsed: false,
		},
		{
			Name:       "Group 2",
			LastActive: time.Now(),
			Sections: []AdvancedChecklistSection{
				{
					Id:        openTasksSectionID,
					Name:      openTasksSectionName,
					Collapsed: false,
				},
				{
					Id:        completedTasksSectionID,
					Name:      completedTasksSectionName,
					Collapsed: false,
				},
			},
			Tasks: []AdvancedChecklistTask{
				{
					Id:          "6e87a727-a761-48a4-90ca-f86bb962f294",
					Description: "AdvancedChecklistTask 3",
					Completed:   true,
					CreatedAt:   time.Now(),
					UpdatedAt:   time.Now(),
				},
				{
					Id:          "d55e6eda-3683-4226-8e4a-89d0eb4efa3a",
					Description: "AdvancedChecklistTask 4",
					Completed:   false,
					CreatedAt:   time.Now(),
					UpdatedAt:   time.Now(),
				},
			},
			Collapsed: false,
		},
	}

	nt := AdvancedCheckListToNoteText(checklist)
	require.NotEmpty(t, nt)
	require.Contains(t, nt, "AdvancedChecklistTask 1")
	require.Contains(t, nt, "1.0.0")

	nChecklist, err := NoteTextToAdvancedChecklist(nt, false)
	require.NoError(t, err)
	require.Equal(t, "1.0.0", nChecklist.SchemaVersion)
}

func TestAddAdvancedChecklistTask(t *testing.T) {
	cl := AdvancedChecklist{
		Groups: []AdvancedChecklistGroup{
			{
				Name: "Group 1",
				Tasks: []AdvancedChecklistTask{
					{
						Id:          "0ff7fc6c-c196-4b02-b70e-ad7b3d01fe48",
						Description: "AdvancedChecklistTask 1",
						Completed:   true,
						CreatedAt:   time.Now(),
						UpdatedAt:   time.Now(),
					},
				},
			},
		},
	}

	require.NoError(t, cl.AddTask("Group 1", "AdvancedChecklistTask 2"))
	require.Len(t, cl.Groups[0].Tasks, 2)
	require.Equal(t, "AdvancedChecklistTask 2", cl.Groups[0].Tasks[0].Description)
	// create group if not found
	require.NoError(t, cl.AddTask("Group 69", "AdvancedChecklistTask 2"))
	// check exists, with task
	var foundGroup, foundTask bool

	for x := range cl.Groups {
		if cl.Groups[x].Name == "Group 69" {
			foundGroup = true

			for y := range cl.Groups[x].Tasks {
				if cl.Groups[x].Tasks[y].Description == "AdvancedChecklistTask 2" {
					foundTask = true
				}
			}
		}
	}

	require.True(t, foundGroup)
	require.True(t, foundTask)
}

func TestCompleteAdvancedChecklistTask(t *testing.T) {
	cl := testAdvancedChecklist()

	require.NoError(t, cl.CompleteTask("Group 1", "Task 2"))
	require.Len(t, cl.Groups, 2)
	require.Len(t, cl.Groups[0].Tasks, 3)
	require.False(t, cl.Groups[0].Tasks[0].Completed)
	require.True(t, cl.Groups[0].Tasks[1].Completed)
	require.True(t, cl.Groups[0].Tasks[2].Completed)
}

func TestDeleteAdvancedChecklistTask(t *testing.T) {
	cl := testAdvancedChecklist()

	require.NoError(t, cl.DeleteTask("Group 1", "Task 2"))
	require.Len(t, cl.Groups, 2)
	require.Len(t, cl.Groups[0].Tasks, 2)
	require.Len(t, cl.Groups[1].Tasks, 3)
	require.False(t, cl.Groups[0].Tasks[0].Completed)
	require.True(t, cl.Groups[0].Tasks[1].Completed)
	require.False(t, cl.Groups[1].Tasks[0].Completed)
	require.False(t, cl.Groups[1].Tasks[1].Completed)
	require.True(t, cl.Groups[1].Tasks[2].Completed)
}

func TestReopenAdvancedChecklistTask(t *testing.T) {
	cl := testAdvancedChecklist()

	require.NoError(t, cl.ReopenTask("Group 2", "Task 6"))
	require.Len(t, cl.Groups, 2)
	require.Len(t, cl.Groups[0].Tasks, 3)
	require.Len(t, cl.Groups[1].Tasks, 3)
	require.False(t, cl.Groups[0].Tasks[0].Completed)
	require.False(t, cl.Groups[0].Tasks[1].Completed)
	require.True(t, cl.Groups[0].Tasks[2].Completed)
	require.False(t, cl.Groups[1].Tasks[0].Completed)
	require.False(t, cl.Groups[1].Tasks[1].Completed)
	require.False(t, cl.Groups[1].Tasks[2].Completed)

	// missing group
	err := cl.ReopenTask("Group 3", "Task 6")
	require.Error(t, err)
	require.ErrorIs(t, err, errGroupNotFound)

	// missing task
	err = cl.ReopenTask("Group 2", "Task 7")
	require.Error(t, err)
	require.ErrorIs(t, err, errTaskNotFound)
}

func testAdvancedChecklist() AdvancedChecklist {
	return AdvancedChecklist{
		Groups: []AdvancedChecklistGroup{
			{
				Name: "Group 1",
				Tasks: []AdvancedChecklistTask{
					{
						Id:          "0ff7fc6c-c196-4b02-b70e-ad7b3d01fe48",
						Description: "Task 1",
						Completed:   false,
						CreatedAt:   time.Now(),
						UpdatedAt:   time.Now(),
					},
					{
						Id:          "1ef7fc6c-c196-4b02-b70e-ad7b3d01fe48",
						Description: "Task 2",
						Completed:   false,
						CreatedAt:   time.Now(),
						UpdatedAt:   time.Now(),
					},
					{
						Id:          "6bf7fc6c-c196-4b02-b70e-ad7b3d01fe48",
						Description: "Task 3",
						Completed:   true,
						CreatedAt:   time.Now(),
						UpdatedAt:   time.Now(),
					},
				},
			},
			{
				Name: "Group 2",
				Tasks: []AdvancedChecklistTask{
					{
						Id:          "0aa7fc6c-c196-4b02-b70e-ad7b3d01fe48",
						Description: "Task 4",
						Completed:   false,
						CreatedAt:   time.Now(),
						UpdatedAt:   time.Now(),
					},
					{
						Id:          "1aa7fc6c-c196-4b02-b70e-ad7b3d01fe48",
						Description: "Task 5",
						Completed:   false,
						CreatedAt:   time.Now(),
						UpdatedAt:   time.Now(),
					},
					{
						Id:          "6aa7fc6c-c196-4b02-b70e-ad7b3d01fe48",
						Description: "Task 6",
						Completed:   true,
						CreatedAt:   time.Now(),
						UpdatedAt:   time.Now(),
					},
				},
			},
		},
	}
}
