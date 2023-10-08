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
	"time"

	"github.com/gin-gonic/gin"
)

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

	taskController := controllers.NewTaskController()
	router.GET("/tasks", taskController.GetTasks)
	router.POST("/tasks/new", taskController.NewTask)

	router.GET("/data", dataHandler)
	router.GET("/alerts", alertsHandler) // https://blog.stackademic.com/real-time-communication-with-golang-and-server-sent-events-sse-a-practical-tutorial-1094b37e17f5
	err := router.Run("localhost:3000")
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

func alertsHandler(c *gin.Context) {
	noOfExecution := 10
	progress := 0
	for progress <= noOfExecution {
		progressPercentage := float64(progress) / float64(noOfExecution) * 100

		c.SSEvent("alerts", gin.H{
			"currentTask":        progress,
			"progressPercentage": progressPercentage,
			"noOftasks":          noOfExecution,
			"completed":          false,
		})

		c.Writer.Flush() // Flush the response to ensure the data is sent immediately

		progress += 1
		fmt.Println("Alert: ", progress)
		time.Sleep(2 * time.Second)
	}

	c.SSEvent("alerts", gin.H{
		"completed":          true,
		"progressPercentage": 100,
	})

	c.Writer.Flush() // Flush the response to ensure the data is sent immediately
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
