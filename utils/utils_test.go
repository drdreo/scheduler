package utils

import (
	"testing"
)

func TestParseDuration_Every_6s(t *testing.T) {
	dur, _ := ParseDuration("every 6s")
	if dur.Seconds() != 6 {
		t.Errorf("Duration was incorrect, got: %d, want: %d.", dur, 6)
	}
}

func TestParseDuration_In_6s(t *testing.T) {
	dur, _ := ParseDuration("in 6s")
	if dur.Seconds() != 6 {
		t.Errorf("Duration was incorrect, got: %d, want: %d.", dur, 6)
	}
}

func TestParseDuration_At_1800(t *testing.T) {
	// THIS COULD BE FLAKY
	dur, _ := ParseDuration("at 18:00")
	if dur == 0 {
		t.Errorf("Parsed duration was 0")
	}
}

func TestParseDuration_Every_30s(t *testing.T) {
	dur, _ := ParseDuration("every 30s")
	if dur.Seconds() != 30 {
		t.Errorf("Duration was incorrect, got: %d, want: %d.", dur, 30)
	}
}

func TestParseDuration_Every_15min(t *testing.T) {
	dur, _ := ParseDuration("every 15min")
	if dur.Minutes() != 15 {
		t.Errorf("Duration was incorrect, got: %d, want: %d.", dur, 15)
	}
}

func TestParseDuration_Every_1h(t *testing.T) {
	dur, _ := ParseDuration("every 1h")
	if dur.Minutes() != 60 {
		t.Errorf("Duration was incorrect, got: %d, want: %d.", dur, 60)
	}
}

//
//func TestTest(t *testing.T) {
//	test, _ := time.ParseDuration("at 18:00")
//	if test == 0 {
//		t.Errorf("Parsed duration was 0")
//	}
//}
