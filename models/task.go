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
		Active:   task.Active,
		Schedule: task.Schedule,
	}

	if task.ActivatedTime != nil {
		taskDuration, _ := utils.ParseDuration(task.Schedule)
		remainingTime := utils.CalculateRemainingTime(task.ActivatedTime, taskDuration)
		viewTask.ActivatedTime = task.ActivatedTime.String()
		viewTask.RemainingTime = remainingTime.String()
		viewTask.IsSoon = remainingTime.Seconds() < 60
	}

	return viewTask
}

type Task struct {
	Id            string     `json:"id"`
	Name          string     `json:"name"`
	Active        bool       `json:"active"`
	Schedule      string     `json:"schedule"`
	ActivatedTime *time.Time `json:"activatedTime"` // optional
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

		taskDurationA, _ := utils.ParseDuration(tasks[i].Schedule)
		remainingTimeA := *utils.CalculateRemainingTime(timeA, taskDurationA)
		taskDurationB, _ := utils.ParseDuration(tasks[j].Schedule)
		remainingTimeB := *utils.CalculateRemainingTime(timeB, taskDurationB)

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
