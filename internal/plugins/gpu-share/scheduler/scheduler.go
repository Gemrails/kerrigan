package scheduler

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// Config holds scheduler configuration
type Config struct {
	QueueSize     int           `json:"queue_size"`
	MaxRetries    int           `json:"max_retries"`
	Timeout       time.Duration `json:"timeout"`
	WorkersPerGPU int           `json:"workers_per_gpu"`
}

// Priority levels for tasks
const (
	PriorityLow    = 1
	PriorityNormal = 5
	PriorityHigh   = 9
	PriorityUrgent = 10
)

// TaskState represents task state
type TaskState int

const (
	TaskStatePending TaskState = iota
	TaskStateRunning
	TaskStateCompleted
	TaskStateFailed
	TaskStateCancelled
)

// Task represents a GPU task
type Task struct {
	ID               string
	Type             string
	Priority         int
	GPUID            string
	RequiredMemoryMB int
	CreatedAt        time.Time
	StartedAt        time.Time
	CompletedAt      time.Time
	State            TaskState
	Retries          int
	Result           interface{}
	Error            error
	Input            map[string]interface{}
	Metadata         map[string]interface{}
}

// TaskHandler is the function type for handling tasks
type TaskHandler func(ctx context.Context, task *Task) (*Task, error)

// Scheduler is the GPU task scheduler
type Scheduler struct {
	config  Config
	mu      sync.RWMutex
	queues  map[string]*TaskQueue // GPU ID -> queue
	workers map[string]*Worker    // GPU ID -> worker
	tasks   map[string]*Task      // Task ID -> task
	handler TaskHandler
	stats   *Stats
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	running atomic.Bool
}

// TaskQueue represents a priority queue for tasks
type TaskQueue struct {
	mu       sync.RWMutex
	tasks    []*Task
	notEmpty chan struct{}
}

// Worker represents a worker for a GPU
type Worker struct {
	GPUID       string
	WorkerID    int
	Running     atomic.Bool
	CurrentTask *Task
	mu          sync.RWMutex
}

// Stats holds scheduler statistics
type Stats struct {
	TotalTasks     atomic.Int64
	CompletedTasks atomic.Int64
	FailedTasks    atomic.Int64
	RunningTasks   atomic.Int64
	QueuedTasks    atomic.Int64
	TotalWaitTime  atomic.Int64 // nanoseconds
	TotalRunTime   atomic.Int64 // nanoseconds
	mu             sync.RWMutex
}

// NewScheduler creates a new scheduler
func NewScheduler(config Config) *Scheduler {
	if config.QueueSize == 0 {
		config.QueueSize = 1000
	}
	if config.MaxRetries == 0 {
		config.MaxRetries = 3
	}
	if config.Timeout == 0 {
		config.Timeout = 5 * time.Minute
	}
	if config.WorkersPerGPU == 0 {
		config.WorkersPerGPU = 4
	}

	ctx, cancel := context.WithCancel(context.Background())
	return &Scheduler{
		config:  config,
		queues:  make(map[string]*TaskQueue),
		workers: make(map[string]*Worker),
		tasks:   make(map[string]*Task),
		stats:   &Stats{},
		ctx:     ctx,
		cancel:  cancel,
	}
}

// SetHandler sets the task handler
func (s *Scheduler) SetHandler(handler TaskHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handler = handler
}

// StartWorker starts a worker for a GPU
func (s *Scheduler) StartWorker(ctx context.Context, gpuID string, numWorkers int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Create queue for this GPU if not exists
	if _, exists := s.queues[gpuID]; !exists {
		s.queues[gpuID] = NewTaskQueue()
	}

	// Start workers
	for i := 0; i < numWorkers; i++ {
		worker := &Worker{
			GPUID:    gpuID,
			WorkerID: i,
		}
		s.workers[fmt.Sprintf("%s-%d", gpuID, i)] = worker

		s.wg.Add(1)
		go s.runWorker(ctx, worker)
	}
}

// runWorker runs a worker
func (s *Scheduler) runWorker(ctx context.Context, worker *Worker) {
	defer s.wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		default:
			task := s.dequeue(worker.GPUID)
			if task == nil {
				time.Sleep(100 * time.Millisecond)
				continue
			}

			s.executeTask(ctx, worker, task)
		}
	}
}

// executeTask executes a task
func (s *Scheduler) executeTask(ctx context.Context, worker *Worker, task *Task) {
	worker.Running.Store(true)
	worker.mu.Lock()
	worker.CurrentTask = task
	worker.mu.Unlock()

	// Update task state
	task.State = TaskStateRunning
	task.StartedAt = time.Now()
	s.stats.RunningTasks.Add(1)
	s.stats.QueuedTasks.Add(-1)

	// Execute task
	var result *Task
	var err error

	if s.handler != nil {
		result, err = s.handler(ctx, task)
	} else {
		// Default handler
		result = task
		err = nil
	}

	// Update task result
	task.CompletedAt = time.Now()
	if err != nil {
		task.State = TaskStateFailed
		task.Error = err
		s.stats.FailedTasks.Add(1)

		// Retry if possible
		if task.Retries < s.config.MaxRetries {
			task.Retries++
			task.State = TaskStatePending
			s.Enqueue(task)
		}
	} else {
		task.State = TaskStateCompleted
		task.Result = result.Result
		s.stats.CompletedTasks.Add(1)
	}

	s.stats.RunningTasks.Add(-1)

	// Update stats
	waitTime := task.StartedAt.Sub(task.CreatedAt).Nanoseconds()
	s.stats.TotalWaitTime.Add(waitTime)
	runTime := task.CompletedAt.Sub(task.StartedAt).Nanoseconds()
	s.stats.TotalRunTime.Add(runTime)

	worker.Running.Store(false)
	worker.mu.Lock()
	worker.CurrentTask = nil
	worker.mu.Unlock()
}

// Enqueue adds a task to the queue
func (s *Scheduler) Enqueue(task *Task) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if task.ID == "" {
		task.ID = generateTaskID()
	}
	if task.CreatedAt.IsZero() {
		task.CreatedAt = time.Now()
	}
	if task.Priority == 0 {
		task.Priority = PriorityNormal
	}

	// Create queue if not exists
	gpuID := task.GPUID
	if gpuID == "" {
		gpuID = "default"
	}

	if _, exists := s.queues[gpuID]; !exists {
		s.queues[gpuID] = NewTaskQueue()
	}

	s.queues[gpuID].Enqueue(task)
	s.tasks[task.ID] = task
	s.stats.TotalTasks.Add(1)
	s.stats.QueuedTasks.Add(1)
}

// dequeue removes and returns the highest priority task
func (s *Scheduler) dequeue(gpuID string) *Task {
	s.mu.Lock()
	defer s.mu.Unlock()

	queue, exists := s.queues[gpuID]
	if !exists || queue == nil {
		return nil
	}

	return queue.Dequeue()
}

// GetTask gets a task by ID
func (s *Scheduler) GetTask(taskID string) (*Task, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	task, exists := s.tasks[taskID]
	if !exists {
		return nil, fmt.Errorf("task not found: %s", taskID)
	}

	return task, nil
}

// CancelTask cancels a task
func (s *Scheduler) CancelTask(taskID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, exists := s.tasks[taskID]
	if !exists {
		return fmt.Errorf("task not found: %s", taskID)
	}

	if task.State == TaskStateRunning {
		return fmt.Errorf("cannot cancel running task")
	}

	task.State = TaskStateCancelled
	return nil
}

// ListTasks lists all tasks
func (s *Scheduler) ListTasks() []*Task {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tasks := make([]*Task, 0, len(s.tasks))
	for _, task := range s.tasks {
		tasks = append(tasks, task)
	}

	return tasks
}

// ListGPUTasks lists tasks for a specific GPU
func (s *Scheduler) ListGPUTasks(gpuID string) []*Task {
	s.mu.RLock()
	defer s.mu.RUnlock()

	queue, exists := s.queues[gpuID]
	if !exists {
		return nil
	}

	return queue.List()
}

// GetStats returns scheduler statistics
func (s *Scheduler) GetStats() map[string]interface{} {
	stats := map[string]interface{}{
		"total_tasks":     s.stats.TotalTasks.Load(),
		"completed_tasks": s.stats.CompletedTasks.Load(),
		"failed_tasks":    s.stats.FailedTasks.Load(),
		"running_tasks":   s.stats.RunningTasks.Load(),
		"queued_tasks":    s.stats.QueuedTasks.Load(),
		"total_wait_time": s.stats.TotalWaitTime.Load() / 1e6, // ms
		"total_run_time":  s.stats.TotalRunTime.Load() / 1e6,  // ms
	}

	if s.stats.CompletedTasks.Load() > 0 {
		stats["avg_wait_time_ms"] = float64(s.stats.TotalWaitTime.Load()) / float64(s.stats.CompletedTasks.Load()) / 1e6
		stats["avg_run_time_ms"] = float64(s.stats.TotalRunTime.Load()) / float64(s.stats.CompletedTasks.Load()) / 1e6
	}

	return stats
}

// Stop stops the scheduler
func (s *Scheduler) Stop() {
	s.cancel()
	s.wg.Wait()
	s.running.Store(false)
}

// NewTaskQueue creates a new task queue
func NewTaskQueue() *TaskQueue {
	return &TaskQueue{
		notEmpty: make(chan struct{}, 1),
	}
}

// Enqueue adds a task to the queue
func (q *TaskQueue) Enqueue(task *Task) {
	q.mu.Lock()
	defer q.mu.Unlock()

	// Insert task in priority order
	inserted := false
	for i, t := range q.tasks {
		if task.Priority > t.Priority {
			q.tasks = append(q.tasks[:i], append([]*Task{task}, q.tasks[i:]...)...)
			inserted = true
			break
		}
	}

	if !inserted {
		q.tasks = append(q.tasks, task)
	}

	// Notify waiting workers
	select {
	case q.notEmpty <- struct{}{}:
	default:
	}
}

// Dequeue removes and returns the highest priority task
func (q *TaskQueue) Dequeue() *Task {
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.tasks) == 0 {
		return nil
	}

	task := q.tasks[0]
	q.tasks = q.tasks[1:]

	return task
}

// List returns all tasks in the queue
func (q *TaskQueue) List() []*Task {
	q.mu.RLock()
	defer q.mu.RUnlock()

	tasks := make([]*Task, len(q.tasks))
	copy(tasks, q.tasks)

	return tasks
}

// Len returns the number of tasks in the queue
func (q *TaskQueue) Len() int {
	q.mu.RLock()
	defer q.mu.RUnlock()

	return len(q.tasks)
}

// NewTask creates a new task
func NewTask(taskType, gpuID string, priority int, memoryMB int, input map[string]interface{}) *Task {
	return &Task{
		Type:             taskType,
		Priority:         priority,
		GPUID:            gpuID,
		RequiredMemoryMB: memoryMB,
		Input:            input,
		State:            TaskStatePending,
		CreatedAt:        time.Now(),
		Metadata:         make(map[string]interface{}),
	}
}

// Helper functions
func generateTaskID() string {
	return fmt.Sprintf("task-%d-%d", time.Now().UnixNano(), time.Now().Nanosecond()%1000)
}
