// Package scheduler manages concurrent task execution with a worker pool.
package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/robfig/cron/v3"

	"github.com/bingdork/bingdork/internal/core"
	"github.com/bingdork/bingdork/internal/logger"
)

// Task represents a unit of work for the scheduler.
type Task struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type"`
	Query     *core.SearchQuery      `json:"query,omitempty"`
	Payload   map[string]interface{} `json:"payload,omitempty"`
	Priority  int                    `json:"priority"`
	CreatedAt time.Time              `json:"created_at"`
	State     TaskState              `json:"state"`
	Retries   int                    `json:"retries"`
	MaxRetry  int                    `json:"max_retry"`
	Result    interface{}            `json:"result,omitempty"`
	Error     string                 `json:"error,omitempty"`
}

// TaskState represents the current state of a task.
type TaskState string

const (
	TaskPending    TaskState = "pending"
	TaskRunning    TaskState = "running"
	TaskCompleted  TaskState = "completed"
	TaskFailed     TaskState = "failed"
	TaskCancelled  TaskState = "cancelled"
	TaskRetrying   TaskState = "retrying"
)

// TaskHandler is a function that processes a task.
type TaskHandler func(ctx context.Context, task *Task) error

// Scheduler manages task execution with a configurable worker pool.
type Scheduler struct {
	cfg        *core.SchedulerConfig
	log        *logger.Logger
	handlers   map[string]TaskHandler
	workerCh   chan *Task
	doneCh     chan struct{}
	stopCh     chan struct{}
	wg         sync.WaitGroup
	mu         sync.RWMutex

	// Task tracking
	pending   []*Task
	running   map[string]*Task
	completed []*Task
	failed    []*Task

	// Stats
	tasksQueued     int64
	tasksCompleted  int64
	tasksFailed     int64
	tasksCancelled  int64

	// Cron
	cron     *cron.Cron
	cronJobs map[string]cron.EntryID

	// Persistence
	stateFile string
}

// New creates a new Scheduler.
func New(cfg *core.SchedulerConfig, log *logger.Logger) (*Scheduler, error) {
	s := &Scheduler{
		cfg:      cfg,
		log:      log,
		handlers: make(map[string]TaskHandler),
		workerCh: make(chan *Task, cfg.QueueSize),
		doneCh:   make(chan struct{}),
		stopCh:   make(chan struct{}),
		running:  make(map[string]*Task),
		stateFile: cfg.StateFile,
		cron:     cron.New(),
		cronJobs: make(map[string]cron.EntryID),
	}

	// Load persisted state if resume is enabled
	if cfg.Resume && cfg.StateFile != "" {
		if err := s.loadState(); err != nil {
			log.Warn("failed to load scheduler state", logger.LogFields{"error": err})
		}
	}

	return s, nil
}

// Start launches the worker pool.
func (s *Scheduler) Start(ctx context.Context) {
	s.log.Info("starting scheduler",
		logger.LogFields{
			"workers": s.cfg.Workers,
			"queue":   s.cfg.QueueSize,
		})

	// Start workers
	for i := 0; i < s.cfg.Workers; i++ {
		s.wg.Add(1)
		go s.worker(ctx, i)
	}

	// Start cron scheduler
	s.cron.Start()

	// Re-queue pending tasks from state
	s.mu.Lock()
	for _, task := range s.pending {
		if task.State == TaskPending {
			s.enqueueTask(task)
		}
	}
	s.mu.Unlock()

	s.log.Info("scheduler started")
}

// Stop gracefully shuts down the scheduler.
func (s *Scheduler) Stop() {
	s.log.Info("stopping scheduler")

	// Stop cron
	s.cron.Stop()

	// Signal workers to stop
	close(s.stopCh)

	// Wait for all workers to finish
	s.wg.Wait()
	close(s.doneCh)

	// Save state
	if s.cfg.Resume && s.stateFile != "" {
		if err := s.saveState(); err != nil {
			s.log.Error("failed to save scheduler state", err)
		}
	}

	s.log.Info("scheduler stopped",
		logger.LogFields{
			"completed": atomic.LoadInt64(&s.tasksCompleted),
			"failed":    atomic.LoadInt64(&s.tasksFailed),
			"cancelled": atomic.LoadInt64(&s.tasksCancelled),
		})
}

// Submit adds a task to the scheduler.
func (s *Scheduler) Submit(task *Task) (string, error) {
	if task.ID == "" {
		task.ID = uuid.New().String()
	}
	task.CreatedAt = time.Now()
	task.State = TaskPending

	s.mu.Lock()
	s.pending = append(s.pending, task)
	atomic.AddInt64(&s.tasksQueued, 1)
	s.mu.Unlock()

	s.enqueueTask(task)

	s.log.Debug("task submitted", logger.LogFields{
		"task_id": task.ID,
		"type":    task.Type,
		"query":   task.Query.Query,
	})

	return task.ID, nil
}

// SubmitQuery is a convenience method for search query tasks.
func (s *Scheduler) SubmitQuery(ctx context.Context, query *core.SearchQuery) (string, error) {
	task := &Task{
		ID:   uuid.New().String(),
		Type: "search",
		Query: query,
		Priority: 1,
		MaxRetry: s.cfg.RetryCount,
	}
	return s.Submit(task)
}

// RegisterHandler registers a handler for a task type.
func (s *Scheduler) RegisterHandler(taskType string, handler TaskHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handlers[taskType] = handler
	s.log.Debug("handler registered", logger.LogFields{"type": taskType})
}

// Schedule adds a recurring task via cron expression.
func (s *Scheduler) Schedule(expr string, task *Task) (string, error) {
	entryID, err := s.cron.AddFunc(expr, func() {
		_, err := s.Submit(task)
		if err != nil {
			s.log.Error("cron task submission failed", err)
		}
	})
	if err != nil {
		return "", fmt.Errorf("invalid cron expression: %w", err)
	}

	id := uuid.New().String()
	s.mu.Lock()
	s.cronJobs[id] = entryID
	s.mu.Unlock()

	s.log.Info("cron job scheduled", logger.LogFields{
		"id":   id,
		"expr": expr,
		"task": task.Type,
	})

	return id, nil
}

// Unschedule removes a cron job.
func (s *Scheduler) Unschedule(id string) {
	s.mu.RLock()
	entryID, ok := s.cronJobs[id]
	s.mu.RUnlock()
	if ok {
		s.cron.Remove(entryID)
		s.mu.Lock()
		delete(s.cronJobs, id)
		s.mu.Unlock()
	}
}

// Status returns current scheduler status.
func (s *Scheduler) Status() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return map[string]interface{}{
		"workers":          s.cfg.Workers,
		"queue_size":       len(s.workerCh),
		"pending":          len(s.pending),
		"running":          len(s.running),
		"completed":        len(s.completed),
		"failed":           len(s.failed),
		"total_queued":     atomic.LoadInt64(&s.tasksQueued),
		"total_completed":  atomic.LoadInt64(&s.tasksCompleted),
		"total_failed":     atomic.LoadInt64(&s.tasksFailed),
		"total_cancelled":  atomic.LoadInt64(&s.tasksCancelled),
	}
}

// Cancel cancels a task by ID.
func (s *Scheduler) Cancel(taskID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if task, ok := s.running[taskID]; ok {
		task.State = TaskCancelled
		delete(s.running, taskID)
		atomic.AddInt64(&s.tasksCancelled, 1)
		return true
	}
	for i, task := range s.pending {
		if task.ID == taskID {
			task.State = TaskCancelled
			s.pending = append(s.pending[:i], s.pending[i+1:]...)
			atomic.AddInt64(&s.tasksCancelled, 1)
			return true
		}
	}
	return false
}

// enqueueTask sends a task to the worker channel (non-blocking with priority).
func (s *Scheduler) enqueueTask(task *Task) {
	go func() {
		select {
		case s.workerCh <- task:
		default:
			s.log.Warn("worker queue full, task dropped", logger.LogFields{
				"task_id": task.ID,
			})
		}
	}()
}

// worker processes tasks from the queue.
func (s *Scheduler) worker(ctx context.Context, id int) {
	defer s.wg.Done()
	log := s.log.With(logger.LogFields{"worker_id": id})
	log.Debug("worker started")

	for {
		select {
		case <-s.stopCh:
			log.Debug("worker stopped")
			return
		case task := <-s.workerCh:
			s.processTask(ctx, task, log)
		}
	}
}

// processTask handles a single task execution.
func (s *Scheduler) processTask(ctx context.Context, task *Task, log *logger.Logger) {
	// Mark as running
	s.mu.Lock()
	task.State = TaskRunning
	s.running[task.ID] = task
	s.mu.Unlock()

	// Find handler
	s.mu.RLock()
	handler, ok := s.handlers[task.Type]
	s.mu.RUnlock()

	if !ok {
		log.Error("no handler registered for task type", nil, logger.LogFields{
			"task_id":   task.ID,
			"task_type": task.Type,
		})
		s.completeTask(task, TaskFailed, fmt.Errorf("no handler for type: %s", task.Type))
		return
	}

	// Execute with timeout
	taskCtx := ctx
	if s.cfg.JobTimeout > 0 {
		var cancel context.CancelFunc
		taskCtx, cancel = context.WithTimeout(ctx, s.cfg.JobTimeout)
		defer cancel()
	}

	log.Debug("executing task", logger.LogFields{
		"task_id": task.ID,
		"type":    task.Type,
	})

	start := time.Now()
	err := handler(taskCtx, task)
	duration := time.Since(start)

	if err != nil {
		log.Error("task failed", err, logger.LogFields{
			"task_id":  task.ID,
			"duration": duration.String(),
		})
		task.Error = err.Error()

		// Retry logic
		if task.Retries < task.MaxRetry {
			task.Retries++
			task.State = TaskRetrying
			log.Info("retrying task", logger.LogFields{
				"task_id": task.ID,
				"retry":   task.Retries,
				"max":     task.MaxRetry,
			})
			// Re-enqueue with backoff
			backoff := time.Duration(task.Retries) * time.Second
			time.AfterFunc(backoff, func() {
				s.enqueueTask(task)
			})
			return
		}

		s.completeTask(task, TaskFailed, err)
		return
	}

	log.Debug("task completed", logger.LogFields{
		"task_id":  task.ID,
		"duration": duration.String(),
	})
	s.completeTask(task, TaskCompleted, nil)
}

// completeTask finalizes a task and updates statistics.
func (s *Scheduler) completeTask(task *Task, state TaskState, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	task.State = state
	if err != nil {
		task.Error = err.Error()
	}

	delete(s.running, task.ID)

	switch state {
	case TaskCompleted:
		s.completed = append(s.completed, task)
		atomic.AddInt64(&s.tasksCompleted, 1)
	case TaskFailed:
		s.failed = append(s.failed, task)
		atomic.AddInt64(&s.tasksFailed, 1)
	}
}

// saveState persists scheduler state to disk.
func (s *Scheduler) saveState() error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	state := map[string]interface{}{
		"pending":  s.pending,
		"running":  s.running,
		"completed": s.completed,
		"failed":   s.failed,
		"stats": map[string]int64{
			"queued":    atomic.LoadInt64(&s.tasksQueued),
			"completed": atomic.LoadInt64(&s.tasksCompleted),
			"failed":    atomic.LoadInt64(&s.tasksFailed),
			"cancelled": atomic.LoadInt64(&s.tasksCancelled),
		},
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}

	dir := filepath.Dir(s.stateFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	return os.WriteFile(s.stateFile, data, 0644)
}

// loadState restores scheduler state from disk.
func (s *Scheduler) loadState() error {
	data, err := os.ReadFile(s.stateFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var state struct {
		Pending   []*Task         `json:"pending"`
		Running   map[string]*Task `json:"running"`
		Completed []*Task         `json:"completed"`
		Failed    []*Task         `json:"failed"`
	}

	if err := json.Unmarshal(data, &state); err != nil {
		return err
	}

	s.mu.Lock()
	s.pending = state.Pending
	s.running = state.Running
	s.completed = state.Completed
	s.failed = state.Failed

	// Reset running tasks to pending (they were interrupted)
	for _, task := range s.running {
		task.State = TaskPending
		s.pending = append(s.pending, task)
	}
	s.running = make(map[string]*Task)
	s.mu.Unlock()

	s.log.Info("scheduler state loaded", logger.LogFields{
		"pending":  len(state.Pending),
		"running":  len(state.Running),
		"completed": len(state.Completed),
		"failed":   len(state.Failed),
	})

	return nil
}

// Done returns a channel that's closed when the scheduler fully stops.
func (s *Scheduler) Done() <-chan struct{} {
	return s.doneCh
}

// NewSearchTask creates a search task from a query and handler.
func NewSearchTask(query *core.SearchQuery, handler TaskHandler) *Task {
	return &Task{
		ID:       uuid.New().String(),
		Type:     "search",
		Query:    query,
		Priority: 1,
		MaxRetry: 3,
	}
}

// BatchConfig holds configuration for batch job processing.
type BatchConfig struct {
	Queries    []string          `json:"queries"`
	Provider   core.ProviderID   `json:"provider"`
	Concurrent int               `json:"concurrent"`
	Delay      time.Duration     `json:"delay"`
	MaxResults int               `json:"max_results"`
	Filters    []core.Filter     `json:"filters"`
}

// BatchJob manages a batch of search tasks.
type BatchJob struct {
	ID         string      `json:"id"`
	Config     BatchConfig `json:"config"`
	CreatedAt  time.Time   `json:"created_at"`
	TaskIDs    []string    `json:"task_ids"`
	mu         sync.Mutex
}

// NewBatchJob creates a new batch job.
func NewBatchJob(cfg BatchConfig) *BatchJob {
	return &BatchJob{
		ID:        uuid.New().String(),
		Config:    cfg,
		CreatedAt: time.Now(),
	}
}

// SubmitAll submits all queries in the batch to the scheduler.
func (bj *BatchJob) SubmitAll(ctx context.Context, sched *Scheduler) error {
	bj.mu.Lock()
	defer bj.mu.Unlock()

	for i, q := range bj.Config.Queries {
		query := &core.SearchQuery{
			Query:      q,
			Provider:   bj.Config.Provider,
			MaxResults: bj.Config.MaxResults,
		}

		taskID, err := sched.SubmitQuery(ctx, query)
		if err != nil {
			return fmt.Errorf("submitting batch query %d: %w", i, err)
		}
		bj.TaskIDs = append(bj.TaskIDs, taskID)

		// Inter-query delay
		if bj.Config.Delay > 0 && i < len(bj.Config.Queries)-1 {
			time.Sleep(bj.Config.Delay)
		}
	}

	return nil
}
