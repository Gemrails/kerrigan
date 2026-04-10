package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

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

	p.config = Config{
		OpenClawPort:           18789,
		MaxConcurrentTasks:     10,
		TaskTimeout:            30 * time.Minute,
		RetryAttempts:          3,
		RegistryUpdateInterval: 5 * time.Minute,
	}

	if len(configBytes) > 0 {
		if err := json.Unmarshal(configBytes, &p.config); err != nil {
			return fmt.Errorf("failed to parse config: %w", err)
		}
	}

	p.registry = NewRegistry()

	if p.config.OpenClawPort > 0 {
		p.openclaw = NewOpenClawAdapter(fmt.Sprintf("localhost:%d", p.config.OpenClawPort))
	}

	if p.config.DeerFlowURL != "" {
		p.deerflow = NewDeerFlowAdapter(p.config.DeerFlowURL)
	}

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

	if p.openclaw != nil {
		if err := p.openclaw.Start(p.ctx); err != nil {
			log.Warn("Failed to start OpenClaw adapter", "error", err)
		}
	}

	if p.deerflow != nil {
		if err := p.deerflow.Start(p.ctx); err != nil {
			log.Warn("Failed to start DeerFlow adapter", "error", err)
		}
	}

	p.wg.Add(1)
	go p.updateRegistry()

	if err := p.dispatcher.Start(p.ctx); err != nil {
		return fmt.Errorf("failed to start dispatcher: %w", err)
	}

	p.running = true
	log.Info("Agent plugin started successfully")
	return nil
}

func (p *AgentPlugin) updateRegistry() {
	defer p.wg.Done()

	ticker := time.NewTicker(p.config.RegistryUpdateInterval)
	defer ticker.Stop()

	for {
		select {
		case <-p.ctx.Done():
			return
		case <-ticker.C:
			log.Debug("Refreshing agent registry")
		}
	}
}

// Shutdown stops the plugin gracefully
func (p *AgentPlugin) Shutdown() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.running {
		return nil
	}

	log.Info("Stopping agent plugin...")

	if p.dispatcher != nil {
		p.dispatcher.Stop()
	}

	if p.openclaw != nil {
		p.openclaw.Stop()
	}
	if p.deerflow != nil {
		p.deerflow.Stop()
	}

	p.cancel()
	p.wg.Wait()

	p.running = false
	log.Info("Agent plugin stopped")
	return nil
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
