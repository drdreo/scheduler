package models

import (
	"context"
	"errors"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"github.com/rs/zerolog/log"
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
	TargetTime    string
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
		viewTask.TargetTime = time.Now().Add(*remainingTime).Format(time.TimeOnly)
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
			log.Debug().Msgf("Could not find existing schedule, created new one - %s", author)
			return &newSchedule, nil
		} else {
			log.Error().Err(err).Msgf("Something went wrong trying to find scheduler for %s", author)
			return nil, err
		}
	}
	log.Debug().Msgf("Found existing schedule - %s", result.Author)
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
		log.Error().Err(err).Msg("Something went wrong trying to insert new schedule")
		return nil, err
	}

	return schedule, nil
}

func (m TaskDBModel) ReplaceSchedule(scheduler *Scheduler) error {
	author := "1337"
	dbName := "SchedulerCluster"
	collectionName := "schedules"
	collection := m.Client.Database(dbName).Collection(collectionName)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	filter := bson.D{{"author", author}}

	_, err := collection.ReplaceOne(ctx, filter, scheduler)
	if err != nil {
		log.Error().Err(err).Msg("Something went wrong trying to update a schedule")
		return err
	}

	return nil
}

func (m TaskDBModel) InsertTask(author string, task *Task) (*Scheduler, error) {
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

	opts := options.FindOneAndUpdate().SetReturnDocument(options.After)

	var updatedSchedule *Scheduler

	err := collection.FindOneAndUpdate(ctx, filter, update, opts).Decode(&updatedSchedule)
	if err != nil {
		log.Error().Err(err).Msg("Something went wrong trying to insert a task")
		return nil, err
	}

	return updatedSchedule, nil
}

func (m TaskDBModel) DeleteTasks(taskIds []string) (*Scheduler, error) {
	author := "1337"
	dbName := "SchedulerCluster"
	collectionName := "schedules"
	collection := m.Client.Database(dbName).Collection(collectionName)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	filter := bson.D{{"author", author}}

	// construct pull filters to pull items of the tasks array by ids
	pullFilters := bson.A{}
	for _, taskId := range taskIds {
		pullFilters = append(pullFilters, bson.M{"id": taskId})
	}
	pullFilter := bson.M{"$or": pullFilters}
	update := bson.M{
		"$pull": bson.M{"tasks": pullFilter},
	}

	opts := options.FindOneAndUpdate().SetReturnDocument(options.After)

	var updatedSchedule *Scheduler

	err := collection.FindOneAndUpdate(ctx, filter, update, opts).Decode(&updatedSchedule)
	if err != nil {
		log.Error().Err(err).Msg("Something went wrong trying to delete tasks")
		return nil, err
	}

	return updatedSchedule, nil
}
