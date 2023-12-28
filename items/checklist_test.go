package items

import (
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestTasklistParsing(t *testing.T) {
	exampleChecklist := `- [ ] Task 4\n- [ ] Task 3\\n\n- [ ] Task 2\n- [x] Task 1`

	tasks, err := NoteTextToTasks(exampleChecklist)
	require.NoError(t, err)

	require.NotNil(t, tasks)
	require.Len(t, tasks, 4)
}

func TestChecklistToNoteTextAndBack(t *testing.T) {
	var tasklist Tasklist
	// note: unsorted before being transformed
	tasklist.UpdatedAt = time.UnixMicro(1703773168000000)
	nt := TasksToNoteText(Tasks{
		{
			Title:     "Task 1",
			Completed: true,
		},
		{
			Title:     "Task 2",
			Completed: false,
		},
		{
			Title:     "Task 3",
			Completed: true,
		},
		{
			Title:     "Task 4",
			Completed: false,
		},
	})

	require.Equal(t, "- [ ] Task 2\\n- [ ] Task 4\\n- [x] Task 1\\n- [x] Task 3", nt)
	require.NotEmpty(t, nt)
	require.Contains(t, nt, "Task 1")
	tasks, err := NoteTextToTasks(nt)
	require.NoError(t, err)
	require.Equal(t, "Task 2", tasks[0].Title)
	require.False(t, tasks[0].Completed)
	require.Equal(t, "Task 4", tasks[1].Title)
	require.False(t, tasks[1].Completed)
	require.Equal(t, "Task 1", tasks[2].Title)
	require.True(t, tasks[2].Completed)
	require.Equal(t, "Task 3", tasks[3].Title)
	require.True(t, tasks[3].Completed)
}

func TestChecklistToNoteText(t *testing.T) {
	// - [ ] Task 4\n- [ ] Task 3\\nTest\n- [ ] Task 2\n- [x] Task 1
	tasks, err := NoteTextToTasks(`- [ ] Task 4\n- [ ] Task 3\\nTest\n- [ ] Task 2\n- [x] Task 1`)
	require.NoError(t, err)
	require.Len(t, tasks, 4)
	require.False(t, tasks[0].Completed)
	require.False(t, tasks[1].Completed)
	require.False(t, tasks[2].Completed)
	require.True(t, tasks[3].Completed)
}

func TestNoteTextToChecklist(t *testing.T) {
	// - [ ] Task 4\n- [ ] Task 3\\nTest\n- [ ] Task 2\n- [x] Task 1
	tasks, err := NoteTextToTasks(`- [ ] Task 4\n- [ ] Task 3\\nTest\n- [ ] Task 2\n- [x] Task 1`)
	require.NoError(t, err)
	require.Len(t, tasks, 4)
	require.False(t, tasks[0].Completed)
	require.False(t, tasks[1].Completed)
	require.False(t, tasks[2].Completed)
	require.True(t, tasks[3].Completed)
}

//
// func TestAddChecklistTask(t *testing.T) {
// 	cl := Tasklist{
// 		Groups: []TasklistGroup{
// 			{
// 				Name: "Group 1",
// 				Tasks: []Task{
// 					{
// 						Id:          "0ff7fc6c-c196-4b02-b70e-ad7b3d01fe48",
// 						Description: "TasklistTask 1",
// 						Completed:   true,
// 						CreatedAt:   time.Now(),
// 						UpdatedAt:   time.Now(),
// 					},
// 				},
// 			},
// 		},
// 	}
//
// 	require.NoError(t, cl.AddTask("TasklistTask 2"))
// 	require.Len(t, cl.Groups[0].Tasks, 2)
// 	require.Equal(t, "TasklistTask 2", cl.Groups[0].Tasks[1].Description)
// }
//
// func TestCompleteChecklistTask(t *testing.T) {
// 	cl := Tasklist{
// 		Groups: []TasklistGroup{
// 			{
// 				Name: "Group 1",
// 				Tasks: []Task{
// 					{
// 						Id:          "0ff7fc6c-c196-4b02-b70e-ad7b3d01fe48",
// 						Description: "TasklistTask 1",
// 						Completed:   false,
// 						CreatedAt:   time.Now(),
// 						UpdatedAt:   time.Now(),
// 					},
// 					{
// 						Id:          "1ef7fc6c-c196-4b02-b70e-ad7b3d01fe48",
// 						Description: "TasklistTask 2",
// 						Completed:   false,
// 						CreatedAt:   time.Now(),
// 						UpdatedAt:   time.Now(),
// 					},
// 					{
// 						Id:          "6bf7fc6c-c196-4b02-b70e-ad7b3d01fe48",
// 						Description: "TasklistTask 3",
// 						Completed:   true,
// 						CreatedAt:   time.Now(),
// 						UpdatedAt:   time.Now(),
// 					},
// 				},
// 			},
// 			{
// 				Name: "Group 2",
// 				Tasks: []Task{
// 					{
// 						Id:          "0aa7fc6c-c196-4b02-b70e-ad7b3d01fe48",
// 						Description: "TasklistTask 4",
// 						Completed:   false,
// 						CreatedAt:   time.Now(),
// 						UpdatedAt:   time.Now(),
// 					},
// 					{
// 						Id:          "1aa7fc6c-c196-4b02-b70e-ad7b3d01fe48",
// 						Description: "TasklistTask 5",
// 						Completed:   false,
// 						CreatedAt:   time.Now(),
// 						UpdatedAt:   time.Now(),
// 					},
// 					{
// 						Id:          "6aa7fc6c-c196-4b02-b70e-ad7b3d01fe48",
// 						Description: "TasklistTask 6",
// 						Completed:   true,
// 						CreatedAt:   time.Now(),
// 						UpdatedAt:   time.Now(),
// 					},
// 				},
// 			},
// 		},
// 	}
//
// 	require.NoError(t, cl.CompleteTask("TasklistTask 2"))
// 	require.Len(t, cl.Groups, 2)
// 	require.Len(t, cl.Groups[0].Tasks, 3)
// 	require.False(t, cl.Groups[0].Tasks[0].Completed)
// 	require.True(t, cl.Groups[0].Tasks[1].Completed)
// 	require.True(t, cl.Groups[0].Tasks[2].Completed)
// }
