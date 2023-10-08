package models

type TasksPageData struct {
	Tasks []Task
}

type Task struct {
	Name     string
	Active   bool
	Schedule string
}

type NewTaskFormData struct {
	Name     string `form:"task-name" validate:"required"`
	Schedule string `form:"task-schedule" validate:"required"`
}
