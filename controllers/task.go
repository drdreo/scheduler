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

const useDB = true

type TaskController struct {
	template *template.Template
	// ... add fields like database connection or services here.
	sc      *StreamController
	taskDBM *models.TaskDBModel
}

func NewTaskController(streamController *StreamController, templates []string, taskDBM *models.TaskDBModel) *TaskController {
	tmpl, err := template.ParseFiles(templates...)
	if err != nil {
		log.Fatalf("Failed to parse templates: %v", err)
	}

	return &TaskController{
		template: tmpl,
		sc:       streamController,
		taskDBM:  taskDBM,
	}
}

func (tc *TaskController) GetTasks(c *gin.Context) {
	tasks := tc.getTasks()
	checkExpiredTasks(tasks)
	err := tc.writeTasksData(tasks)
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

	newTask := models.Task{Id: utils.Uuid(), Name: formData.Name, Schedule: formData.Schedule}

	if useDB {
		err = tc.insertNewTask("1337", &newTask)
	} else {
		scheduler := tc.readSchedulerData()
		scheduler.Tasks = append(scheduler.Tasks, &newTask)
		err = tc.writeSchedulerData(scheduler)
	}

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

func (tc *TaskController) TasksUpdate(c *gin.Context) {

	for {
		taskUpdateTpl := tc.GetTasksUpdate()
		c.SSEvent("tasks-update", taskUpdateTpl)
		c.Writer.Flush() // Flush the response to ensure the data is sent immediately

		time.Sleep(1 * time.Second)
	}
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
		log.Println("[INFO] Schedule not found. Skipping to register tasks")
		return
	}

	checkExpiredTasks(scheduler.Tasks)
	err := tc.writeTasksData(scheduler.Tasks)
	if err != nil {
		log.Println("[ERROR] Could not update tasks after checking expirey")
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
	go func(task *models.Task) {
		taskDuration, _ := utils.ParseDuration(task.Schedule)
		log.Printf("Task '%s' registered in - %s", task.Name, taskDuration)

		time.Sleep(taskDuration)

		log.Printf("Task expired - %s", task.Name)
		tc.sc.Message <- &Event{
			Message: task,
			Type:    EVENT_TASK_ALERT,
		}
	}(task)
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

	scheduler := tc.readSchedulerData()

	for _, task := range scheduler.Tasks {
		for _, id := range formData.TaskIds {
			if id == task.Id {
				activatedTime := time.Now()
				task.ActivatedTime = &activatedTime

				tc.RegisterTaskSchedule(task)
			}
		}
	}

	models.SortTasks(scheduler.Tasks)

	err := tc.writeSchedulerData(scheduler)
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

	scheduler := tc.readSchedulerData()

	for _, task := range scheduler.Tasks {
		for _, id := range formData.TaskIds {
			if id == task.Id {
				task.ActivatedTime = nil
			}
		}
	}

	models.SortTasks(scheduler.Tasks)

	err := tc.writeSchedulerData(scheduler)
	if err != nil {
		log.Println("Warning: Could not write scheduler data")
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

	taskIds, _ := c.GetPostFormArray("task-ids")

	log.Printf("ids %s", taskIds)

	scheduler := tc.readSchedulerData()
	tasksToKeep := make([]*models.Task, 0, len(scheduler.Tasks))
	for _, task := range scheduler.Tasks {
		found := false

		for _, id := range formData.TaskIds {
			if id == task.Id {
				found = true
				break
			}
		}

		if !found {
			tasksToKeep = append(tasksToKeep, task)
		}
	}

	scheduler.Tasks = tasksToKeep

	models.SortTasks(scheduler.Tasks)

	err := tc.writeSchedulerData(scheduler)
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
			log.Println("[ERROR] Could not query schedule")
		}
		return scheduler
	} else {
		if err := utils.ParseJSONFile("schedule.json", &scheduler); err != nil {
			log.Fatal(err)
		}
	}

	return &scheduler
}

func (tc *TaskController) writeSchedulerData(scheduler *models.Scheduler) error {

	if useDB {
		err := tc.taskDBM.ReplaceSchedule(scheduler)
		if err != nil {
			log.Println("[ERROR] Could not replace schedule")
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
	_, err := tc.taskDBM.InsertOne(author, newTask)
	if err != nil {
		return err
	}
	return nil
}

func checkExpiredTasks(tasks []*models.Task) {
	for _, task := range tasks {
		if !task.IsActive() {
			task.ActivatedTime = nil
		}
	}
}
