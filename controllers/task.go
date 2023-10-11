package controllers

import (
	"bytes"
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
	startedTime time.Time
	template    *template.Template
	// You can add fields like a database connection or services here.
}

func NewTaskController(templates []string) *TaskController {
	// Parse the templates and check for any parsing errors
	tmpl, err := template.ParseFiles(templates...)
	if err != nil {
		log.Fatalf("Failed to parse templates: %v", err)
	}
	return &TaskController{
		startedTime: time.Now(),
		template:    tmpl,
	}
}

func (tc *TaskController) GetTasks(c *gin.Context) {
	tasks := readTaskData().Tasks

	for i := range tasks {
		taskDuration, _ := utils.ParseDuration(tasks[i].Schedule)
		remainingTime := utils.CalculateRemainingTime(tc.startedTime, taskDuration)
		tasks[i].RemainingTime = &remainingTime
	}

	c.HTML(http.StatusOK, "pages/tasks", models.TasksPageData{
		Tasks: tasks,
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
		c.HTML(http.StatusOK, "response/new-task.html", gin.H{"Error": "FAILED TO PARSE SCHEDULE"})
		return
	}

	scheduler := readTaskData()
	newTask := models.Task{Id: utils.Uuid(), Name: formData.Name, Schedule: formData.Schedule, Active: false}
	scheduler.Tasks = append(scheduler.Tasks, newTask)

	err = writeTaskData(&scheduler)
	if err != nil {
		c.HTML(http.StatusOK, "response/new-task.html", gin.H{"NaErrorme": "FAILED TO CREATE TASK"})
		return
	}
	c.HTML(http.StatusOK, "response/new-task.html", gin.H{"Name": formData.Name})
}

func (tc *TaskController) TasksUpdate(c *gin.Context) {
	tasks := readTaskData().Tasks

	for {
		for i := range tasks {
			taskDuration, _ := utils.ParseDuration(tasks[i].Schedule)
			remainingTime := utils.CalculateRemainingTime(tc.startedTime, taskDuration)
			tasks[i].RemainingTime = &remainingTime
		}

		taskListTpl, _ := renderTemplate(c, tc.template, "tasks/table-body", models.TasksUpdateData{
			Tasks: tasks,
		})

		c.SSEvent("tasks-update", taskListTpl)

		c.Writer.Flush() // Flush the response to ensure the data is sent immediately

		time.Sleep(1 * time.Second)
	}
}

func (tc *TaskController) TasksActivate(c *gin.Context) {
	formData := &models.ActivateTaskFormData{}
	if err := c.Bind(formData); err != nil {
		return
	}

	scheduler := readTaskData()
	tasks := scheduler.Tasks

	for i := range tasks {
		taskDuration, _ := utils.ParseDuration(tasks[i].Schedule)
		remainingTime := utils.CalculateRemainingTime(tc.startedTime, taskDuration)
		tasks[i].RemainingTime = &remainingTime

		for _, id := range formData.TaskIds {
			if id == tasks[i].Id {
				tasks[i].Active = true
			}
		}
	}

	scheduler.Tasks = tasks
	writeTaskData(&scheduler)

	c.HTML(http.StatusOK, "tasks/table-body", models.TasksUpdateData{
		Tasks: tasks,
	})
}

func (tc *TaskController) TasksDeactivate(c *gin.Context) {
	formData := &models.ActivateTaskFormData{}
	if err := c.Bind(formData); err != nil {
		return
	}

	scheduler := readTaskData()
	tasks := scheduler.Tasks

	for i := range tasks {
		taskDuration, _ := utils.ParseDuration(tasks[i].Schedule)
		remainingTime := utils.CalculateRemainingTime(tc.startedTime, taskDuration)
		tasks[i].RemainingTime = &remainingTime

		for _, id := range formData.TaskIds {
			if id == tasks[i].Id {
				tasks[i].Active = false
			}
		}
	}

	scheduler.Tasks = tasks

	writeTaskData(&scheduler)

	c.HTML(http.StatusOK, "tasks/table-body", models.TasksUpdateData{
		Tasks: tasks,
	})
}

func readTaskData() models.Scheduler {
	var scheduler models.Scheduler

	if err := utils.ParseJSONFile("schedule.json", &scheduler); err != nil {
		log.Fatal(err)
		// c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read JSON file"})
	}

	return scheduler
}

func writeTaskData(scheduler *models.Scheduler) error {
	jsonData, err := json.MarshalIndent(scheduler, "", "    ")
	if err != nil {
		return err
	}

	err = os.WriteFile("schedule.json", jsonData, 0644)
	if err != nil {
		return err
	}
	return nil
}

func renderTemplate(c *gin.Context, template *template.Template, tmplName string, data interface{}) (string, error) {
	var tplContent bytes.Buffer

	err := template.ExecuteTemplate(&tplContent, tmplName, data)
	if err != nil {
		log.Fatal("err: ", err)

		return "", err
	}

	return tplContent.String(), nil
}
