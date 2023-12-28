package items

import (
	"fmt"
	"go.elara.ws/pcre"
	"sort"
	"strings"
	"time"
)

type Tasks []Task

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
	var matches []string

	r := pcre.MustCompile(`(?<!\\)\\n`)
	matches = r.Split(text, -1)

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

func (tasks *Tasks) Sort() {
	dt := *tasks
	sort.Slice(dt, func(i, j int) bool {
		return !dt[i].Completed && dt[j].Completed
	})

	*tasks = dt
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

		sep := "\\n"
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

	c.Tasks = append(c.Tasks, newTask)

	return nil
}

func (c *Tasklist) CompleteTask(taskTitle string) error {
	// check group exists
	var taskFound bool

	var updatedTasks []Task

	for _, t := range c.Tasks {
		taskFound = true
		t.Completed = true
		updatedTasks = append(updatedTasks, t)
	}

	c.Tasks = updatedTasks

	if !taskFound {
		return fmt.Errorf("task not found")
	}

	return nil
}

func (c *Tasklist) DeleteTask(taskTitle string) error {
	// check group exists
	var taskFound bool

	var updatedTasks []Task

	for _, g := range c.Tasks {
		if g.Title != taskTitle {
			updatedTasks = append(updatedTasks, g)
		}

		c.Tasks = updatedTasks
	}

	if !taskFound {
		return fmt.Errorf("task not found")
	}

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
