package items

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestTasklistParsing(t *testing.T) {
	t.Parallel()

	exampleChecklist := `- [ ] Task 4\n- [ ] Task 3\\n\n- [ ] Task 2\n- [x] Task 1`

	tasks, err := NoteTextToTasks(exampleChecklist)
	require.NoError(t, err)

	require.NotNil(t, tasks)
	require.Len(t, tasks, 4)
}

func TestChecklistToNoteTextAndBack(t *testing.T) {
	t.Parallel()

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

	require.Equal(t, "- [ ] Task 2\n- [ ] Task 4\n- [x] Task 1\n- [x] Task 3", nt)
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
	t.Parallel()

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
	t.Parallel()

	// - [ ] Task 4\n- [ ] Task 3\\nTest\n- [ ] Task 2\n- [x] Task 1
	tasks, err := NoteTextToTasks(`- [ ] Task 4\n- [ ] Task 3\\nTest\n- [ ] Task 2\n- [x] Task 1`)
	require.NoError(t, err)
	require.Len(t, tasks, 4)
	require.False(t, tasks[0].Completed)
	require.False(t, tasks[1].Completed)
	require.False(t, tasks[2].Completed)
	require.True(t, tasks[3].Completed)
}

func TestNoteTextToChecklist2(t *testing.T) {
	t.Parallel()

	tasks, err := NoteTextToTasks(`- [ ] here is task two\n- [x] here is task one`)
	require.NoError(t, err)
	require.Len(t, tasks, 2)
	require.Equal(t, "here is task two", tasks[0].Title)
	require.False(t, tasks[0].Completed)
	require.Equal(t, "here is task one", tasks[1].Title)
	require.True(t, tasks[1].Completed)

	tasks, err = NoteTextToTasks(``)
	require.Empty(t, tasks)
}

func TestAddChecklistTask(t *testing.T) {
	t.Parallel()

	updateTime := time.Now()

	cl := Tasklist{
		UUID:       "7eacf350-f4ce-44dd-8525-2457b19047dd",
		Duplicates: nil,
		Title:      "Test List",
		Tasks: []Task{
			{
				Title:     "Task One",
				Completed: false,
			},
			{
				Title:     "Task Two",
				Completed: false,
			},
		},
		UpdatedAt: updateTime,
		Trashed:   false,
	}

	require.NoError(t, cl.AddTask("Task Three"))
	require.Len(t, cl.Tasks, 3)
	require.Equal(t, "Task Three", cl.Tasks[0].Title)
	require.Equal(t, "Task One", cl.Tasks[1].Title)
	require.Equal(t, "Task Two", cl.Tasks[2].Title)
}

func TestReopenTask(t *testing.T) {
	t.Parallel()

	updateTime := time.Now()

	cl := Tasklist{
		UUID:       "7eacf350-f4ce-44dd-8525-2457b19047dd",
		Duplicates: nil,
		Title:      "Test List",
		Tasks: []Task{
			{
				Title:     "Task One",
				Completed: true,
			},
			{
				Title:     "Task Two",
				Completed: false,
			},
		},
		UpdatedAt: updateTime,
		Trashed:   false,
	}

	require.NoError(t, cl.ReopenTask("Task One"))
	require.Len(t, cl.Tasks, 2)
	require.Equal(t, "Task One", cl.Tasks[0].Title)
	require.Equal(t, "Task Two", cl.Tasks[1].Title)
}

func TestCompleteTask(t *testing.T) {
	t.Parallel()

	updateTime := time.Now()

	cl := Tasklist{
		UUID:       "7eacf350-f4ce-44dd-8525-2457b19047dd",
		Duplicates: nil,
		Title:      "Test List",
		Tasks: []Task{
			{
				Title:     "Task One",
				Completed: false,
			},
			{
				Title:     "Task Two",
				Completed: false,
			},
		},
		UpdatedAt: updateTime,
		Trashed:   false,
	}

	require.NoError(t, cl.CompleteTask("Task One"))
	require.Len(t, cl.Tasks, 2)
	require.Equal(t, "Task One", cl.Tasks[0].Title)
	require.True(t, cl.Tasks[0].Completed)
	require.Equal(t, "Task Two", cl.Tasks[1].Title)
	require.False(t, cl.Tasks[1].Completed)

	err := cl.CompleteTask("Task One")
	require.Error(t, err)
	require.ErrorContains(t, err, taskAlreadyCompleted)
}

func TestDeleteTask(t *testing.T) {
	t.Parallel()

	updateTime := time.Now()

	cl := Tasklist{
		UUID:       "7eacf350-f4ce-44dd-8525-2457b19047dd",
		Duplicates: nil,
		Title:      "Test List",
		Tasks: []Task{
			{
				Title:     "Task One",
				Completed: false,
			},
			{
				Title:     "Task Two",
				Completed: false,
			},
		},
		UpdatedAt: updateTime,
		Trashed:   false,
	}

	require.NoError(t, cl.DeleteTask("Task One"))
	require.Len(t, cl.Tasks, 1)
	require.Equal(t, "Task Two", cl.Tasks[0].Title)
	require.False(t, cl.Tasks[0].Completed)
}
