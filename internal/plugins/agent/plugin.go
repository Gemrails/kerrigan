package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/kerrigan/kerrigan/internal/core/plugin"
	"github.com/kerrigan/kerrigan/pkg/log"
)

// PluginName defines the plugin name
const PluginName = "agent"

// Config holds the agent plugin configuration
type Config struct {
	// Agent framework configuration
	OpenClawPort int    `json:"openclaw_port"` // Default: 18789
	DeerFlowURL  string `json:"deerflow_url"`  // e.g., "http://localhost:8000"

	// Dispatcher configuration
	MaxConcurrentTasks int           `json:"max_concurrent_tasks"` // Default: 10
	TaskTimeout        time.Duration `json:"task_timeout"`         // Default: 30 minutes
	RetryAttempts      int           `json:"retry_attempts"`       // Default: 3

	// Registry
	RegistryUpdateInterval time.Duration `json:"registry_update_interval"` // Default: 5 minutes
}

// DefaultConfig returns default configuration
func DefaultConfig() Config {
	return Config{
		OpenClawPort:           18789,
		MaxConcurrentTasks:     10,
		TaskTimeout:            30 * time.Minute,
		RetryAttempts:          3,
		RegistryUpdateInterval: 5 * time.Minute,
	}
}

// AgentPlugin implements the agent plugin for dispatching tasks to remote nodes
type AgentPlugin struct {
	config      Config
	mu          sync.RWMutex
	registry    *Registry
	dispatcher  *Dispatcher
	openclaw    *OpenClawAdapter
	deerflow    *DeerFlowAdapter
	initialized bool
	running     bool
	ctx         context.Context
	cancel      context.CancelFunc
	wg          sync.WaitGroup
}

// NewPlugin creates a new agent plugin instance
func NewPlugin() *AgentPlugin {
	ctx, cancel := context.WithCancel(context.Background())
	return &AgentPlugin{
		ctx:    ctx,
		cancel: cancel,
	}
}

// Name returns the plugin name
func (p *AgentPlugin) Name() string {
	return PluginName
}

// Version returns the plugin version
func (p *AgentPlugin) Version() string {
	return "1.0.0"
}

// Initialize initializes the plugin with configuration
func (p *AgentPlugin) Initialize(configBytes []byte) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.initialized {
		return fmt.Errorf("plugin already initialized")
	}

	// Parse configuration
	p.config = DefaultConfig()
	if len(configBytes) > 0 {
		if err := json.Unmarshal(configBytes, &p.config); err != nil {
			return fmt.Errorf("failed to parse config: %w", err)
		}
	}

	// Initialize registry
	p.registry = NewRegistry()

	// Initialize adapters
	if p.config.OpenClawPort > 0 {
		p.openclaw = NewOpenClawAdapter(fmt.Sprintf("localhost:%d", p.config.OpenClawPort))
	}

	if p.config.DeerFlowURL != "" {
		p.deerflow = NewDeerFlowAdapter(p.config.DeerFlowURL)
	}

	// Initialize dispatcher
	p.dispatcher = NewDispatcher(p.config, p.registry)

	p.initialized = true
	log.Info("Agent plugin initialized",
		"openclaw_port", p.config.OpenClawPort,
		"deerflow_url", p.config.DeerFlowURL,
		"max_concurrent", p.config.MaxConcurrentTasks)
	return nil
}

// Start starts the plugin
func (p *AgentPlugin) Start() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.running {
		return fmt.Errorf("plugin already running")
	}

	if !p.initialized {
		return fmt.Errorf("plugin not initialized")
	}

	log.Info("Starting agent plugin...")

	// Start OpenClaw adapter
	if p.openclaw != nil {
		if err := p.openclaw.Start(p.ctx); err != nil {
			return fmt.Errorf("failed to start OpenClaw adapter: %w", err)
		}
	}

	// Start DeerFlow adapter
	if p.deerflow != nil {
		if err := p.deerflow.Start(p.ctx); err != nil {
			if p.openclaw != nil {
				p.openclaw.Stop()
			}
			return fmt.Errorf("failed to start DeerFlow adapter: %w", err)
		}
	}

	// Start registry update worker
	p.wg.Add(1)
	go p.updateRegistry()

	// Start dispatcher
	if err := p.dispatcher.Start(p.ctx); err != nil {
		return fmt.Errorf("failed to start dispatcher: %w", err)
	}

	p.running = true
	log.Info("Agent plugin started successfully")
	return nil
}

// updateRegistry periodically updates the registry with known nodes
func (p *AgentPlugin) updateRegistry() {
	defer p.wg.Done()

	ticker := time.NewTicker(p.config.RegistryUpdateInterval)
	defer ticker.Stop()

	for {
		select {
		case <-p.ctx.Done():
			return
		case <-ticker.C:
			p.refreshRegistry()
		}
	}
}

// refreshRegistry refreshes the registry with available nodes
func (p *AgentPlugin) refreshRegistry() {
	// Add any statically configured or discovered nodes here
	// For now, this is a placeholder for dynamic node discovery
	log.Debug("Refreshing agent registry")
}

// Stop stops the plugin gracefully
func (p *AgentPlugin) Stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.running {
		return nil
	}

	log.Info("Stopping agent plugin...")

	// Stop dispatcher
	if p.dispatcher != nil {
		p.dispatcher.Stop()
	}

	// Stop adapters
	if p.openclaw != nil {
		p.openclaw.Stop()
	}
	if p.deerflow != nil {
		p.deerflow.Stop()
	}

	// Cancel context and wait for workers
	p.cancel()
	p.wg.Wait()

	p.running = false
	log.Info("Agent plugin stopped")
	return nil
}

// GetInfo returns plugin metadata
func (p *AgentPlugin) GetInfo() plugin.PluginInfo {
	return plugin.PluginInfo{
		ID:          "agent-plugin",
		Name:        PluginName,
		Version:     "1.0.0",
		Description: "Agent framework plugin for dispatching tasks to OpenClaw and DeerFlow nodes",
		Author:      "Kerrigan Team",
		License:     "MIT",
		Capabilities: []plugin.Capability{
			plugin.CapabilityTaskExecution,
			plugin.CapabilityResourceQuery,
		},
	}
}

// GetCapabilities returns plugin capabilities
func (p *AgentPlugin) GetCapabilities() []plugin.Capability {
	return []plugin.Capability{
		plugin.CapabilityTaskExecution,
		plugin.CapabilityResourceQuery,
	}
}

// GetState returns current plugin state
func (p *AgentPlugin) GetState() plugin.PluginState {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if !p.initialized {
		return plugin.StateInstalled
	}
	if p.running {
		return plugin.StateRunning
	}
	return plugin.StatePaused
}

// GetResourceProvider returns the resource provider interface
func (p *AgentPlugin) GetResourceProvider() (plugin.ResourceProvider, bool) {
	return p, true
}

// GetTaskExecutor returns the task executor interface
func (p *AgentPlugin) GetTaskExecutor() (plugin.TaskExecutor, bool) {
	return p, true
}

// QueryResources returns available agent resources
func (p *AgentPlugin) QueryResources() (plugin.ResourceList, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if !p.running {
		return plugin.ResourceList{}, fmt.Errorf("plugin not running")
	}

	nodes := p.registry.ListNodes()
	resources := make([]plugin.Resource, 0, len(nodes))

	for _, node := range nodes {
		resources = append(resources, plugin.Resource{
			ID:        plugin.ResourceID(node.NodeID),
			Type:      plugin.ResourceTypeCPU,
			Name:      fmt.Sprintf("Agent Node - %s", node.AgentType),
			Available: node.Status == AgentStatusOnline,
			Capacity: plugin.ResourceCapacity{
				CPU: 4, // Placeholder
			},
			Properties: map[string]interface{}{
				"agent_type":   node.AgentType,
				"endpoint":     node.Endpoint,
				"capabilities": node.Capabilities,
				"load":         node.Load,
				"metadata":     node.Metadata,
			},
		})
	}

	return plugin.ResourceList{
		Resources:  resources,
		TotalCount: len(resources),
		Timestamp:  time.Now(),
	}, nil
}

// AllocateResources allocates agent resources for a request
func (p *AgentPlugin) AllocateResources(req plugin.ResourceRequest) (plugin.Allocation, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if !p.running {
		return plugin.Allocation{}, fmt.Errorf("plugin not running")
	}

	// Find a suitable node
	node := p.registry.FindNode(func(n *AgentNode) bool {
		return n.Status == AgentStatusOnline && n.Load < 0.8
	})

	if node == nil {
		return plugin.Allocation{}, fmt.Errorf("no available agent nodes")
	}

	// Mark node as busy
	node.Status = AgentStatusBusy
	node.Load += 0.1

	allocationID := plugin.AllocationID(fmt.Sprintf("agent-alloc-%d", time.Now().UnixNano()))

	return plugin.Allocation{
		ID:        allocationID,
		Resources: []plugin.ResourceID{plugin.ResourceID(node.NodeID)},
		StartTime: time.Now(),
		EndTime:   time.Now().Add(req.Duration),
		Status:    plugin.AllocationActive,
	}, nil
}

// ReleaseResources releases allocated resources
func (p *AgentPlugin) ReleaseResources(id plugin.AllocationID) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Extract node ID from allocation
	nodeID := string(id)
	if len(nodeID) > 11 {
		nodeID = nodeID[11:] // Remove "agent-alloc-" prefix
	}

	node := p.registry.GetNode(nodeID)
	if node != nil {
		node.Status = AgentStatusOnline
		if node.Load > 0.1 {
			node.Load -= 0.1
		}
	}

	log.Debug("Released agent allocation", "id", id)
	return nil
}

// GetResourceStats returns current resource statistics
func (p *AgentPlugin) GetResourceStats() (plugin.ResourceStats, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if !p.running {
		return plugin.ResourceStats{}, fmt.Errorf("plugin not running")
	}

	nodes := p.registry.ListNodes()
	var totalLoad float64
	onlineCount := 0

	for _, node := range nodes {
		if node.Status == AgentStatusOnline {
			onlineCount++
			totalLoad += node.Load
		}
	}

	avgLoad := 0.0
	if onlineCount > 0 {
		avgLoad = totalLoad / float64(onlineCount)
	}

	return plugin.ResourceStats{
		TotalResources:    int64(len(nodes)),
		AvailableCount:    int64(onlineCount),
		AllocatedCount:    int64(len(nodes) - onlineCount),
		Utilization:       avgLoad,
		AvailableCapacity: plugin.ResourceCapacity{CPU: float64(onlineCount) * 4},
		Timestamp:         time.Now(),
	}, nil
}

// Execute executes a task and returns result
func (p *AgentPlugin) Execute(ctx context.Context, task plugin.Task) (*plugin.Result, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if !p.running {
		return nil, fmt.Errorf("plugin not running")
	}

	// Convert to internal task request
	req := &TaskRequest{
		ID:       task.ID,
		Type:     task.Type,
		Prompt:   task.Payload["prompt"].(string),
		Timeout:  task.Timeout,
		Metadata: task.Metadata,
	}

	// Determine required capabilities
	if caps, ok := task.Payload["capabilities"].([]string); ok {
		for _, c := range caps {
			req.Capabilities = append(req.Capabilities, AgentCapability(c))
		}
	}

	// Execute via dispatcher
	result, err := p.dispatcher.Dispatch(ctx, req)
	if err != nil {
		return &plugin.Result{
			TaskID:   task.ID,
			Success:  false,
			Error:    err.Error(),
			Duration: 0,
		}, nil
	}

	return &plugin.Result{
		TaskID:   result.TaskID,
		Success:  result.Success,
		Output:   result.Output,
		Error:    result.Error,
		Duration: result.Duration,
	}, nil
}

// QueryStatus returns the status of a task
func (p *AgentPlugin) QueryStatus(taskID string) (plugin.TaskStatus, error) {
	result := p.dispatcher.GetResult(taskID)
	if result == nil {
		return plugin.TaskStatusPending, nil
	}

	switch {
	case result.Success:
		return plugin.TaskStatusCompleted, nil
	default:
		return plugin.TaskStatusFailed, nil
	}
}

// CancelTask cancels a running task
func (p *AgentPlugin) CancelTask(taskID string) error {
	return p.dispatcher.CancelTask(taskID)
}

// ListTasks returns list of tasks
func (p *AgentPlugin) ListTasks(filter plugin.TaskFilter) ([]plugin.Task, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// Return empty list - tasks are managed by dispatcher
	return []plugin.Task{}, nil
}

// RegisterNode registers a new agent node
func (p *AgentPlugin) RegisterNode(node *AgentNode) error {
	return p.registry.AddNode(node)
}

// UnregisterNode removes an agent node
func (p *AgentPlugin) UnregisterNode(nodeID string) error {
	return p.registry.RemoveNode(nodeID)
}

// GetRegistry returns the node registry
func (p *AgentPlugin) GetRegistry() *Registry {
	return p.registry
}

// GetDispatcher returns the task dispatcher
func (p *AgentPlugin) GetDispatcher() *Dispatcher {
	return p.dispatcher
}

// GetOpenClawAdapter returns the OpenClaw adapter
func (p *AgentPlugin) GetOpenClawAdapter() *OpenClawAdapter {
	return p.openclaw
}

// GetDeerFlowAdapter returns the DeerFlow adapter
func (p *AgentPlugin) GetDeerFlowAdapter() *DeerFlowAdapter {
	return p.deerflow
}

// GetConfig returns the current configuration
func (p *AgentPlugin) GetConfig() Config {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.config
}

// Ensure AgentPlugin implements required interfaces
var _ plugin.Plugin = (*AgentPlugin)(nil)
var _ plugin.ResourceProvider = (*AgentPlugin)(nil)
var _ plugin.TaskExecutor = (*AgentPlugin)(nil)
