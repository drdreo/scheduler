package models

import (
	"context"
	"errors"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"log"
	"scheduler/utils"
	"sort"
	"time"
)

type TaskTrigger string

const (
	Popup   TaskTrigger = "popup"
	Audio   TaskTrigger = "audio"
	WebHook TaskTrigger = "webhook"
)

type TasksPageData struct {
	Tasks []*TaskVM
}

type TasksUpdateData struct {
	Tasks []*TaskVM
}

// TaskVM is the Task View Model
type TaskVM struct {
	Id            string
	Name          string
	Active        bool
	Schedule      string
	Trigger       TaskTrigger
	IsSoon        bool
	RemainingTime string
	ActivatedTime string
}

func (task *Task) ToTaskVM() *TaskVM {
	viewTask := &TaskVM{
		Id:       task.Id,
		Name:     task.Name,
		Active:   task.IsActive(),
		Schedule: task.Schedule,
		Trigger:  task.Trigger,
	}

	if task.IsActive() {
		viewTask.ActivatedTime = task.ActivatedTime.String()
		remainingTime := task.GetRemainingTime()
		viewTask.RemainingTime = task.GetRemainingTime().String()
		viewTask.IsSoon = remainingTime.Seconds() < 60
	}

	return viewTask
}

type Task struct {
	Id            string      `json:"id"`
	Name          string      `json:"name"`
	Schedule      string      `json:"schedule"`
	ActivatedTime *time.Time  `json:"activatedTime"` // optional
	Trigger       TaskTrigger `json: "trigger"`
}

func (task *Task) GetRemainingTime() *time.Duration {
	taskDuration, _ := utils.ParseDuration(task.Schedule)
	return utils.CalculateRemainingTime(task.ActivatedTime, taskDuration)
}

func (task *Task) IsActive() bool {
	if task.ActivatedTime == nil {
		return false
	}

	remaining := task.GetRemainingTime()
	return remaining.Seconds() > 0
}

type NewTaskFormData struct {
	Name     string      `form:"task-name" validate:"required"`
	Schedule string      `form:"task-schedule" validate:"required"`
	Trigger  TaskTrigger `form:"task-trigger" validate:"required"`
}

type ActivateTaskFormData struct {
	TaskIds []string `form:"task-ids" validate:"required"`
}

type DeleteTaskFormData struct {
	TaskIds []string `form:"task-ids" validate:"required"`
}

func GetViewTasks(tasks []*Task) []*TaskVM {
	var viewTasks []*TaskVM

	for _, task := range tasks {
		viewTasks = append(viewTasks, task.ToTaskVM())
	}

	return viewTasks
}

func SortTasks(tasks []*Task) {
	sort.Slice(tasks, func(i, j int) bool {
		timeA := tasks[i].ActivatedTime
		timeB := tasks[j].ActivatedTime

		if timeA == nil && timeB == nil {
			return false
		}
		if timeA == nil {
			return false
		}
		if timeB == nil {
			return true
		}

		remainingTimeA := *tasks[i].GetRemainingTime()
		remainingTimeB := *tasks[j].GetRemainingTime()

		if remainingTimeA <= 0 {
			if remainingTimeB <= 0 {
				// if both remainingTimes are <= 0, sort them based on secondary criteria
				return tasks[i].Name < tasks[j].Name
			}
			return false
		}

		if remainingTimeB <= 0 {
			return true
		}

		return remainingTimeA < remainingTimeB
	})
}

// https://github.com/mongodb-university/atlas_starter_go/blob/master/main.go
type TaskDBModel struct {
	Client *mongo.Client
}

func (m TaskDBModel) GetScheduleByAuthor() (*Scheduler, error) {
	author := "1337"
	dbName := "SchedulerCluster"
	collectionName := "schedules"
	collection := m.Client.Database(dbName).Collection(collectionName)

	var result Scheduler
	filter := bson.D{{"author", author}}
	err := collection.FindOne(context.TODO(), filter).Decode(&result)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			// if we dont find it in the DB, we create a new one
			newSchedule := Scheduler{
				Author: author,
				Tasks:  []*Task{},
			}
			m.InsertSchedule(&newSchedule)
			return &newSchedule, nil
		} else {
			log.Printf("Something went wrong trying to find scheduler for %s", author)
			return nil, err
		}
	}
	log.Println("Found a document with ", result)
	return &result, nil
}

func (m TaskDBModel) InsertSchedule(schedule *Scheduler) (*Scheduler, error) {
	log.Printf("[INFO] Inserting new schedule for author[%s]", schedule.Author)

	dbName := "SchedulerCluster"
	collectionName := "schedules"
	collection := m.Client.Database(dbName).Collection(collectionName)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := collection.InsertOne(ctx, schedule)
	if err != nil {
		log.Println("[ERROR] Something went wrong trying to insert a task:")
		panic(err)
	}

	return schedule, nil
}

func (m TaskDBModel) InsertOne(author string, task *Task) (*Scheduler, error) {
	dbName := "SchedulerCluster"
	collectionName := "schedules"
	collection := m.Client.Database(dbName).Collection(collectionName)

	filter := bson.D{{"author", author}}
	update := bson.D{
		{"$push", bson.D{
			{"tasks", task},
		}},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var result *mongo.SingleResult
	result = collection.FindOneAndUpdate(ctx, filter, update, options.FindOneAndUpdate().SetReturnDocument(options.After))
	if result.Err() != nil {
		log.Println("[ERROR] Something went wrong trying to insert a task:")
		panic(result.Err())
	}

	_schedule := Scheduler{}
	decodeErr := result.Decode(&_schedule)
	if decodeErr != nil {
		log.Println("[ERROR] Something went wrong trying to decode the document:")
		panic(decodeErr)
	}
	return &_schedule, nil
}

func (m TaskDBModel) ReplaceSchedule(scheduler *Scheduler) error {
	author := "1337"
	dbName := "SchedulerCluster"
	collectionName := "tasks"
	collection := m.Client.Database(dbName).Collection(collectionName)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	filter := bson.D{{"author", author}}

	_, err := collection.ReplaceOne(ctx, filter, scheduler)
	if err != nil {
		log.Println("Something went wrong trying to update one document:")
		return err
	}

	return nil
}
