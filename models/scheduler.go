package models

type Scheduler struct {
	Author string
	Tasks  []*Task `json:"tasks"`
}
