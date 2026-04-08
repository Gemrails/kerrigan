package registry

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// Version represents a semantic version
type Version struct {
	Major int
	Minor int
	Patch int
}

func (v Version) String() string {
	return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
}

// PluginState represents the state of a plugin
type PluginState string

const (
	StateUnknown   PluginState = "unknown"
	StateInstalled PluginState = "installed"
	StateLoading   PluginState = "loading"
	StateStarting  PluginState = "starting"
	StateRunning   PluginState = "running"
	StatePaused    PluginState = "paused"
	StateStopping  PluginState = "stopping"
	StateFailed    PluginState = "failed"
)

// Capability represents what a plugin can do
type Capability string

// PluginInfo represents the metadata of a plugin
type PluginInfo struct {
	ID           string       `json:"id"`
	Name         string       `json:"name"`
	Version      string       `json:"version"`
	Description  string       `json:"description"`
	Author       string       `json:"author"`
	License      string       `json:"license"`
	Homepage     string       `json:"homepage"`
	Capabilities []Capability `json:"capabilities"`
	ResourceType string       `json:"resource_type"`
}

// Registry manages local plugin metadata storage
type Registry struct {
	mu       sync.RWMutex
	logger   *logrus.Logger
	dataDir  string
	plugins  map[string]*PluginEntry
	versions map[string][]Version
}

// PluginEntry represents a registered plugin
type PluginEntry struct {
	Info        PluginInfo        `json:"info"`
	InstallTime time.Time         `json:"install_time"`
	UpdateTime  time.Time         `json:"update_time"`
	Status      PluginState       `json:"status"`
	Path        string            `json:"path"`
	Checksum    string            `json:"checksum"`
	Metadata    map[string]string `json:"metadata"`
}

// NewRegistry creates a new plugin registry
func NewRegistry(dataDir string, logger *logrus.Logger) (*Registry, error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("create registry directory: %w", err)
	}

	reg := &Registry{
		logger:   logger,
		dataDir:  dataDir,
		plugins:  make(map[string]*PluginEntry),
		versions: make(map[string][]Version),
	}

	if err := reg.load(); err != nil {
		logger.Warnf("Failed to load registry data: %v", err)
	}

	return reg, nil
}

// Register registers a new plugin in the registry
func (r *Registry) Register(info PluginInfo, path, checksum string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.plugins[info.ID]; exists {
		return fmt.Errorf("plugin %s already registered", info.ID)
	}

	entry := &PluginEntry{
		Info:        info,
		InstallTime: time.Now(),
		UpdateTime:  time.Now(),
		Status:      StateInstalled,
		Path:        path,
		Checksum:    checksum,
		Metadata:    make(map[string]string),
	}

	r.plugins[info.ID] = entry
	r.versions[info.ID] = append(r.versions[info.ID], parseVersion(info.Version))

	if err := r.save(); err != nil {
		r.logger.Errorf("Failed to save registry: %v", err)
	}

	r.logger.Infof("Registered plugin: %s v%s", info.Name, info.Version)
	return nil
}

// Unregister removes a plugin from the registry
func (r *Registry) Unregister(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.plugins[id]; !exists {
		return fmt.Errorf("plugin %s not found", id)
	}

	delete(r.plugins, id)
	delete(r.versions, id)

	if err := r.save(); err != nil {
		return fmt.Errorf("save registry: %w", err)
	}

	r.logger.Infof("Unregistered plugin: %s", id)
	return nil
}

// Get returns plugin information by ID
func (r *Registry) Get(id string) (*PluginInfo, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	entry, exists := r.plugins[id]
	if !exists {
		return nil, false
	}

	return &entry.Info, true
}

// GetEntry returns full plugin entry by ID
func (r *Registry) GetEntry(id string) (*PluginEntry, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	entry, exists := r.plugins[id]
	if !exists {
		return nil, false
	}

	return entry, true
}

// List returns all registered plugins
func (r *Registry) List() []PluginInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]PluginInfo, 0, len(r.plugins))
	for _, entry := range r.plugins {
		result = append(result, entry.Info)
	}

	return result
}

// Update updates plugin metadata
func (r *Registry) Update(id string, status PluginState, metadata map[string]string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	entry, exists := r.plugins[id]
	if !exists {
		return fmt.Errorf("plugin %s not found", id)
	}

	entry.Status = status
	entry.UpdateTime = time.Now()

	if metadata != nil {
		for k, v := range metadata {
			entry.Metadata[k] = v
		}
	}

	if err := r.save(); err != nil {
		return fmt.Errorf("save registry: %w", err)
	}

	return nil
}

// UpdateVersion adds a new version to the plugin
func (r *Registry) UpdateVersion(id string, version string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	entry, exists := r.plugins[id]
	if !exists {
		return fmt.Errorf("plugin %s not found", id)
	}

	entry.Info.Version = version
	entry.UpdateTime = time.Now()
	r.versions[id] = append(r.versions[id], parseVersion(version))

	if err := r.save(); err != nil {
		return fmt.Errorf("save registry: %w", err)
	}

	return nil
}

// GetVersions returns all versions of a plugin
func (r *Registry) GetVersions(id string) []Version {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.versions[id]
}

// GetLatestVersion returns the latest version of a plugin
func (r *Registry) GetLatestVersion(id string) (Version, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	versions := r.versions[id]
	if len(versions) == 0 {
		return Version{}, false
	}

	return versions[len(versions)-1], true
}

// FindByName finds plugins by name
func (r *Registry) FindByName(name string) []PluginInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []PluginInfo
	for _, entry := range r.plugins {
		if entry.Info.Name == name {
			result = append(result, entry.Info)
		}
	}

	return result
}

// FindByResourceType finds plugins by resource type
func (r *Registry) FindByResourceType(resourceType string) []PluginInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []PluginInfo
	for _, entry := range r.plugins {
		if entry.Info.ResourceType == resourceType {
			result = append(result, entry.Info)
		}
	}

	return result
}

// FindByCapability finds plugins by capability
func (r *Registry) FindByCapability(cap Capability) []PluginInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []PluginInfo
	for _, entry := range r.plugins {
		for _, c := range entry.Info.Capabilities {
			if c == cap {
				result = append(result, entry.Info)
				break
			}
		}
	}

	return result
}

// SetStatus sets the status of a plugin
func (r *Registry) SetStatus(id string, status PluginState) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	entry, exists := r.plugins[id]
	if !exists {
		return fmt.Errorf("plugin %s not found", id)
	}

	entry.Status = status
	entry.UpdateTime = time.Now()

	return r.save()
}

// GetStatus returns the status of a plugin
func (r *Registry) GetStatus(id string) (PluginState, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	entry, exists := r.plugins[id]
	if !exists {
		return "", false
	}

	return entry.Status, true
}

// GetPath returns the path of a plugin
func (r *Registry) GetPath(id string) (string, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	entry, exists := r.plugins[id]
	if !exists {
		return "", false
	}

	return entry.Path, true
}

// GetChecksum returns the checksum of a plugin
func (r *Registry) GetChecksum(id string) (string, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	entry, exists := r.plugins[id]
	if !exists {
		return "", false
	}

	return entry.Checksum, true
}

// save persists registry data to disk
func (r *Registry) save() error {
	data := r.marshal()

	filename := filepath.Join(r.dataDir, "registry.json")
	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("write registry file: %w", err)
	}

	return nil
}

// load loads registry data from disk
func (r *Registry) load() error {
	filename := filepath.Join(r.dataDir, "registry.json")

	data, err := os.ReadFile(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read registry file: %w", err)
	}

	r.unmarshal(data)

	return nil
}

// marshal serializes the registry to JSON
func (r *Registry) marshal() []byte {
	type registryData struct {
		Plugins  map[string]*PluginEntry `json:"plugins"`
		Versions map[string][]Version    `json:"versions"`
	}

	data := registryData{
		Plugins:  r.plugins,
		Versions: r.versions,
	}

	result, _ := json.MarshalIndent(data, "", "  ")
	return result
}

// unmarshal deserializes JSON to registry
func (r *Registry) unmarshal(data []byte) {
	type registryData struct {
		Plugins  map[string]*PluginEntry `json:"plugins"`
		Versions map[string][]Version    `json:"versions"`
	}

	var rd registryData
	if err := json.Unmarshal(data, &rd); err != nil {
		r.logger.Errorf("Failed to unmarshal registry: %v", err)
		return
	}

	r.plugins = rd.Plugins
	r.versions = rd.Versions
}

// Clear removes all plugins from the registry
func (r *Registry) Clear() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.plugins = make(map[string]*PluginEntry)
	r.versions = make(map[string][]Version)

	if err := r.save(); err != nil {
		return fmt.Errorf("save registry: %w", err)
	}

	r.logger.Info("Registry cleared")
	return nil
}

// Count returns the number of registered plugins
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.plugins)
}

// Export exports registry data
func (r *Registry) Export() ([]byte, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.marshal(), nil
}

// Import imports registry data
func (r *Registry) Import(data []byte) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.unmarshal(data)

	if err := r.save(); err != nil {
		return fmt.Errorf("save registry: %w", err)
	}

	return nil
}

// parseVersion parses a version string into Version struct
func parseVersion(v string) Version {
	var major, minor, patch int
	fmt.Sscanf(v, "%d.%d.%d", &major, &minor, &patch)

	return Version{
		Major: major,
		Minor: minor,
		Patch: patch,
	}
}

// ConvertToPluginInfo converts PluginInfo to map for compatibility with plugin package
func (r *Registry) ConvertToPluginInfo() []map[string]interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]map[string]interface{}, 0, len(r.plugins))
	for _, entry := range r.plugins {
		result = append(result, map[string]interface{}{
			"id":            entry.Info.ID,
			"name":          entry.Info.Name,
			"version":       entry.Info.Version,
			"description":   entry.Info.Description,
			"author":        entry.Info.Author,
			"license":       entry.Info.License,
			"homepage":      entry.Info.Homepage,
			"capabilities":  entry.Info.Capabilities,
			"resource_type": entry.Info.ResourceType,
			"status":        entry.Status,
		})
	}
	return result
}
