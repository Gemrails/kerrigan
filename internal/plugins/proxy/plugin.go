package proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/kerrigan/kerrigan/internal/plugins/proxy/pricing"
	"github.com/kerrigan/kerrigan/internal/plugins/proxy/server"
	"github.com/kerrigan/kerrigan/internal/plugins/proxy/stats"
	"github.com/kerrigan/kerrigan/internal/plugins/proxy/tunnel"
	"github.com/kerrigan/kerrigan/pkg/log"
)

// PluginName defines the plugin name
const PluginName = "proxy"

// SupportedRegions defines supported geographic regions
var SupportedRegions = []string{"CN", "US", "JP", "SG", "EU", "HK"}

// Config holds the proxy plugin configuration
type Config struct {
	// Server configuration
	HTTPProxyPort  int  `json:"http_proxy_port"` // HTTP proxy port (default: 1080)
	SOCKS5Port     int  `json:"socks5_port"`     // SOCKS5 proxy port (default: 1081)
	TransparentTLS bool `json:"transparent_tls"` // Enable transparent TLS proxy

	// Tunnel configuration
	EnableKCP       bool   `json:"enable_kcp"`        // Enable KCP tunnel
	EnableTLSObfusc bool   `json:"enable_tls_obfusc"` // Enable TLS obfuscation
	TunnelPort      int    `json:"tunnel_port"`       // Tunnel listen port
	KCPSecret       string `json:"kcp_secret"`        // KCP encryption secret

	// Network configuration
	MaxConnections int `json:"max_connections"` // Max concurrent connections
	IdleTimeout    int `json:"idle_timeout"`    // Idle timeout in seconds
	KeepAlive      int `json:"keepalive"`       // Keep-alive interval in seconds

	// Traffic shaping
	DownloadSpeedKBPS int `json:"download_speed_kbps"` // Download speed limit (KB/s)
	UploadSpeedKBPS   int `json:"upload_speed_kbps"`   // Upload speed limit (KB/s)

	// Pricing configuration
	PricingConfig pricing.Config `json:"pricing"`

	// Statistics
	StatsEnabled  bool   `json:"stats_enabled"`  // Enable traffic statistics
	StatsInterval int    `json:"stats_interval"` // Stats report interval (seconds)
	StatsDir      string `json:"stats_dir"`      // Statistics directory

	// TUN/TAP configuration
	EnableTUNTAP bool   `json:"enable_tun_tap"` // Enable TUN/TAP device
	TUNDevice    string `json:"_tun_device"`    // TUN device name
	TUNIP        string `json:"tun_ip"`         // TUN device IP
	TUNGateway   string `json:"tun_gateway"`    // TUN gateway
}

// ProxyPlugin implements the network proxy plugin
type ProxyPlugin struct {
	config Config

	// Server components
	httpProxy    *server.HTTPProxy
	socks5Proxy  *server.SOCKS5Proxy
	tunnelServer *tunnel.Server

	// Traffic management
	shaper *TrafficShaper

	// Pricing engine
	pricingEngine *pricing.Engine

	// Statistics
	statsCollector *stats.Collector

	// State management
	mu          sync.RWMutex
	initialized bool
	running     bool
	ctx         context.Context
	cancel      context.CancelFunc
	wg          sync.WaitGroup

	// Connection tracking
	connections map[string]*ConnectionInfo
	connMutex   sync.RWMutex
}

// ConnectionInfo tracks active proxy connections
type ConnectionInfo struct {
	ID        string
	ClientIP  net.IP
	Protocol  string // "http", "socks5", "tunnel"
	RemoteIP  string
	BytesIn   int64
	BytesOut  int64
	StartTime time.Time
	Region    string
}

// NewPlugin creates a new proxy plugin instance
func NewPlugin() *ProxyPlugin {
	ctx, cancel := context.WithCancel(context.Background())
	return &ProxyPlugin{
		ctx:         ctx,
		cancel:      cancel,
		connections: make(map[string]*ConnectionInfo),
	}
}

// Name returns the plugin name
func (p *ProxyPlugin) Name() string {
	return PluginName
}

// Version returns the plugin version
func (p *ProxyPlugin) Version() string {
	return "1.0.0"
}

// Initialize initializes the plugin with configuration
func (p *ProxyPlugin) Initialize(configBytes []byte) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.initialized {
		return fmt.Errorf("plugin already initialized")
	}

	// Parse configuration
	if len(configBytes) > 0 {
		if err := json.Unmarshal(configBytes, &p.config); err != nil {
			return fmt.Errorf("failed to parse config: %w", err)
		}
	}

	// Set defaults
	p.setDefaults()

	// Initialize pricing engine
	p.pricingEngine = pricing.NewEngine(p.config.PricingConfig)

	// Initialize statistics collector
	if p.config.StatsEnabled {
		p.statsCollector = stats.NewCollector(p.config.StatsDir, p.config.StatsInterval)
	}

	// Initialize traffic shaper
	p.shaper = NewTrafficShaper(p.config.DownloadSpeedKBPS, p.config.UploadSpeedKBPS)

	// Initialize server components
	if err := p.initServers(); err != nil {
		return fmt.Errorf("failed to initialize servers: %w", err)
	}

	p.initialized = true
	log.Info("Proxy plugin initialized", "http_port", p.config.HTTPProxyPort, "socks5_port", p.config.SOCKS5Port)
	return nil
}

// setDefaults sets default configuration values
func (p *ProxyPlugin) setDefaults() {
	if p.config.HTTPProxyPort == 0 {
		p.config.HTTPProxyPort = 1080
	}
	if p.config.SOCKS5Port == 0 {
		p.config.SOCKS5Port = 1081
	}
	if p.config.TunnelPort == 0 {
		p.config.TunnelPort = 1082
	}
	if p.config.MaxConnections == 0 {
		p.config.MaxConnections = 10000
	}
	if p.config.IdleTimeout == 0 {
		p.config.IdleTimeout = 300 // 5 minutes
	}
	if p.config.KeepAlive == 0 {
		p.config.KeepAlive = 60
	}
	if p.config.StatsInterval == 0 {
		p.config.StatsInterval = 60
	}
	if p.config.StatsDir == "" {
		p.config.StatsDir = "/var/lib/kerrigan/proxy/stats"
	}
}

// initServers initializes all server components
func (p *ProxyPlugin) initServers() error {
	// Initialize HTTP proxy
	p.httpProxy = server.NewHTTPProxy(server.HTTPProxyConfig{
		Port:        p.config.HTTPProxyPort,
		IdleTimeout: time.Duration(p.config.IdleTimeout) * time.Second,
		Shaper:      p.shaper,
		Stats:       p.statsCollector,
	})

	// Initialize SOCKS5 proxy
	p.socks5Proxy = server.NewSOCKS5Proxy(server.SOCKS5ProxyConfig{
		Port:        p.config.SOCKS5Port,
		IdleTimeout: time.Duration(p.config.IdleTimeout) * time.Second,
		Shaper:      p.shaper,
		Stats:       p.statsCollector,
	})

	// Initialize tunnel server if enabled
	if p.config.EnableKCP || p.config.EnableTLSObfusc {
		p.tunnelServer = tunnel.NewServer(tunnel.Config{
			ListenPort: p.config.TunnelPort,
			EnableKCP:  p.config.EnableKCP,
			EnableTLS:  p.config.EnableTLSObfusc,
			KCPSecret:  p.config.KCPSecret,
			MaxConns:   p.config.MaxConnections,
		})
	}

	return nil
}

// Start starts the proxy services
func (p *ProxyPlugin) Start() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.running {
		return fmt.Errorf("plugin already running")
	}

	if !p.initialized {
		return fmt.Errorf("plugin not initialized")
	}

	log.Info("Starting proxy plugin...")

	// Start HTTP proxy
	if err := p.httpProxy.Start(); err != nil {
		return fmt.Errorf("failed to start HTTP proxy: %w", err)
	}

	// Start SOCKS5 proxy
	if err := p.socks5Proxy.Start(); err != nil {
		p.httpProxy.Stop()
		return fmt.Errorf("failed to start SOCKS5 proxy: %w", err)
	}

	// Start tunnel server
	if p.tunnelServer != nil {
		if err := p.tunnelServer.Start(); err != nil {
			p.httpProxy.Stop()
			p.socks5Proxy.Stop()
			return fmt.Errorf("failed to start tunnel server: %w", err)
		}
	}

	// Start statistics collector
	if p.statsCollector != nil {
		p.statsCollector.Start()
	}

	// Start background workers
	p.startWorkers()

	p.running = true
	log.Info("Proxy plugin started successfully")
	return nil
}

// startWorkers starts background worker goroutines
func (p *ProxyPlugin) startWorkers() {
	// Connection cleanup worker
	p.wg.Add(1)
	go p.cleanupConnections()
}

// cleanupConnections periodically cleans up stale connections
func (p *ProxyPlugin) cleanupConnections() {
	defer p.wg.Done()

	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-p.ctx.Done():
			return
		case <-ticker.C:
			p.cleanupStaleConnections()
		}
	}
}

// cleanupStaleConnections removes connections that have exceeded idle timeout
func (p *ProxyPlugin) cleanupStaleConnections() {
	p.connMutex.Lock()
	defer p.connMutex.Unlock()

	timeout := time.Duration(p.config.IdleTimeout) * time.Second
	now := time.Now()

	for id, conn := range p.connections {
		if now.Sub(conn.StartTime) > timeout {
			delete(p.connections, id)
		}
	}
}

// Stop stops all proxy services
func (p *ProxyPlugin) Stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.running {
		return nil
	}

	log.Info("Stopping proxy plugin...")

	// Stop all servers
	if p.httpProxy != nil {
		p.httpProxy.Stop()
	}
	if p.socks5Proxy != nil {
		p.socks5Proxy.Stop()
	}
	if p.tunnelServer != nil {
		p.tunnelServer.Stop()
	}

	// Stop statistics collector
	if p.statsCollector != nil {
		p.statsCollector.Stop()
	}

	// Cancel context and wait for workers
	p.cancel()
	p.wg.Wait()

	p.running = false
	log.Info("Proxy plugin stopped")
	return nil
}

// GetResourceProvider returns the resource provider interface
func (p *ProxyPlugin) GetResourceProvider() (ResourceProvider, bool) {
	return p, true
}

// GetTaskExecutor returns the task executor interface (not implemented for proxy)
func (p *ProxyPlugin) GetTaskExecutor() (TaskExecutor, bool) {
	return nil, false
}

// QueryResources queries available proxy resources
func (p *ProxyPlugin) QueryResources() (ResourceList, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if !p.running {
		return ResourceList{}, fmt.Errorf("plugin not running")
	}

	resources := []Resource{
		{
			ID:        "bandwidth-default",
			Type:      ResourceTypeBandwidth,
			Name:      "Network Bandwidth",
			Available: true,
			Location:  "auto",
			Price:     p.pricingEngine.GetDefaultPrice(),
			Capacity: ResourceCapacity{
				Bandwidth: int64(p.config.MaxConnections) * 1024 * 1024, // Estimated
			},
			Properties: map[string]interface{}{
				"http_port":   p.config.HTTPProxyPort,
				"socks5_port": p.config.SOCKS5Port,
				"tunnel_port": p.config.TunnelPort,
				"max_conns":   p.config.MaxConnections,
				"regions":     SupportedRegions,
				"kcp_enabled": p.config.EnableKCP,
				"tls_enabled": p.config.EnableTLSObfusc,
			},
		},
	}

	// Add regional resources
	for _, region := range SupportedRegions {
		resources = append(resources, Resource{
			ID:        fmt.Sprintf("bandwidth-%s", region),
			Type:      ResourceTypeBandwidth,
			Name:      fmt.Sprintf("Bandwidth - %s", region),
			Available: true,
			Location:  region,
			Price:     p.pricingEngine.GetRegionPrice(region),
			Capacity: ResourceCapacity{
				Bandwidth: int64(p.config.MaxConnections) * 512 * 1024,
			},
		})
	}

	return ResourceList{
		Resources:  resources,
		TotalCount: len(resources),
		Timestamp:  time.Now(),
	}, nil
}

// AllocateResources allocates bandwidth resources
func (p *ProxyPlugin) AllocateResources(req ResourceRequest) (Allocation, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if !p.running {
		return Allocation{}, fmt.Errorf("plugin not running")
	}

	// Check connection limit
	p.connMutex.RLock()
	currentConns := len(p.connections)
	p.connMutex.RUnlock()

	if currentConns >= p.config.MaxConnections {
		return Allocation{}, fmt.Errorf("max connections reached")
	}

	allocationID := AllocationID(fmt.Sprintf("alloc-%d", time.Now().UnixNano()))

	return Allocation{
		ID:        allocationID,
		Resources: []ResourceID{ResourceID("bandwidth-default")},
		StartTime: time.Now(),
		EndTime:   time.Now().Add(req.Duration),
		Status:    AllocationActive,
	}, nil
}

// ReleaseResources releases allocated resources
func (p *ProxyPlugin) ReleaseResources(id AllocationID) error {
	// Connection cleanup is handled automatically
	log.Debug("Released allocation", "id", id)
	return nil
}

// GetResourceStats returns current resource statistics
func (p *ProxyPlugin) GetResourceStats() (ResourceStats, error) {
	p.connMutex.RLock()
	activeConns := len(p.connections)
	p.connMutex.RUnlock()

	var bandwidthCapacity int64
	if p.config.MaxConnections > 0 {
		bandwidthCapacity = int64(p.config.MaxConnections) * 1024 * 1024
	}

	return ResourceStats{
		TotalResources:    int64(p.config.MaxConnections),
		AvailableCount:    int64(p.config.MaxConnections - activeConns),
		AllocatedCount:    int64(activeConns),
		Utilization:       float64(activeConns) / float64(p.config.MaxConnections),
		AvailableCapacity: ResourceCapacity{Bandwidth: bandwidthCapacity},
		Timestamp:         time.Now(),
	}, nil
}

// GetConnectionInfo returns connection information for a session
func (p *ProxyPlugin) GetConnectionInfo(id string) (*ConnectionInfo, bool) {
	p.connMutex.RLock()
	defer p.connMutex.RUnlock()

	info, exists := p.connections[id]
	return info, exists
}

// AddConnection adds a new connection to tracking
func (p *ProxyPlugin) AddConnection(id string, info *ConnectionInfo) {
	p.connMutex.Lock()
	defer p.connMutex.Unlock()

	p.connections[id] = info

	if p.statsCollector != nil {
		p.statsCollector.RecordConnection(info.Region, info.BytesIn, info.BytesOut)
	}
}

// RemoveConnection removes a connection from tracking
func (p *ProxyPlugin) RemoveConnection(id string) {
	p.connMutex.Lock()
	defer p.connMutex.Unlock()

	if conn, exists := p.connections[id]; exists && p.statsCollector != nil {
		p.statsCollector.RecordDisconnection(conn.Region, conn.BytesIn, conn.BytesOut)
	}

	delete(p.connections, id)
}

// GetPricingEngine returns the pricing engine
func (p *ProxyPlugin) GetPricingEngine() *pricing.Engine {
	return p.pricingEngine
}

// GetStatsCollector returns the statistics collector
func (p *ProxyPlugin) GetStatsCollector() *stats.Collector {
	return p.statsCollector
}

// GetTrafficShaper returns the traffic shaper
func (p *ProxyPlugin) GetTrafficShaper() *TrafficShaper {
	return p.shaper
}

// GetConfig returns the current configuration
func (p *ProxyPlugin) GetConfig() *Config {
	p.mu.RLock()
	defer p.mu.RUnlock()

	configCopy := p.config
	return &configCopy
}

// UpdateConfig updates the proxy configuration
func (p *ProxyPlugin) UpdateConfig(newConfig Config) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.running {
		return fmt.Errorf("cannot update config while running")
	}

	p.config = newConfig
	p.setDefaults()

	// Re-initialize components
	if err := p.initServers(); err != nil {
		return fmt.Errorf("failed to reinitialize servers: %w", err)
	}

	return nil
}

// GetActiveConnections returns all active connections
func (p *ProxyPlugin) GetActiveConnections() []*ConnectionInfo {
	p.connMutex.RLock()
	defer p.connMutex.RUnlock()

	connections := make([]*ConnectionInfo, 0, len(p.connections))
	for _, conn := range p.connections {
		connections = append(connections, conn)
	}

	return connections
}

// TrafficShaper manages traffic shaping
type TrafficShaper struct {
	downloadLimitKBPS int
	uploadLimitKBPS   int
	mu                sync.RWMutex
}

// NewTrafficShaper creates a new traffic shaper
func NewTrafficShaper(downloadKBPS, uploadKBPS int) *TrafficShaper {
	return &TrafficShaper{
		downloadLimitKBPS: downloadKBPS,
		uploadLimitKBPS:   uploadKBPS,
	}
}

// SetLimits sets traffic shaping limits
func (ts *TrafficShaper) SetLimits(downloadKBPS, uploadKBPS int) {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	ts.downloadLimitKBPS = downloadKBPS
	ts.uploadLimitKBPS = uploadKBPS
}

// GetDownloadLimit returns the download speed limit in bytes per second
func (ts *TrafficShaper) GetDownloadLimit() int {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	return ts.downloadLimitKBPS * 1024
}

// GetUploadLimit returns the upload speed limit in bytes per second
func (ts *TrafficShaper) GetUploadLimit() int {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	return ts.uploadLimitKBPS * 1024
}

// Ensure ProxyPlugin implements required interfaces
var _ Plugin = (*ProxyPlugin)(nil)
var _ ResourceProvider = (*ProxyPlugin)(nil)
