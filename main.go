package main

import (
	"fmt"
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
	fmt.Println("main running... ")
	router := gin.Default()

	templates := getTemplateFiles("templates")
	router.LoadHTMLFiles(templates...)

	router.Static("/static", "./static")
	router.GET("/", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/tasks")
	})

	taskController := controllers.NewTaskController(templates)
	taskController.RegisterAllTasksSchedules()

	router.GET("/tasks", taskController.GetTasks)
	router.GET("/tasks/new", taskController.GetNewTaskForm)
	router.POST("/tasks/new", taskController.NewTask)
	router.GET("/tasks-update", taskController.TasksUpdate) // https://blog.stackademic.com/real-time-communication-with-golang-and-server-sent-events-sse-a-practical-tutorial-1094b37e17f5
	router.PUT("/tasks/activate", taskController.TasksActivate)
	router.PUT("/tasks/deactivate", taskController.TasksDeactivate)
	router.PUT("/tasks/:id/done", taskController.TaskDone)

	router.GET("/sse-alerts", taskController.SubscribeToAlerts)

	router.GET("/data", dataHandler)
	err := router.Run(getPort())
	if err != nil {
		log.Panic(err)
	}

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
		fmt.Println("Error walking directory:", err)
	}

	fmt.Println("Found templates:")
	for _, file := range files {
		fmt.Println(" - ", file)
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
