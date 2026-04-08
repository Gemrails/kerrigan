package models

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Model represents a machine learning model
type Model struct {
	ID            string                 `json:"id"`             // Model ID
	Name          string                 `json:"name"`           // Model name
	Vendor        string                 `json:"vendor"`         // Model vendor
	Version       string                 `json:"version"`        // Model version
	Type          string                 `json:"type"`           // Model type (llama, chatglm, qwen, etc.)
	Architecture  string                 `json:"architecture"`   // Architecture (llama, gpt, etc.)
	Parameters    int                    `json:"parameters"`     // Parameters in billions
	Quantization  string                 `json:"quantization"`   // Quantization (Q4_K_M, INT4, etc.)
	ContextLength int                    `json:"context_length"` // Context length
	MaxTokens     int                    `json:"max_tokens"`     // Max tokens
	GPUMemoryMB   int                    `json:"gpu_memory_mb"`  // Required GPU memory in MB
	FilePath      string                 `json:"file_path"`      // Model file path
	FileSizeMB    int                    `json:"file_size_mb"`   // File size in MB
	SHA256        string                 `json:"sha256"`         // File checksum
	License       string                 `json:"license"`        // License
	Category      string                 `json:"category"`       // Category (chat, code, embedding, etc.)
	Tags          []string               `json:"tags"`           // Tags
	CreatedAt     time.Time              `json:"created_at"`     // Created at
	UpdatedAt     time.Time              `json:"updated_at"`     // Updated at
	Downloads     int                    `json:"downloads"`      // Download count
	Rating        float64                `json:"rating"`         // Rating
	Metadata      map[string]interface{} `json:"metadata"`       // Additional metadata
}

// LoRAAdapter represents a LoRA adapter
type LoRAAdapter struct {
	ID          string                 `json:"id"`            // Adapter ID
	Name        string                 `json:"name"`          // Adapter name
	BaseModelID string                 `json:"base_model_id"` // Base model ID
	Version     string                 `json:"version"`       // Version
	FilePath    string                 `json:"file_path"`     // File path
	FileSizeMB  int                    `json:"file_size_mb"`  // File size
	Rank        int                    `json:"rank"`          // LoRA rank
	Alpha       float64                `json:"alpha"`         // Alpha
	License     string                 `json:"license"`       // License
	Author      string                 `json:"author"`        // Author
	Tags        []string               `json:"tags"`          // Tags
	CreatedAt   time.Time              `json:"created_at"`    // Created at
	Downloads   int                    `json:"downloads"`     // Download count
	Metadata    map[string]interface{} `json:"metadata"`      // Additional metadata
}

// ModelMetadata holds model metadata
type ModelMetadata struct {
	Description  string              `json:"description"`  // Description
	README       string              `json:"readme"`       // README content
	Examples     []Example           `json:"examples"`     // Usage examples
	Requirements []Requirement       `json:"requirements"` // Requirements
	Performance  *PerformanceMetrics `json:"performance"`  // Performance metrics
	Limitations  []string            `json:"limitations"`  // Limitations
}

// Example represents a usage example
type Example struct {
	Prompt   string `json:"prompt"`   // Input prompt
	Output   string `json:"output"`   // Expected output
	Language string `json:"language"` // Language
	Comment  string `json:"comment"`  // Comment
}

// Requirement represents a requirement
type Requirement struct {
	Type    string `json:"type"`    // Requirement type
	Name    string `json:"name"`    // Requirement name
	Version string `json:"version"` // Version
}

// PerformanceMetrics holds performance metrics
type PerformanceMetrics struct {
	ThroughputTokensPerSec float64 `json:"throughput_tokens_per_sec"` // Throughput
	LatencyMs              float64 `json:"latency_ms"`                // Latency
	FirstTokenMs           float64 `json:"first_token_ms"`            // First token latency
	MemoryMB               int     `json:"memory_mb"`                 // Memory usage
}

// Registry is the model registry
type Registry struct {
	config   RegistryConfig
	mu       sync.RWMutex
	models   map[string]*Model
	adapters map[string]*LoRAAdapter
	metadata map[string]*ModelMetadata
}

// RegistryConfig holds registry configuration
type RegistryConfig struct {
	CachePath          string   `json:"cache_path"`           // Cache path
	PreloadModels      []string `json:"preload_models"`       // Preload models
	EnableAutoDownload bool     `json:"enable_auto_download"` // Auto download
	MaxCacheSizeMB     int      `json:"max_cache_size_mb"`    // Max cache size
}

// NewRegistry creates a new model registry
func NewRegistry(cachePath string) *Registry {
	if cachePath == "" {
		cachePath = "/var/lib/kerrigan/models"
	}

	return &Registry{
		config: RegistryConfig{
			CachePath:          cachePath,
			PreloadModels:      []string{},
			EnableAutoDownload: false,
			MaxCacheSizeMB:     100 * 1024, // 100GB
		},
		models:   make(map[string]*Model),
		adapters: make(map[string]*LoRAAdapter),
		metadata: make(map[string]*ModelMetadata),
	}
}

// LoadModels loads available models
func (r *Registry) LoadModels() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Load predefined models
	r.loadPredefinedModels()

	// Load adapters
	r.loadPredefinedAdapters()

	// Try to load from disk
	if err := r.loadFromDisk(); err != nil {
		return fmt.Errorf("failed to load models from disk: %w", err)
	}

	return nil
}

// loadPredefinedModels loads predefined models
func (r *Registry) loadPredefinedModels() {
	models := []*Model{
		{
			ID:            "llama2-7b",
			Name:          "Llama 2 7B",
			Vendor:        "Meta",
			Version:       "7b",
			Type:          "llama",
			Architecture:  "llama",
			Parameters:    7,
			Quantization:  "Q4_K_M",
			ContextLength: 4096,
			MaxTokens:     2048,
			GPUMemoryMB:   4608,
			License:       "open source",
			Category:      "chat",
			Tags:          []string{"llama", "chat", "7b", "meta"},
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
			Downloads:     100000,
			Rating:        4.5,
			Metadata: map[string]interface{}{
				"supports_streaming": true,
				"supports_tools":     false,
			},
		},
		{
			ID:            "llama2-13b",
			Name:          "Llama 2 13B",
			Vendor:        "Meta",
			Version:       "13b",
			Type:          "llama",
			Architecture:  "llama",
			Parameters:    13,
			Quantization:  "Q4_K_M",
			ContextLength: 4096,
			MaxTokens:     2048,
			GPUMemoryMB:   8192,
			License:       "open source",
			Category:      "chat",
			Tags:          []string{"llama", "chat", "13b", "meta"},
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
			Downloads:     80000,
			Rating:        4.6,
			Metadata: map[string]interface{}{
				"supports_streaming": true,
				"supports_tools":     false,
			},
		},
		{
			ID:            "llama3-8b",
			Name:          "Llama 3 8B",
			Vendor:        "Meta",
			Version:       "8b",
			Type:          "llama",
			Architecture:  "llama",
			Parameters:    8,
			Quantization:  "Q4_K_M",
			ContextLength: 8192,
			MaxTokens:     4096,
			GPUMemoryMB:   5120,
			License:       "open source",
			Category:      "chat",
			Tags:          []string{"llama", "chat", "8b", "meta", "llama3"},
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
			Downloads:     50000,
			Rating:        4.7,
			Metadata: map[string]interface{}{
				"supports_streaming": true,
				"supports_tools":     true,
			},
		},
		{
			ID:            "chatglm3-6b",
			Name:          "ChatGLM3-6B",
			Vendor:        "Zhipu AI",
			Version:       "6b",
			Type:          "chatglm",
			Architecture:  "chatglm",
			Parameters:    6,
			Quantization:  "INT4",
			ContextLength: 8192,
			MaxTokens:     4096,
			GPUMemoryMB:   3900,
			License:       "open source",
			Category:      "chat",
			Tags:          []string{"chatglm", "chat", "6b", "zhipu"},
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
			Downloads:     70000,
			Rating:        4.4,
			Metadata: map[string]interface{}{
				"supports_streaming": true,
				"supports_tools":     false,
			},
		},
		{
			ID:            "qwen-7b",
			Name:          "Qwen 7B",
			Vendor:        "Alibaba",
			Version:       "7b",
			Type:          "qwen",
			Architecture:  "qwen",
			Parameters:    7,
			Quantization:  "Q4_K_M",
			ContextLength: 8192,
			MaxTokens:     4096,
			GPUMemoryMB:   4608,
			License:       "open source",
			Category:      "chat",
			Tags:          []string{"qwen", "chat", "7b", "alibaba"},
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
			Downloads:     60000,
			Rating:        4.5,
			Metadata: map[string]interface{}{
				"supports_streaming": true,
				"supports_tools":     true,
			},
		},
		{
			ID:            "qwen-14b",
			Name:          "Qwen 14B",
			Vendor:        "Alibaba",
			Version:       "14b",
			Type:          "qwen",
			Architecture:  "qwen",
			Parameters:    14,
			Quantization:  "Q4_K_M",
			ContextLength: 8192,
			MaxTokens:     4096,
			GPUMemoryMB:   8600,
			License:       "open source",
			Category:      "chat",
			Tags:          []string{"qwen", "chat", "14b", "alibaba"},
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
			Downloads:     40000,
			Rating:        4.6,
			Metadata: map[string]interface{}{
				"supports_streaming": true,
				"supports_tools":     true,
			},
		},
		{
			ID:            "baichuan2-13b",
			Name:          "Baichuan2 13B",
			Vendor:        "ByteDance",
			Version:       "13b",
			Type:          "baichuan",
			Architecture:  "baichuan",
			Parameters:    13,
			Quantization:  "Q4_K_M",
			ContextLength: 4096,
			MaxTokens:     2048,
			GPUMemoryMB:   7800,
			License:       "open source",
			Category:      "chat",
			Tags:          []string{"baichuan", "chat", "13b", "bytedance"},
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
			Downloads:     30000,
			Rating:        4.3,
			Metadata: map[string]interface{}{
				"supports_streaming": true,
				"supports_tools":     false,
			},
		},
		{
			ID:            "yi-6b",
			Name:          "Yi 6B",
			Vendor:        "01.AI",
			Version:       "6b",
			Type:          "yi",
			Architecture:  "yi",
			Parameters:    6,
			Quantization:  "Q4_K_M",
			ContextLength: 4096,
			MaxTokens:     2048,
			GPUMemoryMB:   3900,
			License:       "open source",
			Category:      "chat",
			Tags:          []string{"yi", "chat", "6b", "01.ai"},
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
			Downloads:     25000,
			Rating:        4.4,
			Metadata: map[string]interface{}{
				"supports_streaming": true,
				"supports_tools":     false,
			},
		},
		{
			ID:            "stablelm-3b",
			Name:          "StableLM 3B",
			Vendor:        "Stability AI",
			Version:       "3b",
			Type:          "stablelm",
			Architecture:  "stablelm",
			Parameters:    3,
			Quantization:  "Q4_K_M",
			ContextLength: 4096,
			MaxTokens:     2048,
			GPUMemoryMB:   2400,
			License:       "open source",
			Category:      "chat",
			Tags:          []string{"stablelm", "chat", "3b", "stability"},
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
			Downloads:     20000,
			Rating:        4.2,
			Metadata: map[string]interface{}{
				"supports_streaming": true,
				"supports_tools":     false,
			},
		},
		{
			ID:            "deepseek-llm-7b",
			Name:          "DeepSeek LLM 7B",
			Vendor:        "DeepSeek",
			Version:       "7b",
			Type:          "deepseek",
			Architecture:  "deepseek",
			Parameters:    7,
			Quantization:  "Q4_K_M",
			ContextLength: 4096,
			MaxTokens:     2048,
			GPUMemoryMB:   4608,
			License:       "open source",
			Category:      "chat",
			Tags:          []string{"deepseek", "chat", "7b"},
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
			Downloads:     15000,
			Rating:        4.3,
			Metadata: map[string]interface{}{
				"supports_streaming": true,
				"supports_tools":     false,
			},
		},
		// Embedding models
		{
			ID:            "bge-large-zh-v1.5",
			Name:          "BGE Large ZH v1.5",
			Vendor:        "Beijing Academy of AI",
			Version:       "v1.5",
			Type:          "embedding",
			Architecture:  "bert",
			Parameters:    0.3,
			Quantization:  "FP16",
			ContextLength: 512,
			MaxTokens:     512,
			GPUMemoryMB:   800,
			License:       "open source",
			Category:      "embedding",
			Tags:          []string{"embedding", "chinese", "bge"},
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
			Downloads:     50000,
			Rating:        4.8,
			Metadata: map[string]interface{}{
				"dimension": 1024,
			},
		},
		{
			ID:            "bge-base-zh-v1.5",
			Name:          "BGE Base ZH v1.5",
			Vendor:        "Beijing Academy of AI",
			Version:       "v1.5",
			Type:          "embedding",
			Architecture:  "bert",
			Parameters:    0.1,
			Quantization:  "FP16",
			ContextLength: 512,
			MaxTokens:     512,
			GPUMemoryMB:   400,
			License:       "open source",
			Category:      "embedding",
			Tags:          []string{"embedding", "chinese", "bge"},
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
			Downloads:     40000,
			Rating:        4.7,
			Metadata: map[string]interface{}{
				"dimension": 768,
			},
		},
		// Image generation models
		{
			ID:            "stable-diffusion-xl-base-1.0",
			Name:          "Stable Diffusion XL Base 1.0",
			Vendor:        "Stability AI",
			Version:       "1.0",
			Type:          "image",
			Architecture:  "sd-xl",
			Parameters:    3.5,
			Quantization:  "FP16",
			ContextLength: 77,
			MaxTokens:     77,
			GPUMemoryMB:   8000,
			License:       "open source",
			Category:      "image-generation",
			Tags:          []string{"image", "sd", "stable diffusion"},
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
			Downloads:     80000,
			Rating:        4.6,
			Metadata: map[string]interface{}{
				"resolution": "1024x1024",
			},
		},
	}

	for _, model := range models {
		r.models[model.ID] = model
	}
}

// loadPredefinedAdapters loads predefined LoRA adapters
func (r *Registry) loadPredefinedAdapters() {
	adapters := []*LoRAAdapter{
		{
			ID:          "lora-llama2-chat-template",
			Name:        "Llama 2 Chat Template",
			BaseModelID: "llama2-7b",
			Version:     "v1",
			Rank:        64,
			Alpha:       32.0,
			License:     "open source",
			Author:      "Community",
			Tags:        []string{"llama", "chat", "template"},
			CreatedAt:   time.Now(),
			Downloads:   5000,
		},
		{
			ID:          "lora-code-assistant",
			Name:        "Code Assistant",
			BaseModelID: "codeLlama-7b",
			Version:     "v1",
			Rank:        32,
			Alpha:       16.0,
			License:     "open source",
			Author:      "Community",
			Tags:        []string{"code", "assistant"},
			CreatedAt:   time.Now(),
			Downloads:   3000,
		},
		{
			ID:          "lora-math-tutor",
			Name:        "Math Tutor",
			BaseModelID: "llama2-13b",
			Version:     "v1",
			Rank:        128,
			Alpha:       64.0,
			License:     "open source",
			Author:      "Community",
			Tags:        []string{"math", "tutor", "education"},
			CreatedAt:   time.Now(),
			Downloads:   2000,
		},
	}

	for _, adapter := range adapters {
		r.adapters[adapter.ID] = adapter
	}
}

// loadFromDisk loads models from disk
func (r *Registry) loadFromDisk() error {
	// Try to load models.json from cache path
	modelsFile := filepath.Join(r.config.CachePath, "models.json")
	if _, err := os.Stat(modelsFile); err == nil {
		data, err := os.ReadFile(modelsFile)
		if err != nil {
			return fmt.Errorf("failed to read models file: %w", err)
		}

		var loadedModels []Model
		if err := json.Unmarshal(data, &loadedModels); err != nil {
			return fmt.Errorf("failed to parse models: %w", err)
		}

		for _, model := range loadedModels {
			r.models[model.ID] = &model
		}
	}

	return nil
}

// SaveToDisk saves models to disk
func (r *Registry) SaveToDisk() error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Create cache directory if not exists
	if err := os.MkdirAll(r.config.CachePath, 0755); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}

	// Save models
	models := make([]Model, 0, len(r.models))
	for _, model := range r.models {
		models = append(models, *model)
	}

	data, err := json.MarshalIndent(models, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal models: %w", err)
	}

	modelsFile := filepath.Join(r.config.CachePath, "models.json")
	if err := os.WriteFile(modelsFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write models file: %w", err)
	}

	return nil
}

// GetModel gets a model by ID
func (r *Registry) GetModel(modelID string) (*Model, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	model, exists := r.models[modelID]
	if !exists {
		return nil, fmt.Errorf("model not found: %s", modelID)
	}

	return model, nil
}

// ListModels lists all models
func (r *Registry) ListModels() []*Model {
	r.mu.RLock()
	defer r.mu.RUnlock()

	models := make([]*Model, 0, len(r.models))
	for _, model := range r.models {
		models = append(models, model)
	}

	return models
}

// ListModelsByType lists models by type
func (r *Registry) ListModelsByType(modelType string) []*Model {
	r.mu.RLock()
	defer r.mu.RUnlock()

	models := make([]*Model, 0)
	for _, model := range r.models {
		if model.Type == modelType {
			models = append(models, model)
		}
	}

	return models
}

// ListModelsByCategory lists models by category
func (r *Registry) ListModelsByCategory(category string) []*Model {
	r.mu.RLock()
	defer r.mu.RUnlock()

	models := make([]*Model, 0)
	for _, model := range r.models {
		if model.Category == category {
			models = append(models, model)
		}
	}

	return models
}

// SearchModels searches models by query
func (r *Registry) SearchModels(query string) []*Model {
	r.mu.RLock()
	defer r.mu.RUnlock()

	query = toLower(query)
	models := make([]*Model, 0)

	for _, model := range r.models {
		if contains(model.Name, query) || contains(model.Vendor, query) || contains(model.Type, query) {
			models = append(models, model)
		} else {
			for _, tag := range model.Tags {
				if contains(tag, query) {
					models = append(models, model)
					break
				}
			}
		}
	}

	return models
}

// GetAdapter gets a LoRA adapter by ID
func (r *Registry) GetAdapter(adapterID string) (*LoRAAdapter, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	adapter, exists := r.adapters[adapterID]
	if !exists {
		return nil, fmt.Errorf("adapter not found: %s", adapterID)
	}

	return adapter, nil
}

// ListAdapters lists all adapters
func (r *Registry) ListAdapters() []*LoRAAdapter {
	r.mu.RLock()
	defer r.mu.RUnlock()

	adapters := make([]*LoRAAdapter, 0, len(r.adapters))
	for _, adapter := range r.adapters {
		adapters = append(adapters, adapter)
	}

	return adapters
}

// ListAdaptersByBaseModel lists adapters for a base model
func (r *Registry) ListAdaptersByBaseModel(baseModelID string) []*LoRAAdapter {
	r.mu.RLock()
	defer r.mu.RUnlock()

	adapters := make([]*LoRAAdapter, 0)
	for _, adapter := range r.adapters {
		if adapter.BaseModelID == baseModelID {
			adapters = append(adapters, adapter)
		}
	}

	return adapters
}

// AddModel adds a new model
func (r *Registry) AddModel(model *Model) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.models[model.ID]; exists {
		return fmt.Errorf("model already exists: %s", model.ID)
	}

	r.models[model.ID] = model
	return nil
}

// AddAdapter adds a new LoRA adapter
func (r *Registry) AddAdapter(adapter *LoRAAdapter) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.adapters[adapter.ID]; exists {
		return fmt.Errorf("adapter already exists: %s", adapter.ID)
	}

	r.adapters[adapter.ID] = adapter
	return nil
}

// DeleteModel deletes a model
func (r *Registry) DeleteModel(modelID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.models[modelID]; !exists {
		return fmt.Errorf("model not found: %s", modelID)
	}

	delete(r.models, modelID)
	return nil
}

// GetModelMetadata gets model metadata
func (r *Registry) GetModelMetadata(modelID string) (*ModelMetadata, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	metadata, exists := r.metadata[modelID]
	if !exists {
		// Try to load from disk
		metadataFile := filepath.Join(r.config.CachePath, modelID, "metadata.json")
		if _, err := os.Stat(metadataFile); err == nil {
			data, err := os.ReadFile(metadataFile)
			if err != nil {
				return nil, fmt.Errorf("failed to read metadata: %w", err)
			}

			metadata = &ModelMetadata{}
			if err := json.Unmarshal(data, metadata); err != nil {
				return nil, fmt.Errorf("failed to parse metadata: %w", err)
			}
		} else {
			// Return default metadata
			metadata = &ModelMetadata{
				Description:  "No description available",
				Examples:     []Example{},
				Requirements: []Requirement{},
			}
		}
	}

	return metadata, nil
}

// SetModelMetadata sets model metadata
func (r *Registry) SetModelMetadata(modelID string, metadata *ModelMetadata) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.metadata[modelID] = metadata
	return nil
}

// GetCachePath returns the cache path
func (r *Registry) GetCachePath() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.config.CachePath
}

// GetModelCount returns the number of models
func (r *Registry) GetModelCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.models)
}

// GetAdapterCount returns the number of adapters
func (r *Registry) GetAdapterCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.adapters)
}

// Helper functions
func toLower(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 32
		}
		result[i] = c
	}
	return string(result)
}

func contains(s, substr string) bool {
	s = toLower(s)
	substr = toLower(substr)
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsAny(s, substr))
}

func containsAny(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
