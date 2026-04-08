package inference

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// Config holds inference engine configuration
type Config struct {
	Port         int           `json:"port"`
	ModelCache   string        `json:"model_cache"`
	MaxBatchSize int           `json:"max_batch_size"`
	Timeout      time.Duration `json:"timeout"`
}

// Request represents an inference request
type Request struct {
	ModelID           string                 `json:"model_id"`
	Prompt            string                 `json:"prompt"`
	MaxTokens         int                    `json:"max_tokens"`
	Temperature       float64                `json:"temperature"`
	TopP              float64                `json:"top_p"`
	TopK              int                    `json:"top_k"`
	RepetitionPenalty float64                `json:"repetition_penalty"`
	Stream            bool                   `json:"stream"`
	GPUID             string                 `json:"gpu_id"`
	Stop              []string               `json:"stop"`
	Metadata          map[string]interface{} `json:"metadata"`
}

// Response represents an inference response
type Response struct {
	Text         string                 `json:"text"`
	TokenCount   int                    `json:"token_count"`
	FinishReason string                 `json:"finish_reason"`
	Latency      time.Duration          `json:"latency"`
	ModelID      string                 `json:"model_id"`
	Metadata     map[string]interface{} `json:"metadata"`
}

// Session represents an inference session
type Session struct {
	ID        string
	ModelID   string
	GPUID     string
	CreatedAt time.Time
	LastUsed  time.Time
	Context   []int
	mu        sync.RWMutex
}

// Engine is the inference engine
type Engine struct {
	config   Config
	mu       sync.RWMutex
	sessions map[string]*Session
	models   map[string]*ModelInfo
	server   *Server
	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup
}

// ModelInfo holds loaded model information
type ModelInfo struct {
	ID           string                 `json:"id"`
	Name         string                 `json:"name"`
	Type         string                 `json:"type"`
	Version      string                 `json:"version"`
	Parameters   int                    `json:"parameters"` // in billions
	Quantization string                 `json:"quantization"`
	ContextLen   int                    `json:"context_len"`
	MaxTokens    int                    `json:"max_tokens"`
	GPUMemory    int                    `json:"gpu_memory"` // in MB
	Loaded       bool                   `json:"loaded"`
	LoadedAt     time.Time              `json:"loaded_at"`
	FilePath     string                 `json:"file_path"`
	Metadata     map[string]interface{} `json:"metadata"`
}

// Server represents the HTTP server for inference
type Server struct {
	Port   int
	Engine *Engine
}

// NewEngine creates a new inference engine
func NewEngine(config Config) *Engine {
	if config.MaxBatchSize == 0 {
		config.MaxBatchSize = 8
	}
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}
	if config.Port == 0 {
		config.Port = 8080
	}

	ctx, cancel := context.WithCancel(context.Background())
	return &Engine{
		config:   config,
		sessions: make(map[string]*Session),
		models:   make(map[string]*ModelInfo),
		ctx:      ctx,
		cancel:   cancel,
	}
}

// Start starts the inference engine
func (e *Engine) Start(ctx context.Context) error {
	// Load available models
	e.loadModels()

	// Start HTTP server
	e.server = &Server{
		Port:   e.config.Port,
		Engine: e,
	}

	e.wg.Add(1)
	go e.server.Run(e.ctx)

	return nil
}

// loadModels loads available models from cache
func (e *Engine) loadModels() {
	// Preloaded models
	models := []*ModelInfo{
		{
			ID:           "llama2-7b",
			Name:         "Llama 2 7B",
			Type:         "llama",
			Version:      "7b",
			Parameters:   7,
			Quantization: "Q4_K_M",
			ContextLen:   4096,
			MaxTokens:    2048,
			GPUMemory:    4608,
			Loaded:       false,
			FilePath:     "/models/llama-2-7b-chat.Q4_K_M.gguf",
			Metadata: map[string]interface{}{
				"vendor":  "Meta",
				"license": "open source",
				"context": "chat",
			},
		},
		{
			ID:           "llama2-13b",
			Name:         "Llama 2 13B",
			Type:         "llama",
			Version:      "13b",
			Parameters:   13,
			Quantization: "Q4_K_M",
			ContextLen:   4096,
			MaxTokens:    2048,
			GPUMemory:    8192,
			Loaded:       false,
			FilePath:     "/models/llama-2-13b-chat.Q4_K_M.gguf",
			Metadata: map[string]interface{}{
				"vendor":  "Meta",
				"license": "open source",
				"context": "chat",
			},
		},
		{
			ID:           "chatglm3-6b",
			Name:         "ChatGLM3-6B",
			Type:         "chatglm",
			Version:      "6b",
			Parameters:   6,
			Quantization: "INT4",
			ContextLen:   8192,
			MaxTokens:    4096,
			GPUMemory:    3900,
			Loaded:       false,
			FilePath:     "/models/chatglm3-6b-int4.bin",
			Metadata: map[string]interface{}{
				"vendor":  "Zhipu AI",
				"license": "open source",
				"context": "chat",
			},
		},
		{
			ID:           "qwen-7b",
			Name:         "Qwen 7B",
			Type:         "qwen",
			Version:      "7b",
			Parameters:   7,
			Quantization: "Q4_K_M",
			ContextLen:   8192,
			MaxTokens:    4096,
			GPUMemory:    4608,
			Loaded:       false,
			FilePath:     "/models/qwen-7b-chat.Q4_K_M.gguf",
			Metadata: map[string]interface{}{
				"vendor":  "Alibaba",
				"license": "open source",
				"context": "chat",
			},
		},
		{
			ID:           "qwen-14b",
			Name:         "Qwen 14B",
			Type:         "qwen",
			Version:      "14b",
			Parameters:   14,
			Quantization: "Q4_K_M",
			ContextLen:   8192,
			MaxTokens:    4096,
			GPUMemory:    8600,
			Loaded:       false,
			FilePath:     "/models/qwen-14b-chat.Q4_K_M.gguf",
			Metadata: map[string]interface{}{
				"vendor":  "Alibaba",
				"license": "open source",
				"context": "chat",
			},
		},
		{
			ID:           "baichuan2-13b",
			Name:         "Baichuan2 13B",
			Type:         "baichuan",
			Version:      "13b",
			Parameters:   13,
			Quantization: "Q4_K_M",
			ContextLen:   4096,
			MaxTokens:    2048,
			GPUMemory:    7800,
			Loaded:       false,
			FilePath:     "/models/baichuan2-13b-chat.Q4_K_M.gguf",
			Metadata: map[string]interface{}{
				"vendor":  "ByteDance",
				"license": "open source",
				"context": "chat",
			},
		},
		{
			ID:           "Yi-6b",
			Name:         "Yi 6B",
			Type:         "yi",
			Version:      "6b",
			Parameters:   6,
			Quantization: "Q4_K_M",
			ContextLen:   4096,
			MaxTokens:    2048,
			GPUMemory:    3900,
			Loaded:       false,
			FilePath:     "/models/yi-6b-chat.Q4_K_M.gguf",
			Metadata: map[string]interface{}{
				"vendor":  "01.AI",
				"license": "open source",
				"context": "chat",
			},
		},
		{
			ID:           "stablelm-3b",
			Name:         "StableLM 3B",
			Type:         "stablelm",
			Version:      "3b",
			Parameters:   3,
			Quantization: "Q4_K_M",
			ContextLen:   4096,
			MaxTokens:    2048,
			GPUMemory:    2400,
			Loaded:       false,
			FilePath:     "/models/stablelm-3b-chat.Q4_K_M.gguf",
			Metadata: map[string]interface{}{
				"vendor":  "Stability AI",
				"license": "open source",
				"context": "chat",
			},
		},
	}

	for _, model := range models {
		e.models[model.ID] = model
	}
}

// Inference performs inference
func (e *Engine) Inference(ctx context.Context, req *Request) (*Response, error) {
	start := time.Now()

	// Validate request
	if req.ModelID == "" {
		return nil, fmt.Errorf("model_id is required")
	}
	if req.Prompt == "" {
		return nil, fmt.Errorf("prompt is required")
	}

	// Set defaults
	if req.MaxTokens == 0 {
		req.MaxTokens = 512
	}
	if req.Temperature == 0 {
		req.Temperature = 0.7
	}
	if req.TopP == 0 {
		req.TopP = 0.9
	}
	if req.TopK == 0 {
		req.TopK = 40
	}
	if req.RepetitionPenalty == 0 {
		req.RepetitionPenalty = 1.1
	}

	// Get model
	e.mu.RLock()
	model, exists := e.models[req.ModelID]
	e.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("model not found: %s", req.ModelID)
	}

	// Check if model is loaded, if not, load it
	if !model.Loaded {
		if err := e.loadModel(req.ModelID, req.GPUID); err != nil {
			return nil, fmt.Errorf("failed to load model: %w", err)
		}
	}

	// Perform inference (placeholder for actual inference)
	text, tokens, err := e.performInference(ctx, req, model)
	if err != nil {
		return nil, fmt.Errorf("inference failed: %w", err)
	}

	finishReason := "stop"
	if tokens >= req.MaxTokens {
		finishReason = "length"
	}

	return &Response{
		Text:         text,
		TokenCount:   tokens,
		FinishReason: finishReason,
		Latency:      time.Since(start),
		ModelID:      req.ModelID,
		Metadata: map[string]interface{}{
			"temperature": req.Temperature,
			"top_p":       req.TopP,
			"top_k":       req.TopK,
		},
	}, nil
}

// loadModel loads a model into GPU memory
func (e *Engine) loadModel(modelID, gpuID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	model, exists := e.models[modelID]
	if !exists {
		return fmt.Errorf("model not found: %s", modelID)
	}

	if model.Loaded {
		return nil
	}

	// Placeholder for actual model loading
	// In production, use llama.cpp, vLLM, or other inference frameworks
	model.Loaded = true
	model.LoadedAt = time.Now()

	return nil
}

// performInference performs the actual inference
func (e *Engine) performInference(ctx context.Context, req *Request, model *ModelInfo) (string, int, error) {
	// Placeholder for actual inference implementation
	// In production, integrate with llama.cpp, vLLM, or other frameworks

	// Simulate inference
	time.Sleep(100 * time.Millisecond)

	// Return simulated response
	response := fmt.Sprintf("[Generated response for: %s]", req.Prompt[:min(50, len(req.Prompt))])
	return response, len(response) / 4, nil
}

// BatchInference performs batch inference
func (e *Engine) BatchInference(ctx context.Context, requests []*Request) ([]*Response, error) {
	// Group requests by model
	batches := make(map[string][]*Request)
	for _, req := range requests {
		batches[req.ModelID] = append(batches[req.ModelID], req)
	}

	results := make([]*Response, 0, len(requests))
	var mu sync.Mutex

	var wg sync.WaitGroup
	for modelID, batch := range batches {
		wg.Add(1)
		go func(modelID string, batch []*Request) {
			defer wg.Done()

			for _, req := range batch {
				resp, err := e.Inference(ctx, req)
				mu.Lock()
				if err != nil {
					results = append(results, &Response{
						Text:  "",
						Error: err.Error(),
					})
				} else {
					results = append(results, resp)
				}
				mu.Unlock()
			}
		}(modelID, batch)
	}

	wg.Wait()
	return results, nil
}

// CreateSession creates a new inference session
func (e *Engine) CreateSession(ctx context.Context, modelID, gpuID string) (*Session, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	model, exists := e.models[modelID]
	if !exists {
		return nil, fmt.Errorf("model not found: %s", modelID)
	}

	session := &Session{
		ID:        generateSessionID(),
		ModelID:   modelID,
		GPUID:     gpuID,
		CreatedAt: time.Now(),
		LastUsed:  time.Now(),
		Context:   []int{},
	}

	e.sessions[session.ID] = session
	return session, nil
}

// GetSession gets a session by ID
func (e *Engine) GetSession(sessionID string) (*Session, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	session, exists := e.sessions[sessionID]
	if !exists {
		return nil, fmt.Errorf("session not found: %sessionID")
	}

	return session, nil
}

// DeleteSession deletes a session
func (e *Engine) DeleteSession(sessionID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.sessions[sessionID]; !exists {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	delete(e.sessions, sessionID)
	return nil
}

// UpdateSessionContext updates session context
func (e *Engine) UpdateSessionContext(sessionID string, tokens []int) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	session, exists := e.sessions[sessionID]
	if !exists {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	session.mu.Lock()
	session.Context = append(session.Context, tokens...)
	// Limit context length
	if len(session.Context) > 4096 {
		session.Context = session.Context[len(session.Context)-4096:]
	}
	session.mu.Unlock()
	session.LastUsed = time.Now()

	return nil
}

// ListModels lists available models
func (e *Engine) ListModels() []*ModelInfo {
	e.mu.RLock()
	defer e.mu.RUnlock()

	models := make([]*ModelInfo, 0, len(e.models))
	for _, model := range e.models {
		models = append(models, model)
	}

	return models
}

// GetModel gets model info
func (e *Engine) GetModel(modelID string) (*ModelInfo, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	model, exists := e.models[modelID]
	if !exists {
		return nil, fmt.Errorf("model not found: %s", modelID)
	}

	return model, nil
}

// Stop stops the inference engine
func (e *Engine) Stop() {
	e.cancel()
	e.wg.Wait()
}

// Run starts the server
func (s *Server) Run(ctx context.Context) {
	// Placeholder for HTTP server
	// In production, implement REST API endpoints
}

// Helper functions
func generateSessionID() string {
	return fmt.Sprintf("sess-%d-%d", time.Now().UnixNano(), time.Now().Nanosecond())
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Response with error field for batch results
type ResponseWithError struct {
	Response
	Error string `json:"error,omitempty"`
}
