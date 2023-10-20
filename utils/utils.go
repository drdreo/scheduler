package utils

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"os"
	"regexp"
	"strings"
	"time"
)

// ParseDuration parses a string schedule input into go time data.
// Supported formats are: (<every|in> 6h30m) or (at 18:00)
// If the input cannot be parsed, it returns a time.Duration of 0
func ParseDuration(input string) (time.Duration, error) {
	pattern := regexp.MustCompile(`^(every|in|at) (\d+[a-zA-Z]+|\d{2}:\d{2})$`)
	matches := pattern.FindStringSubmatch(strings.ToLower(input))

	if len(matches) != 3 {
		err := fmt.Errorf("invalid duration format, found <%s>, allowed <every|in|at>", input)
		log.Print(err)
		return 0, err
	}

	command := matches[1]
	timeValue := matches[2]

	if command == "at" {
		targetTime, err := timeUntil(timeValue)
		if err != nil {
			err = fmt.Errorf("invalid duration format, found <%s>, allowed <every|in|at>", input)
			log.Print(err)
			return 0, err
		}
		return targetTime, nil
	}

	timeValue = fixTimeUnit(timeValue)

	return time.ParseDuration(timeValue)
}

func fixTimeUnit(timeValue string) string {
	var fixedTime = timeValue

	fixedTime = strings.Replace(fixedTime, "min", "m", -1)
	fixedTime = strings.Replace(fixedTime, "sec", "s", -1)

	return fixedTime
}

func timeUntil(timeStr string) (time.Duration, error) {

	currentTime := time.Now()
	year, month, day := currentTime.Date()

	targetTime, err := time.Parse(time.DateTime, fmt.Sprintf("%d-%d-%d %s:00", year, month, day, timeStr))
	if err != nil {
		log.Println("[ERROR] Error parsing time: ", err)
		return 0, err
	}

	return targetTime.Sub(currentTime), nil
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
		log.Println("Error: ", err)
		return
	}

	uuid = fmt.Sprintf("%X-%X-%X-%X-%X", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])

	return
}

func RenderTemplate(template *template.Template, tmplName string, data interface{}) (string, error) {
	var tplContent bytes.Buffer

	err := template.ExecuteTemplate(&tplContent, tmplName, data)
	if err != nil {
		log.Fatal("err: ", err)

		return "", err
	}

	return tplContent.String(), nil
}

func RemoveSliceQck(s []interface{}, i int) []interface{} {
	s[i] = s[len(s)-1]
	return s[:len(s)-1]
}
