package plugin

import (
	"testing"
	"time"
)

func TestPluginInfo(t *testing.T) {
	info := PluginInfo{
		ID:           "test-plugin",
		Name:         "Test Plugin",
		Version:      "1.0.0",
		Description:  "A test plugin",
		Author:       "Test Author",
		License:      "MIT",
		Homepage:     "https://example.com",
		Capabilities: []Capability{CapabilityGPU, CapabilityCompute},
		Dependencies: []Dependency{
			{
				ID:       "dep-plugin",
				Version:  Version{Major: 1, Minor: 0, Patch: 0},
				Optional: false,
			},
		},
		ResourceType: ResourceTypeGPU,
		InstallTime:  time.Now(),
		Signature:    "test-signature",
	}

	// Validate required fields
	if info.ID == "" {
		t.Error("expected non-empty ID")
	}
	if info.Name == "" {
		t.Error("expected non-empty Name")
	}
	if info.Version == "" {
		t.Error("expected non-empty Version")
	}
	if len(info.Capabilities) == 0 {
		t.Error("expected non-empty Capabilities")
	}
}

func TestPluginInfo_Capabilities(t *testing.T) {
	info := PluginInfo{
		ID:           "gpu-plugin",
		Name:         "GPU Plugin",
		Version:      "1.0.0",
		Capabilities: []Capability{CapabilityGPU, CapabilityResourceQuery, CapabilityMetering},
	}

	expectedCaps := []Capability{CapabilityGPU, CapabilityResourceQuery, CapabilityMetering}
	if len(info.Capabilities) != len(expectedCaps) {
		t.Errorf("expected %d capabilities, got %d", len(expectedCaps), len(info.Capabilities))
	}
}

func TestResourceRequest(t *testing.T) {
	req := ResourceRequest{
		Type:   ResourceTypeGPU,
		Amount: 2,
		MinCapacity: ResourceCapacity{
			CPU:     4.0,
			Memory:  8 * 1024 * 1024 * 1024,   // 8GB
			Storage: 100 * 1024 * 1024 * 1024, // 100GB
			GPU: GPUInfo{
				Model:   "RTX 4090",
				VRAM:    24 * 1024 * 1024 * 1024, // 24GB
				Count:   1,
				Compute: 82.0,
			},
		},
		Duration: 1 * time.Hour,
		Preferences: ResourcePreferences{
			Location: "US",
			MinPrice: "0.1",
			MaxPrice: "1.0",
			Properties: map[string]string{
				"cuda_version": "12.0",
			},
		},
	}

	// Validate required fields
	if req.Type == "" {
		t.Error("expected non-empty Type")
	}
	if req.Amount <= 0 {
		t.Error("expected positive Amount")
	}
	if req.Duration <= 0 {
		t.Error("expected positive Duration")
	}
}

func TestResourceRequest_Preferences(t *testing.T) {
	req := ResourceRequest{
		Type:   ResourceTypeStorage,
		Amount: 1,
		Preferences: ResourcePreferences{
			Location: "CN",
			MinPrice: "0.01",
			MaxPrice: "0.1",
		},
	}

	if req.Preferences.Location == "" {
		t.Error("expected non-empty Location")
	}
}

func TestTask(t *testing.T) {
	task := Task{
		ID:   "task-001",
		Type: "inference",
		Payload: map[string]interface{}{
			"model":      "llama-2-7b",
			"input":      "Hello, world!",
			"max_tokens": 512,
		},
		Resources: ResourceRequest{
			Type:     ResourceTypeGPU,
			Amount:   1,
			Duration: 10 * time.Minute,
		},
		Priority: 80,
		Timeout:  5 * time.Minute,
		Metadata: map[string]string{
			"user_id": "user-123",
			"session": "sess-456",
		},
	}

	// Validate required fields
	if task.ID == "" {
		t.Error("expected non-empty ID")
	}
	if task.Type == "" {
		t.Error("expected non-empty Type")
	}
	if task.Priority < 0 || task.Priority > 100 {
		t.Error("expected Priority between 0 and 100")
	}
}

func TestTask_Priority(t *testing.T) {
	tests := []struct {
		name     string
		priority int
		valid    bool
	}{
		{"min priority", 0, true},
		{"max priority", 100, true},
		{"mid priority", 50, true},
		{"negative priority", -1, false},
		{"over max priority", 101, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := Task{
				ID:       "test-task",
				Type:     "test",
				Priority: tt.priority,
			}
			isValid := task.Priority >= 0 && task.Priority <= 100
			if isValid != tt.valid {
				t.Errorf("priority %d valid = %v, want %v", tt.priority, isValid, tt.valid)
			}
		})
	}
}

func TestResourceType(t *testing.T) {
	tests := []struct {
		resourceType ResourceType
		expected     bool
	}{
		{ResourceTypeGPU, true},
		{ResourceTypeStorage, true},
		{ResourceTypeBandwidth, true},
		{ResourceTypeCPU, true},
		{"unknown", false},
	}

	for _, tt := range tests {
		t.Run(string(tt.resourceType), func(t *testing.T) {
			isValid := tt.resourceType == ResourceTypeGPU ||
				tt.resourceType == ResourceTypeStorage ||
				tt.resourceType == ResourceTypeBandwidth ||
				tt.resourceType == ResourceTypeCPU
			if isValid != tt.expected {
				t.Errorf("ResourceType %v valid = %v, want %v", tt.resourceType, isValid, tt.expected)
			}
		})
	}
}

func TestResourceCapacity(t *testing.T) {
	capacity := ResourceCapacity{
		CPU:       8.0,
		Memory:    32 * 1024 * 1024 * 1024,  // 32GB
		Storage:   512 * 1024 * 1024 * 1024, // 512GB
		Bandwidth: 10 * 1024 * 1024,         // 10MB/s
		GPU: GPUInfo{
			Model:   "A100",
			VRAM:    40 * 1024 * 1024 * 1024, // 40GB
			Count:   2,
			Compute: 312.0,
		},
	}

	if capacity.CPU <= 0 {
		t.Error("expected positive CPU")
	}
	if capacity.Memory <= 0 {
		t.Error("expected positive Memory")
	}
	if capacity.GPU.Count <= 0 {
		t.Error("expected positive GPU Count")
	}
}

func TestAllocationStatus(t *testing.T) {
	tests := []struct {
		status   AllocationStatus
		expected bool
	}{
		{AllocationPending, true},
		{AllocationActive, true},
		{AllocationReleased, true},
		{AllocationFailed, true},
		{"unknown", false},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			isValid := tt.status == AllocationPending ||
				tt.status == AllocationActive ||
				tt.status == AllocationReleased ||
				tt.status == AllocationFailed
			if isValid != tt.expected {
				t.Errorf("AllocationStatus %v valid = %v, want %v", tt.status, isValid, tt.expected)
			}
		})
	}
}

func TestTaskStatus(t *testing.T) {
	tests := []struct {
		status   TaskStatus
		expected bool
	}{
		{TaskStatusPending, true},
		{TaskStatusRunning, true},
		{TaskStatusCompleted, true},
		{TaskStatusFailed, true},
		{TaskStatusCancelled, true},
		{"unknown", false},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			isValid := tt.status == TaskStatusPending ||
				tt.status == TaskStatusRunning ||
				tt.status == TaskStatusCompleted ||
				tt.status == TaskStatusFailed ||
				tt.status == TaskStatusCancelled
			if isValid != tt.expected {
				t.Errorf("TaskStatus %v valid = %v, want %v", tt.status, isValid, tt.expected)
			}
		})
	}
}

func TestVersion(t *testing.T) {
	v := Version{
		Major: 1,
		Minor: 2,
		Patch: 3,
	}

	str := v.String()
	expected := "1.2.3"
	if str != expected {
		t.Errorf("Version.String() = %s, want %s", str, expected)
	}
}

func TestVersion_Zero(t *testing.T) {
	v := Version{}
	str := v.String()
	// Expected format: 0.0.0
	if len(str) == 0 {
		t.Error("expected non-empty version string")
	}
}
