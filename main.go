package main

import (
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"scheduler/controllers"
	"scheduler/models"
	"scheduler/utils"
	"github.com/gin-gonic/gin"
)

func getPort() string {
	port := os.Getenv("PORT")
	if port == "" {
		port = "localhost:3000"
	} else {
		port = ":" + port
	}

	return port
}

func main() {
	log.Println("main running... ")
	router := gin.Default()

	templates := getTemplateFiles("templates")
	router.LoadHTMLFiles(templates...)

	streamController := controllers.NewStreamController()

	taskController := controllers.NewTaskController(streamController, templates)
	taskController.RegisterAllTasksSchedules()

	router.Static("/static", "./static")
	router.GET("/", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/tasks")
	})

	router.GET("/tasks", taskController.GetTasks)
	router.GET("/tasks/new", taskController.GetNewTaskForm)
	router.POST("/tasks/new", taskController.NewTask)
	router.GET("/tasks-update", taskController.TasksUpdate) // https://blog.stackademic.com/real-time-communication-with-golang-and-server-sent-events-sse-a-practical-tutorial-1094b37e17f5
	router.PUT("/tasks/activate", taskController.TasksActivate)
	router.PUT("/tasks/deactivate", taskController.TasksDeactivate)
	router.PUT("/tasks/:id/done", taskController.TaskDone)

	// Add event-streaming headers
	router.GET("/stream", controllers.StreamHeadersMiddleware(), streamController.ServeHTTP(), func(c *gin.Context) {
		v, ok := c.Get("clientChan")
		if !ok {
			return
		}
		clientChan, ok := v.(controllers.ClientChan)
		if !ok {
			return
		}
		c.Stream(func(w io.Writer) bool {
			// Stream message to client from message channel
			if event, ok := <-clientChan; ok {
				log.Printf("[DEBUG] Trying to send event[%d] - %s", event.Type, event.Message)

				switch event.Type {
				case controllers.EVENT_TASK_ALERT:
					handleTaskAlertEvent(c, event, taskController)
				case controllers.EVENT_TASKS_UPDATE:
					handleTasksUpdateEvent(c, taskController)
				default:
					// generic "message" event handler
					c.SSEvent("message", event.Message)
				}

				return true
			}
			return false
		})
	})

	router.GET("/data", dataHandler)
	err := router.Run(getPort())
	if err != nil {
		log.Panic(err)
	}

}

func handleTaskAlertEvent(c *gin.Context, event *controllers.Event, taskController *controllers.TaskController) {
	if task, isTask := event.Message.(*models.Task); isTask {
		alertTpl := taskController.GetAlertTpl(task)
		c.SSEvent("task-alert", alertTpl)
	}
	if task, isTask := event.Message.(*models.Task); isTask {
		alertTpl := taskController.GetAlertTpl(task)
		c.SSEvent("task-alert", alertTpl)
	}
}

func handleTasksUpdateEvent(c *gin.Context, taskController *controllers.TaskController) {
	taskUpdateTpl := taskController.GetTasksUpdate()
	c.SSEvent("tasks-update", taskUpdateTpl)
}

func getTemplateFiles(directory string) []string {
	var files []string

	err := filepath.Walk(directory, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && filepath.Ext(path) == ".html" {
			files = append(files, path)
		}
		return nil
	})

	if err != nil {
		log.Println("[ERROR] Error walking directory:", err)
	}

	log.Println("[DEBUG] Found templates:")
	for _, file := range files {
		log.Println("[DEBUG] - ", file)
	}

	return files
}

func dataHandler(c *gin.Context) {
	var scheduler models.Scheduler

	if err := utils.ParseJSONFile("schedule.json", &scheduler); err != nil {
		log.Fatal(err)
		// c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read JSON file"})
	}

	c.JSON(http.StatusOK, &scheduler)
}
