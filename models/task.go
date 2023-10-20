package models

import (
	"scheduler/utils"
	"sort"
	"time"
)

type TasksPageData struct {
	Tasks []*TaskVM
}

type TasksUpdateData struct {
	Tasks []*TaskVM
}

// TaskVM is the Task View Model
type TaskVM struct {
	Id            string
	Name          string
	Active        bool
	Schedule      string
	IsSoon        bool
	RemainingTime string
	ActivatedTime string
}

func (task *Task) ToTaskVM() *TaskVM {
	viewTask := &TaskVM{
		Id:       task.Id,
		Name:     task.Name,
		Active:   task.IsActive(),
		Schedule: task.Schedule,
	}

	if task.IsActive() {
		viewTask.ActivatedTime = task.ActivatedTime.String()
		remainingTime := task.GetRemainingTime()
		viewTask.RemainingTime = task.GetRemainingTime().String()
		viewTask.IsSoon = remainingTime.Seconds() < 60
	}

	return viewTask
}

type Task struct {
	Id            string     `json:"id"`
	Name          string     `json:"name"`
	Schedule      string     `json:"schedule"`
	ActivatedTime *time.Time `json:"activatedTime"` // optional
}

func (task *Task) GetRemainingTime() *time.Duration {
	taskDuration, _ := utils.ParseDuration(task.Schedule)
	return utils.CalculateRemainingTime(task.ActivatedTime, taskDuration)
}

func (task *Task) IsActive() bool {
	if task.ActivatedTime == nil {
		return false
	}

	remaining := task.GetRemainingTime()
	return remaining.Seconds() > 0
}

type NewTaskFormData struct {
	Name     string `form:"task-name" validate:"required"`
	Schedule string `form:"task-schedule" validate:"required"`
}

type ActivateTaskFormData struct {
	TaskIds []string `form:"task-ids" validate:"required"`
}

func GetViewTasks(tasks []*Task) []*TaskVM {
	var viewTasks []*TaskVM

	for _, task := range tasks {
		viewTasks = append(viewTasks, task.ToTaskVM())
	}

	return viewTasks
}

func SortTasks(tasks []*Task) {
	sort.Slice(tasks, func(i, j int) bool {
		timeA := tasks[i].ActivatedTime
		timeB := tasks[j].ActivatedTime

		if timeA == nil && timeB == nil {
			return false
		}
		if timeA == nil {
			return false
		}
		if timeB == nil {
			return true
		}

		remainingTimeA := *tasks[i].GetRemainingTime()
		remainingTimeB := *tasks[j].GetRemainingTime()

		if remainingTimeA <= 0 {
			if remainingTimeB <= 0 {
				// if both remainingTimes are <= 0, sort them based on secondary criteria
				return tasks[i].Name < tasks[j].Name
			}
			return false
		}

		if remainingTimeB <= 0 {
			return true
		}

		return remainingTimeA < remainingTimeB
	})
}
