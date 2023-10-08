package utils

import (
	"encoding/json"
	"fmt"
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
