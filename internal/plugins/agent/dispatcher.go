package agent

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/kerrigan/kerrigan/pkg/log"
)

// TaskRequest represents a task to be dispatched to an agent node
type TaskRequest struct {
	ID           string                 `json:"id"`
	Type         string                 `json:"type"` // "research", "code", "general"
	Prompt       string                 `json:"prompt"`
	Capabilities []AgentCapability      `json:"capabilities"` // Required capabilities
	Timeout      time.Duration          `json:"timeout"`
	Metadata     map[string]interface{} `json:"metadata"`
}

// TaskResult represents the result of a task execution
type TaskResult struct {
	TaskID   string                 `json:"task_id"`
	Success  bool                   `json:"success"`
	Output   map[string]interface{} `json:"output"`
	Error    string                 `json:"error"`
	Duration time.Duration          `json:"duration"`
	NodeID   string                 `json:"node_id"`
}

// Dispatcher dispatches tasks to agent nodes
type Dispatcher struct {
	config   Config
	registry *Registry

	mu      sync.RWMutex
	tasks   map[string]*TaskRequest
	results map[string]*TaskResult
	running int32

	// Task queue
	taskQueue chan *TaskRequest

	// Result callbacks
	resultCallbacks map[string]chan *TaskResult
	callbackMu      sync.RWMutex

	// Active tasks tracking
	activeTasks   map[string]*activeTask
	activeTasksMu sync.Mutex
}

type activeTask struct {
	request   *TaskRequest
	startTime time.Time
	nodeID    string
	ctx       context.Context
	cancel    context.CancelFunc
}

// NewDispatcher creates a new dispatcher
func NewDispatcher(config Config, registry *Registry) *Dispatcher {
	return &Dispatcher{
		config:          config,
		registry:        registry,
		tasks:           make(map[string]*TaskRequest),
		results:         make(map[string]*TaskResult),
		taskQueue:       make(chan *TaskRequest, config.MaxConcurrentTasks*2),
		resultCallbacks: make(map[string]chan *TaskResult),
		activeTasks:     make(map[string]*activeTask),
	}
}

// Start starts the dispatcher
func (d *Dispatcher) Start(ctx context.Context) error {
	if !atomic.CompareAndSwapInt32(&d.running, 0, 1) {
		return fmt.Errorf("dispatcher already running")
	}

	// Start worker pool
	workerCount := d.config.MaxConcurrentTasks
	for i := 0; i < workerCount; i++ {
		go d.worker(ctx, i)
	}

	// Start result processor
	go d.processResults(ctx)

	log.Info("Dispatcher started", "workers", workerCount)
	return nil
}

// Stop stops the dispatcher
func (d *Dispatcher) Stop() {
	if !atomic.CompareAndSwapInt32(&d.running, 1, 0) {
		return
	}

	// Cancel all active tasks
	d.activeTasksMu.Lock()
	for _, task := range d.activeTasks {
		if task.cancel != nil {
			task.cancel()
		}
	}
	d.activeTasksMu.Unlock()

	// Wait for workers to finish
	close(d.taskQueue)

	log.Info("Dispatcher stopped")
}

// worker is a worker goroutine that processes tasks
func (d *Dispatcher) worker(ctx context.Context, id int) {
	log.Debug("Dispatcher worker started", "id", id)

	for {
		select {
		case <-ctx.Done():
			log.Debug("Dispatcher worker stopping", "id", id)
			return
		case req, ok := <-d.taskQueue:
			if !ok {
				return
			}
			d.executeTask(ctx, req)
		}
	}
}

// executeTask executes a task on a suitable node
func (d *Dispatcher) executeTask(ctx context.Context, req *TaskRequest) {
	startTime := time.Now()

	// Find a suitable node
	node := d.findBestNode(req)
	if node == nil {
		d.storeResult(&TaskResult{
			TaskID:   req.ID,
			Success:  false,
			Error:    "no available agent nodes",
			Duration: time.Since(startTime),
		})
		return
	}

	// Create context with timeout
	taskCtx, cancel := context.WithTimeout(ctx, req.Timeout)
	defer cancel()

	// Track active task
	active := &activeTask{
		request:   req,
		startTime: startTime,
		nodeID:    node.NodeID,
		ctx:       taskCtx,
		cancel:    cancel,
	}
	d.activeTasksMu.Lock()
	d.activeTasks[req.ID] = active
	d.activeTasksMu.Unlock()

	// Execute based on agent type
	var result *TaskResult

	switch node.AgentType {
	case AgentTypeOpenClaw:
		result = d.executeOpenClawTask(taskCtx, req, node)
	case AgentTypeDeerFlow:
		result = d.executeDeerFlowTask(taskCtx, req, node)
	default:
		result = &TaskResult{
			TaskID:   req.ID,
			Success:  false,
			Error:    fmt.Sprintf("unknown agent type: %s", node.AgentType),
			Duration: time.Since(startTime),
			NodeID:   node.NodeID,
		}
	}

	// Update node load
	if result.Success {
		d.registry.UpdateNodeLoad(node.NodeID, node.Load-0.1)
	} else {
		d.registry.UpdateNodeStatus(node.NodeID, AgentStatusOnline)
		d.registry.UpdateNodeLoad(node.NodeID, node.Load-0.1)
	}

	// Remove from active tasks
	d.activeTasksMu.Lock()
	delete(d.activeTasks, req.ID)
	d.activeTasksMu.Unlock()

	// Store result
	d.storeResult(result)

	log.Debug("Task executed",
		"task_id", req.ID,
		"node", node.NodeID,
		"success", result.Success,
		"duration", result.Duration)
}

// findBestNode finds the best node for a task
func (d *Dispatcher) findBestNode(req *TaskRequest) *AgentNode {
	// If specific capabilities are required, filter by them
	if len(req.Capabilities) > 0 {
		for _, cap := range req.Capabilities {
			nodes := d.registry.GetNodesByCapability(cap)
			if len(nodes) > 0 {
				// Find best node among capable ones
				return d.selectBestNode(nodes)
			}
		}
		return nil
	}

	// Otherwise, find any online node with low load
	return d.registry.FindNode(func(n *AgentNode) bool {
		return n.Status == AgentStatusOnline && n.Load < 0.8
	})
}

// selectBestNode selects the best node from a list
func (d *Dispatcher) selectBestNode(nodes []*AgentNode) *AgentNode {
	var best *AgentNode
	var minLoad float64 = 1.0

	for _, node := range nodes {
		if node.Status == AgentStatusOnline && node.Load < minLoad {
			best = node
			minLoad = node.Load
		}
	}

	if best != nil {
		// Mark node as busy
		d.registry.UpdateNodeStatus(best.NodeID, AgentStatusBusy)
		d.registry.UpdateNodeLoad(best.NodeID, best.Load+0.1)
	}

	return best
}

// executeOpenClawTask executes a task via OpenClaw
func (d *Dispatcher) executeOpenClawTask(ctx context.Context, req *TaskRequest, node *AgentNode) *TaskResult {
	// This would call the OpenClaw adapter
	// For now, return a placeholder result
	return &TaskResult{
		TaskID:   req.ID,
		Success:  false,
		Error:    "OpenClaw adapter not connected",
		NodeID:   node.NodeID,
		Duration: 0,
	}
}

// executeDeerFlowTask executes a task via DeerFlow
func (d *Dispatcher) executeDeerFlowTask(ctx context.Context, req *TaskRequest, node *AgentNode) *TaskResult {
	// This would call the DeerFlow adapter
	// For now, return a placeholder result
	return &TaskResult{
		TaskID:   req.ID,
		Success:  false,
		Error:    "DeerFlow adapter not connected",
		NodeID:   node.NodeID,
		Duration: 0,
	}
}

// processResults processes results and notifies callbacks
func (d *Dispatcher) processResults(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			d.cleanupStaleResults()
		}
	}
}

// cleanupStaleResults removes old results
func (d *Dispatcher) cleanupStaleResults() {
	d.mu.Lock()
	defer d.mu.Unlock()

	if len(d.results) > 1000 {
		for id := range d.results {
			delete(d.results, id)
			if len(d.results) <= 1000 {
				break
			}
		}
	}
}

// Dispatch dispatches a task
func (d *Dispatcher) Dispatch(ctx context.Context, req *TaskRequest) (*TaskResult, error) {
	if atomic.LoadInt32(&d.running) == 0 {
		return nil, fmt.Errorf("dispatcher not running")
	}

	// Set default timeout
	if req.Timeout == 0 {
		req.Timeout = d.config.TaskTimeout
	}

	// Store task
	d.mu.Lock()
	d.tasks[req.ID] = req
	d.mu.Unlock()

	// Send to queue
	select {
	case d.taskQueue <- req:
		log.Debug("Task dispatched", "task_id", req.ID, "type", req.Type)
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(d.config.TaskTimeout):
		return nil, fmt.Errorf("task queue full")
	}

	// Wait for result
	return d.waitForResult(ctx, req.ID)
}

// waitForResult waits for a task result
func (d *Dispatcher) waitForResult(ctx context.Context, taskID string) (*TaskResult, error) {
	timeout := d.config.TaskTimeout

	select {
	case result := <-d.getOrCreateCallback(taskID):
		return result, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(timeout):
		return nil, fmt.Errorf("task timeout after %v", timeout)
	}
}

// getOrCreateCallback gets or creates a result callback channel
func (d *Dispatcher) getOrCreateCallback(taskID string) chan *TaskResult {
	d.callbackMu.Lock()
	defer d.callbackMu.Unlock()

	if ch, exists := d.resultCallbacks[taskID]; exists {
		return ch
	}

	ch := make(chan *TaskResult, 1)
	d.resultCallbacks[taskID] = ch
	return ch
}

// storeResult stores a task result and notifies waiters
func (d *Dispatcher) storeResult(result *TaskResult) {
	d.mu.Lock()
	d.results[result.TaskID] = result
	d.mu.Unlock()

	// Notify callback
	d.callbackMu.RLock()
	if ch, exists := d.resultCallbacks[result.TaskID]; exists {
		select {
		case ch <- result:
		default:
		}
	}
	d.callbackMu.RUnlock()
}

// GetResult returns a result for a task
func (d *Dispatcher) GetResult(taskID string) *TaskResult {
	d.mu.RLock()
	defer d.mu.RUnlock()

	return d.results[taskID]
}

// CancelTask cancels a running task
func (d *Dispatcher) CancelTask(taskID string) error {
	d.activeTasksMu.Lock()
	defer d.activeTasksMu.Unlock()

	if task, exists := d.activeTasks[taskID]; exists && task.cancel != nil {
		task.cancel()
		delete(d.activeTasks, taskID)
		log.Debug("Task cancelled", "task_id", taskID)
		return nil
	}

	return fmt.Errorf("task not found or already completed")
}

// GetActiveTasks returns the number of active tasks
func (d *Dispatcher) GetActiveTasks() int {
	d.activeTasksMu.Lock()
	defer d.activeTasksMu.Unlock()

	return len(d.activeTasks)
}

// GetQueuedTasks returns the number of queued tasks
func (d *Dispatcher) GetQueuedTasks() int {
	return len(d.taskQueue)
}

// IsRunning returns whether the dispatcher is running
func (d *Dispatcher) IsRunning() bool {
	return atomic.LoadInt32(&d.running) == 1
}
