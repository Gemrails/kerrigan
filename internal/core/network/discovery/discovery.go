package discovery

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/kerrigan/kerrigan/pkg/log"
	"github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
)

const (
	// BootstrapTimeout is the timeout for bootstrapping
	BootstrapTimeout = 30 * time.Second
	// DiscoveryInterval is the interval for peer discovery
	DiscoveryInterval = 5 * time.Minute
	// ProviderRefreshInterval is the interval for refreshing provider records
	ProviderRefreshInterval = 1 * time.Hour
)

// Config holds the discovery service configuration
type Config struct {
	BootstrapPeers []string
	EnableDHT      bool
	EnableMDNS     bool
	Port           int
}

// ServiceAdvertisement holds service advertisement information
type ServiceAdvertisement struct {
	ServiceName string
	PeerID      peer.ID
	Addrs       []multiaddr.Multiaddr
	Metadata    map[string]string
	ExpiresAt   time.Time
}

// ServiceDiscovery manages node discovery using Kademlia DHT
type ServiceDiscovery struct {
	ctx            context.Context
	cancel         context.CancelFunc
	config         *Config
	host           host.Host
	dht            *dht.IpfsDHT
	wg             sync.WaitGroup
	running        bool
	advertisements map[string]*ServiceAdvertisement
	adMutex        sync.RWMutex
	peers          map[peer.ID]*peer.AddrInfo
	peersMutex     sync.RWMutex
}

// New creates a new discovery service
func New(cfg *Config, host host.Host) (*ServiceDiscovery, error) {
	ctx, cancel := context.WithCancel(context.Background())

	ds := &ServiceDiscovery{
		ctx:            ctx,
		cancel:         cancel,
		config:         cfg,
		host:           host,
		advertisements: make(map[string]*ServiceAdvertisement),
		peers:          make(map[peer.ID]*peer.AddrInfo),
	}

	return ds, nil
}

// Start starts the discovery service
func (ds *ServiceDiscovery) Start() error {
	if ds.running {
		return fmt.Errorf("discovery service already running")
	}

	log.Info("Starting discovery service...")

	// Start DHT if enabled
	if ds.config.EnableDHT {
		err := ds.startDHT()
		if err != nil {
			return fmt.Errorf("failed to start DHT: %w", err)
		}
	}

	// Bootstrap from seed nodes
	if len(ds.config.BootstrapPeers) > 0 {
		ds.wg.Add(1)
		go ds.bootstrapWorker()
	}

	// Start periodic discovery
	ds.wg.Add(1)
	go ds.discoveryWorker()

	ds.running = true
	log.Info("Discovery service started successfully")

	return nil
}

// Stop stops the discovery service
func (ds *ServiceDiscovery) Stop() error {
	if !ds.running {
		return nil
	}

	log.Info("Stopping discovery service...")
	ds.cancel()
	ds.wg.Wait()

	if ds.dht != nil {
		ds.dht.Close()
	}

	ds.running = false
	log.Info("Discovery service stopped")

	return nil
}

// startDHT starts the Kademlia DHT
func (ds *ServiceDiscovery) startDHT() error {
	var opts []dht.Option
	opts = append(opts, dht.Mode(dht.ModeAuto))
	opts = append(opts, dht.Concurrency(10))
	opts = append(opts, dht.BucketSize(20))
	opts = append(opts, dht.QueryTimeout(10*time.Second))

	dhtInstance, err := dht.New(ds.ctx, ds.host, opts...)
	if err != nil {
		return fmt.Errorf("failed to create DHT: %w", err)
	}

	ds.dht = dhtInstance

	log.Info("DHT started", "peer_id", ds.host.ID())

	return nil
}

// bootstrapWorker bootstraps the DHT from seed nodes
func (ds *ServiceDiscovery) bootstrapWorker() {
	defer ds.wg.Done()

	ctx, cancel := context.WithTimeout(ds.ctx, BootstrapTimeout)
	defer cancel()

	log.Info("Bootstrapping from seed nodes...")

	for _, addrStr := range ds.config.BootstrapPeers {
		addr, err := multiaddr.NewMultiaddr(addrStr)
		if err != nil {
			log.Warn("Invalid bootstrap peer address", "addr", addrStr, "error", err)
			continue
		}

		addrInfo, err := peer.AddrInfoFromP2pAddr(addr)
		if err != nil {
			log.Warn("Failed to parse bootstrap peer", "addr", addrStr, "error", err)
			continue
		}

		err = ds.host.Connect(ctx, *addrInfo)
		if err != nil {
			log.Warn("Failed to connect to bootstrap peer", "addr", addrStr, "error", err)
			continue
		}

		log.Info("Connected to bootstrap peer", "peer", addrInfo.ID)

		// Add to DHT routing
		if ds.dht != nil {
			err = ds.dht.Bootstrap(ctx)
			if err != nil {
				log.Warn("Failed to bootstrap DHT", "error", err)
			}
		}
	}

	log.Info("Bootstrap completed")
}

// discoveryWorker performs periodic peer discovery
func (ds *ServiceDiscovery) discoveryWorker() {
	defer ds.wg.Done()

	ticker := time.NewTicker(DiscoveryInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ds.ctx.Done():
			return
		case <-ticker.C:
			ds.discoverPeers()
		}
	}
}

// discoverPeers discovers new peers
func (ds *ServiceDiscovery) discoverPeers() {
	if ds.dht == nil {
		return
	}

	ctx := context.Background()

	// Find nearest peers to our own ID
	peers, err := ds.dht.FindNearestPeers(ds.host.ID(), 20)
	if err != nil {
		log.Error("Failed to find peers", "error", err)
		return
	}

	ds.peersMutex.Lock()
	for _, pid := range peers {
		if pid != ds.host.ID() {
			// Get peer addrs
			addrs, err := ds.dht.FindPeer(ctx, pid)
			if err == nil {
				ds.peers[pid] = &addrs
			}
		}
	}
	ds.peersMutex.Unlock()

	log.Info("Discovered peers", "count", len(peers))
}

// AdvertiseService advertises a service
func (ds *ServiceDiscovery) AdvertiseService(serviceName string, metadata map[string]string) error {
	if ds.dht == nil {
		return fmt.Errorf("DHT not initialized")
	}

	ctx := context.Background()

	// Create advertisement
	ad := &ServiceAdvertisement{
		ServiceName: serviceName,
		PeerID:      ds.host.ID(),
		Addrs:       ds.host.Addrs(),
		Metadata:    metadata,
		ExpiresAt:   time.Now().Add(ProviderRefreshInterval),
	}

	// Announce to DHT
	err := ds.dht.Provide(ctx, ds.host.ID(), true)
	if err != nil {
		return fmt.Errorf("failed to provide: %w", err)
	}

	ds.adMutex.Lock()
	ds.advertisements[serviceName] = ad
	ds.adMutex.Unlock()

	log.Info("Service advertised", "service", serviceName)

	return nil
}

// DiscoverService discovers peers providing a service
func (ds *ServiceDiscovery) DiscoverService(ctx context.Context, serviceName string) ([]peer.AddrInfo, error) {
	if ds.dht == nil {
		return nil, fmt.Errorf("DHT not initialized")
	}

	// Search for providers
	providers, err := ds.dht.FindProvidersAsync(ctx, ds.host.ID(), 20)
	if err != nil {
		return nil, fmt.Errorf("failed to find providers: %w", err)
	}

	var result []peer.AddrInfo
	for p := range providers {
		result = append(result, p)
	}

	log.Info("Found service providers", "service", serviceName, "count", len(result))

	return result, nil
}

// FindPeer finds peer information
func (ds *ServiceDiscovery) FindPeer(ctx context.Context, peerID peer.ID) (*peer.AddrInfo, error) {
	// Check cached peers first
	ds.peersMutex.RLock()
	if info, ok := ds.peers[peerID]; ok {
		ds.peersMutex.RUnlock()
		return info, nil
	}
	ds.peersMutex.RUnlock()

	// Query DHT
	if ds.dht != nil {
		info, err := ds.dht.FindPeer(ctx, peerID)
		if err == nil {
			ds.peersMutex.Lock()
			ds.peers[peerID] = &info
			ds.peersMutex.Unlock()
			return &info, nil
		}
	}

	return nil, fmt.Errorf("peer not found: %s", peerID)
}

// GetAllPeers returns all discovered peers
func (ds *ServiceDiscovery) GetAllPeers() map[peer.ID]*peer.AddrInfo {
	ds.peersMutex.RLock()
	defer ds.peersMutex.RUnlock()

	result := make(map[peer.ID]*peer.AddrInfo)
	for k, v := range ds.peers {
		result[k] = v
	}

	return result
}

// GetAdvertisedServices returns all advertised services
func (ds *ServiceDiscovery) GetAdvertisedServices() map[string]*ServiceAdvertisement {
	ds.adMutex.RLock()
	defer ds.adMutex.RUnlock()

	result := make(map[string]*ServiceAdvertisement)
	for k, v := range ds.advertisements {
		result[k] = v
	}

	return result
}

// ConnectToPeer connects to a discovered peer
func (ds *ServiceDiscovery) ConnectToPeer(ctx context.Context, peerID peer.ID) error {
	// Find peer info
	info, err := ds.FindPeer(ctx, peerID)
	if err != nil {
		return fmt.Errorf("peer not found: %w", err)
	}

	// Connect
	err = ds.host.Connect(ctx, *info)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}

	log.Info("Connected to peer via discovery", "peer", peerID)

	return nil
}

// RoutingTableSize returns the size of the DHT routing table
func (ds *ServiceDiscovery) RoutingTableSize() int {
	if ds.dht == nil {
		return 0
	}

	rt := ds.dht.RoutingTable()
	if rt == nil {
		return 0
	}

	return rt.Size()
}

// RefreshProviders refreshes provider records
func (ds *ServiceDiscovery) RefreshProviders() {
	ds.adMutex.RLock()
	defer ds.adMutex.RUnlock()

	ctx := context.Background()

	for serviceName, ad := range ds.advertisements {
		if time.Now().After(ad.ExpiresAt) {
			// Re-advertise
			err := ds.dht.Provide(ctx, ds.host.ID(), true)
			if err != nil {
				log.Error("Failed to re-advertise service", "service", serviceName, "error", err)
				continue
			}

			ad.ExpiresAt = time.Now().Add(ProviderRefreshInterval)
			log.Info("Refreshed service advertisement", "service", serviceName)
		}
	}
}

// GetHost returns the libp2p host
func (ds *ServiceDiscovery) GetHost() host.Host {
	return ds.host
}
