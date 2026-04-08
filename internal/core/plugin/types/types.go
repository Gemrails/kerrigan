package types

import (
	"context"
	"time"
)

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

type ResourceType string

const (
	ResourceTypeGPU       ResourceType = "gpu"
	ResourceTypeStorage   ResourceType = "storage"
	ResourceTypeBandwidth ResourceType = "bandwidth"
	ResourceTypeCPU       ResourceType = "cpu"
)

type Capability string

const (
	CapabilityGPU           Capability = "gpu"
	CapabilityStorage       Capability = "storage"
	CapabilityBandwidth     Capability = "bandwidth"
	CapabilityCompute       Capability = "compute"
	CapabilityTaskExecution Capability = "task_execution"
	CapabilityResourceQuery Capability = "resource_query"
	CapabilityMetering      Capability = "metering"
)

type ResourceLimits struct {
	MaxCPU        int64
	MaxMemory     int64
	MaxStorage    int64
	MaxBandwidth  int64
	MaxConcurrent int
	Timeout       time.Duration
}

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

type PluginInfo struct {
	ID           string
	Name         string
	Version      string
	Description  string
	Author       string
	License      string
	Homepage     string
	Capabilities []Capability
	ResourceType ResourceType
	InstallTime  time.Time
	Signature    string
}

type Resource struct {
	ID         string
	Type       ResourceType
	Name       string
	Capacity   ResourceCapacity
	Available  bool
	Price      string
	Location   string
	Properties map[string]interface{}
}

type ResourceCapacity struct {
	CPU       float64
	Memory    int64
	Storage   int64
	Bandwidth int64
	GPU       GPUInfo
}

type GPUInfo struct {
	Model   string
	VRAM    int64
	Count   int
	Compute float64
}

type Task struct {
	ID        string
	Type      string
	Payload   map[string]interface{}
	Resources ResourceType
	Priority  int
	Timeout   time.Duration
	Metadata  map[string]string
}

type Result struct {
	TaskID   string
	Success  bool
	Output   map[string]interface{}
	Error    string
	Duration time.Duration
}

type TaskStatus string

const (
	TaskStatusPending  TaskStatus = "pending"
	TaskStatusRunning  TaskStatus = "running"
	TaskStatusDone     TaskStatus = "completed"
	TaskStatusFailed   TaskStatus = "failed"
	TaskStatusCanceled TaskStatus = "cancelled"
)

type Plugin interface {
	Init(ctx context.Context, cfg PluginConfig) error
	Start() error
	Stop() error
	GetInfo() PluginInfo
	GetCapabilities() []Capability
	GetState() PluginState
	GetResourceProvider() (ResourceProvider, bool)
	GetTaskExecutor() (TaskExecutor, bool)
}

type ResourceProvider interface {
	QueryResources() ([]Resource, error)
}

type TaskExecutor interface {
	Execute(ctx context.Context, task Task) (*Result, error)
	QueryStatus(taskID string) (TaskStatus, error)
}
