package main

import (
	"context"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"html/template"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"scheduler/controllers"
	"scheduler/models"
	"scheduler/utils"
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
	log.Info().Msg("main running... ")

	// setup logger
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stdout})

	loadEnv()

	router := gin.Default()
	templates := getTemplateFiles("templates")

	myFuncMap := template.FuncMap{
		"formatAsDate": utils.FormatAsDate,
	}
	router.SetFuncMap(myFuncMap)
	tpl, _ := template.New("any").Funcs(myFuncMap).ParseFiles(templates...)

	router.SetHTMLTemplate(tpl)

	client := connectToMongo()
	// Defer the disconnection of the client
	defer func() {
		log.Info().Msg("Disconnecting mongo client")
		if err := client.Disconnect(context.Background()); err != nil {
			log.Error().Err(err).Msg("Error disconnecting from MongoDB")
		}
	}()

	taskDB := models.TaskDBModel{Client: client}

	streamController := controllers.NewStreamController()

	taskController := controllers.NewTaskController(streamController, tpl, &taskDB)
	taskController.RegisterAllTasksSchedules()
	//	taskController.RegisterRefreshInterval()

	router.Static("/static", "./static")
	router.GET("/", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/tasks")
	})

	router.GET("/tasks", taskController.GetTasks)
	router.GET("/tasks/new", taskController.GetNewTaskForm) // FOR HTMX
	router.POST("/tasks/new", taskController.NewTask)
	//	router.GET("/tasks-update", taskController.TasksUpdate) // FOR HTMX
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
		log.Panic().Err(err).Msgf("Could not run app on port: %d", getPort())
	}

}

func loadEnv() {
	isProd := os.Getenv("APP_ENV") == "production"
	if isProd {
		log.Info().Msg("Production environment detected")
		// In production, env vars should already be set by the hosting provider, we can access them directly without using godotenv.
		return
	}

	log.Debug().Msg("Local dev - trying to read environment variables from .env")
	err := godotenv.Load()
	if err != nil {
		log.Fatal().Err(err).Msg("Error loading .env file")
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
			log.Debug().Int("type", event.Type).Any("message", event.Message).Msg("Sending event")

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

		eventName := "task-alert"
		if task.Trigger == "audio" {
			eventName = "audio-alert"
		}
		c.SSEvent(eventName, alertTpl)
	} else {
		log.Error().Msg("Event and message type dont match")
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
		log.Error().Err(err).Msg("Error walking directory")
	}

	log.Debug().Msg("Found templates:")
	for _, file := range files {
		log.Debug().Str("file", file).Msg(" - ")
	}

	return files
}

func dataHandler(c *gin.Context) {
	var scheduler models.Scheduler

	if err := utils.ParseJSONFile("schedule.json", &scheduler); err != nil {
		log.Fatal().Err(err).Msg("Failed to parse schedule.json")
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
		log.Fatal().Err(err).Msg("There was a problem connecting to your Atlas cluster. Check that the URI includes a valid username and password, and that your IP address has been added to the access list. Error: ")
		panic(err)
	}

	log.Info().Msg("Connected to MongoDB!")

	return client
}
