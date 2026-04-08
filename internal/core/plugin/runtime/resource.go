package runtime

import (
	"crypto/rand"
	"sync"
	"time"

	"github.com/kerrigan/kerrigan/internal/core/plugin/types"
	"github.com/sirupsen/logrus"
)

// ResourceManager manages resource allocation for plugins
type ResourceManager struct {
	mu            sync.RWMutex
	logger        *logrus.Logger
	allocations   map[string]*ResourceAllocation
	resourcePools map[types.ResourceType]*ResourcePool
	limits        map[types.ResourceType]types.ResourceLimits
}

// ResourceAllocation represents allocated resources for a plugin
type ResourceAllocation struct {
	ID           string
	pluginID     string
	resourceType types.ResourceType
	limits       types.ResourceLimits
	allocated    bool
	mu           sync.Mutex
}

// ResourcePool represents a pool of resources for a type
type ResourcePool struct {
	mu           sync.RWMutex
	resourceType types.ResourceType
	totalCap     types.ResourceCapacity
	availableCap types.ResourceCapacity
	usedCap      types.ResourceCapacity
	allocations  int64
}

// NewResourceManager creates a new resource manager
func NewResourceManager(logger *logrus.Logger) *ResourceManager {
	rm := &ResourceManager{
		logger:        logger,
		allocations:   make(map[string]*ResourceAllocation),
		resourcePools: make(map[types.ResourceType]*ResourcePool),
		limits:        make(map[types.ResourceType]types.ResourceLimits),
	}

	// Initialize resource pools
	rm.resourcePools[types.ResourceTypeGPU] = &ResourcePool{
		resourceType: types.ResourceTypeGPU,
		totalCap:     types.ResourceCapacity{CPU: 0, Memory: 0, Storage: 0, Bandwidth: 0},
		availableCap: types.ResourceCapacity{GPU: types.GPUInfo{Model: "Unknown", VRAM: 0, Count: 0, Compute: 0}},
		usedCap:      types.ResourceCapacity{},
	}

	rm.resourcePools[types.ResourceTypeStorage] = &ResourcePool{
		resourceType: types.ResourceTypeStorage,
		totalCap:     types.ResourceCapacity{Storage: 0},
		availableCap: types.ResourceCapacity{},
		usedCap:      types.ResourceCapacity{},
	}

	rm.resourcePools[types.ResourceTypeBandwidth] = &ResourcePool{
		resourceType: types.ResourceTypeBandwidth,
		totalCap:     types.ResourceCapacity{Bandwidth: 0},
		availableCap: types.ResourceCapacity{},
		usedCap:      types.ResourceCapacity{},
	}

	rm.resourcePools[types.ResourceTypeCPU] = &ResourcePool{
		resourceType: types.ResourceTypeCPU,
		totalCap:     types.ResourceCapacity{CPU: 0, Memory: 0},
		availableCap: types.ResourceCapacity{},
		usedCap:      types.ResourceCapacity{},
	}

	// Set default limits
	rm.limits[types.ResourceTypeGPU] = types.ResourceLimits{
		MaxCPU:        4000,                     // 4 cores
		MaxMemory:     8 * 1024 * 1024 * 1024,   // 8GB
		MaxStorage:    100 * 1024 * 1024 * 1024, // 100GB
		MaxBandwidth:  100 * 1024 * 1024,        // 100MB/s
		MaxConcurrent: 10,
		Timeout:       30 * time.Minute,
	}

	rm.limits[types.ResourceTypeStorage] = types.ResourceLimits{
		MaxCPU:        1000,                          // 1 core
		MaxMemory:     512 * 1024 * 1024,             // 512MB
		MaxStorage:    1 * 1024 * 1024 * 1024 * 1024, // 1TB
		MaxBandwidth:  50 * 1024 * 1024,              // 50MB/s
		MaxConcurrent: 5,
		Timeout:       60 * time.Minute,
	}

	rm.limits[types.ResourceTypeBandwidth] = types.ResourceLimits{
		MaxCPU:        500,               // 0.5 cores
		MaxMemory:     256 * 1024 * 1024, // 256MB
		MaxStorage:    0,
		MaxBandwidth:  1 * 1024 * 1024 * 1024, // 1GB/s
		MaxConcurrent: 50,
		Timeout:       10 * time.Minute,
	}

	rm.limits[types.ResourceTypeCPU] = types.ResourceLimits{
		MaxCPU:        2000,                    // 2 cores
		MaxMemory:     2 * 1024 * 1024 * 1024,  // 2GB
		MaxStorage:    10 * 1024 * 1024 * 1024, // 10GB
		MaxBandwidth:  10 * 1024 * 1024,        // 10MB/s
		MaxConcurrent: 20,
		Timeout:       30 * time.Minute,
	}

	return rm
}

// CanAllocate checks if resources can be allocated
func (rm *ResourceManager) CanAllocate(resourceType types.ResourceType, capabilities []types.Capability) bool {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	pool, exists := rm.resourcePools[resourceType]
	if !exists {
		return false
	}

	pool.mu.RLock()
	// Check if there's any capacity available
	available := pool.availableCap.CPU > 0 || pool.availableCap.Memory > 0 ||
		pool.availableCap.Storage > 0 || pool.availableCap.Bandwidth > 0 ||
		pool.availableCap.GPU.VRAM > 0
	pool.mu.RUnlock()

	// If no pool is set up yet (all zeros), allow allocation
	return available || pool.totalCap.CPU == 0
}

// Allocate allocates resources for a plugin
func (rm *ResourceManager) Allocate(resourceType types.ResourceType, capabilities []types.Capability) *ResourceAllocation {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	pool, exists := rm.resourcePools[resourceType]
	if !exists {
		return nil
	}

	limits := rm.limits[resourceType]

	pool.mu.Lock()
	defer pool.mu.Unlock()

	// Update pool usage
	pool.allocations++
	pool.usedCap.CPU += float64(limits.MaxCPU) / 1000
	pool.usedCap.Memory += limits.MaxMemory

	allocation := &ResourceAllocation{
		ID:           generateAllocationID(),
		resourceType: resourceType,
		limits:       limits,
		allocated:    true,
	}

	rm.allocations[allocation.ID] = allocation

	rm.logger.Debugf("Allocated resources for resource type %s: CPU=%d, Memory=%d",
		resourceType, limits.MaxCPU, limits.MaxMemory)

	return allocation
}

// Release releases allocated resources
func (rm *ResourceManager) Release(allocation *ResourceAllocation) {
	if allocation == nil || !allocation.allocated {
		return
	}

	rm.mu.Lock()
	defer rm.mu.Unlock()

	pool, exists := rm.resourcePools[allocation.resourceType]
	if !exists {
		return
	}

	pool.mu.Lock()
	defer pool.mu.Unlock()

	pool.allocations--
	pool.usedCap.CPU -= float64(allocation.limits.MaxCPU) / 1000
	pool.usedCap.Memory -= allocation.limits.MaxMemory

	delete(rm.allocations, allocation.ID)
	allocation.allocated = false

	rm.logger.Debugf("Released resources for allocation %s", allocation.ID)
}

// GetLimits returns resource limits for a resource type
func (rm *ResourceManager) GetLimits(resourceType types.ResourceType) types.ResourceLimits {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	return rm.limits[resourceType]
}

// SetLimits sets resource limits for a resource type
func (rm *ResourceManager) SetLimits(resourceType types.ResourceType, limits types.ResourceLimits) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	rm.limits[resourceType] = limits
}

// GetStats returns resource statistics
func (rm *ResourceManager) GetStats() ResourceStats {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	stats := ResourceStats{
		Pools: make(map[types.ResourceType]PoolStats),
	}

	for rt, pool := range rm.resourcePools {
		pool.mu.RLock()
		stats.Pools[rt] = PoolStats{
			ResourceType:     rt,
			TotalAllocations: pool.allocations,
			TotalCPU:         pool.totalCap.CPU,
			AvailableCPU:     pool.availableCap.CPU,
			UsedCPU:          pool.usedCap.CPU,
			TotalMemory:      pool.totalCap.Memory,
			AvailableMemory:  pool.availableCap.Memory,
			UsedMemory:       pool.usedCap.Memory,
		}
		pool.mu.RUnlock()
	}

	return stats
}

// ResourceStats contains resource statistics
type ResourceStats struct {
	Pools map[types.ResourceType]PoolStats
}

// PoolStats contains statistics for a resource pool
type PoolStats struct {
	ResourceType     types.ResourceType
	TotalAllocations int64
	TotalCPU         float64
	AvailableCPU     float64
	UsedCPU          float64
	TotalMemory      int64
	AvailableMemory  int64
	UsedMemory       int64
}

// SetPoolCapacity sets the total capacity for a resource pool
func (rm *ResourceManager) SetPoolCapacity(resourceType types.ResourceType, capacity types.ResourceCapacity) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	pool, exists := rm.resourcePools[resourceType]
	if !exists {
		return
	}

	pool.mu.Lock()
	defer pool.mu.Unlock()

	pool.totalCap = capacity
	pool.availableCap = capacity

	rm.logger.Infof("Set capacity for %s pool: CPU=%.2f, Memory=%d, Storage=%d, Bandwidth=%d",
		resourceType, capacity.CPU, capacity.Memory, capacity.Storage, capacity.Bandwidth)
}

// UpdateAvailable updates available resources (e.g., after hardware detection)
func (rm *ResourceManager) UpdateAvailable(resourceType types.ResourceType, available types.ResourceCapacity) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	pool, exists := rm.resourcePools[resourceType]
	if !exists {
		return
	}

	pool.mu.Lock()
	defer pool.mu.Unlock()

	pool.availableCap.CPU = available.CPU - pool.usedCap.CPU
	pool.availableCap.Memory = available.Memory - pool.usedCap.Memory
	pool.availableCap.Storage = available.Storage - pool.usedCap.Storage
	pool.availableCap.Bandwidth = available.Bandwidth - pool.usedCap.Bandwidth
}

// generateAllocationID generates a unique allocation ID
func generateAllocationID() string {
	return "alloc-" + time.Now().Format("20060102150405") + "-" + randomString(8)
}

func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	rand.Read(b)
	for i := range b {
		b[i] = letters[int(b[i])%len(letters)]
	}
	return string(b)
}
