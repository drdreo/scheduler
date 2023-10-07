package main

import (
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type TodoPageData struct {
	PageTitle string
	Todos     []Todo
}

type Todo struct {
	Title string
	Done  bool
}

type Task struct {
	Active   bool   `json:"active"`
	Name     string `json:"name"`
	Interval string `json:"interval"`
}

type Scheduler struct {
	Tasks []Task `json:"tasks"`
}

func main() {
	testInterval()
	fmt.Println("main running... ")
	router := gin.Default()

	router.LoadHTMLGlob("templates/**/*.html")

	//	router.GET("/", func(c *gin.Context) {
	//		c.Redirect(http.StatusMovedPermanently, "/test")
	//	})
	router.GET("/", func(ctx *gin.Context) {
		ctx.HTML(http.StatusOK, "index.tpl", nil)
	})
	router.GET("/test", func(c *gin.Context) {

		data := TodoPageData{
			PageTitle: "My TODO list",
			Todos: []Todo{
				{Title: "Task 1", Done: false},
				{Title: "Task 2", Done: true},
				{Title: "Task 3", Done: true},
			}}
		c.HTML(http.StatusOK, "pages/test.html", data)
	})

	router.GET("/data", dataHandler)
	router.POST("/add-task", addTaskHandler)
	router.GET("/alerts", alertsHandler) // https://blog.stackademic.com/real-time-communication-with-golang-and-server-sent-events-sse-a-practical-tutorial-1094b37e17f5
	testAsync()
	err := router.Run("localhost:3000")
	if err != nil {
		panic(err)
	}

}

func addTaskHandler(c *gin.Context) {
	name := c.PostForm("task-name")
	c.HTML(http.StatusOK, "response/add-task.html", gin.H{"Name": name})
}

func dataHandler(c *gin.Context) {
	data, err := os.ReadFile("schedule.json")
	if err != nil {
		log.Fatal(err)
	}
	var scheduler Scheduler
	err = json.Unmarshal(data, &scheduler)
	if err != nil {
		log.Fatal(err)
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

func testAsync() {
	noOfExecution := 10
	progress := 0
	go func() {
		for progress <= noOfExecution {
			progress += 1
			fmt.Println("Progress: ", progress)
			time.Sleep(2 * time.Second)
		}
	}()

}

func testInterval() {
	intervalStr := "every 6s"
	duration, err := parseDuration(intervalStr)
	if err != nil {
		panic(err)
	}
	fmt.Println("duration: ", duration)
}

func parseDuration(input string) (time.Duration, error) {
	regex := regexp.MustCompile(`^every (\d+)([a-zA-Z]+)$`)
	matches := regex.FindStringSubmatch(strings.ToLower(input))

	if len(matches) != 3 {
		return 0, fmt.Errorf("invalid duration format - `every <interval>`")
	}

	numericValue, err := strconv.Atoi(matches[1])
	if err != nil {
		return 0, fmt.Errorf("invalid <interval> value")
	}

	unit := strings.ToLower(matches[2])
	switch unit {
	case "ms", "millisecond", "milliseconds":
		return time.Duration(numericValue) * time.Millisecond, nil
	case "s", "sec", "second", "seconds":
		return time.Duration(numericValue) * time.Second, nil
	case "m", "min", "minute", "minutes":
		return time.Duration(numericValue) * time.Minute, nil
	case "h", "hr", "hour", "hours":
		return time.Duration(numericValue) * time.Hour, nil
	default:
		return 0, fmt.Errorf("invalid time unit")
	}
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
