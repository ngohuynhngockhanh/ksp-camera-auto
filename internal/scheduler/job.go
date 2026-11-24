package scheduler

import (
	"encoding/json"
	"errors"
	"time"
)

// Job represents a configured, scheduled task.
type Job struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Enabled     bool      `json:"enabled"`
	Schedule    Schedule  `json:"schedule"`
	ActionType  string    `json:"actionType"`
	Action      Action    `json:"action"`
	NextRunAt   time.Time `json:"nextRunAt,omitempty"`
	LastRunAt   time.Time `json:"lastRunAt,omitempty"`
	LastStatus  string    `json:"lastStatus,omitempty"` // "success", "error", "running"
	LastMessage string    `json:"lastMessage,omitempty"`
	CreatedAt   time.Time `json:"createdAt,omitempty"`
	UpdatedAt   time.Time `json:"updatedAt,omitempty"`
}

type jobStorage struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Enabled     bool            `json:"enabled"`
	Schedule    Schedule        `json:"schedule"`
	ActionType  string          `json:"actionType"`
	Action      json.RawMessage `json:"action"`
	NextRunAt   time.Time       `json:"nextRunAt,omitempty"`
	LastRunAt   time.Time       `json:"lastRunAt,omitempty"`
	LastStatus  string          `json:"lastStatus,omitempty"`
	LastMessage string          `json:"lastMessage,omitempty"`
	CreatedAt   time.Time       `json:"createdAt,omitempty"`
	UpdatedAt   time.Time       `json:"updatedAt,omitempty"`
}

// MarshalJSON preserves custom serialization for Action interface.
func (j *Job) MarshalJSON() ([]byte, error) {
	var rawAction json.RawMessage
	if j.Action != nil {
		b, err := json.Marshal(j.Action)
		if err != nil {
			return nil, err
		}
		rawAction = b
	}
	actionType := j.ActionType
	if actionType == "" && j.Action != nil {
		actionType = j.Action.Type()
	}
	return json.Marshal(&jobStorage{
		ID:          j.ID,
		Name:        j.Name,
		Enabled:     j.Enabled,
		Schedule:    j.Schedule,
		ActionType:  actionType,
		Action:      rawAction,
		NextRunAt:   j.NextRunAt,
		LastRunAt:   j.LastRunAt,
		LastStatus:  j.LastStatus,
		LastMessage: j.LastMessage,
		CreatedAt:   j.CreatedAt,
		UpdatedAt:   j.UpdatedAt,
	})
}

// UnmarshalJSON reconstructs the polymorphic Action based on ActionType.
func (j *Job) UnmarshalJSON(data []byte) error {
	var js jobStorage
	if err := json.Unmarshal(data, &js); err != nil {
		return err
	}
	j.ID = js.ID
	j.Name = js.Name
	j.Enabled = js.Enabled
	j.Schedule = js.Schedule
	j.ActionType = js.ActionType
	j.NextRunAt = js.NextRunAt
	j.LastRunAt = js.LastRunAt
	j.LastStatus = js.LastStatus
	j.LastMessage = js.LastMessage
	j.CreatedAt = js.CreatedAt
	j.UpdatedAt = js.UpdatedAt

	// If action payload was directly in js.Action
	if js.ActionType != "" && len(js.Action) > 0 && string(js.Action) != "null" {
		act, err := ParseAction(js.ActionType, js.Action)
		if err != nil {
			return err
		}
		j.Action = act
	} else if js.ActionType != "" {
		// Attempt to parse action fields that might have been provided flat in data
		if act, err := ParseAction(js.ActionType, data); err == nil {
			j.Action = act
		}
	}
	return nil
}

// Clone creates a deep copy of Job.
func (j *Job) Clone() *Job {
	if j == nil {
		return nil
	}
	cp := *j
	if len(j.Schedule.DaysOfWeek) > 0 {
		days := make([]int, len(j.Schedule.DaysOfWeek))
		copy(days, j.Schedule.DaysOfWeek)
		cp.Schedule.DaysOfWeek = days
	}
	// Re-serialize/deserialize action to create independent instance
	if j.Action != nil {
		if b, err := json.Marshal(j.Action); err == nil {
			if act, err := ParseAction(j.Action.Type(), b); err == nil {
				cp.Action = act
			}
		}
	}
	return &cp
}

// Validate checks the job integrity.
func (j *Job) Validate() error {
	if j.ID == "" {
		return errors.New("job ID cannot be empty")
	}
	if j.Name == "" {
		return errors.New("job name cannot be empty")
	}
	if err := j.Schedule.Validate(); err != nil {
		return err
	}
	if j.Action == nil {
		return errors.New("job action cannot be nil")
	}
	if err := j.Action.Validate(); err != nil {
		return err
	}
	return nil
}

// ExecutionRecord records the outcome of a single job run in the circular history log.
type ExecutionRecord struct {
	ID          string        `json:"id"`
	JobID       string        `json:"jobId"`
	JobName     string        `json:"jobName"`
	ActionType  string        `json:"actionType"`
	TriggeredAt time.Time     `json:"triggeredAt"`
	Duration    time.Duration `json:"duration"`
	DurationMs  int64         `json:"durationMs"`
	Status      string        `json:"status"` // "success" | "error"
	Message     string        `json:"message"`
	Error       string        `json:"error,omitempty"`
}
