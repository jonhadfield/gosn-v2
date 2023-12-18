package items

import "time"

type ChecklistTask struct {
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

type ChecklistSection struct {
	Id        string `json:"id"`
	Name      string `json:"name"`
	Collapsed bool   `json:"collapsed"`
}
type ChecklistGroup struct {
	Name       string             `json:"name"`
	LastActive time.Time          `json:"lastActive"`
	Sections   []ChecklistSection `json:"sections"`
	Tasks      []ChecklistTask    `json:"tasks"`
	Collapsed  bool               `json:"collapsed"`
}
type Checklist struct {
	Title           string
	SchemaVersion   string           `json:"schemaVersion"`
	Groups          []ChecklistGroup `json:"groups"`
	DefaultSections []DefaultSection `json:"defaultSections"`
}
type Checklists []Checklist
