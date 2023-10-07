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

	err := router.Run("localhost:3000")
	if err != nil {
		panic(err)
	}

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

func testInterval() {
	intervalStr := "every 36s"
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
