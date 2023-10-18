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
	"sort"
	"time"

	"github.com/gin-gonic/gin"
)

type TaskController struct {
	template *template.Template
	// ... add fields like database connection or services here.
}

func NewTaskController(templates []string) *TaskController {
	tmpl, err := template.ParseFiles(templates...)
	if err != nil {
		log.Fatalf("Failed to parse templates: %v", err)
	}
	return &TaskController{
		template: tmpl,
	}
}

func (tc *TaskController) GetTasks(c *gin.Context) {
	tasks := getTasks()

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

		taskListTpl, _ := renderTemplate(tc.template, "tasks/table-body", models.TasksUpdateData{
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

	scheduler := readSchedulerData()

	for _, task := range scheduler.Tasks {
		for _, id := range formData.TaskIds {
			if id == task.Id {
				task.Active = true
				activatedTime := time.Now()
				task.ActivatedTime = &activatedTime
				taskDuration, _ := utils.ParseDuration(task.Schedule)
				task.RemainingTime = utils.CalculateRemainingTime(task.ActivatedTime, taskDuration)
			}
		}
	}

	sortTasks(scheduler.Tasks)

	err := writeSchedulerData(scheduler)
	if err != nil {
		log.Fatal("Could not write scheduler data")
	}
	c.HTML(http.StatusOK, "tasks/table-body", models.TasksUpdateData{
		Tasks: scheduler.Tasks,
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
				taskDuration, _ := utils.ParseDuration(task.Schedule)
				task.RemainingTime = utils.CalculateRemainingTime(task.ActivatedTime, taskDuration)
			}
		}
	}

	sortTasks(scheduler.Tasks)

	err := writeSchedulerData(scheduler)
	if err != nil {
		log.Println("Warning: Could not write scheduler data")
	}

	c.HTML(http.StatusOK, "tasks/table-body", models.TasksUpdateData{
		Tasks: scheduler.Tasks,
	})
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

func getTasks() []*models.Task {
	scheduler := readSchedulerData()
	tasks := scheduler.Tasks

	for _, task := range tasks {
		taskDuration, _ := utils.ParseDuration(task.Schedule)
		task.RemainingTime = utils.CalculateRemainingTime(task.ActivatedTime, taskDuration)
	}

	sortTasks(tasks)

	return tasks
}

func sortTasks(tasks []*models.Task) {
	sort.Slice(tasks, func(i, j int) bool {
		timeA := tasks[i].RemainingTime
		timeB := tasks[j].RemainingTime

		if timeA == nil && timeB == nil {
			return false
		}
		if timeA == nil {
			return false
		}
		if timeB == nil {
			return true
		}

		return *timeA < *timeB
	})
}

func renderTemplate(template *template.Template, tmplName string, data interface{}) (string, error) {
	var tplContent bytes.Buffer

	err := template.ExecuteTemplate(&tplContent, tmplName, data)
	if err != nil {
		log.Fatal("err: ", err)

		return "", err
	}

	return tplContent.String(), nil
}
