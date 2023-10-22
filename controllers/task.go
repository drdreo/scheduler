package controllers

import (
	"encoding/json"
	"github.com/rs/zerolog/log"
	"html/template"
	"net/http"
	"os"
	"scheduler/models"
	"scheduler/utils"
	"time"

	"github.com/gin-gonic/gin"
)

const useDB = true

type TaskController struct {
	template *template.Template
	// ... add fields like database connection or services here.
	sc           *StreamController
	taskDBM      *models.TaskDBModel
	taskRegistry map[string]*time.Timer
}

func NewTaskController(streamController *StreamController, template *template.Template, taskDBM *models.TaskDBModel) *TaskController {
	return &TaskController{
		template:     template,
		sc:           streamController,
		taskDBM:      taskDBM,
		taskRegistry: make(map[string]*time.Timer),
	}
}

func (tc *TaskController) GetTasks(c *gin.Context) {
	tasks := tc.getTasks()
	checkExpiredTasks(tasks)
	err := tc.writeTasksData(tasks)
	if err != nil {
		log.Fatal().Msg("Could not write tasks data")
	}

	viewTasks := models.GetViewTasks(tasks)

	c.HTML(http.StatusOK, "pages/tasks", models.TasksPageData{
		Tasks: viewTasks,
	})
}

func (tc *TaskController) GetNewTaskForm(c *gin.Context) {
	c.HTML(http.StatusOK, "tasks/new-form", gin.H{})
}

func (tc *TaskController) NewTask(c *gin.Context) {
	formData := &models.NewTaskFormData{}
	if err := c.Bind(formData); err != nil {
		return
	}

	log.Info().Str("task", formData.Name).Msg("Adding new task")

	_, err := utils.ParseDuration(formData.Schedule)
	if err != nil {
		log.Warn().Str("schedule", formData.Schedule).Msg("Failed to parse schedule")
		c.HTML(http.StatusOK, "response/new-task.html", gin.H{"Error": "FAILED TO PARSE SCHEDULE"})
		return
	}

	newTask := models.Task{Id: utils.Uuid(), Name: formData.Name, Schedule: formData.Schedule, Trigger: formData.Trigger}

	author := "1337"
	err = tc.insertNewTask(author, &newTask)

	log.Info().Str("task", formData.Name).Str("author", author).Msg("Added new task")

	tc.sc.Message <- &Event{
		Message: nil,
		Type:    EVENT_TASKS_UPDATE,
	}

	if err != nil {
		c.HTML(http.StatusOK, "response/new-task.html", gin.H{"Error": "FAILED TO SAVE TASK"})
		return
	}
	c.HTML(http.StatusOK, "response/new-task.html", gin.H{"Name": formData.Name})
}

func (tc *TaskController) GetTasksUpdate() string {
	tasks := tc.getTasks()
	viewTasks := models.GetViewTasks(tasks)
	taskListTpl, _ := utils.RenderTemplate(tc.template, "tasks/table-body", models.TasksUpdateData{
		Tasks: viewTasks,
	})

	return taskListTpl
}

func (tc *TaskController) RegisterAllTasksSchedules() {
	scheduler := tc.readSchedulerData()
	if scheduler == nil {
		log.Info().Msg("Schedule not found. Skipping to register tasks")
		return
	}

	checkExpiredTasks(scheduler.Tasks)
	err := tc.writeTasksData(scheduler.Tasks)
	if err != nil {
		log.Error().Msg("Could not update tasks after checking expirey")
	}

	for _, task := range scheduler.Tasks {
		if task.IsActive() {
			tc.RegisterTaskSchedule(task)
		}
	}
}

func (tc *TaskController) RegisterRefreshInterval() {
	go func() {
		for {
			time.Sleep(5 * time.Second)
			tc.sc.Message <- &Event{
				Message: nil,
				Type:    EVENT_TASKS_UPDATE,
			}
		}
	}()
}

func (tc *TaskController) RegisterTaskSchedule(task *models.Task) {
	taskDuration, _ := utils.ParseDuration(task.Schedule)
	log.Debug().Str("task", task.Name).Dur("duration", taskDuration).Msg("Register task")

	timer := time.AfterFunc(taskDuration, func() {
		log.Debug().Str("task", task.Name).Msg("Task expired ")
		tc.sc.Message <- &Event{
			Message: task,
			Type:    EVENT_TASK_ALERT,
		}

		isRepetitive := utils.IsRepetitiveSchedule(task.Schedule)
		if isRepetitive {
			tc.ResetTask(task)
		} else {
			tc.UnregisterTask(task)
		}
	})

	tc.taskRegistry[task.Id] = timer
}

func (tc *TaskController) UnregisterTask(task *models.Task) {
	log.Debug().Str("task", task.Name).Msg("Unregistering task")

	if timer, exists := tc.taskRegistry[task.Id]; exists {
		timer.Stop()
		delete(tc.taskRegistry, task.Id)
		log.Info().Str("task", task.Name).Msg("Unregistered task")
	}
}

func (tc *TaskController) ResetTask(task *models.Task) {
	log.Debug().Str("task", task.Name).Msg("Resetting task")

	if timer, exists := tc.taskRegistry[task.Id]; exists {
		newActivatedTime := time.Now()
		task.ActivatedTime = &newActivatedTime
		taskDuration, _ := utils.ParseDuration(task.Schedule)
		timer.Reset(taskDuration)

		err := tc.taskDBM.UpdateTaskActivatedTime(task)
		if err == nil {
			log.Info().Str("task", task.Name).Msg("Reset task")
		}
	}
}

func (tc *TaskController) GetAlertTpl(task *models.Task) string {
	var tplName string
	if task.Trigger == "popup" {
		tplName = "alerts/popup"
	} else if task.Trigger == "audio" {
		tplName = "alerts/audio"
	}

	alertTpl, _ := utils.RenderTemplate(tc.template, tplName, models.AlertPopupData{
		Task: task.ToTaskVM(),
	})

	return alertTpl
}

func (tc *TaskController) TasksActivate(c *gin.Context) {
	formData := &models.ActivateTaskFormData{}
	if err := c.Bind(formData); err != nil {
		return
	}

	scheduler := tc.readSchedulerData()

	activatedTime := time.Now()
	tc.updateTaskActivationByIds(scheduler.Tasks, formData.TaskIds, &activatedTime)

	err := tc.writeSchedulerData(scheduler)
	if err != nil {
		log.Fatal().Msg("Could not write scheduler data")
	}

	viewTasks := models.GetViewTasks(scheduler.Tasks)
	c.HTML(http.StatusOK, "tasks/table-body", models.TasksUpdateData{
		Tasks: viewTasks,
	})
}

func (tc *TaskController) TasksDeactivate(c *gin.Context) {
	formData := &models.ActivateTaskFormData{}
	if err := c.Bind(formData); err != nil {
		return
	}

	scheduler := tc.readSchedulerData()

	tc.updateTaskActivationByIds(scheduler.Tasks, formData.TaskIds, nil)

	err := tc.writeSchedulerData(scheduler)
	if err != nil {
		log.Warn().Msg("Warning: Could not write scheduler data")
	}

	viewTasks := models.GetViewTasks(scheduler.Tasks)

	c.HTML(http.StatusOK, "tasks/table-body", models.TasksUpdateData{
		Tasks: viewTasks,
	})
}

func (tc *TaskController) TasksDelete(c *gin.Context) {
	formData := &models.DeleteTaskFormData{}
	if err := c.Bind(formData); err != nil {
		return
	}
	log.Debug().Strs("taskIds", formData.TaskIds).Msg("Deleting tasks")

	scheduler := tc.readSchedulerData()

	tc.updateTaskActivationByIds(scheduler.Tasks, formData.TaskIds, nil)

	updatedSchedule, _ := tc.taskDBM.DeleteTasks(formData.TaskIds)
	log.Info().Strs("taskIds", formData.TaskIds).Msg("Deleted tasks")

	viewTasks := models.GetViewTasks(updatedSchedule.Tasks)

	c.HTML(http.StatusOK, "tasks/table-body", models.TasksUpdateData{
		Tasks: viewTasks,
	})
}

func (tc *TaskController) TaskDone(c *gin.Context) {
	taskId := c.Param("id")
	log.Debug().Str("taskId", taskId).Msg("Task done")

	tc.sc.Message <- &Event{
		Message: nil,
		Type:    EVENT_TASKS_UPDATE,
	}
	c.String(http.StatusOK, "")
}

func (tc *TaskController) readSchedulerData() *models.Scheduler {
	var scheduler models.Scheduler

	if useDB {
		scheduler, err := tc.taskDBM.GetScheduleByAuthor()
		if err != nil {
			log.Error().Msg("Could not query schedule")
		}
		return scheduler
	} else {
		if err := utils.ParseJSONFile("schedule.json", &scheduler); err != nil {
			log.Fatal().Err(err)
		}
	}

	return &scheduler
}

func (tc *TaskController) writeSchedulerData(scheduler *models.Scheduler) error {

	if useDB {
		err := tc.taskDBM.ReplaceSchedule(scheduler)
		if err != nil {
			log.Error().Msg("Could not replace schedule")
			return err
		}
		return nil
	} else {
		jsonData, err := json.MarshalIndent(scheduler, "", "    ")
		if err != nil {
			return err
		}
		return os.WriteFile("schedule.json", jsonData, 0644)
	}
}

func (tc *TaskController) writeTasksData(tasks []*models.Task) error {
	scheduler := tc.readSchedulerData()
	scheduler.Tasks = tasks
	return tc.writeSchedulerData(scheduler)
}

func (tc *TaskController) getTasks() []*models.Task {
	scheduler := tc.readSchedulerData()
	tasks := scheduler.Tasks

	models.SortTasks(tasks)

	return tasks
}

func (tc *TaskController) insertNewTask(author string, newTask *models.Task) error {
	_, err := tc.taskDBM.InsertTask(author, newTask)
	if err != nil {
		return err
	}
	return nil
}

// updateTaskActivationByIds searches through the provided task array, either sets ActivatedTime & registers the task (activate)
// or unsets ActivatedTime & unregisters the task (de-activate / delete)
// it will sort the tasks afterwards and returns a new array of affected tasks
func (tc *TaskController) updateTaskActivationByIds(tasks []*models.Task, taskIds []string, activatedTime *time.Time) []*models.Task {
	taskIDSet := make(map[string]bool)
	for _, id := range taskIds {
		taskIDSet[id] = true
	}

	affectedTasks := make([]*models.Task, 0, len(taskIds))
	for _, task := range tasks {
		if taskIDSet[task.Id] {
			affectedTasks = append(affectedTasks, task)
			task.ActivatedTime = activatedTime

			if activatedTime == nil {
				tc.UnregisterTask(task)
			} else {
				tc.RegisterTaskSchedule(task)
			}
		}
	}

	models.SortTasks(tasks)

	return affectedTasks
}

func checkExpiredTasks(tasks []*models.Task) {
	for _, task := range tasks {
		if !task.IsActive() {
			task.ActivatedTime = nil
		}
	}
}
