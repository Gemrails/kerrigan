package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/kerrigan/kerrigan/pkg/log"
)

// OpenClawAdapter communicates with OpenClaw Gateway
type OpenClawAdapter struct {
	endpoint string // ws://host:18789
	conn     *websocket.Conn
	mu       sync.Mutex
	running  bool
	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup
}

// OpenClawMessage represents a message to/from OpenClaw Gateway
type OpenClawMessage struct {
	Type    string          `json:"type"`
	Content json.RawMessage `json:"content,omitempty"`
}

// OpenClawTaskRequest represents a task request to OpenClaw
type OpenClawTaskRequest struct {
	TaskID   string `json:"task_id"`
	Prompt   string `json:"prompt"`
	Thinking string `json:"thinking,omitempty"` // "low", "medium", "high"
	Model    string `json:"model,omitempty"`
}

// OpenClawTaskResponse represents a task response from OpenClaw
type OpenClawTaskResponse struct {
	TaskID       string `json:"task_id"`
	Status       string `json:"status"` // "pending", "running", "completed", "failed"
	Result       string `json:"result,omitempty"`
	Error        string `json:"error,omitempty"`
	FinishReason string `json:"finish_reason,omitempty"`
}

// OpenClawStatusResponse represents a status query response
type OpenClawStatusResponse struct {
	TaskID   string `json:"task_id"`
	Status   string `json:"status"`
	Progress int    `json:"progress,omitempty"`
	Result   string `json:"result,omitempty"`
	Error    string `json:"error,omitempty"`
}

// NewOpenClawAdapter creates a new OpenClaw adapter
func NewOpenClawAdapter(endpoint string) *OpenClawAdapter {
	return &OpenClawAdapter{
		endpoint: endpoint,
	}
}

// Start starts the OpenClaw adapter
func (a *OpenClawAdapter) Start(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.running {
		return fmt.Errorf("OpenClaw adapter already running")
	}

	a.ctx, a.cancel = context.WithCancel(ctx)
	a.running = true

	// Try to connect to gateway
	if err := a.connect(); err != nil {
		log.Warn("OpenClaw gateway not available", "error", err)
		// Don't fail - adapter will work when gateway becomes available
	}

	// Start heartbeat worker
	a.wg.Add(1)
	go a.heartbeatWorker()

	log.Info("OpenClaw adapter started", "endpoint", a.endpoint)
	return nil
}

// connect establishes WebSocket connection to OpenClaw Gateway
func (a *OpenClawAdapter) connect() error {
	if a.conn != nil {
		a.conn.Close()
	}

	u := url.URL{Scheme: "ws", Host: a.endpoint, Path: "/"}
	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		return fmt.Errorf("failed to connect to OpenClaw gateway: %w", err)
	}

	a.conn = conn
	log.Info("Connected to OpenClaw Gateway", "endpoint", a.endpoint)
	return nil
}

// Stop stops the OpenClaw adapter
func (a *OpenClawAdapter) Stop() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.running {
		return nil
	}

	a.cancel()
	a.wg.Wait()

	if a.conn != nil {
		a.conn.Close()
		a.conn = nil
	}

	a.running = false
	log.Info("OpenClaw adapter stopped")
	return nil
}

// heartbeatWorker sends periodic heartbeats to keep connection alive
func (a *OpenClawAdapter) heartbeatWorker() {
	defer a.wg.Done()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-a.ctx.Done():
			return
		case <-ticker.C:
			a.sendHeartbeat()
		}
	}
}

// sendHeartbeat sends a heartbeat to the gateway
func (a *OpenClawAdapter) sendHeartbeat() {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.conn == nil {
		return
	}

	msg := OpenClawMessage{
		Type: "ping",
	}
	if err := a.conn.WriteJSON(msg); err != nil {
		log.Warn("Failed to send heartbeat to OpenClaw", "error", err)
		// Try to reconnect
		go a.connect()
	}
}

// ExecuteTask executes a task via OpenClaw Gateway
func (a *OpenClawAdapter) ExecuteTask(ctx context.Context, req *OpenClawTaskRequest) (*OpenClawTaskResponse, error) {
	a.mu.Lock()
	conn := a.conn
	a.mu.Unlock()

	if conn == nil {
		// Try to reconnect
		if err := a.connect(); err != nil {
			return nil, fmt.Errorf("OpenClaw gateway not connected: %w", err)
		}
		a.mu.Lock()
		conn = a.conn
		a.mu.Unlock()
	}

	// Build message
	msg := OpenClawMessage{
		Type: "task",
	}
	content, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal task request: %w", err)
	}
	msg.Content = content

	// Send task
	if err := conn.WriteJSON(msg); err != nil {
		return nil, fmt.Errorf("failed to send task: %w", err)
	}

	// Read response with timeout
	respCh := make(chan *OpenClawTaskResponse, 1)
	errCh := make(chan error, 1)

	go func() {
		var resp OpenClawMessage
		if err := conn.ReadJSON(&resp); err != nil {
			errCh <- err
			return
		}

		var taskResp OpenClawTaskResponse
		if err := json.Unmarshal(resp.Content, &taskResp); err != nil {
			errCh <- err
			return
		}
		respCh <- &taskResp
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case err := <-errCh:
		return nil, fmt.Errorf("failed to read response: %w", err)
	case resp := <-respCh:
		return resp, nil
	}
}

// QueryStatus queries the status of a task
func (a *OpenClawAdapter) QueryStatus(ctx context.Context, taskID string) (*OpenClawStatusResponse, error) {
	a.mu.Lock()
	conn := a.conn
	a.mu.Unlock()

	if conn == nil {
		return nil, fmt.Errorf("OpenClaw gateway not connected")
	}

	// Build status request
	req := map[string]string{
		"task_id": taskID,
	}
	msg := OpenClawMessage{
		Type:    "status",
		Content: mustMarshal(req),
	}

	// Send status request
	if err := conn.WriteJSON(msg); err != nil {
		return nil, fmt.Errorf("failed to send status request: %w", err)
	}

	// Read response
	var resp OpenClawMessage
	if err := conn.ReadJSON(&resp); err != nil {
		return nil, fmt.Errorf("failed to read status response: %w", err)
	}

	var statusResp OpenClawStatusResponse
	if err := json.Unmarshal(resp.Content, &statusResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal status response: %w", err)
	}

	return &statusResp, nil
}

// IsConnected returns whether the adapter is connected to the gateway
func (a *OpenClawAdapter) IsConnected() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.conn != nil
}

// GetEndpoint returns the gateway endpoint
func (a *OpenClawAdapter) GetEndpoint() string {
	return a.endpoint
}

// mustMarshal marshals a value or panics
func mustMarshal(v interface{}) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return data
}
