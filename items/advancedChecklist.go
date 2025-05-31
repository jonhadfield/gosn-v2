package items

import (
	"encoding/json"
	"errors"
	"sort"
	"strconv"
	"time"
)

const (
	AdvancedChecklistNoteType = "com.sncommunity.advanced-checklist"
	openTasksSectionID        = "open-tasks"
	openTasksSectionName      = "Open"
	completedTasksSectionID   = "completed-tasks"
	completedTasksSectionName = "Completed"
)

type AdvancedChecklistTasks []AdvancedChecklistTask

type AdvancedChecklistTask struct {
	Id          string    `json:"id"`
	Description string    `json:"description"`
	Completed   bool      `json:"completed"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type DefaultSection struct {
	Id   string `json:"id"`
	Name string `json:"name"`
}

type AdvancedChecklistSection struct {
	Id        string `json:"id"`
	Name      string `json:"name"`
	Collapsed bool   `json:"collapsed"`
}
type AdvancedChecklistGroup struct {
	Name       string                     `json:"name"`
	LastActive time.Time                  `json:"lastActive"`
	Sections   []AdvancedChecklistSection `json:"sections"`
	Tasks      AdvancedChecklistTasks     `json:"tasks"`
	Collapsed  bool                       `json:"collapsed"`
}
type AdvancedChecklist struct {
	UUID            string                   `json:"-"`
	Duplicates      []AdvancedChecklist      `json:"-"`
	Title           string                   `json:"-"`
	SchemaVersion   string                   `json:"schemaVersion"`
	Groups          []AdvancedChecklistGroup `json:"groups"`
	DefaultSections []DefaultSection         `json:"defaultSections"`
	UpdatedAt       time.Time                `json:"updatedAt"`
	Trashed         bool                     `json:"trashed"`
}

func NoteTextToAdvancedChecklist(text string, quoted bool) (cl AdvancedChecklist, err error) {
	if quoted {
		text, err = strconv.Unquote(text)
		if err != nil {
			return AdvancedChecklist{}, err
		}
	}

	err = json.Unmarshal([]byte(text), &cl)
	if err != nil {
		return AdvancedChecklist{}, err
	}

	// cl.Sort()

	return cl, nil
}

// func NoteTextToAdvancedChecklist(text string) (cl AdvancedChecklist, err error) {
// 	unquoted, err := strconv.Unquote(text)
// 	if err != nil {
// 		return AdvancedChecklist{}, err
// 	}
//
// 	err = json.Unmarshal([]byte(unquoted), &cl)
// 	if err != nil {
// 		return AdvancedChecklist{}, err
// 	}
//
// 	// cl.Sort()
//
// 	return cl, nil
// }

func AdvancedCheckListToNoteText(cl AdvancedChecklist) string {
	// mcl, err := json.MarshalIndent(cl, "", "  ")
	mcl, err := json.Marshal(cl)
	if err != nil {
		return ""
	}

	return string(mcl)
}

type AdvancedChecklists []AdvancedChecklist

func (c *AdvancedChecklist) AddTask(groupName, taskTitle string) error {
	// create new task
	newTask := AdvancedChecklistTask{
		Completed:   false,
		Id:          GenUUID(),
		Description: taskTitle,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// check group exists
	var groupFound bool
	if _, groupFound = c.GetGroup(groupName); !groupFound {
		// create new group
		c.Groups = append(c.Groups, AdvancedChecklistGroup{
			Name:       groupName,
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
			Tasks:     []AdvancedChecklistTask{newTask},
			Collapsed: false,
		})

		return nil
	}

	// find and update existing group
	for x := range c.Groups {
		if c.Groups[x].Name == groupName {
			c.Groups[x].Tasks = append([]AdvancedChecklistTask{newTask}, c.Groups[x].Tasks...)
			break
		}
	}

	return nil
}

func (c *AdvancedChecklist) CompleteTask(groupName, taskTitle string) error {
	// check group exists
	var groupFound, taskFound bool

	var groups []AdvancedChecklistGroup

	for _, g := range c.Groups {
		if g.Name != groupName {
			groups = append(groups, g)

			continue
		}

		groupFound = true

		var updatedTasks []AdvancedChecklistTask

		for _, t := range g.Tasks {
			if t.Description != taskTitle {
				updatedTasks = append(updatedTasks, t)

				continue
			}

			if t.Completed {
				return errors.New(taskAlreadyCompleted)
			}

			taskFound = true

			t.Completed = true

			updatedTasks = append(updatedTasks, t)
		}

		g.Tasks = updatedTasks

		groups = append(groups, g)
	}

	if !groupFound {
		return errGroupNotFound
	}

	if !taskFound {
		return errTaskNotFound
	}

	c.Groups = groups

	return nil
}

func (c *AdvancedChecklist) ReopenTask(groupName, taskTitle string) error {
	// check group exists
	var groupFound, taskFound bool

	var groups []AdvancedChecklistGroup

	for _, g := range c.Groups {
		if g.Name != groupName {
			groups = append(groups, g)

			continue
		}

		groupFound = true

		var updatedTasks []AdvancedChecklistTask

		for _, t := range g.Tasks {
			if t.Description != taskTitle {
				updatedTasks = append(updatedTasks, t)

				continue
			}

			taskFound = true

			t.Completed = false

			updatedTasks = append(updatedTasks, t)
		}

		g.Tasks = updatedTasks

		groups = append(groups, g)
	}

	if !groupFound {
		return errGroupNotFound
	}

	if !taskFound {
		return errTaskNotFound
	}

	c.Groups = groups

	return nil
}

func (c *AdvancedChecklist) DeleteTask(groupName, taskTitle string) error {
	// check group exists
	var groupFound, taskFound bool

	var groups []AdvancedChecklistGroup

	for _, g := range c.Groups {
		if g.Name != groupName {
			groups = append(groups, g)

			continue
		}

		groupFound = true

		var updatedTasks []AdvancedChecklistTask

		for _, t := range g.Tasks {
			if t.Description != taskTitle {
				updatedTasks = append(updatedTasks, t)

				continue
			}

			taskFound = true
		}

		g.Tasks = updatedTasks

		groups = append(groups, g)
	}

	if !groupFound {
		return errGroupNotFound
	}

	if !taskFound {
		return errTaskNotFound
	}

	c.Groups = groups

	return nil
}

func (c *AdvancedChecklist) AddGroup(groupName string) error {
	// check if group already exists
	for _, g := range c.Groups {
		if g.Name == groupName {
			return errors.New("group already exists")
		}
	}

	c.Groups = append(c.Groups, AdvancedChecklistGroup{
		Name:       groupName,
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
		Tasks:     AdvancedChecklistTasks{},
		Collapsed: false,
	})

	return nil
}

func (c *AdvancedChecklist) DeleteGroup(groupName string) error {
	// check group exists
	var groupFound bool

	var groups []AdvancedChecklistGroup

	for _, g := range c.Groups {
		if g.Name != groupName {
			groups = append(groups, g)

			continue
		}

		groupFound = true
	}

	if !groupFound {
		return errGroupNotFound
	}

	c.Groups = groups

	return nil
}

func (c *AdvancedChecklist) GetGroup(title string) (AdvancedChecklistGroup, bool) {
	// find group
	for _, g := range c.Groups {
		if g.Name == title {
			return g, true
		}
	}

	return AdvancedChecklistGroup{}, false
}

func (c *AdvancedChecklist) Sort() {
	// sort groups
	sort.Slice(c.Groups, func(i, j int) bool {
		return c.Groups[i].LastActive.Unix() > c.Groups[j].LastActive.Unix()
	})

	for x := range c.Groups {
		// sort group sections by name
		sort.Slice(c.Groups[x].Sections, func(i, j int) bool {
			return c.Groups[x].Sections[i].Name < c.Groups[x].Sections[j].Name
		})

		c.Groups[x].Tasks.Sort()
	}
}

func (ts *Tasks) FilterAdvancedChecklistTasks(tasks Tasks, completed bool) (filtered Tasks) {
	for _, task := range tasks {
		if task.Completed == completed {
			filtered = append(filtered, task)
		}
	}

	return filtered
}

func (cs AdvancedChecklists) Sort() {
	// sort checklists by last-updated descending
	sort.Slice(cs, func(i, j int) bool {
		return cs[i].UpdatedAt.Unix() > cs[j].UpdatedAt.Unix()
	})
}

func (t *AdvancedChecklistTasks) Sort() {
	// sort tasks by updated date descending
	sort.Slice(*t, func(i, j int) bool {
		dt := *t
		return dt[i].UpdatedAt.Unix() > dt[j].UpdatedAt.Unix()
	})
}
