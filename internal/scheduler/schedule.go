package scheduler

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Schedule defines when a job runs: daily at HH:MM, optionally filtered by days of the week.
type Schedule struct {
	Time       string `json:"time"`                 // "HH:MM" in 24-hour format, e.g. "07:00"
	DaysOfWeek []int  `json:"daysOfWeek,omitempty"` // 0=Sunday, 1=Monday... 6=Saturday. Empty = every day
}

// UnmarshalJSON handles both integer arrays and string representations of days of week.
func (s *Schedule) UnmarshalJSON(data []byte) error {
	var raw struct {
		Time       string `json:"time"`
		DaysOfWeek any    `json:"daysOfWeek"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	s.Time = strings.TrimSpace(raw.Time)
	s.DaysOfWeek = parseDaysOfWeek(raw.DaysOfWeek)
	return nil
}

func parseDaysOfWeek(v any) []int {
	if v == nil {
		return nil
	}
	var res []int
	switch list := v.(type) {
	case []any:
		for _, item := range list {
			if d, ok := parseSingleDay(item); ok {
				res = append(res, d)
			}
		}
	case []int:
		for _, d := range list {
			res = append(res, normalizeDayInt(d))
		}
	case []float64:
		for _, d := range list {
			res = append(res, normalizeDayInt(int(d)))
		}
	case []string:
		for _, str := range list {
			if d, ok := parseDayString(str); ok {
				res = append(res, d)
			}
		}
	case string:
		parts := strings.Split(list, ",")
		for _, p := range parts {
			if d, ok := parseDayString(strings.TrimSpace(p)); ok {
				res = append(res, d)
			}
		}
	}

	return cleanAndDedupeDays(res)
}

func parseSingleDay(v any) (int, bool) {
	switch val := v.(type) {
	case int:
		return normalizeDayInt(val), true
	case int64:
		return normalizeDayInt(int(val)), true
	case float64:
		return normalizeDayInt(int(val)), true
	case string:
		return parseDayString(val)
	}
	return 0, false
}

func parseDayString(s string) (int, bool) {
	s = strings.TrimSpace(strings.ToLower(s))
	if n, err := strconv.Atoi(s); err == nil {
		return normalizeDayInt(n), true
	}
	switch s {
	case "sun", "sunday", "cn", "chủ nhật":
		return 0, true
	case "mon", "monday", "t2", "thứ hai":
		return 1, true
	case "tue", "tuesday", "t3", "thứ ba":
		return 2, true
	case "wed", "wednesday", "t4", "thứ tư":
		return 3, true
	case "thu", "thursday", "t5", "thứ năm":
		return 4, true
	case "fri", "friday", "t6", "thứ sáu":
		return 5, true
	case "sat", "saturday", "t7", "thứ bảy":
		return 6, true
	}
	return 0, false
}

func normalizeDayInt(d int) int {
	if d == 7 {
		return 0 // Sunday in ISO/cron is 7 or 0
	}
	return d % 7
}

func cleanAndDedupeDays(days []int) []int {
	if len(days) == 0 {
		return nil
	}
	seen := make(map[int]bool)
	var out []int
	for _, d := range days {
		nd := normalizeDayInt(d)
		if nd >= 0 && nd <= 6 && !seen[nd] {
			seen[nd] = true
			out = append(out, nd)
		}
	}
	sort.Ints(out)
	if len(out) == 7 {
		return nil // all 7 days == every day
	}
	return out
}

// Validate checks that Time is valid HH:MM and days of week are in 0..6.
func (s Schedule) Validate() error {
	if s.Time == "" {
		return errors.New("schedule time is required (format HH:MM)")
	}
	t, err := time.Parse("15:04", s.Time)
	if err != nil {
		return fmt.Errorf("invalid schedule time %q: must be HH:MM in 24h format", s.Time)
	}
	_ = t
	for _, d := range s.DaysOfWeek {
		if d < 0 || d > 6 {
			return fmt.Errorf("invalid day of week: %d (must be 0=Sunday..6=Saturday)", d)
		}
	}
	return nil
}

// RunsOn returns whether the schedule applies to a given weekday.
func (s Schedule) RunsOn(day time.Weekday) bool {
	if len(s.DaysOfWeek) == 0 {
		return true
	}
	target := int(day)
	for _, d := range s.DaysOfWeek {
		if d == target {
			return true
		}
	}
	return false
}

// NextRun calculates the earliest occurrence of this schedule strictly after the 'from' timestamp.
func (s Schedule) NextRun(from time.Time) time.Time {
	if from.IsZero() {
		from = time.Now()
	}
	t, err := time.Parse("15:04", s.Time)
	if err != nil {
		return time.Time{}
	}
	targetHour := t.Hour()
	targetMin := t.Minute()
	loc := from.Location()

	for offset := range 15 {
		candidateDate := from.AddDate(0, 0, offset)
		candidate := time.Date(candidateDate.Year(), candidateDate.Month(), candidateDate.Day(), targetHour, targetMin, 0, 0, loc)
		if candidate.After(from) && s.RunsOn(candidate.Weekday()) {
			return candidate
		}
	}
	return time.Time{}
}

// PrevRun calculates the latest scheduled occurrence strictly before or at the 'from' timestamp (looking back up to 15 days).
func (s Schedule) PrevRun(from time.Time) time.Time {
	if from.IsZero() {
		from = time.Now()
	}
	t, err := time.Parse("15:04", s.Time)
	if err != nil {
		return time.Time{}
	}
	targetHour := t.Hour()
	targetMin := t.Minute()
	loc := from.Location()

	for offset := 0; offset <= 15; offset++ {
		candidateDate := from.AddDate(0, 0, -offset)
		candidate := time.Date(candidateDate.Year(), candidateDate.Month(), candidateDate.Day(), targetHour, targetMin, 0, 0, loc)
		if !candidate.After(from) && s.RunsOn(candidate.Weekday()) {
			return candidate
		}
	}
	return time.Time{}
}

// Summary returns a friendly human string, e.g. "Daily at 07:00" or "Mon, Wed, Fri at 17:00".
func (s Schedule) Summary() string {
	if len(s.DaysOfWeek) == 0 {
		return fmt.Sprintf("Daily at %s", s.Time)
	}
	if len(s.DaysOfWeek) == 5 && s.DaysOfWeek[0] == 1 && s.DaysOfWeek[4] == 5 {
		return fmt.Sprintf("Mon-Fri at %s", s.Time)
	}
	if len(s.DaysOfWeek) == 2 && s.DaysOfWeek[0] == 0 && s.DaysOfWeek[1] == 6 {
		return fmt.Sprintf("Weekends at %s", s.Time)
	}
	names := []string{"Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"}
	var dayStr []string
	for _, d := range s.DaysOfWeek {
		if d >= 0 && d < len(names) {
			dayStr = append(dayStr, names[d])
		}
	}
	return fmt.Sprintf("%s at %s", strings.Join(dayStr, ", "), s.Time)
}
