package controllers

import (
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"os"
	"scheduler/models"
	"scheduler/utils"
	"time"

	"github.com/gin-gonic/gin"
)

type TaskController struct {
	template *template.Template
	// ... add fields like database connection or services here.
	alertChannel chan *models.Task
	sc           *StreamController
}

func NewTaskController(streamController *StreamController, templates []string) *TaskController {
	tmpl, err := template.ParseFiles(templates...)
	if err != nil {
		log.Fatalf("Failed to parse templates: %v", err)
	}
	return &TaskController{
		template:     tmpl,
		alertChannel: make(chan *models.Task),
		sc:           streamController,
	}
}

func (tc *TaskController) GetTasks(c *gin.Context) {
	tasks := getTasks()
	checkExpiredTasks(tasks)
	err := writeTasksData(tasks)
	if err != nil {
		log.Fatal("Could not write tasks data")
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

	_, err := utils.ParseDuration(formData.Schedule)
	if err != nil {
		log.Print(err)
		c.HTML(http.StatusOK, "response/new-task.html", gin.H{"Error": "FAILED TO PARSE SCHEDULE"})
		return
	}

	scheduler := readSchedulerData()
	newTask := models.Task{Id: utils.Uuid(), Name: formData.Name, Schedule: formData.Schedule, Active: false}
	scheduler.Tasks = append(scheduler.Tasks, &newTask)

	err = writeSchedulerData(scheduler)
	if err != nil {
		c.HTML(http.StatusOK, "response/new-task.html", gin.H{"NaErrorme": "FAILED TO SAVE TASK"})
		return
	}
	c.HTML(http.StatusOK, "response/new-task.html", gin.H{"Name": formData.Name})
}

func (tc *TaskController) TasksUpdate(c *gin.Context) {

	for {
		tasks := getTasks()
		viewTasks := models.GetViewTasks(tasks)
		taskListTpl, _ := utils.RenderTemplate(tc.template, "tasks/table-body", models.TasksUpdateData{
			Tasks: viewTasks,
		})

		c.SSEvent("tasks-update", taskListTpl)
		c.Writer.Flush() // Flush the response to ensure the data is sent immediately

		time.Sleep(1 * time.Second)
	}
}

func (tc *TaskController) RegisterAllTasksSchedules() {
	scheduler := readSchedulerData()

	for _, task := range scheduler.Tasks {
		tc.RegisterTaskSchedule(task)
	}
}

func (tc *TaskController) RegisterTaskSchedule(task *models.Task) {
	go func(task *models.Task) {
		taskDuration, _ := utils.ParseDuration(task.Schedule)
		log.Printf("Task '%s' registered in - %s", task.Name, taskDuration)

		time.Sleep(taskDuration)

		log.Printf("Task expired - %s", task.Name)
		tc.sc.Message <- task
		tc.alertChannel <- task
	}(task)
}

func (tc *TaskController) SubscribeToAlerts(c *gin.Context) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.SSEvent("subscribe", "Success")
	c.Writer.Flush()

	for {
		task := <-tc.alertChannel
		alertTpl, _ := utils.RenderTemplate(tc.template, "alerts/popup", models.AlertPopupData{
			Task: task.ToTaskVM(),
		})

		log.Printf("Sending alert for - %s", task.Name)
		c.SSEvent("task-alert", alertTpl)
		c.Writer.Flush()
	}
}

func (tc *TaskController) GetAlertTpl(task *models.Task) string {
	alertTpl, _ := utils.RenderTemplate(tc.template, "alerts/popup", models.AlertPopupData{
		Task: task.ToTaskVM(),
	})

	return alertTpl
}

func (tc *TaskController) TasksActivate(c *gin.Context) {
	formData := &models.ActivateTaskFormData{}
	if err := c.Bind(formData); err != nil {
		return
	}

	scheduler := readSchedulerData()

	for _, task := range scheduler.Tasks {
		for _, id := range formData.TaskIds {
			if id == task.Id {
				task.Active = true
				activatedTime := time.Now()
				task.ActivatedTime = &activatedTime

				tc.RegisterTaskSchedule(task)
			}
		}
	}

	models.SortTasks(scheduler.Tasks)

	err := writeSchedulerData(scheduler)
	if err != nil {
		log.Fatal("Could not write scheduler data")
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

	scheduler := readSchedulerData()

	for _, task := range scheduler.Tasks {
		for _, id := range formData.TaskIds {
			if id == task.Id {
				task.Active = false
				task.ActivatedTime = nil
			}
		}
	}

	models.SortTasks(scheduler.Tasks)

	err := writeSchedulerData(scheduler)
	if err != nil {
		log.Println("Warning: Could not write scheduler data")
	}

	viewTasks := models.GetViewTasks(scheduler.Tasks)

	c.HTML(http.StatusOK, "tasks/table-body", models.TasksUpdateData{
		Tasks: viewTasks,
	})
}

func (tc *TaskController) TaskDone(c *gin.Context) {
	taskId := c.Param("id")
	log.Printf("Task doned %s", taskId)

	c.String(http.StatusOK, "")
}

func readSchedulerData() *models.Scheduler {
	var scheduler models.Scheduler

	if err := utils.ParseJSONFile("schedule.json", &scheduler); err != nil {
		log.Fatal(err)
	}

	return &scheduler
}

func writeSchedulerData(scheduler *models.Scheduler) error {
	jsonData, err := json.MarshalIndent(scheduler, "", "    ")
	if err != nil {
		return err
	}

	return os.WriteFile("schedule.json", jsonData, 0644)
}

func writeTasksData(tasks []*models.Task) error {
	scheduler := readSchedulerData()
	scheduler.Tasks = tasks
	return writeSchedulerData(scheduler)
}

func getTasks() []*models.Task {
	scheduler := readSchedulerData()
	tasks := scheduler.Tasks

	models.SortTasks(tasks)

	return tasks
}

func checkExpiredTasks(tasks []*models.Task) {
	for _, task := range tasks {
		taskDuration, _ := utils.ParseDuration(task.Schedule)
		remainingTime := utils.CalculateRemainingTime(task.ActivatedTime, taskDuration)
		if task.ActivatedTime != nil && remainingTime.Seconds() <= 0 {
			task.Active = false
		}
	}
}
