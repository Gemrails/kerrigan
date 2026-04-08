package plugin

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/kerrigan/kerrigan/internal/core/plugin/loader"
	"github.com/kerrigan/kerrigan/internal/core/plugin/registry"
	"github.com/kerrigan/kerrigan/internal/core/plugin/runtime"
	"github.com/sirupsen/logrus"
)

// Manager manages the plugin system
type Manager struct {
	mu        sync.RWMutex
	logger    *logrus.Logger
	runtime   *runtime.Runtime
	registry  *registry.Registry
	loader    *loader.Loader
	dataDir   string
	configDir string
	logDir    string
	pluginDir string
}

// Config contains configuration for the plugin manager
type Config struct {
	DataDir   string
	LogDir    string
	PluginDir string
}

// NewManager creates a new plugin manager
func NewManager(cfg Config, logger *logrus.Logger) (*Manager, error) {
	// Ensure directories exist
	dirs := []string{cfg.DataDir, cfg.LogDir, cfg.PluginDir}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("create directory %s: %w", dir, err)
		}
	}

	// Create runtime
	rt, err := runtime.NewRuntime(
		filepath.Join(cfg.DataDir, "runtime"),
		filepath.Join(cfg.DataDir, "config"),
		filepath.Join(cfg.LogDir, "plugins"),
		logger,
	)
	if err != nil {
		return nil, fmt.Errorf("create runtime: %w", err)
	}

	// Create registry
	reg, err := registry.NewRegistry(filepath.Join(cfg.DataDir, "registry"), logger)
	if err != nil {
		return nil, fmt.Errorf("create registry: %w", err)
	}

	// Create loader
	ldr, err := loader.NewLoader(cfg.PluginDir, logger)
	if err != nil {
		return nil, fmt.Errorf("create loader: %w", err)
	}

	return &Manager{
		logger:    logger,
		runtime:   rt,
		registry:  reg,
		loader:    ldr,
		dataDir:   cfg.DataDir,
		logDir:    cfg.LogDir,
		pluginDir: cfg.PluginDir,
	}, nil
}

// Install installs a plugin from a path
func (m *Manager) Install(ctx context.Context, pluginPath string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.logger.Infof("Installing plugin from %s", pluginPath)

	// Load manifest from the plugin path
	ldr := m.loader
	_ = ldr // Use loader methods

	// For now, we'll implement basic installation
	// In production, this would copy the plugin and validate it
	pluginID := filepath.Base(pluginPath)

	// Check if already installed
	if _, exists := m.registry.Get(pluginID); exists {
		return fmt.Errorf("plugin %s already installed", pluginID)
	}

	// Register the plugin
	if err := m.registry.Register(
		registry.PluginInfo{
			ID:          pluginID,
			Name:        pluginID,
			Version:     "1.0.0",
			Description: "Installed plugin",
		},
		pluginPath,
		"",
	); err != nil {
		return fmt.Errorf("register plugin: %w", err)
	}

	m.logger.Infof("Plugin %s installed successfully", pluginID)
	return nil
}

// Uninstall uninstalls a plugin
func (m *Manager) Uninstall(ctx context.Context, pluginID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.logger.Infof("Uninstalling plugin %s", pluginID)

	// Stop if running
	state, exists := m.runtime.GetPluginState(pluginID)
	if exists && state == StateRunning {
		if err := m.runtime.StopPlugin(ctx, pluginID); err != nil {
			m.logger.Warnf("Failed to stop plugin before uninstall: %v", err)
		}
	}

	// Unload from runtime
	if _, exists := m.runtime.GetPlugin(pluginID); exists {
		if err := m.runtime.UnloadPlugin(ctx, pluginID); err != nil {
			return fmt.Errorf("unload plugin: %w", err)
		}
	}

	// Unregister from registry
	if err := m.registry.Unregister(pluginID); err != nil {
		return fmt.Errorf("unregister plugin: %w", err)
	}

	m.logger.Infof("Plugin %s uninstalled successfully", pluginID)
	return nil
}

// Enable enables a plugin (load and start)
func (m *Manager) Enable(ctx context.Context, pluginID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.logger.Infof("Enabling plugin %s", pluginID)

	// Get plugin info from registry
	info, exists := m.registry.Get(pluginID)
	if !exists {
		return fmt.Errorf("plugin %s not found in registry", pluginID)
	}

	// Load the plugin (placeholder - actual implementation would load the plugin binary)
	// For now, we'll just update the registry status
	if err := m.registry.SetStatus(pluginID, StateInstalled); err != nil {
		return fmt.Errorf("set status: %w", err)
	}

	m.logger.Infof("Plugin %s enabled successfully", pluginID)
	return nil
}

// Disable disables a plugin (stop and unload)
func (m *Manager) Disable(ctx context.Context, pluginID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.logger.Infof("Disabling plugin %s", pluginID)

	// Stop if running
	state, exists := m.runtime.GetPluginState(pluginID)
	if exists && state == StateRunning {
		if err := m.runtime.StopPlugin(ctx, pluginID); err != nil {
			m.logger.Warnf("Failed to stop plugin: %v", err)
		}
	}

	// Unload from runtime
	if _, exists := m.runtime.GetPlugin(pluginID); exists {
		if err := m.runtime.UnloadPlugin(ctx, pluginID); err != nil {
			return fmt.Errorf("unload plugin: %w", err)
		}
	}

	// Update status
	if err := m.registry.SetStatus(pluginID, StateInstalled); err != nil {
		return fmt.Errorf("set status: %w", err)
	}

	m.logger.Infof("Plugin %s disabled successfully", pluginID)
	return nil
}

// Start starts a plugin
func (m *Manager) Start(ctx context.Context, pluginID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.logger.Infof("Starting plugin %s", pluginID)

	// Check if registered
	if _, exists := m.registry.Get(pluginID); !exists {
		return fmt.Errorf("plugin %s not found", pluginID)
	}

	// Start in runtime
	if err := m.runtime.StartPlugin(ctx, pluginID); err != nil {
		return fmt.Errorf("start plugin: %w", err)
	}

	// Update registry status
	if err := m.registry.SetStatus(pluginID, StateRunning); err != nil {
		m.logger.Warnf("Failed to update registry status: %v", err)
	}

	m.logger.Infof("Plugin %s started successfully", pluginID)
	return nil
}

// Stop stops a plugin
func (m *Manager) Stop(ctx context.Context, pluginID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.logger.Infof("Stopping plugin %s", pluginID)

	// Stop in runtime
	if err := m.runtime.StopPlugin(ctx, pluginID); err != nil {
		return fmt.Errorf("stop plugin: %w", err)
	}

	// Update registry status
	if err := m.registry.SetStatus(pluginID, StateInstalled); err != nil {
		m.logger.Warnf("Failed to update registry status: %v", err)
	}

	m.logger.Infof("Plugin %s stopped successfully", pluginID)
	return nil
}

// List returns all installed plugins
func (m *Manager) List() []PluginInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.registry.List()
}

// Get returns plugin information
func (m *Manager) Get(pluginID string) (*PluginInfo, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.registry.Get(pluginID)
}

// GetState returns the state of a plugin
func (m *Manager) GetState(pluginID string) (PluginState, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// First check runtime state
	if state, exists := m.runtime.GetPluginState(pluginID); exists {
		return state, true
	}

	// Fall back to registry state
	return m.registry.GetStatus(pluginID)
}

// Discover discovers available plugins in the plugin directory
func (m *Manager) Discover() ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.loader.Discover()
}

// SetSandboxPermission sets sandbox permission for a plugin
func (m *Manager) SetSandboxPermission(pluginID, permission string, allowed bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	m.runtime.SetSandboxPermission(pluginID, permission, allowed)
}

// GetResourceProvider returns the resource provider for a plugin
func (m *Manager) GetResourceProvider(pluginID string) (ResourceProvider, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.runtime.GetResourceProvider(pluginID)
}

// GetTaskExecutor returns the task executor for a plugin
func (m *Manager) GetTaskExecutor(pluginID string) (TaskExecutor, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.runtime.GetTaskExecutor(pluginID)
}

// GetStats returns plugin system statistics
func (m *Manager) GetStats() ManagerStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rtStats := m.runtime.GetStats()

	return ManagerStats{
		TotalPlugins:   m.registry.Count(),
		RunningPlugins: rtStats.RunningPlugins,
		PluginStates:   rtStats.PluginStates,
		ResourceStats:  rtStats.ResourceStats,
		Timestamp:      time.Now(),
	}
}

// ManagerStats contains statistics about the plugin manager
type ManagerStats struct {
	TotalPlugins   int                    `json:"total_plugins"`
	RunningPlugins int                    `json:"running_plugins"`
	PluginStates   map[string]PluginState `json:"plugin_states"`
	ResourceStats  runtime.ResourceStats  `json:"resource_stats"`
	Timestamp      time.Time              `json:"timestamp"`
}

// Shutdown shuts down the plugin manager gracefully
func (m *Manager) Shutdown(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.logger.Info("Shutting down plugin manager...")

	// Shutdown runtime (stops all plugins)
	if err := m.runtime.Shutdown(ctx); err != nil {
		m.logger.Errorf("Error shutting down runtime: %v", err)
	}

	m.logger.Info("Plugin manager shut down complete")
	return nil
}

// EnableVerification enables signature verification
func (m *Manager) EnableVerification() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.loader.EnableVerification()
}

// DisableVerification disables signature verification
func (m *Manager) DisableVerification() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.loader.DisableVerification()
}

// SetResourceLimits sets resource limits for a resource type
func (m *Manager) SetResourceLimits(resourceType ResourceType, limits ResourceLimits) {
	m.mu.RLock()
	// Access would need to go through runtime's resource manager
	// This is a placeholder - actual implementation would expose this
	m.mu.RUnlock()
}

// GetRegistry returns the registry instance
func (m *Manager) GetRegistry() *registry.Registry {
	return m.registry
}

// GetLoader returns the loader instance
func (m *Manager) GetLoader() *loader.Loader {
	return m.loader
}

// GetRuntime returns the runtime instance
func (m *Manager) GetRuntime() *runtime.Runtime {
	return m.runtime
}

// ValidatePlugin validates a plugin before installation
func (m *Manager) ValidatePlugin(ctx context.Context, pluginPath string) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Load and verify manifest
	manifest, err := m.loader.LoadManifest(filepath.Base(pluginPath))
	if err != nil {
		return fmt.Errorf("load manifest: %w", err)
	}

	// Verify manifest structure
	if err := m.loader.VerifyManifest(manifest); err != nil {
		return fmt.Errorf("verify manifest: %w", err)
	}

	// Verify signature if enabled
	if err := m.loader.VerifySignature(manifest.ID, manifest); err != nil {
		return fmt.Errorf("verify signature: %w", err)
	}

	return nil
}

// BackupRegistry backs up the plugin registry
func (m *Manager) BackupRegistry(path string) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	data, err := m.registry.Export()
	if err != nil {
		return fmt.Errorf("export registry: %w", err)
	}

	return os.WriteFile(path, data, 0644)
}

// RestoreRegistry restores the plugin registry from a backup
func (m *Manager) RestoreRegistry(path string) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read backup: %w", err)
	}

	return m.registry.Import(data)
}

// GetPluginDirectory returns the plugin directory path
func (m *Manager) GetPluginDirectory() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.pluginDir
}

// GetDataDirectory returns the data directory path
func (m *Manager) GetDataDirectory() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.dataDir
}

// Clear clears all registered plugins
func (m *Manager) Clear(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Stop all plugins
	for _, info := range m.registry.List() {
		if state, _ := m.runtime.GetPluginState(info.ID); state == StateRunning {
			m.runtime.StopPlugin(ctx, info.ID)
		}
		m.runtime.UnloadPlugin(ctx, info.ID)
	}

	// Clear registry
	if err := m.registry.Clear(); err != nil {
		return fmt.Errorf("clear registry: %w", err)
	}

	m.logger.Info("All plugins cleared")
	return nil
}
