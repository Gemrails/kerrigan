package plugins

import (
	"context"
	"time"
)

type Plugin interface {
	Name() string
	Version() string
	Initialize(configBytes []byte) error
	Shutdown() error
}

type ResourceProvider interface {
	ListResources() ([]Resource, error)
	GetResource(id string) (*Resource, error)
	AllocateResource(id string, memoryMB int) error
	ReleaseResource(id string, memoryMB int) error
}

type TaskExecutor interface {
	ExecuteTask(ctx context.Context, task *Task) (*TaskResult, error)
}

type Resource struct {
	ID        string
	Type      string
	Name      string
	Capacity  float64
	Available float64
	Metadata  map[string]interface{}
}

type Task struct {
	ID               string
	Type             string
	RequiredMemoryMB int
	Metadata         map[string]interface{}
}

type TaskResult struct {
	Status string
	Output map[string]interface{}
	Error  string
}

type TaskStatus string

const (
	TaskStatusPending   TaskStatus = "pending"
	TaskStatusRunning   TaskStatus = "running"
	TaskStatusCompleted TaskStatus = "completed"
	TaskStatusFailed    TaskStatus = "failed"
)

type TaskExecutorResult struct {
	TaskID   string
	Success  bool
	Output   map[string]interface{}
	Error    string
	Duration time.Duration
}
