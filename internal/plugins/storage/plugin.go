package storage

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/kerrigan/kerrigan/internal/plugins/storage/cache"
	"github.com/kerrigan/kerrigan/internal/plugins/storage/download"
	"github.com/kerrigan/kerrigan/internal/plugins/storage/ipfs"
	"github.com/kerrigan/kerrigan/internal/plugins/storage/market"
	"github.com/kerrigan/kerrigan/pkg/log"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
)

// PluginName is the name of the storage plugin
const PluginName = "storage"

// Plugin implements the storage plugin for distributed file storage
type Plugin struct {
	ctx    context.Context
	cancel context.CancelFunc
	config *Config

	ipfsClient *ipfs.Client
	market     *market.Market
	cacheMgr   *cache.Manager
	downloader *download.Manager

	host         host.Host
	quotaManager *quotaManager

	mu      sync.RWMutex
	running bool
	wg      sync.WaitGroup
}

// Config holds the storage plugin configuration
type Config struct {
	IPFSAPI         string
	DataDir         string
	MaxStorageQuota int64
	CacheSize       int64
	EnableMarket    bool
	DownloadPeers   int
	PrefetchEnabled bool
	PinLocal        bool
}

// New creates a new storage plugin
func New(cfg *Config) (*Plugin, error) {
	if cfg == nil {
		cfg = &Config{}
	}

	// Set defaults
	if cfg.IPFSAPI == "" {
		cfg.IPFSAPI = "localhost:5001"
	}
	if cfg.MaxStorageQuota == 0 {
		cfg.MaxStorageQuota = 100 * 1024 * 1024 * 1024 // 100GB
	}
	if cfg.CacheSize == 0 {
		cfg.CacheSize = 10 * 1024 * 1024 * 1024 // 10GB
	}
	if cfg.DownloadPeers == 0 {
		cfg.DownloadPeers = 4
	}

	ctx, cancel := context.WithCancel(context.Background())

	p := &Plugin{
		ctx:    ctx,
		cancel: cancel,
		config: cfg,
	}

	return p, nil
}

// Start starts the storage plugin
func (p *Plugin) Start() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.running {
		return fmt.Errorf("storage plugin already running")
	}

	log.Info("Starting storage plugin...")

	// Initialize IPFS client
	ipfsClient, err := ipfs.NewClient(p.ctx, p.config.IPFSAPI)
	if err != nil {
		return fmt.Errorf("failed to create IPFS client: %w", err)
	}
	p.ipfsClient = ipfsClient

	// Initialize market if enabled
	if p.config.EnableMarket {
		market, err := market.New(p.ctx, p.config.DataDir)
		if err != nil {
			return fmt.Errorf("failed to create market: %w", err)
		}
		p.market = market
	}

	// Initialize cache manager
	cacheMgr, err := cache.New(p.ctx, p.config.CacheSize, p.config.DataDir)
	if err != nil {
		return fmt.Errorf("failed to create cache manager: %w", err)
	}
	p.cacheMgr = cacheMgr

	// Initialize download manager
	downloader, err := download.New(p.ctx, p.config.DownloadPeers)
	if err != nil {
		return fmt.Errorf("failed to create download manager: %w", err)
	}
	p.downloader = downloader

	// Initialize quota manager
	p.quotaManager = newQuotaManager(p.config.MaxStorageQuota)

	// Start background workers
	p.startWorkers()

	p.running = true
	log.Info("Storage plugin started successfully")

	return nil
}

// Stop stops the storage plugin
func (p *Plugin) Stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.running {
		return nil
	}

	log.Info("Stopping storage plugin...")
	p.cancel()
	p.wg.Wait()

	if p.ipfsClient != nil {
		p.ipfsClient.Close()
	}
	if p.cacheMgr != nil {
		p.cacheMgr.Close()
	}
	if p.downloader != nil {
		p.downloader.Close()
	}
	if p.market != nil {
		p.market.Close()
	}

	p.running = false
	log.Info("Storage plugin stopped")

	return nil
}

// startWorkers starts background workers
func (p *Plugin) startWorkers() {
	p.wg.Add(1)
	go p.cacheCleanupWorker()

	p.wg.Add(1)
	go p.quotaCheckWorker()
}

// cacheCleanupWorker periodically cleans up cache
func (p *Plugin) cacheCleanupWorker() {
	defer p.wg.Done()

	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-p.ctx.Done():
			return
		case <-ticker.C:
			if p.cacheMgr != nil {
				p.cacheMgr.Cleanup()
			}
		}
	}
}

// quotaCheckWorker periodically checks storage quota
func (p *Plugin) quotaCheckWorker() {
	defer p.wg.Done()

	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-p.ctx.Done():
			return
		case <-ticker.C:
			p.quotaManager.checkAndEvict()
		}
	}
}

// SetHost sets the libp2p host for P2P communication
func (p *Plugin) SetHost(h host.Host) {
	p.host = h
}

// Upload uploads data to IPFS and returns the CID
func (p *Plugin) Upload(ctx context.Context, data []byte) (string, error) {
	if !p.running {
		return "", fmt.Errorf("plugin not running")
	}

	// Check quota
	if !p.quotaManager.canStore(int64(len(data))) {
		return "", fmt.Errorf("storage quota exceeded")
	}

	// Add to IPFS
	cid, err := p.ipfsClient.Add(ctx, data)
	if err != nil {
		return "", fmt.Errorf("failed to add to IPFS: %w", err)
	}

	// Pin if configured
	if p.config.PinLocal {
		if err := p.ipfsClient.Pin(ctx, cid); err != nil {
			log.Warn("Failed to pin data", "cid", cid, "error", err)
		}
	}

	// Update quota
	p.quotaManager.addUsed(int64(len(data)))

	log.Info("Data uploaded", "cid", cid, "size", len(data))

	return cid, nil
}

// UploadFile uploads a file to IPFS
func (p *Plugin) UploadFile(ctx context.Context, path string) (string, int64, error) {
	if !p.running {
		return "", 0, fmt.Errorf("plugin not running")
	}

	size, err := p.quotaManager.getFileSize(path)
	if err != nil {
		return "", 0, fmt.Errorf("failed to get file size: %w", err)
	}

	if !p.quotaManager.canStore(size) {
		return "", 0, fmt.Errorf("storage quota exceeded")
	}

	cid, size, err := p.ipfsClient.AddFile(ctx, path)
	if err != nil {
		return "", 0, fmt.Errorf("failed to add file to IPFS: %w", err)
	}

	if p.config.PinLocal {
		if err := p.ipfsClient.Pin(ctx, cid); err != nil {
			log.Warn("Failed to pin file", "cid", cid, "error", err)
		}
	}

	p.quotaManager.addUsed(size)

	log.Info("File uploaded", "cid", cid, "size", size)

	return cid, size, nil
}

// Download downloads data from IPFS by CID
func (p *Plugin) Download(ctx context.Context, cid string) ([]byte, error) {
	if !p.running {
		return nil, fmt.Errorf("plugin not running")
	}

	// Check cache first
	data, err := p.cacheMgr.Get(ctx, cid)
	if err == nil && data != nil {
		log.Debug("Cache hit", "cid", cid)
		return data, nil
	}

	// Download from IPFS
	data, err = p.ipfsClient.Cat(ctx, cid)
	if err != nil {
		return nil, fmt.Errorf("failed to download from IPFS: %w", err)
	}

	// Store in cache
	if err := p.cacheMgr.Put(ctx, cid, data); err != nil {
		log.Warn("Failed to cache data", "cid", cid, "error", err)
	}

	return data, nil
}

// DownloadFromPeers downloads data from P2P peers
func (p *Plugin) DownloadFromPeers(ctx context.Context, cid string, peers []peer.ID) ([]byte, error) {
	if !p.running {
		return nil, fmt.Errorf("plugin not running")
	}

	if p.host == nil {
		return nil, fmt.Errorf("P2P host not set")
	}

	// Try cache first
	data, err := p.cacheMgr.Get(ctx, cid)
	if err == nil && data != nil {
		log.Debug("Cache hit for peer download", "cid", cid)
		return data, nil
	}

	// Download from peers
	data, err = p.downloader.Download(ctx, p.host, cid, peers)
	if err != nil {
		return nil, fmt.Errorf("failed to download from peers: %w", err)
	}

	// Cache the result
	if err := p.cacheMgr.Put(ctx, cid, data); err != nil {
		log.Warn("Failed to cache downloaded data", "cid", cid, "error", err)
	}

	return data, nil
}

// Pin pins a CID to local IPFS node
func (p *Plugin) Pin(ctx context.Context, cid string) error {
	if !p.running {
		return fmt.Errorf("plugin not running")
	}

	return p.ipfsClient.Pin(ctx, cid)
}

// Unpin unpins a CID from local IPFS node
func (p *Plugin) Unpin(ctx context.Context, cid string) error {
	if !p.running {
		return fmt.Errorf("plugin not running")
	}

	return p.ipfsClient.Unpin(ctx, cid)
}

// GetPinList returns all pinned CIDs
func (p *Plugin) GetPinList(ctx context.Context) ([]string, error) {
	if !p.running {
		return nil, fmt.Errorf("plugin not running")
	}

	return p.ipfsClient.PinList(ctx)
}

// GetStorageInfo returns storage quota information
func (p *Plugin) GetStorageInfo() *StorageInfo {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return &StorageInfo{
		MaxQuota:  p.config.MaxStorageQuota,
		UsedQuota: p.quotaManager.getUsed(),
		CacheSize: p.config.CacheSize,
		CacheUsed: p.cacheMgr.Used(),
		Running:   p.running,
	}
}

// RegisterStorageProvider registers this node as a storage provider in the market
func (p *Plugin) RegisterStorageProvider(ctx context.Context, pricePerGB int64, minDuration time.Duration) error {
	if !p.running {
		return fmt.Errorf("plugin not running")
	}

	if p.market == nil {
		return fmt.Errorf("market not enabled")
	}

	return p.market.RegisterProvider(ctx, pricePerGB, minDuration)
}

// UpdateStoragePrice updates the storage price
func (p *Plugin) UpdateStoragePrice(ctx context.Context, pricePerGB int64) error {
	if p.market == nil {
		return fmt.Errorf("market not enabled")
	}

	return p.market.UpdatePrice(ctx, pricePerGB)
}

// GetProviders returns available storage providers from the market
func (p *Plugin) GetProviders(ctx context.Context, size int64) ([]*market.Provider, error) {
	if p.market == nil {
		return nil, fmt.Errorf("market not enabled")
	}

	return p.market.FindProviders(ctx, size)
}

// RequestStorage requests storage from a provider
func (p *Plugin) RequestStorage(ctx context.Context, providerID string, cid string, duration time.Duration) error {
	if p.market == nil {
		return fmt.Errorf("market not enabled")
	}

	return p.market.RequestStorage(ctx, providerID, cid, duration)
}

// GenerateStorageProof generates a storage proof for a CID
func (p *Plugin) GenerateStorageProof(ctx context.Context, cid string) (*market.StorageProof, error) {
	if p.market == nil {
		return nil, fmt.Errorf("market not enabled")
	}

	return p.market.GenerateProof(ctx, cid)
}

// VerifyStorageProof verifies a storage proof
func (p *Plugin) VerifyStorageProof(ctx context.Context, proof *market.StorageProof) (bool, error) {
	if p.market == nil {
		return false, fmt.Errorf("market not enabled")
	}

	return p.market.VerifyProof(ctx, proof)
}

// GetCachedData returns cached data if available
func (p *Plugin) GetCachedData(ctx context.Context, cid string) ([]byte, error) {
	return p.cacheMgr.Get(ctx, cid)
}

// Prefetch prefetches data from IPFS to cache
func (p *Plugin) Prefetch(ctx context.Context, cid string) error {
	if !p.config.PrefetchEnabled {
		return nil
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		data, err := p.ipfsClient.Cat(ctx, cid)
		if err != nil {
			log.Warn("Prefetch failed", "cid", cid, "error", err)
			return
		}

		if err := p.cacheMgr.Put(ctx, cid, data); err != nil {
			log.Warn("Prefetch cache put failed", "cid", cid, "error", err)
		}
	}()

	return nil
}

// StorageInfo holds storage quota information
type StorageInfo struct {
	MaxQuota  int64
	UsedQuota int64
	CacheSize int64
	CacheUsed int64
	Running   bool
}

// quotaManager manages storage quota
type quotaManager struct {
	mu        sync.Mutex
	maxQuota  int64
	usedQuota int64
	usedFiles map[string]int64
}

func newQuotaManager(maxQuota int64) *quotaManager {
	return &quotaManager{
		maxQuota:  maxQuota,
		usedQuota: 0,
		usedFiles: make(map[string]int64),
	}
}

func (qm *quotaManager) canStore(size int64) bool {
	qm.mu.Lock()
	defer qm.mu.Unlock()
	return qm.usedQuota+size <= qm.maxQuota
}

func (qm *quotaManager) addUsed(size int64) {
	qm.mu.Lock()
	defer qm.mu.Unlock()
	qm.usedQuota += size
}

func (qm *quotaManager) removeUsed(size int64) {
	qm.mu.Lock()
	defer qm.mu.Unlock()
	qm.usedQuota -= size
	if qm.usedQuota < 0 {
		qm.usedQuota = 0
	}
}

func (qm *quotaManager) getUsed() int64 {
	qm.mu.Lock()
	defer qm.mu.Unlock()
	return qm.usedQuota
}

func (qm *quotaManager) checkAndEvict() {
	// Placeholder for quota eviction logic
}

func (qm *quotaManager) getFileSize(path string) (int64, error) {
	return 0, nil // Placeholder
}
