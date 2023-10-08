package controllers

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"scheduler/models"
	"scheduler/utils"

	"github.com/gin-gonic/gin"
)

type TaskController struct {
	// You can add fields like a database connection or services here.
}

func NewTaskController() *TaskController {
	return &TaskController{}
}

func (tc *TaskController) GetTasks(c *gin.Context) {
	tasks := readTaskData().Tasks
	c.HTML(http.StatusOK, "pages/tasks", models.TasksPageData{
		Tasks: tasks,
	})
}

func (tc *TaskController) NewTask(c *gin.Context) {
	formData := &models.NewTaskFormData{}
	if err := c.Bind(formData); err != nil {
		return
	}

	scheduler := readTaskData()
	newTask := models.Task{Name: formData.Name, Schedule: formData.Schedule, Active: true}
	scheduler.Tasks = append(scheduler.Tasks, newTask)

	err := writeTaskData(&scheduler)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "response/new-task.html", gin.H{"Name": "FAILED TO CREATE TASK"})
		return
	}
	c.HTML(http.StatusOK, "response/new-task.html", gin.H{"Name": formData.Name})
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
