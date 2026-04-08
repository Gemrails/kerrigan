package cache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/kerrigan/kerrigan/pkg/log"
)

// CacheEntry represents a cache entry
type CacheEntry struct {
	Key       string
	Size      int64
	CreatedAt time.Time
	AccessAt  time.Time
	Path      string
}

// Manager manages the LRU cache for hot data
type Manager struct {
	ctx      context.Context
	maxSize  int64
	dataDir  string
	entries  map[string]*CacheEntry
	mu       sync.RWMutex
	usedSize int64
	wg       sync.WaitGroup
	closed   bool
}

// New creates a new cache manager
func New(ctx context.Context, maxSize int64, dataDir string) (*Manager, error) {
	if maxSize <= 0 {
		maxSize = 10 * 1024 * 1024 * 1024 // Default 10GB
	}

	if dataDir == "" {
		dataDir = filepath.Join(os.TempDir(), "kerrigan-storage-cache")
	}

	m := &Manager{
		ctx:      ctx,
		maxSize:  maxSize,
		dataDir:  dataDir,
		entries:  make(map[string]*CacheEntry),
		usedSize: 0,
	}

	// Create data directory
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	// Load existing cache entries
	if err := m.loadEntries(); err != nil {
		log.Warn("Failed to load cache entries", "error", err)
	}

	// Start cleanup worker
	m.wg.Add(1)
	go m.cleanupWorker()

	return m, nil
}

// Close closes the cache manager
func (m *Manager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return
	}

	m.closed = true
	m.wg.Wait()

	// Persist entries
	m.saveEntries()
}

// Get retrieves data from cache
func (m *Manager) Get(ctx context.Context, key string) ([]byte, error) {
	m.mu.RLock()
	if m.closed {
		m.mu.RUnlock()
		return nil, fmt.Errorf("cache manager closed")
	}
	m.mu.RUnlock()

	m.mu.Lock()
	defer m.mu.Unlock()

	entry, ok := m.entries[key]
	if !ok {
		return nil, fmt.Errorf("cache miss: %s", key)
	}

	// Update access time
	entry.AccessAt = time.Now()

	// Read file
	data, err := os.ReadFile(entry.Path)
	if err != nil {
		// Remove invalid entry
		delete(m.entries, key)
		m.usedSize -= entry.Size
		return nil, fmt.Errorf("failed to read cache file: %w", err)
	}

	log.Debug("Cache hit", "key", key, "size", len(data))
	return data, nil
}

// Put stores data in cache
func (m *Manager) Put(ctx context.Context, key string, data []byte) error {
	m.mu.RLock()
	if m.closed {
		m.mu.RUnlock()
		return fmt.Errorf("cache manager closed")
	}
	m.mu.RUnlock()

	size := int64(len(data))

	// Check if we need to evict
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.evictIfNeeded(size); err != nil {
		return fmt.Errorf("failed to evict: %w", err)
	}

	// Generate file path
	filePath := m.getFilePath(key)

	// Write data to file
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write cache file: %w", err)
	}

	// Update entries
	entry := &CacheEntry{
		Key:       key,
		Size:      size,
		CreatedAt: time.Now(),
		AccessAt:  time.Now(),
		Path:      filePath,
	}

	m.entries[key] = entry
	m.usedSize += size

	log.Debug("Cached data", "key", key, "size", size)

	return nil
}

// Has checks if key exists in cache
func (m *Manager) Has(key string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	_, ok := m.entries[key]
	return ok
}

// Delete removes a key from cache
func (m *Manager) Delete(key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	entry, ok := m.entries[key]
	if !ok {
		return nil
	}

	// Remove file
	if err := os.Remove(entry.Path); err != nil {
		log.Warn("Failed to remove cache file", "path", entry.Path, "error", err)
	}

	// Update stats
	delete(m.entries, key)
	m.usedSize -= entry.Size

	return nil
}

// Cleanup performs cleanup of stale entries
func (m *Manager) Cleanup() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	var toRemove []string

	for key, entry := range m.entries {
		// Remove entries older than 24 hours
		if now.Sub(entry.AccessAt) > 24*time.Hour {
			toRemove = append(toRemove, key)
		}
	}

	for _, key := range toRemove {
		entry := m.entries[key]
		os.Remove(entry.Path)
		delete(m.entries, key)
		m.usedSize -= entry.Size

		log.Debug("Cleaned up cache entry", "key", key)
	}

	// Save state
	m.saveEntries()
}

// Used returns the current cache usage
func (m *Manager) Used() int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.usedSize
}

// MaxSize returns the maximum cache size
func (m *Manager) MaxSize() int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.maxSize
}

// Count returns the number of cache entries
func (m *Manager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.entries)
}

// GetStats returns cache statistics
func (m *Manager) GetStats() *Stats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &Stats{
		MaxSize:    m.maxSize,
		UsedSize:   m.usedSize,
		EntryCount: len(m.entries),
	}

	return stats
}

// Stats represents cache statistics
type Stats struct {
	MaxSize    int64
	UsedSize   int64
	EntryCount int
}

// evictIfNeeded evicts entries if needed
func (m *Manager) evictIfNeeded(requiredSize int64) error {
	for m.usedSize+requiredSize > m.maxSize && len(m.entries) > 0 {
		if err := m.evictLRU(); err != nil {
			return err
		}
	}
	return nil
}

// evicts the least recently used entry
func (m *Manager) evictLRU() error {
	var oldest *CacheEntry
	var oldestKey string

	for key, entry := range m.entries {
		if oldest == nil || entry.AccessAt.Before(oldest.AccessAt) {
			oldest = entry
			oldestKey = key
		}
	}

	if oldest == nil {
		return fmt.Errorf("no entries to evict")
	}

	// Remove file
	if err := os.Remove(oldest.Path); err != nil {
		log.Warn("Failed to remove evicted cache file", "path", oldest.Path, "error", err)
	}

	// Update stats
	delete(m.entries, oldestKey)
	m.usedSize -= oldest.Size

	log.Debug("Evicted LRU entry", "key", oldestKey, "size", oldest.Size)

	return nil
}

// cleanupWorker periodically performs cleanup
func (m *Manager) cleanupWorker() {
	defer m.wg.Done()

	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.Cleanup()
		}
	}
}

// getFilePath returns the file path for a key
func (m *Manager) getFilePath(key string) string {
	hash := sha256.Sum256([]byte(key))
	hashStr := hex.EncodeToString(hash[:])
	return filepath.Join(m.dataDir, hashStr+".cache")
}

// loadEntries loads cache entries from disk
func (m *Manager) loadEntries() error {
	// Scan directory for cache files
	files, err := os.ReadDir(m.dataDir)
	if err != nil {
		return err
	}

	for _, f := range files {
		if f.IsDir() || filepath.Ext(f.Name()) != ".cache" {
			continue
		}

		path := filepath.Join(m.dataDir, f.Name())
		info, err := os.Stat(path)
		if err != nil {
			continue
		}

		// Extract key from filename
		key := f.Name()[:len(f.Name())-6] // Remove .cache extension

		entry := &CacheEntry{
			Key:       key,
			Size:      info.Size(),
			CreatedAt: info.ModTime(),
			AccessAt:  info.ModTime(),
			Path:      path,
		}

		m.entries[key] = entry
		m.usedSize += entry.Size
	}

	log.Info("Loaded cache entries", "count", len(m.entries), "size", m.usedSize)
	return nil
}

// saveEntries persists cache entries to disk
func (m *Manager) saveEntries() error {
	// Write metadata file
	path := filepath.Join(m.dataDir, "metadata.json")
	data := fmt.Sprintf("{\"entries\": %d, \"used_size\": %d}", len(m.entries), m.usedSize)
	return os.WriteFile(path, []byte(data), 0644)
}

// PrefetchStrategy defines the prefetching strategy
type PrefetchStrategy int

const (
	// PrefetchOnAccess prefetches when data is accessed
	PrefetchOnAccess PrefetchStrategy = iota
	// PrefetchOnPredict prefetches based on prediction
	PrefetchOnPredict
	// PrefetchOnBatch prefetches in batches
	PrefetchOnBatch
)

// Prefetcher prefetches data into cache
type Prefetcher struct {
	cache     *Manager
	strategy  PrefetchStrategy
	prefetchQ chan string
	wg        sync.WaitGroup
}

// NewPrefetcher creates a new prefetcher
func NewPrefetcher(cache *Manager, strategy PrefetchStrategy) *Prefetcher {
	return &Prefetcher{
		cache:     cache,
		strategy:  strategy,
		prefetchQ: make(chan string, 1000),
	}
}

// Start starts the prefetcher
func (p *Prefetcher) Start(ctx context.Context) {
	p.wg.Add(1)
	go p.worker(ctx)
}

// Stop stops the prefetcher
func (p *Prefetcher) Stop() {
	close(p.prefetchQ)
	p.wg.Wait()
}

// Add adds a CID to prefetch queue
func (p *Prefetcher) Add(cid string) {
	select {
	case p.prefetchQ <- cid:
	default:
		log.Warn("Prefetch queue full, dropping request")
	}
}

// worker processes prefetch requests
func (p *Prefetcher) worker(ctx context.Context) {
	defer p.wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case cid, ok := <-p.prefetchQ:
			if !ok {
				return
			}
			// Placeholder for actual prefetch logic
			log.Debug("Prefetching", "cid", cid)
		}
	}
}
