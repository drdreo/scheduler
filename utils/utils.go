package utils

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

func TestInterval() {
	intervalStr := "every 6s"
	duration, err := ParseDuration(intervalStr)
	if err != nil {
		panic(err)
	}
	fmt.Println("duration: ", duration)
}

func ParseDuration(input string) (time.Duration, error) {
	pattern := regexp.MustCompile(`^(every|in|at) (\d+)([a-zA-Z]+|\d{2}:\d{2})$`)
	matches := pattern.FindStringSubmatch(strings.ToLower(input))

	if len(matches) != 4 {
		err := fmt.Errorf("invalid duration format, found <%s>, allowed <every|in|at>", input)
		log.Print(err)
		return 0, err
	}

	//	command := matches[1]

	numericValue, err := strconv.Atoi(matches[2])
	if err != nil {
		err := fmt.Errorf("invalid <interval> value")
		log.Print(err)
		return 0, err
	}

	// TODO: add at syntax
	unit := strings.ToLower(matches[3])
	switch unit {
	case "s", "sec", "second", "seconds":
		return time.Duration(numericValue) * time.Second, nil
	case "m", "min", "minute", "minutes":
		return time.Duration(numericValue) * time.Minute, nil
	case "h", "hr", "hour", "hours":
		return time.Duration(numericValue) * time.Hour, nil
	case "d", "days":
		return time.Duration(numericValue) * time.Hour * 24, nil
	default:
		err := fmt.Errorf("invalid time unit")
		log.Print(err)
		return 0, err
	}
}

func ParseJSONFile(filePath string, result interface{}) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(data, result); err != nil {
		return err
	}

	return nil
}

func CalculateRemainingTime(startedTime *time.Time, duration time.Duration) *time.Duration {
	if startedTime == nil {
		return nil
	}
	elapsed := time.Since(*startedTime)
	remaining := duration - elapsed

	if remaining < 0 {
		remaining = 0
	}

	return &remaining
}

func Uuid() (uuid string) {

	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		fmt.Println("Error: ", err)
		return
	}

	uuid = fmt.Sprintf("%X-%X-%X-%X-%X", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])

	return
}
