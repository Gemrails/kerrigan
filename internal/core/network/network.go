package network

import (
	"context"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/kerrigan/kerrigan/internal/core/network/control"
	"github.com/kerrigan/kerrigan/internal/core/network/data"
	"github.com/kerrigan/kerrigan/internal/core/network/discovery"
	"github.com/kerrigan/kerrigan/internal/core/network/tunnel"
	"github.com/kerrigan/kerrigan/pkg/log"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
	"github.com/multiformats/go-multiaddr"
)

const (
	// DefaultControlPort is the default control plane port
	DefaultControlPort = 38888
	// DefaultDataPort is the default data plane port
	DefaultDataPort = 38889
	// NetworkShutdownTimeout is the timeout for network shutdown
	NetworkShutdownTimeout = 30 * time.Second
)

// NetworkStatus represents the network status
type NetworkStatus int

const (
	NetworkStatusStopped NetworkStatus = iota
	NetworkStatusStarting
	NetworkStatusRunning
	NetworkStatusStopping
)

// Config holds the complete network configuration
type Config struct {
	// Node identification
	NodeID string

	// Control plane config
	ControlPort       int
	ControlListenAddr string
	BootstrapPeers    []string

	// Data plane config
	DataPort       int
	DataListenAddr string
	EncryptionKey  []byte
	KCPMode        string
	KCPParams      *data.KCPParams

	// Discovery config
	EnableDHT      bool
	DiscoveryPeers []string

	// Tunnel config
	EnableHolePunching bool
	EnableRelay        bool

	// Network config
	MaxPeers          int
	ConnectionTimeout time.Duration
}

// DefaultConfig returns the default network configuration
func DefaultConfig() *Config {
	return &Config{
		ControlPort:        DefaultControlPort,
		DataPort:           DefaultDataPort,
		EnableDHT:          true,
		EnableHolePunching: true,
		EnableRelay:        true,
		MaxPeers:           50,
		ConnectionTimeout:  10 * time.Second,
		KCPParams:          data.DefaultKCPParams(),
	}
}

// Network coordinates all P2P network components
type Network struct {
	ctx    context.Context
	cancel context.CancelFunc
	config *Config
	status atomic.Int32

	control   *control.ControlPlane
	data      *data.DataPlane
	discovery *discovery.ServiceDiscovery
	tunnel    *tunnel.TunnelManager

	wg sync.WaitGroup
	mu sync.RWMutex

	// Event handlers
	eventHandlers []EventHandler
}

// EventHandler handles network events
type EventHandler interface {
	OnPeerConnected(peerID peer.ID)
	OnPeerDisconnected(peerID peer.ID)
	OnMessageReceived(peerID peer.ID, data []byte)
}

// EventHandlerFunc is a function adapter for EventHandler
type EventHandlerFunc func(event Event)

// Event represents a network event
type Event struct {
	Type      string
	PeerID    peer.ID
	Timestamp time.Time
	Data      interface{}
}

// New creates a new network instance
func New(cfg *Config) (*Network, error) {
	ctx, cancel := context.WithCancel(context.Background())

	net := &Network{
		ctx:    ctx,
		cancel: cancel,
		config: cfg,
	}

	net.status.Store(int32(NetworkStatusStopped))

	return net, nil
}

// Start starts the network and all its components
func (n *Network) Start() error {
	if !n.status.CompareAndSwap(int32(NetworkStatusStopped), int32(NetworkStatusStarting)) {
		return fmt.Errorf("network already running or starting")
	}

	log.Info("Starting network...")

	// Initialize control plane
	controlCfg := &control.Config{
		ListenAddr:     n.config.ControlListenAddr,
		Port:           n.config.ControlPort,
		BootstrapPeers: n.config.BootstrapPeers,
	}

	cp, err := control.New(controlCfg)
	if err != nil {
		n.status.Store(int32(NetworkStatusStopped))
		return fmt.Errorf("failed to create control plane: %w", err)
	}
	n.control = cp

	// Initialize data plane
	dataCfg := &data.Config{
		ListenAddr:    n.config.DataListenAddr,
		Port:          n.config.DataPort,
		EncryptionKey: n.config.EncryptionKey,
		KCPMode:       n.config.KCPMode,
		KCPParams:     n.config.KCPParams,
	}

	dp, err := data.New(dataCfg)
	if err != nil {
		n.status.Store(int32(NetworkStatusStopped))
		return fmt.Errorf("failed to create data plane: %w", err)
	}
	n.data = dp

	// Start control plane
	err = n.control.Start()
	if err != nil {
		n.status.Store(int32(NetworkStatusStopped))
		return fmt.Errorf("failed to start control plane: %w", err)
	}

	// Start data plane
	err = n.data.Start()
	if err != nil {
		n.control.Stop()
		n.status.Store(int32(NetworkStatusStopped))
		return fmt.Errorf("failed to start data plane: %w", err)
	}

	// Initialize discovery service
	discoveryCfg := &discovery.Config{
		BootstrapPeers: n.config.DiscoveryPeers,
		EnableDHT:      n.config.EnableDHT,
		Port:           n.config.ControlPort,
	}

	ds, err := discovery.New(discoveryCfg, n.control.GetHost())
	if err != nil {
		n.data.Stop()
		n.control.Stop()
		n.status.Store(int32(NetworkStatusStopped))
		return fmt.Errorf("failed to create discovery service: %w", err)
	}
	n.discovery = ds

	err = n.discovery.Start()
	if err != nil {
		n.data.Stop()
		n.control.Stop()
		n.status.Store(int32(NetworkStatusStopped))
		return fmt.Errorf("failed to start discovery service: %w", err)
	}

	// Initialize tunnel manager
	tunnelCfg := &tunnel.Config{
		EnableHolePunching: n.config.EnableHolePunching,
		EnableRelay:        n.config.EnableRelay,
	}

	tm, err := tunnel.New(tunnelCfg, n.control.GetHost())
	if err != nil {
		n.discovery.Stop()
		n.data.Stop()
		n.control.Stop()
		n.status.Store(int32(NetworkStatusStopped))
		return fmt.Errorf("failed to create tunnel manager: %w", err)
	}
	n.tunnel = tm

	err = n.tunnel.Start()
	if err != nil {
		n.discovery.Stop()
		n.data.Stop()
		n.control.Stop()
		n.status.Store(int32(NetworkStatusStopped))
		return fmt.Errorf("failed to start tunnel manager: %w", err)
	}

	// Advertise our services
	err = n.discovery.AdvertiseService("kerrigan", map[string]string{
		"control_port": fmt.Sprintf("%d", n.config.ControlPort),
		"data_port":    fmt.Sprintf("%d", n.config.DataPort),
	})
	if err != nil {
		log.Warn("Failed to advertise service", "error", err)
	}

	n.status.Store(int32(NetworkStatusRunning))
	log.Info("Network started successfully")
	log.Info("Network info",
		"peer_id", n.control.GetPeerID(),
		"control_addr", n.control.GetListenAddrs(),
		"data_port", n.data.GetPort(),
	)

	return nil
}

// Stop stops the network and all its components
func (n *Network) Stop() error {
	if !n.status.CompareAndSwap(int32(NetworkStatusRunning), int32(NetworkStatusStopping)) {
		return fmt.Errorf("network not running")
	}

	log.Info("Stopping network...")
	n.cancel()
	n.wg.Wait()

	// Stop in reverse order
	if n.tunnel != nil {
		n.tunnel.Stop()
	}

	if n.discovery != nil {
		n.discovery.Stop()
	}

	if n.data != nil {
		n.data.Stop()
	}

	if n.control != nil {
		n.control.Stop()
	}

	n.status.Store(int32(NetworkStatusStopped))
	log.Info("Network stopped")

	return nil
}

// Connect connects to a peer
func (n *Network) Connect(ctx context.Context, addr multiaddr.Multiaddr) (peer.ID, error) {
	if n.status.Load() != int32(NetworkStatusRunning) {
		return "", fmt.Errorf("network not running")
	}

	// First try discovery
	info, err := peer.AddrInfoFromP2pAddr(addr)
	if err == nil {
		err = n.discovery.ConnectToPeer(ctx, info.ID)
		if err == nil {
			return info.ID, nil
		}
	}

	// Fall back to direct control plane connection
	return n.control.Connect(ctx, addr)
}

// ConnectWithAddrInfo connects to a peer using AddrInfo
func (n *Network) ConnectWithAddrInfo(ctx context.Context, info peer.AddrInfo) error {
	if n.status.Load() != int32(NetworkStatusRunning) {
		return fmt.Errorf("network not running")
	}

	// Add to peer store
	n.control.GetHost().Peerstore().AddAddrs(info.ID, info.Addrs, peerstore.TempAddrTTL)

	// Connect via control plane
	err := n.control.ConnectWithAddrInfo(ctx, info)
	if err != nil {
		return err
	}

	// Establish data plane connection
	addr := info.Addrs[0].String()
	_, err = n.data.Connect(ctx, info.ID, addr)
	if err != nil {
		log.Warn("Failed to establish data connection", "peer", info.ID, "error", err)
	}

	return nil
}

// Disconnect disconnects from a peer
func (n *Network) Disconnect(peerID peer.ID) error {
	if n.status.Load() != int32(NetworkStatusRunning) {
		return fmt.Errorf("network not running")
	}

	// Close tunnel
	n.tunnel.CloseTunnel(peerID)

	// Disconnect data plane
	n.data.Disconnect(peerID)

	// Disconnect control plane
	return n.control.Disconnect(peerID)
}

// Send sends data to a peer via the data plane
func (n *Network) Send(ctx context.Context, peerID peer.ID, data []byte) (int, error) {
	if n.status.Load() != int32(NetworkStatusRunning) {
		return 0, fmt.Errorf("network not running")
	}

	return n.data.Send(ctx, peerID, data)
}

// Receive receives data from a peer via the data plane
func (n *Network) Receive(ctx context.Context, peerID peer.ID, buf []byte) (int, error) {
	if n.status.Load() != int32(NetworkStatusRunning) {
		return 0, fmt.Errorf("network not running")
	}

	return n.data.Receive(ctx, peerID, buf)
}

// GetPeerID returns the local peer ID
func (n *Network) GetPeerID() peer.ID {
	if n.control == nil {
		return ""
	}
	return n.control.GetPeerID()
}

// GetControlAddrs returns the control plane addresses
func (n *Network) GetControlAddrs() []multiaddr.Multiaddr {
	if n.control == nil {
		return nil
	}
	return n.control.GetListenAddrs()
}

// GetDataPort returns the data plane port
func (n *Network) GetDataPort() int {
	if n.data == nil {
		return 0
	}
	return n.data.GetPort()
}

// GetControlPort returns the control plane port
func (n *Network) GetControlPort() int {
	if n.control == nil {
		return 0
	}
	return n.control.GetPort()
}

// GetConnectedPeers returns all connected peers
func (n *Network) GetConnectedPeers() map[peer.ID]*control.PeerInfo {
	if n.control == nil {
		return nil
	}
	return n.control.GetAllPeers()
}

// GetDataConnections returns all data plane connections
func (n *Network) GetDataConnections() map[peer.ID]*data.ConnectionInfo {
	if n.data == nil {
		return nil
	}
	return n.data.GetAllConnections()
}

// GetDiscoveryPeers returns all discovered peers
func (n *Network) GetDiscoveryPeers() map[peer.ID]*peer.AddrInfo {
	if n.discovery == nil {
		return nil
	}
	return n.discovery.GetAllPeers()
}

// GetTunnels returns all active tunnels
func (n *Network) GetTunnels() map[peer.ID]*tunnel.TunnelInfo {
	if n.tunnel == nil {
		return nil
	}
	return n.tunnel.GetAllTunnels()
}

// IsConnected checks if connected to a peer
func (n *Network) IsConnected(peerID peer.ID) bool {
	if n.control == nil {
		return false
	}
	return n.control.IsConnected(peerID)
}

// IsDataConnected checks if data plane is connected to a peer
func (n *Network) IsDataConnected(peerID peer.ID) bool {
	if n.data == nil {
		return false
	}
	return n.data.IsConnected(peerID)
}

// GetStatus returns the current network status
func (n *Network) GetStatus() NetworkStatus {
	return NetworkStatus(n.status.Load())
}

// IsRunning checks if the network is running
func (n *Network) IsRunning() bool {
	return n.status.Load() == int32(NetworkStatusRunning)
}

// GetNetworkInfo returns network information
func (n *Network) GetNetworkInfo() *NetworkInfo {
	n.mu.RLock()
	defer n.mu.RUnlock()

	info := &NetworkInfo{
		Status:       NetworkStatus(n.status.Load()),
		PeerID:       n.GetPeerID(),
		ControlAddrs: n.GetControlAddrs(),
		DataPort:     n.GetDataPort(),
		ControlPort:  n.GetControlPort(),
	}

	if n.control != nil {
		info.ConnectedPeers = len(n.control.GetAllPeers())
	}

	if n.data != nil {
		info.ActiveDataConnections = int(n.data.GetStats().ActiveConnections.Load())
	}

	if n.discovery != nil {
		info.DiscoveredPeers = len(n.discovery.GetAllPeers())
		info.RoutingTableSize = n.discovery.RoutingTableSize()
	}

	return info
}

// NetworkInfo holds network information
type NetworkInfo struct {
	Status                NetworkStatus
	PeerID                peer.ID
	ControlAddrs          []multiaddr.Multiaddr
	DataPort              int
	ControlPort           int
	ConnectedPeers        int
	ActiveDataConnections int
	DiscoveredPeers       int
	RoutingTableSize      int
}

// RegisterEventHandler registers an event handler
func (n *Network) RegisterEventHandler(handler EventHandler) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.eventHandlers = append(n.eventHandlers, handler)
}

// DiscoverPeers discovers new peers
func (n *Network) DiscoverPeers(ctx context.Context, serviceName string) ([]peer.AddrInfo, error) {
	if n.discovery == nil {
		return nil, fmt.Errorf("discovery not initialized")
	}

	return n.discovery.DiscoverService(ctx, serviceName)
}

// EstablishTunnel establishes a tunnel to a peer
func (n *Network) EstablishTunnel(ctx context.Context, peerID peer.ID) (*tunnel.TunnelInfo, error) {
	if n.tunnel == nil {
		return nil, fmt.Errorf("tunnel manager not initialized")
	}

	return n.tunnel.EstablishTunnel(ctx, peerID)
}

// GetControlPlane returns the control plane
func (n *Network) GetControlPlane() *control.ControlPlane {
	return n.control
}

// GetDataPlane returns the data plane
func (n *Network) GetDataPlane() *data.DataPlane {
	return n.data
}

// GetDiscovery returns the discovery service
func (n *Network) GetDiscovery() *discovery.ServiceDiscovery {
	return n.discovery
}

// GetTunnelManager returns the tunnel manager
func (n *Network) GetTunnelManager() *tunnel.TunnelManager {
	return n.tunnel
}

// GetMultiaddr returns the multiaddr for this node
func (n *Network) GetMultiaddr() string {
	if n.control == nil {
		return ""
	}

	addrs := n.control.GetListenAddrs()
	if len(addrs) == 0 {
		return ""
	}

	return addrs[0].String()
}

// GetAdvertisedServices returns all advertised services
func (n *Network) GetAdvertisedServices() map[string]*discovery.ServiceAdvertisement {
	if n.discovery == nil {
		return nil
	}
	return n.discovery.GetAdvertisedServices()
}

// ResolvePeerAddr resolves a peer address from string
func (n *Network) ResolvePeerAddr(addrStr string) (*peer.AddrInfo, error) {
	addr, err := multiaddr.NewMultiaddr(addrStr)
	if err != nil {
		return nil, fmt.Errorf("invalid address: %w", err)
	}

	return peer.AddrInfoFromP2pAddr(addr)
}

// PingPeer pings a peer and returns the latency
func (n *Network) PingPeer(ctx context.Context, peerID peer.ID) (time.Duration, error) {
	if n.control == nil {
		return 0, fmt.Errorf("control plane not initialized")
	}

	// Use libp2p ping
	// This is a simplified version - actual implementation would use ping protocol
	start := time.Now()

	stream, err := n.control.OpenStream(ctx, peerID)
	if err != nil {
		return 0, err
	}
	defer stream.Close()

	latency := time.Since(start)
	return latency, nil
}

// GetStats returns network statistics
func (n *Network) GetStats() Stats {
	stats := Stats{}

	if n.data != nil {
		dpStats := n.data.GetStats()
		stats.DataPlane = dpStats
	}

	if n.control != nil {
		stats.ControlPlanePeers = len(n.control.GetAllPeers())
	}

	if n.discovery != nil {
		stats.DiscoveredPeers = len(n.discovery.GetAllPeers())
		stats.RoutingTableSize = n.discovery.RoutingTableSize()
	}

	return stats
}

// Stats holds network statistics
type Stats struct {
	ControlPlanePeers int
	DiscoveredPeers   int
	RoutingTableSize  int
	DataPlane         data.Stats
}
