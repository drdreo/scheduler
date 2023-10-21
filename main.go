package main

import (
	"context"
	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"scheduler/controllers"
	"scheduler/models"
	"scheduler/utils"
	"github.com/gin-gonic/gin"
	"time"
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

	loadEnv()

	router := gin.Default()
	templates := getTemplateFiles("templates")
	router.LoadHTMLFiles(templates...)

	client := connectToMongo()
	// Defer the disconnection of the client
	defer func() {
		log.Println("[INFO] Disconnecting mongo client")
		if err := client.Disconnect(context.Background()); err != nil {
			log.Println("Error disconnecting from MongoDB:", err)
		}
	}()

	taskDB := models.TaskDBModel{Client: client}

	streamController := controllers.NewStreamController()

	taskController := controllers.NewTaskController(streamController, templates, &taskDB)
	taskController.RegisterAllTasksSchedules()
	//	taskController.RegisterRefreshInterval()

	router.Static("/static", "./static")
	router.GET("/", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/tasks")
	})

	router.GET("/tasks", taskController.GetTasks)
	router.GET("/tasks/new", taskController.GetNewTaskForm) // FOR HTMX
	router.POST("/tasks/new", taskController.NewTask)
	router.GET("/tasks-update", taskController.TasksUpdate) // FOR HTMX
	router.PUT("/tasks/activate", taskController.TasksActivate)
	router.PUT("/tasks/deactivate", taskController.TasksDeactivate)
	router.PUT("/tasks/delete", taskController.TasksDelete)
	router.PUT("/tasks/:id/done", taskController.TaskDone)

	router.GET("/stream", controllers.StreamHeadersMiddleware(), streamController.ServeHTTP(), func(c *gin.Context) {
		handleStream(c, taskController)
	})

	router.GET("/data", dataHandler)
	err := router.Run(getPort())
	if err != nil {
		log.Panic(err)
	}

}

func loadEnv() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}
}

func handleStream(c *gin.Context, taskController *controllers.TaskController) {
	v, ok := c.Get("clientChan")
	if !ok {
		return
	}
	clientChan, ok := v.(controllers.ClientChan)
	if !ok {
		return
	}
	c.Stream(func(w io.Writer) bool {
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
}

func handleTaskAlertEvent(c *gin.Context, event *controllers.Event, taskController *controllers.TaskController) {
	if task, isTask := event.Message.(*models.Task); isTask {
		alertTpl := taskController.GetAlertTpl(task)
		c.SSEvent("task-alert", alertTpl)
	} else {
		log.Print("[WARN] Event and message type dont match")
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

func connectToMongo() *mongo.Client {
	serverAPI := options.ServerAPI(options.ServerAPIVersion1)
	mongoUri := os.Getenv("MONGO_URI")
	opts := options.Client().ApplyURI(mongoUri).SetServerAPIOptions(serverAPI)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, opts)
	if err != nil {
		panic(err)
	}

	err = client.Ping(ctx, nil)

	if err != nil {
		log.Println("There was a problem connecting to your Atlas cluster. Check that the URI includes a valid username and password, and that your IP address has been added to the access list. Error: ")
		panic(err)
	}

	log.Println("Connected to MongoDB!\n")

	return client
}
