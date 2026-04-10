package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/kerrigan/kerrigan/pkg/log"
)

// DeerFlowAdapter communicates with DeerFlow API
type DeerFlowAdapter struct {
	baseURL string // http://host:8000
	client  *http.Client
}

// DeerFlowRequest represents a request to DeerFlow
type DeerFlowRequest struct {
	Query    string                 `json:"query"`
	TaskType string                 `json:"task_type,omitempty"` // "research", "code", "general"
	Options  map[string]interface{} `json:"options,omitempty"`
}

// DeerFlowResponse represents a response from DeerFlow
type DeerFlowResponse struct {
	TaskID string                 `json:"task_id"`
	Status string                 `json:"status"` // "pending", "processing", "completed", "failed"
	Result map[string]interface{} `json:"result,omitempty"`
	Error  string                 `json:"error,omitempty"`
}

// DeerFlowTaskStatus represents the status of a DeerFlow task
type DeerFlowTaskStatus struct {
	TaskID   string                 `json:"task_id"`
	Status   string                 `json:"status"`
	Progress float64                `json:"progress,omitempty"`
	Result   map[string]interface{} `json:"result,omitempty"`
	Error    string                 `json:"error,omitempty"`
}

// DeerFlowConfig represents DeerFlow configuration
type DeerFlowConfig struct {
	BaseURL    string
	APIKey     string
	Timeout    time.Duration
	MaxRetries int
}

// NewDeerFlowAdapter creates a new DeerFlow adapter
func NewDeerFlowAdapter(baseURL string) *DeerFlowAdapter {
	return &DeerFlowAdapter{
		baseURL: baseURL,
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// Start starts the DeerFlow adapter
func (a *DeerFlowAdapter) Start(ctx context.Context) error {
	// DeerFlow uses stateless HTTP API, no persistent connection needed
	// Just verify the endpoint is reachable
	if err := a.ping(); err != nil {
		log.Warn("DeerFlow API not available", "error", err)
		// Don't fail - adapter will work when API becomes available
	}

	log.Info("DeerFlow adapter started", "base_url", a.baseURL)
	return nil
}

// ping checks if DeerFlow API is available
func (a *DeerFlowAdapter) ping() error {
	resp, err := a.client.Get(a.baseURL + "/health")
	if err != nil {
		return fmt.Errorf("failed to ping DeerFlow: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("DeerFlow health check failed: %d", resp.StatusCode)
	}

	return nil
}

// Stop stops the DeerFlow adapter
func (a *DeerFlowAdapter) Stop() error {
	// No persistent connections to close for HTTP API
	log.Info("DeerFlow adapter stopped")
	return nil
}

// ExecuteTask executes a task via DeerFlow API
func (a *DeerFlowAdapter) ExecuteTask(ctx context.Context, req *DeerFlowRequest) (*DeerFlowResponse, error) {
	// Build request body
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", a.baseURL+"/api/v1/tasks", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	// Execute request
	resp, err := a.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Check status code
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		return nil, fmt.Errorf("DeerFlow API error: %d - %s", resp.StatusCode, string(respBody))
	}

	// Parse response
	var deerFlowResp DeerFlowResponse
	if err := json.Unmarshal(respBody, &deerFlowResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &deerFlowResp, nil
}

// GetTaskStatus gets the status of a task
func (a *DeerFlowAdapter) GetTaskStatus(ctx context.Context, taskID string) (*DeerFlowTaskStatus, error) {
	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "GET", a.baseURL+"/api/v1/tasks/"+taskID, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	// Execute request
	resp, err := a.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Check status code
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("DeerFlow API error: %d - %s", resp.StatusCode, string(respBody))
	}

	// Parse response
	var status DeerFlowTaskStatus
	if err := json.Unmarshal(respBody, &status); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &status, nil
}

// CancelTask cancels a running task
func (a *DeerFlowAdapter) CancelTask(ctx context.Context, taskID string) error {
	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "DELETE", a.baseURL+"/api/v1/tasks/"+taskID, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Execute request
	resp, err := a.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("DeerFlow API error: %d", resp.StatusCode)
	}

	return nil
}

// ExecuteResearchTask executes a deep research task
func (a *DeerFlowAdapter) ExecuteResearchTask(ctx context.Context, query string) (*DeerFlowResponse, error) {
	req := &DeerFlowRequest{
		Query:    query,
		TaskType: "research",
		Options: map[string]interface{}{
			"depth": "deep",
		},
	}
	return a.ExecuteTask(ctx, req)
}

// ExecuteCodeTask executes a code generation task
func (a *DeerFlowAdapter) ExecuteCodeTask(ctx context.Context, prompt string) (*DeerFlowResponse, error) {
	req := &DeerFlowRequest{
		Query:    prompt,
		TaskType: "code",
		Options: map[string]interface{}{
			"language": "auto",
		},
	}
	return a.ExecuteTask(ctx, req)
}

// IsAvailable returns whether the DeerFlow API is available
func (a *DeerFlowAdapter) IsAvailable() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = ctx // ctx is used by ping internally

	err := a.ping()
	return err == nil
}

// GetBaseURL returns the base URL
func (a *DeerFlowAdapter) GetBaseURL() string {
	return a.baseURL
}

// SetTimeout sets the request timeout
func (a *DeerFlowAdapter) SetTimeout(timeout time.Duration) {
	a.client.Timeout = timeout
}
