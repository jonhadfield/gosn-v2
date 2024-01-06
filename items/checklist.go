package items

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

const (
	SimpleTaskEditorNoteType = "org.standardnotes.simple-task-editor"
	taskAlreadyCompleted     = "task already complete"
	taskNotOpen              = "task not open"
	taskAlreadyOpen          = "task already open"
)

var (
	errMissingContent = errors.New("missing content")
	errGroupNotFound  = errors.New("group not found")
	errTaskNotFound   = errors.New("task not found")
)

type Tasks []Task

func splitTaskText(text string) []string {
	// placeholder for escaped newlines
	placeholder := "ESC_NEWLINE_PLACEHOLDER"

	// replace "\\n" with the placeholder
	escapedNewlineReplaced := strings.ReplaceAll(text, `\\n`, placeholder)

	// split by "\n"
	parts := strings.Split(escapedNewlineReplaced, `\n`)

	// Replace the placeholder back to "\\n" in each part
	for i, part := range parts {
		parts[i] = strings.ReplaceAll(part, placeholder, `\\n`)
	}

	return parts
}

type Task struct {
	Title     string `json:"title"`
	Completed bool   `json:"completed"`
}

type Tasklist struct {
	UUID       string     `json:"-"`
	Duplicates []Tasklist `json:"-"`
	Title      string     `json:"-"`
	Tasks      []Task
	UpdatedAt  time.Time `json:"updatedAt"`
	Trashed    bool      `json:"trashed"`
}

func NoteTextToTasks(text string) (tasks Tasks, err error) {
	if len(text) == 0 {
		return Tasks{}, errMissingContent
	}

	var matches []string

	// support both formats
	if strings.Contains(text, "\n") {
		matches = strings.Split(text, "\n")
	} else {
		matches = splitTaskText(text)
	}

	for _, match := range matches {
		i := strings.Index(match, "]")
		task := Task{
			Title: match[i+2:],
		}

		if match[i-1:i] == "x" {
			task.Completed = true
		}

		tasks = append(tasks, task)
	}

	// put completed at bottom
	tasks.Sort()

	return tasks, nil
}

func (ts *Tasks) Sort() {
	dt := *ts
	sort.Slice(dt, func(i, j int) bool {
		return !dt[i].Completed && dt[j].Completed
	})

	*ts = dt
}

func TasksToNoteText(tasks Tasks) string {
	text := strings.Builder{}
	// put completed at bottom
	tasks.Sort()

	for x, t := range tasks {
		completed := " "
		if t.Completed {
			completed = "x"
		}

		sep := "\n"
		if x == len(tasks)-1 {
			sep = ""
		}

		text.WriteString("- [" + completed + "] " + t.Title + sep)
	}

	return text.String()
}

type Tasklists []Tasklist

func (c *Tasklist) AddTask(taskTitle string) error {
	// create new task
	newTask := Task{
		Title:     taskTitle,
		Completed: false,
	}

	c.Tasks = append([]Task{newTask}, c.Tasks...)

	return nil
}

func (c *Tasklist) CompleteTask(taskTitle string) error {
	// check group exists
	var taskFound bool

	var updatedTasks []Task

	for _, t := range c.Tasks {
		if t.Title == taskTitle {
			if t.Completed {
				return fmt.Errorf(taskAlreadyCompleted)
			}

			taskFound = true
			t.Completed = true
			updatedTasks = append(updatedTasks, t)

			continue
		}

		updatedTasks = append(updatedTasks, t)
	}

	if !taskFound {
		return errTaskNotFound
	}

	c.Tasks = updatedTasks

	return nil
}

func (c *Tasklist) ReopenTask(taskTitle string) error {
	// check group exists
	var taskFound bool

	var updatedTasks []Task

	for _, t := range c.Tasks {
		if t.Title == taskTitle {
			if !t.Completed {
				return fmt.Errorf(taskAlreadyOpen)
			}

			taskFound = true
			t.Completed = false
			updatedTasks = append(updatedTasks, t)

			continue
		}

		updatedTasks = append(updatedTasks, t)
	}

	if !taskFound {
		return errTaskNotFound
	}

	c.Tasks = updatedTasks

	return nil
}

func (c *Tasklist) removeTask(taskTitle string) ([]Task, bool) {
	var taskFound bool

	var updatedTasks []Task

	for _, task := range c.Tasks {
		if task.Title == taskTitle {
			taskFound = true

			continue
		}

		updatedTasks = append(updatedTasks, task)
	}

	return updatedTasks, taskFound
}

func (c *Tasklist) DeleteTask(taskTitle string) error {
	// check group exists
	updatedTasks, taskFound := c.removeTask(taskTitle)

	if !taskFound {
		return errTaskNotFound
	}

	c.Tasks = updatedTasks

	return nil
}

//
// func (c *Tasklist) Sort() {
// 	// sort groups
// 	sort.Slice(c.Groups, func(i, j int) bool {
// 		return c.Groups[i].LastActive.Unix() > c.Groups[j].LastActive.Unix()
// 	})
//
// 	for x := range c.Groups {
// 		// sort group sections by name
// 		sort.Slice(c.Groups[x].Sections, func(i, j int) bool {
// 			return c.Groups[x].Sections[i].Name < c.Groups[x].Sections[j].Name
// 		})
//
// 		c.Groups[x].Tasks.Sort()
// 	}
// }
//
// func (cs Tasklists) Sort() {
// 	// sort checklists by last-updated descending
// 	sort.Slice(cs, func(i, j int) bool {
// 		return cs[i].UpdatedAt.Unix() > cs[j].UpdatedAt.Unix()
// 	})
// }
//
// func (t *Tasks) Sort() {
// 	// sort tasks by updated date descending
// 	sort.Slice(*t, func(i, j int) bool {
// 		dt := *t
// 		return dt[i].UpdatedAt.Unix() > dt[j].UpdatedAt.Unix()
// 	})
// }
