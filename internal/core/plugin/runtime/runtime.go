package runtime

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/kerrigan/kerrigan/internal/core/plugin/types"
	"github.com/sirupsen/logrus"
)

type Runtime struct {
	mu          sync.RWMutex
	plugins     map[string]*pluginInstance
	dataDir     string
	configDir   string
	logDir      string
	logger      *logrus.Logger
	sandbox     *Sandbox
	resourceMgr *ResourceManager
	stateDir    string
}

type pluginInstance struct {
	plugin    types.Plugin
	config    types.PluginConfig
	state     types.PluginState
	startTime time.Time
	resources *ResourceAllocation
	mu        sync.RWMutex
}

func NewRuntime(dataDir, configDir, logDir string, logger *logrus.Logger) (*Runtime, error) {
	dirs := []string{dataDir, configDir, logDir}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("create directory %s: %w", dir, err)
		}
	}

	stateDir := filepath.Join(dataDir, "state")
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		return nil, fmt.Errorf("create state directory: %w", err)
	}

	sandbox := NewSandbox()
	resourceMgr := NewResourceManager(logger)

	return &Runtime{
		plugins:     make(map[string]*pluginInstance),
		dataDir:     dataDir,
		configDir:   configDir,
		logDir:      logDir,
		logger:      logger,
		sandbox:     sandbox,
		resourceMgr: resourceMgr,
		stateDir:    stateDir,
	}, nil
}

func (r *Runtime) LoadPlugin(ctx context.Context, p types.Plugin) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	info := p.GetInfo()

	if _, exists := r.plugins[info.ID]; exists {
		return fmt.Errorf("plugin %s already loaded", info.ID)
	}

	cfg := types.PluginConfig{
		ID:        info.ID,
		Name:      info.Name,
		Version:   info.Version,
		DataDir:   filepath.Join(r.dataDir, info.ID),
		ConfigDir: filepath.Join(r.configDir, info.ID),
		LogDir:    filepath.Join(r.logDir, info.ID),
		Env:       make(map[string]string),
	}

	for _, dir := range []string{cfg.DataDir, cfg.ConfigDir, cfg.LogDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("create plugin directory %s: %w", dir, err)
		}
	}

	if err := p.Init(ctx, cfg); err != nil {
		return fmt.Errorf("init plugin %s: %w", info.ID, err)
	}

	r.plugins[info.ID] = &pluginInstance{
		plugin:    p,
		config:    cfg,
		state:     types.StateInstalled,
		resources: nil,
	}

	r.logger.Infof("Plugin %s v%s loaded successfully", info.Name, info.Version)
	return nil
}

func (r *Runtime) StartPlugin(ctx context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	inst, exists := r.plugins[id]
	if !exists {
		return fmt.Errorf("plugin %s not found", id)
	}

	inst.mu.Lock()
	defer inst.mu.Unlock()

	if inst.state != types.StateInstalled && inst.state != types.StatePaused {
		return fmt.Errorf("cannot start plugin in state %s", inst.state)
	}

	if !r.sandbox.HasPermission(id, "execute") {
		return fmt.Errorf("sandbox permission denied for plugin %s", id)
	}

	inst.state = types.StateStarting
	r.logger.Infof("Starting plugin %s", id)

	if err := inst.plugin.Start(); err != nil {
		inst.state = types.StateFailed
		return fmt.Errorf("start plugin %s: %w", id, err)
	}

	inst.state = types.StateRunning
	inst.startTime = time.Now()

	r.logger.Infof("Plugin %s started successfully", id)
	return nil
}

func (r *Runtime) StopPlugin(ctx context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	inst, exists := r.plugins[id]
	if !exists {
		return fmt.Errorf("plugin %s not found", id)
	}

	inst.mu.Lock()
	defer inst.mu.Unlock()

	if inst.state != types.StateRunning && inst.state != types.StatePaused {
		return fmt.Errorf("cannot stop plugin in state %s", inst.state)
	}

	inst.state = types.StateStopping
	r.logger.Infof("Stopping plugin %s", id)

	if err := inst.plugin.Stop(); err != nil {
		r.logger.Errorf("Error stopping plugin %s: %v", id, err)
		inst.state = types.StateFailed
		return fmt.Errorf("stop plugin %s: %w", id, err)
	}

	inst.state = types.StateInstalled
	r.logger.Infof("Plugin %s stopped successfully", id)
	return nil
}

func (r *Runtime) PausePlugin(ctx context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	inst, exists := r.plugins[id]
	if !exists {
		return fmt.Errorf("plugin %s not found", id)
	}

	inst.mu.Lock()
	defer inst.mu.Unlock()

	if inst.state != types.StateRunning {
		return fmt.Errorf("cannot pause plugin in state %s", inst.state)
	}

	inst.state = types.StatePaused

	r.logger.Infof("Plugin %s paused", id)
	return nil
}

func (r *Runtime) ResumePlugin(ctx context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	inst, exists := r.plugins[id]
	if !exists {
		return fmt.Errorf("plugin %s not found", id)
	}

	inst.mu.Lock()
	defer inst.mu.Unlock()

	if inst.state != types.StatePaused {
		return fmt.Errorf("cannot resume plugin in state %s", inst.state)
	}

	inst.state = types.StateRunning
	r.logger.Infof("Plugin %s resumed", id)
	return nil
}

func (r *Runtime) UnloadPlugin(ctx context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	inst, exists := r.plugins[id]
	if !exists {
		return fmt.Errorf("plugin %s not found", id)
	}

	inst.mu.Lock()
	if inst.state == types.StateRunning || inst.state == types.StatePaused {
		inst.plugin.Stop()
	}
	inst.mu.Unlock()

	delete(r.plugins, id)

	dirs := []string{inst.config.DataDir, inst.config.ConfigDir, inst.config.LogDir}
	for _, dir := range dirs {
		if err := os.RemoveAll(dir); err != nil {
			r.logger.Warnf("Failed to cleanup directory %s: %v", dir, err)
		}
	}

	r.logger.Infof("Plugin %s unloaded", id)
	return nil
}

func (r *Runtime) GetPlugin(id string) (types.Plugin, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	inst, exists := r.plugins[id]
	if !exists {
		return nil, false
	}
	return inst.plugin, true
}

func (r *Runtime) GetPluginState(id string) (types.PluginState, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	inst, exists := r.plugins[id]
	if !exists {
		return "", false
	}
	return inst.state, true
}

func (r *Runtime) ListPlugins() []types.PluginInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]types.PluginInfo, 0, len(r.plugins))
	for _, inst := range r.plugins {
		result = append(result, inst.plugin.GetInfo())
	}
	return result
}

func (r *Runtime) GetResourceProvider(id string) (types.ResourceProvider, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	inst, exists := r.plugins[id]
	if !exists {
		return nil, false
	}

	return inst.plugin.GetResourceProvider()
}

func (r *Runtime) GetTaskExecutor(id string) (types.TaskExecutor, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	inst, exists := r.plugins[id]
	if !exists {
		return nil, false
	}

	return inst.plugin.GetTaskExecutor()
}

func (r *Runtime) SetSandboxPermission(id string, permission string, allowed bool) {
	r.sandbox.SetPermission(id, permission, allowed)
}

type RuntimeStats struct {
	TotalPlugins   int                          `json:"total_plugins"`
	RunningPlugins int                          `json:"running_plugins"`
	PluginStates   map[string]types.PluginState `json:"plugin_states"`
	ResourceStats  ResourceStats                `json:"resource_stats"`
	Timestamp      time.Time                    `json:"timestamp"`
}

func (r *Runtime) GetStats() RuntimeStats {
	r.mu.RLock()
	defer r.mu.RUnlock()

	stats := RuntimeStats{
		TotalPlugins:  len(r.plugins),
		PluginStates:  make(map[string]types.PluginState),
		ResourceStats: r.resourceMgr.GetStats(),
		Timestamp:     time.Now(),
	}

	for id, inst := range r.plugins {
		inst.mu.RLock()
		stats.PluginStates[id] = inst.state
		if inst.state == types.StateRunning {
			stats.RunningPlugins++
		}
		inst.mu.RUnlock()
	}

	return stats
}

func (r *Runtime) Shutdown(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.logger.Info("Shutting down plugin runtime...")

	var wg sync.WaitGroup
	for id, inst := range r.plugins {
		wg.Add(1)
		go func(id string, inst *pluginInstance) {
			defer wg.Done()
			inst.mu.Lock()
			if inst.state == types.StateRunning || inst.state == types.StatePaused {
				inst.plugin.Stop()
			}
			inst.mu.Unlock()
		}(id, inst)
	}

	wg.Wait()

	r.plugins = make(map[string]*pluginInstance)
	r.logger.Info("Plugin runtime shut down complete")
	return nil
}
