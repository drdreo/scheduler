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

	router.GET("/", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/tasks")
	})
	// router.GET("/", func(ctx *gin.Context) {
	// 	ctx.HTML(http.StatusOK, "index.tpl", nil)
	// })

	taskController := controllers.NewTaskController(templates)
	router.GET("/tasks", taskController.GetTasks)
	router.GET("/tasks/new", taskController.GetNewTaskForm)
	router.POST("/tasks/new", taskController.NewTask)
	router.GET("/tasks-update", taskController.TasksUpdate) // https://blog.stackademic.com/real-time-communication-with-golang-and-server-sent-events-sse-a-practical-tutorial-1094b37e17f5
	router.PUT("/tasks/activate", taskController.TasksActivate)
	router.PUT("/tasks/deactivate", taskController.TasksDeactivate)

	router.GET("/data", dataHandler)
	err := router.Run(getPort())
	if err != nil {
		panic(err)
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

//func sendMessageToClients() {
//    for {
//        // Simulate sending a message to all connected WebSocket clients every 5 seconds.
//        time.Sleep(5 * time.Second)
//
//        // Create the message you want to send.
//        message := []byte("Hello from the server!")
//
//        // Iterate through WebSocket clients and send the message.
//        for client := range clients {
//            err := client.WriteMessage(websocket.TextMessage, message)
//            if err != nil {
//                fmt.Println("Error sending message:", err)
//                client.Close()
//                delete(clients, client)
//            }
//        }
//    }
//}
