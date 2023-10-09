package models

import "time"

type TasksPageData struct {
	Tasks []Task
}

type TasksUpdateData struct {
	Tasks []Task
}

type Task struct {
	Name          string
	Active        bool
	Schedule      string
	RemainingTime *time.Duration
}

type NewTaskFormData struct {
	Name     string `form:"task-name" validate:"required"`
	Schedule string `form:"task-schedule" validate:"required"`
}
