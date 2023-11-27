package main

import (
	"context"
	"github.com/getsentry/sentry-go"
	sentrygin "github.com/getsentry/sentry-go/gin"
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

func initSentry() {
	enabled := os.Getenv("SENTRY_ENABLED")
	if enabled != "true" {
		return
	}

	log.Info().Msg("initializing Sentry")

	if err := sentry.Init(sentry.ClientOptions{
		Dsn:              os.Getenv("SENTRY_DSN"),
		Environment:      os.Getenv("APP_ENV"),
		EnableTracing:    true,
		TracesSampleRate: 1.0,
	}); err != nil {
		log.Printf("Sentry initialization failed: %v", err)
	}
}

type Templates struct {
	templates *template.Template
	funcMap   *template.FuncMap
}

func GetTemplateFiles(directory string) []string {
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

func NewTemplates() (*Templates, error) {
	myFuncMap := template.FuncMap{
		"formatAsDate": utils.FormatAsDate,
	}

	tplPaths := GetTemplateFiles("templates")
	tpls, err := template.New("any").Funcs(myFuncMap).ParseFiles(tplPaths...)
	if err != nil {
		return nil, err
	}

	return &Templates{
		funcMap:   &myFuncMap,
		templates: tpls,
	}, nil
}

func main() {
	log.Info().Msg("main running... ")

	// setup logger
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stdout})

	loadEnv()

	initSentry()

	app := gin.Default()

	tpls, err := NewTemplates()
	if err != nil {
		log.Error().Err(err).Msg("Error initializing templates")
	}

	app.SetFuncMap(*tpls.funcMap)
	app.SetHTMLTemplate(tpls.templates)

	app.Use(sentrygin.New(sentrygin.Options{}))

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

	taskController := controllers.NewTaskController(streamController, tpls.templates, &taskDB)
	taskController.RegisterAllTasksSchedules()
	//	taskController.RegisterRefreshInterval()

	app.Static("/static", "./static")
	app.StaticFile("/alert.worker.js", "./static/js/alert.worker.js")
	app.GET("/", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/tasks")
	})

	app.GET("/tasks", taskController.GetTasks)
	app.GET("/tasks/new", taskController.GetNewTaskForm) // FOR HTMX
	app.POST("/tasks/new", taskController.NewTask)
	//	app.GET("/tasks-update", taskController.TasksUpdate) // FOR HTMX
	app.PUT("/tasks/activate", taskController.TasksActivate)
	app.PUT("/tasks/deactivate", taskController.TasksDeactivate)
	app.PUT("/tasks/delete", taskController.TasksDelete)
	app.PUT("/tasks/:id/done", taskController.TaskDone)
	app.GET("/tasks/:id/snooze", taskController.TaskSnooze)

	app.GET("/stream", controllers.StreamHeadersMiddleware(), streamController.ServeHTTP(), func(c *gin.Context) {
		handleStream(c, taskController)
	})

	app.GET("/data", dataHandler)
	err = app.Run(getPort())
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
