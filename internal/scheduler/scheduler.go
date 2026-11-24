package scheduler

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/config"
)

const (
	// MaxHistoryEntries is the capacity of the circular history log.
	MaxHistoryEntries = 50

	defaultTimeout      = 15 * time.Second
	defaultTickInterval = 5 * time.Second
	defaultStorePath    = "schedules.json"
)

// Option configures Scheduler options.
type Option func(*Scheduler)

// WithTimeout sets the execution timeout per action.
func WithTimeout(d time.Duration) Option {
	return func(s *Scheduler) {
		if d > 0 {
			s.timeout = d
		}
	}
}

// WithTickInterval sets the ticker evaluation interval.
func WithTickInterval(d time.Duration) Option {
	return func(s *Scheduler) {
		if d > 0 {
			s.tickInterval = d
		}
	}
}

// WithClockCheck overrides the clock synchronization check function (useful in tests).
func WithClockCheck(fn func() bool) Option {
	return func(s *Scheduler) {
		s.clockCheckFn = fn
	}
}

// Scheduler manages persistent scheduled camera jobs, ticker evaluation, and execution logs.
type Scheduler struct {
	mu           sync.RWMutex
	filePath     string
	inv          *config.Inventory
	timeout      time.Duration
	tickInterval time.Duration

	jobs        map[string]*Job
	jobsOrder   []string
	history     []ExecutionRecord
	runningJobs map[string]bool

	clockCheckFn func() bool
	reconciled   bool
	lastTickTime time.Time

	running bool
	stopCh  chan struct{}
	doneCh  chan struct{}
	execWg  sync.WaitGroup
}

type persistedStore struct {
	Version int               `json:"version,omitempty"`
	Jobs    []*Job            `json:"jobs"`
	History []ExecutionRecord `json:"history,omitempty"`
}

// NewScheduler creates and loads a Scheduler instance. If store is empty, it seeds default jobs.
func NewScheduler(filePath string, inv *config.Inventory, opts ...Option) (*Scheduler, error) {
	if strings.TrimSpace(filePath) == "" {
		filePath = defaultStorePath
	}

	s := &Scheduler{
		filePath:     filePath,
		inv:          inv,
		timeout:      defaultTimeout,
		tickInterval: defaultTickInterval,
		jobs:         make(map[string]*Job),
		jobsOrder:    make([]string, 0),
		history:      make([]ExecutionRecord, 0),
		runningJobs:  make(map[string]bool),
	}

	for _, opt := range opts {
		opt(s)
	}

	if err := s.load(); err != nil {
		return nil, err
	}

	return s, nil
}

// SetInventory updates the inventory pointer dynamically.
func (s *Scheduler) SetInventory(inv *config.Inventory) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.inv = inv
}

// Start initiates the scheduler ticker loop.
func (s *Scheduler) Start() {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.stopCh = make(chan struct{})
	s.doneCh = make(chan struct{})
	s.mu.Unlock()

	go s.loop()
}

// Stop shuts down the scheduler ticker loop gracefully.
func (s *Scheduler) Stop() {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return
	}
	s.running = false
	close(s.stopCh)
	done := s.doneCh
	s.mu.Unlock()

	if done != nil {
		<-done
	}
	s.execWg.Wait()
}

// IsRunning reports whether the ticker loop is running.
func (s *Scheduler) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

func (s *Scheduler) loop() {
	defer close(s.doneCh)
	ticker := time.NewTicker(s.tickInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case now := <-ticker.C:
			s.tick(now)
		}
	}
}

func (s *Scheduler) tick(now time.Time) {
	s.mu.Lock()

	// Check for clock jumps (e.g. NTP sync after boot)
	if !s.lastTickTime.IsZero() {
		diff := now.Sub(s.lastTickTime)
		if diff < -10*time.Second || diff > 60*time.Second {
			s.recalculateNextRunsLocked(now)
			s.reconcileStateLocked(now)
		}
	}
	s.lastTickTime = now

	// Initial reconciliation when system clock is synchronized
	if !s.reconciled && s.isClockReady(now) {
		s.reconciled = true
		s.recalculateNextRunsLocked(now)
		s.reconcileStateLocked(now)
	}

	var dueJobs []*Job
	for _, id := range s.jobsOrder {
		job := s.jobs[id]
		if job == nil || !job.Enabled {
			continue
		}
		if job.NextRunAt.IsZero() {
			job.NextRunAt = job.Schedule.NextRun(now)
			continue
		}
		if !now.Before(job.NextRunAt) {
			dueJobs = append(dueJobs, job.Clone())
			job.NextRunAt = job.Schedule.NextRun(now)
			job.LastRunAt = now
			job.LastStatus = "running"
			job.UpdatedAt = now
		}
	}
	if len(dueJobs) > 0 {
		_ = s.saveLocked()
	}
	s.mu.Unlock()

	for _, j := range dueJobs {
		s.execWg.Add(1)
		go func(job *Job) {
			defer s.execWg.Done()
			s.executeJob(job, 1)
		}(j)
	}
}

func (s *Scheduler) isClockReady(now time.Time) bool {
	if s.clockCheckFn != nil {
		return s.clockCheckFn()
	}
	if now.Year() < 2025 {
		return false
	}
	// Query timedatectl if present
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "timedatectl", "show", "-p", "NTPSynchronized", "--value").Output()
	if err == nil && strings.TrimSpace(string(out)) == "yes" {
		return true
	}
	return true
}

func (s *Scheduler) recalculateNextRunsLocked(now time.Time) {
	for _, j := range s.jobs {
		if j != nil && j.Enabled {
			j.NextRunAt = j.Schedule.NextRun(now)
		}
	}
}

func (s *Scheduler) reconcileStateLocked(now time.Time) {
	// Group reconcilable jobs by state key and find the most recent past run slot
	type candidateSlot struct {
		job     *Job
		prevRun time.Time
	}
	stateSlots := make(map[string]candidateSlot)

	for _, id := range s.jobsOrder {
		job := s.jobs[id]
		if job == nil || !job.Enabled {
			continue
		}
		reconcilable, ok := job.Action.(ReconcilableAction)
		if !ok {
			continue
		}
		keys := reconcilable.StateGroupKeys(s.inv)
		if len(keys) == 0 {
			continue
		}
		prev := job.Schedule.PrevRun(now)
		if prev.IsZero() {
			continue
		}

		for _, k := range keys {
			existing, exists := stateSlots[k]
			if !exists || prev.After(existing.prevRun) {
				stateSlots[k] = candidateSlot{job: job, prevRun: prev}
			}
		}
	}

	seenJobs := make(map[string]bool)
	for _, slot := range stateSlots {
		j := slot.job
		if seenJobs[j.ID] {
			continue
		}
		// If job has never run, or its LastRunAt was before the scheduled slot
		if j.LastRunAt.IsZero() || j.LastRunAt.Before(slot.prevRun) {
			seenJobs[j.ID] = true
			clone := j.Clone()
			j.LastRunAt = now
			j.LastStatus = "running"
			j.UpdatedAt = now
			s.execWg.Add(1)
			go func(job *Job) {
				defer s.execWg.Done()
				s.executeJob(job, 1)
			}(clone)
		}
	}
}

func (s *Scheduler) executeJob(job *Job, attempt int) {
	s.mu.Lock()
	if s.runningJobs[job.ID] {
		s.mu.Unlock()
		return
	}
	s.runningJobs[job.ID] = true
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		delete(s.runningJobs, job.ID)
		s.mu.Unlock()
	}()

	// Calculate overall timeout proportional to number of targets
	timeout := s.timeout
	if targeted, ok := job.Action.(TargetedAction); ok {
		if n := len(targeted.TargetDevices()); n > 1 {
			timeout = s.timeout + time.Duration(n-1)*8*time.Second
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	start := time.Now()
	summary, err := job.Action.Execute(ctx, s.inv, timeout)
	duration := time.Since(start)

	// Transient failure retry once after 2s backoff if not stopped
	if err != nil && attempt < 2 {
		select {
		case <-s.stopCh:
			return
		case <-time.After(2 * time.Second):
			s.executeJob(job, attempt+1)
			return
		}
	}

	rec := ExecutionRecord{
		ID:          fmt.Sprintf("exec-%d-%s", start.Unix(), randID(4)),
		JobID:       job.ID,
		JobName:     job.Name,
		ActionType:  job.ActionType,
		TriggeredAt: start,
		Duration:    duration,
		DurationMs:  duration.Milliseconds(),
		Status:      "success",
		Message:     summary,
	}
	if err != nil {
		rec.Status = "error"
		rec.Error = err.Error()
		if summary == "" {
			rec.Message = err.Error()
		}
	}

	s.mu.Lock()
	if current, ok := s.jobs[job.ID]; ok {
		current.LastRunAt = start
		current.LastStatus = rec.Status
		current.LastMessage = rec.Message
		current.UpdatedAt = time.Now()
	}
	s.addHistoryLocked(rec)
	_ = s.saveLocked()
	s.mu.Unlock()
}

// ListJobs returns all jobs in order.
func (s *Scheduler) ListJobs() []*Job {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*Job, 0, len(s.jobsOrder))
	for _, id := range s.jobsOrder {
		if j, ok := s.jobs[id]; ok {
			out = append(out, j.Clone())
		}
	}
	return out
}

// GetJob finds a job by ID.
func (s *Scheduler) GetJob(id string) (*Job, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	j, ok := s.jobs[id]
	if !ok {
		return nil, false
	}
	return j.Clone(), true
}

// AddJob registers a new job, persists to schedules.json, and recalculates NextRunAt.
func (s *Scheduler) AddJob(job *Job) error {
	if job == nil {
		return errors.New("scheduler: job cannot be nil")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if strings.TrimSpace(job.ID) == "" {
		job.ID = fmt.Sprintf("job-%d-%s", time.Now().Unix(), randID(4))
	}
	if strings.TrimSpace(job.Name) == "" {
		job.Name = fmt.Sprintf("Job %s", job.ID)
	}

	if _, exists := s.jobs[job.ID]; exists {
		return fmt.Errorf("scheduler: job with ID %q already exists", job.ID)
	}

	if job.Action == nil {
		return errors.New("scheduler: action cannot be nil")
	}
	if err := job.Action.Validate(); err != nil {
		return fmt.Errorf("scheduler: action invalid: %w", err)
	}
	job.ActionType = job.Action.Type()

	if err := job.Schedule.Validate(); err != nil {
		return fmt.Errorf("scheduler: schedule invalid: %w", err)
	}

	now := time.Now()
	job.CreatedAt = now
	job.UpdatedAt = now
	if job.Enabled {
		job.NextRunAt = job.Schedule.NextRun(now)
	} else {
		job.NextRunAt = time.Time{}
	}

	s.jobs[job.ID] = job.Clone()
	s.jobsOrder = append(s.jobsOrder, job.ID)

	return s.saveLocked()
}

// UpdateJob updates an existing job and persists to schedules.json.
func (s *Scheduler) UpdateJob(job *Job) error {
	if job == nil {
		return errors.New("scheduler: job cannot be nil")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	existing, ok := s.jobs[job.ID]
	if !ok {
		return fmt.Errorf("scheduler: job %q not found", job.ID)
	}

	if job.Action == nil {
		return errors.New("scheduler: action cannot be nil")
	}
	if err := job.Action.Validate(); err != nil {
		return fmt.Errorf("scheduler: action invalid: %w", err)
	}
	job.ActionType = job.Action.Type()

	if err := job.Schedule.Validate(); err != nil {
		return fmt.Errorf("scheduler: schedule invalid: %w", err)
	}

	now := time.Now()
	existing.Name = job.Name
	existing.Enabled = job.Enabled
	existing.Schedule = job.Schedule
	existing.ActionType = job.ActionType
	existing.Action = job.Action
	existing.UpdatedAt = now

	if existing.Enabled {
		existing.NextRunAt = existing.Schedule.NextRun(now)
	} else {
		existing.NextRunAt = time.Time{}
	}

	return s.saveLocked()
}

// DeleteJob deletes a job by ID and persists to schedules.json.
func (s *Scheduler) DeleteJob(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.jobs[id]; !ok {
		return fmt.Errorf("scheduler: job %q not found", id)
	}

	delete(s.jobs, id)
	newOrder := make([]string, 0, len(s.jobsOrder)-1)
	for _, oid := range s.jobsOrder {
		if oid != id {
			newOrder = append(newOrder, oid)
		}
	}
	s.jobsOrder = newOrder

	return s.saveLocked()
}

// ToggleJob flips or sets the Enabled status of a job.
func (s *Scheduler) ToggleJob(id string, enabled ...bool) (*Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	job, ok := s.jobs[id]
	if !ok {
		return nil, fmt.Errorf("scheduler: job %q not found", id)
	}

	if len(enabled) > 0 {
		job.Enabled = enabled[0]
	} else {
		job.Enabled = !job.Enabled
	}

	now := time.Now()
	job.UpdatedAt = now
	if job.Enabled {
		job.NextRunAt = job.Schedule.NextRun(now)
	} else {
		job.NextRunAt = time.Time{}
	}

	if err := s.saveLocked(); err != nil {
		return nil, err
	}

	return job.Clone(), nil
}

// RunJobNow triggers immediate execution of the specified job.
func (s *Scheduler) RunJobNow(ctx context.Context, id string) (*ExecutionRecord, error) {
	s.mu.RLock()
	job, ok := s.jobs[id]
	if !ok {
		s.mu.RUnlock()
		return nil, fmt.Errorf("scheduler: job %q not found", id)
	}
	jobCopy := job.Clone()
	s.mu.RUnlock()

	if ctx == nil {
		ctx = context.Background()
	}
	runTimeout := s.timeout
	if targeted, ok := jobCopy.Action.(TargetedAction); ok {
		if n := len(targeted.TargetDevices()); n > 1 {
			runTimeout = s.timeout + time.Duration(n-1)*8*time.Second
		}
	}
	cctx, cancel := context.WithTimeout(ctx, runTimeout)
	defer cancel()

	start := time.Now()
	summary, err := jobCopy.Action.Execute(cctx, s.inv, runTimeout)
	duration := time.Since(start)
	rec := ExecutionRecord{
		ID:          fmt.Sprintf("exec-%d-%s", start.Unix(), randID(4)),
		JobID:       jobCopy.ID,
		JobName:     jobCopy.Name,
		ActionType:  jobCopy.ActionType,
		TriggeredAt: start,
		Duration:    duration,
		DurationMs:  duration.Milliseconds(),
		Status:      "success",
		Message:     summary,
	}
	if err != nil {
		rec.Status = "error"
		rec.Error = err.Error()
		if summary == "" {
			rec.Message = err.Error()
		}
	}

	s.mu.Lock()
	if current, exists := s.jobs[id]; exists {
		current.LastRunAt = start
		current.LastStatus = rec.Status
		current.LastMessage = rec.Message
		current.UpdatedAt = time.Now()
	}
	s.addHistoryLocked(rec)
	_ = s.saveLocked()
	s.mu.Unlock()

	return &rec, err
}

// History returns a copy of recent execution records, newest first (up to MaxHistoryEntries).
func (s *Scheduler) History() []ExecutionRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]ExecutionRecord, len(s.history))
	copy(out, s.history)
	return out
}

// ClearHistory resets the execution history log.
func (s *Scheduler) ClearHistory() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.history = make([]ExecutionRecord, 0)
	return s.saveLocked()
}

func (s *Scheduler) addHistoryLocked(rec ExecutionRecord) {
	s.history = append([]ExecutionRecord{rec}, s.history...)
	if len(s.history) > MaxHistoryEntries {
		s.history = s.history[:MaxHistoryEntries]
	}
}

func (s *Scheduler) load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.filePath)
	if errors.Is(err, os.ErrNotExist) || len(strings.TrimSpace(string(data))) == 0 {
		// Clean empty initialization: never automatically seed arbitrary jobs across fleet
		s.jobs = make(map[string]*Job)
		s.jobsOrder = make([]string, 0)
		s.history = make([]ExecutionRecord, 0)
		return s.saveLocked()
	} else if err != nil {
		return fmt.Errorf("scheduler: read %s: %w", s.filePath, err)
	}

	// Try unmarshaling as persistedStore first
	var store persistedStore
	if err := json.Unmarshal(data, &store); err == nil && (len(store.Jobs) > 0 || len(store.History) > 0) {
		s.history = store.History
		if len(s.history) > MaxHistoryEntries {
			s.history = s.history[:MaxHistoryEntries]
		}
		for _, j := range store.Jobs {
			if j != nil {
				s.jobs[j.ID] = j
				s.jobsOrder = append(s.jobsOrder, j.ID)
			}
		}
	} else {
		// Fallback: try raw array of []*Job
		var jobsList []*Job
		if err := json.Unmarshal(data, &jobsList); err == nil && len(jobsList) > 0 {
			for _, j := range jobsList {
				if j != nil {
					s.jobs[j.ID] = j
					s.jobsOrder = append(s.jobsOrder, j.ID)
				}
			}
		}
	}

	// Recalculate NextRunAt for enabled jobs if expired
	now := time.Now()
	for _, j := range s.jobs {
		if j.Enabled && (j.NextRunAt.IsZero() || j.NextRunAt.Before(now)) {
			j.NextRunAt = j.Schedule.NextRun(now)
		}
	}

	return nil
}

// SeedDefault4168Jobs explicitly populates the 07:00 Full-Color and 17:00 Auto schedules
// designed for node 4.168 (Camera02 and Camera03). Used for initial onboarding or test harness.
func (s *Scheduler) SeedDefault4168Jobs() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.seedDefaultJobsLocked()
}

func (s *Scheduler) seedDefaultJobsLocked() error {
	cam2and3 := findCam2And3(s.inv)
	now := time.Now()

	job1Schedule := Schedule{Time: "07:00"}
	job1 := &Job{
		ID:         "job-0700-fullcolor",
		Name:       "07:00 Cam 2 & 3 Full-Color",
		Enabled:    true,
		Schedule:   job1Schedule,
		ActionType: ActionTypePictureDayNight,
		Action: &PictureDayNightAction{
			DeviceIDs:     cam2and3,
			Channel:       0,
			DayNightColor: DayNightFullColor,
		},
		NextRunAt: job1Schedule.NextRun(now),
		CreatedAt: now,
		UpdatedAt: now,
	}

	job2Schedule := Schedule{Time: "17:00"}
	job2 := &Job{
		ID:         "job-1700-auto",
		Name:       "17:00 Cam 2 & 3 Auto",
		Enabled:    true,
		Schedule:   job2Schedule,
		ActionType: ActionTypePictureDayNight,
		Action: &PictureDayNightAction{
			DeviceIDs:     cam2and3,
			Channel:       0,
			DayNightColor: DayNightAuto,
		},
		NextRunAt: job2Schedule.NextRun(now),
		CreatedAt: now,
		UpdatedAt: now,
	}

	s.jobs = map[string]*Job{
		job1.ID: job1,
		job2.ID: job2,
	}
	s.jobsOrder = []string{job1.ID, job2.ID}

	return s.saveLocked()
}

func (s *Scheduler) saveLocked() error {
	store := persistedStore{
		Version: 1,
		Jobs:    make([]*Job, 0, len(s.jobsOrder)),
		History: s.history,
	}
	for _, id := range s.jobsOrder {
		if j, ok := s.jobs[id]; ok {
			store.Jobs = append(store.Jobs, j)
		}
	}

	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return fmt.Errorf("scheduler: marshal store: %w", err)
	}

	dir := filepath.Dir(s.filePath)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("scheduler: mkdir %s: %w", dir, err)
		}
	}

	tmpPath := fmt.Sprintf("%s.tmp.%d", s.filePath, time.Now().UnixNano())
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("scheduler: write temp file %s: %w", tmpPath, err)
	}
	if err := os.Rename(tmpPath, s.filePath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("scheduler: rename %s -> %s: %w", tmpPath, s.filePath, err)
	}

	return nil
}

func findCam2And3(inv *config.Inventory) []string {
	var cam2, cam3 string
	if inv != nil {
		for _, d := range inv.List() {
			nameLower := strings.ToLower(d.Name)
			idLower := strings.ToLower(d.ID)
			hostLower := strings.ToLower(d.Host)

			// Match Cam 2: e.g. "Camera02", "Cam 2", ".202:37777", ".202"
			if cam2 == "" && (strings.Contains(nameLower, "cam 2") || strings.Contains(nameLower, "cam02") ||
				strings.Contains(nameLower, "camera02") || strings.Contains(nameLower, "camera 2") ||
				strings.Contains(idLower, ".202:") || strings.Contains(hostLower, ".202") ||
				strings.HasSuffix(idLower, "-c2") || strings.HasSuffix(idLower, ":2")) {
				cam2 = d.ID
			}

			// Match Cam 3: e.g. "Camera03", "Cam 3", ".203:37777", ".203"
			if cam3 == "" && (strings.Contains(nameLower, "cam 3") || strings.Contains(nameLower, "cam03") ||
				strings.Contains(nameLower, "camera03") || strings.Contains(nameLower, "camera 3") ||
				strings.Contains(idLower, ".203:") || strings.Contains(hostLower, ".203") ||
				strings.HasSuffix(idLower, "-c3") || strings.HasSuffix(idLower, ":3")) {
				cam3 = d.ID
			}
		}
	}

	if cam2 == "" {
		cam2 = "192.168.1.202:37777"
	}
	if cam3 == "" {
		cam3 = "192.168.1.203:37777"
	}
	return []string{cam2, cam3}
}

func randID(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
