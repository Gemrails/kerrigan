package plugin

import (
	"context"
	"fmt"
	"time"
)

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
	Dependencies []Dependency `json:"dependencies"`
	ResourceType ResourceType `json:"resource_type"`
	InstallTime  time.Time    `json:"install_time"`
	Signature    string       `json:"signature"`
}

// Capability represents what a plugin can do
type Capability string

const (
	// Resource capabilities
	CapabilityGPU       Capability = "gpu"
	CapabilityStorage   Capability = "storage"
	CapabilityBandwidth Capability = "bandwidth"
	CapabilityCompute   Capability = "compute"

	// Functional capabilities
	CapabilityTaskExecution Capability = "task_execution"
	CapabilityResourceQuery Capability = "resource_query"
	CapabilityMetering      Capability = "metering"
)

// Dependency represents a plugin dependency
type Dependency struct {
	ID       string  `json:"id"`
	Version  Version `json:"version"`
	Optional bool    `json:"optional"`
}

// Version represents a semantic version
type Version struct {
	Major int
	Minor int
	Patch int
}

func (v Version) String() string {
	return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
}

// PluginConfig contains configuration for plugin initialization
type PluginConfig struct {
	ID        string
	Name      string
	Version   string
	DataDir   string
	ConfigDir string
	LogDir    string
	Env       map[string]string
	Resources ResourceLimits
}

// ResourceLimits defines resource constraints for a plugin
type ResourceLimits struct {
	MaxCPU        int64         // CPU cores (millicores)
	MaxMemory     int64         // bytes
	MaxStorage    int64         // bytes
	MaxBandwidth  int64         // bytes per second
	MaxConcurrent int           // max concurrent tasks
	Timeout       time.Duration // task timeout
}

// PluginState represents the current state of a plugin
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

// Plugin is the main interface that all plugins must implement
type Plugin interface {
	// Init initializes the plugin with configuration
	Init(ctx context.Context, cfg PluginConfig) error

	// Start starts the plugin
	Start() error

	// Stop stops the plugin gracefully
	Stop() error

	// GetInfo returns plugin metadata
	GetInfo() PluginInfo

	// GetCapabilities returns plugin capabilities
	GetCapabilities() []Capability

	// GetState returns current plugin state
	GetState() PluginState

	// GetResourceProvider returns resource provider interface if supported
	GetResourceProvider() (ResourceProvider, bool)

	// GetTaskExecutor returns task executor interface if supported
	GetTaskExecutor() (TaskExecutor, bool)
}

// ResourceProvider handles resource allocation and management
type ResourceProvider interface {
	// QueryResources returns available resources
	QueryResources() (ResourceList, error)

	// AllocateResources allocates resources for a request
	AllocateResources(req ResourceRequest) (Allocation, error)

	// ReleaseResources releases allocated resources
	ReleaseResources(id AllocationID) error

	// GetResourceStats returns current resource statistics
	GetResourceStats() (ResourceStats, error)
}

// ResourceList contains available resources
type ResourceList struct {
	Resources  []Resource
	TotalCount int
	Timestamp  time.Time
}

// Resource represents a single resource unit
type Resource struct {
	ID         ResourceID             `json:"id"`
	Type       ResourceType           `json:"type"`
	Name       string                 `json:"name"`
	Capacity   ResourceCapacity       `json:"capacity"`
	Available  bool                   `json:"available"`
	Price      string                 `json:"price"`    // per unit price
	Location   string                 `json:"location"` // geographic location
	Properties map[string]interface{} `json:"properties"`
}

// ResourceID is a unique identifier for a resource
type ResourceID string

// ResourceType represents the type of resource
type ResourceType string

const (
	ResourceTypeGPU       ResourceType = "gpu"
	ResourceTypeStorage   ResourceType = "storage"
	ResourceTypeBandwidth ResourceType = "bandwidth"
	ResourceTypeCPU       ResourceType = "cpu"
)

// ResourceCapacity describes the capacity of a resource
type ResourceCapacity struct {
	CPU       float64 `json:"cpu"`       // cores
	Memory    int64   `json:"memory"`    // bytes
	Storage   int64   `json:"storage"`   // bytes
	Bandwidth int64   `json:"bandwidth"` // bytes per second
	GPU       GPUInfo `json:"gpu"`
}

// GPUInfo describes GPU resource
type GPUInfo struct {
	Model   string  `json:"model"`
	VRAM    int64   `json:"vram"` // bytes
	Count   int     `json:"count"`
	Compute float64 `json:"compute"` // TFLOPS
}

// ResourceRequest represents a request for resources
type ResourceRequest struct {
	Type        ResourceType        `json:"type"`
	Amount      int                 `json:"amount"`
	MinCapacity ResourceCapacity    `json:"min_capacity"`
	Duration    time.Duration       `json:"duration"`
	Preferences ResourcePreferences `json:"preferences"`
}

// ResourcePreferences specifies preferences for resource selection
type ResourcePreferences struct {
	Location   string            `json:"location"`
	MinPrice   string            `json:"min_price"`
	MaxPrice   string            `json:"max_price"`
	Properties map[string]string `json:"properties"`
}

// Allocation represents a resource allocation
type Allocation struct {
	ID        AllocationID     `json:"id"`
	Resources []ResourceID     `json:"resources"`
	StartTime time.Time        `json:"start_time"`
	EndTime   time.Time        `json:"end_time"`
	Status    AllocationStatus `json:"status"`
}

// AllocationID is a unique identifier for an allocation
type AllocationID string

// AllocationStatus represents the status of an allocation
type AllocationStatus string

const (
	AllocationPending  AllocationStatus = "pending"
	AllocationActive   AllocationStatus = "active"
	AllocationReleased AllocationStatus = "released"
	AllocationFailed   AllocationStatus = "failed"
)

// ResourceStats contains resource statistics
type ResourceStats struct {
	TotalResources    int64            `json:"total_resources"`
	AvailableCount    int64            `json:"available_count"`
	AllocatedCount    int64            `json:"allocated_count"`
	Utilization       float64          `json:"utilization"` // 0-1
	AvailableCapacity ResourceCapacity `json:"available_capacity"`
	Timestamp         time.Time        `json:"timestamp"`
}

// TaskExecutor handles task execution
type TaskExecutor interface {
	// Execute executes a task and returns result
	Execute(ctx context.Context, task Task) (*Result, error)

	// QueryStatus returns the status of a task
	QueryStatus(taskID string) (TaskStatus, error)

	// CancelTask cancels a running task
	CancelTask(taskID string) error

	// ListTasks returns list of tasks
	ListTasks(filter TaskFilter) ([]Task, error)
}

// Task represents a workload to be executed
type Task struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type"`
	Payload   map[string]interface{} `json:"payload"`
	Resources ResourceRequest        `json:"resources"`
	Priority  int                    `json:"priority"` // 0-100, higher is more urgent
	Timeout   time.Duration          `json:"timeout"`
	Metadata  map[string]string      `json:"metadata"`
}

// Result contains the result of a task execution
type Result struct {
	TaskID   string                 `json:"task_id"`
	Success  bool                   `json:"success"`
	Output   map[string]interface{} `json:"output"`
	Error    string                 `json:"error"`
	Duration time.Duration          `json:"duration"`
	Metrics  TaskMetrics            `json:"metrics"`
}

// TaskMetrics contains execution metrics
type TaskMetrics struct {
	CPUUsed     float64 `json:"cpu_used"`
	MemoryUsed  int64   `json:"memory_used"`
	GPUUsed     float64 `json:"gpu_used"`
	StorageUsed int64   `json:"storage_used"`
	NetworkIn   int64   `json:"network_in"`
	NetworkOut  int64   `json:"network_out"`
}

// TaskStatus represents the status of a task
type TaskStatus string

const (
	TaskStatusPending   TaskStatus = "pending"
	TaskStatusRunning   TaskStatus = "running"
	TaskStatusCompleted TaskStatus = "completed"
	TaskStatusFailed    TaskStatus = "failed"
	TaskStatusCancelled TaskStatus = "cancelled"
)

// TaskFilter filters tasks
type TaskFilter struct {
	Status    TaskStatus
	Type      string
	StartTime time.Time
	EndTime   time.Time
	Limit     int
}

// PluginLogger interface for plugin logging
type PluginLogger interface {
	Debug(msg string, args ...interface{})
	Info(msg string, args ...interface{})
	Warn(msg string, args ...interface{})
	Error(msg string, args ...interface{})
	Fatal(msg string, args ...interface{})
}

// PluginMetrics interface for plugin metrics reporting
type PluginMetrics interface {
	IncCounter(name string, tags map[string]string, value int64)
	IncGauge(name string, tags map[string]string, value float64)
	RecordHistogram(name string, tags map[string]string, value float64)
}
