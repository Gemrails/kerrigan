package tunnel

import (
	"context"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/net/conngater"
	"github.com/libp2p/go-libp2p/p2p/net/connmgr"
	"github.com/multiformats/go-multiaddr"
	"github.com/kerrigan/kerrigan/pkg/log"
)

const (
	// HolePunchTimeout is the timeout for hole punching
	HolePunchTimeout = 10 * time.Second
	// RelayTimeout is the timeout for relay connection
	RelayTimeout = 30 * time.Second
	// ConnectionRetryInterval is the interval for connection retry
	ConnectionRetryInterval = 5 * time.Second
	// MaxRetries is the maximum number of connection retries
	MaxRetries = 3
	// DirectConnectionPriority is the priority for direct connections
	DirectConnectionPriority = 1
	// RelayConnectionPriority is the priority for relay connections
	RelayConnectionPriority = 2
)

// ConnectionType represents the type of connection
type ConnectionType int

const (
	ConnectionTypeDirect ConnectionType = iota
	ConnectionTypeRelay
	ConnectionTypeHolePunch
)

// Config holds the tunnel management configuration
type Config struct {
	EnableHolePunching bool
	EnableRelay        bool
	RelayCircuitAddr   string
	ConnectionManager  *connmgr.ConnectionManager
	ConnGater          *conngater.BasicConnectionGater
}

// TunnelInfo holds information about a tunnel
type TunnelInfo struct {
	PeerID         peer.ID
	ConnectionType ConnectionType
	LocalAddr      string
	RemoteAddr     string
	EstablishedAt  time.Time
	LastActive     time.Time
	Latency        time.Duration
	Active         atomic.Bool
}

// TunnelManager manages tunnels between peers with NAT traversal support
type TunnelManager struct {
	ctx          context.Context
	cancel       context.CancelFunc
	config       *Config
	host         host.Host
	tunnels      map[peer.ID]*TunnelInfo
	tunnelsMutex sync.RWMutex
	wg           sync.WaitGroup
	running      atomic.Bool
}

// New creates a new tunnel manager
func New(cfg *Config, host host.Host) (*TunnelManager, error) {
	ctx, cancel := context.WithCancel(context.Background())

	tm := &TunnelManager{
		ctx:     ctx,
		cancel:  cancel,
		config:  cfg,
		host:    host,
		tunnels: make(map[peer.ID]*TunnelInfo),
	}

	return tm, nil
}

// Start starts the tunnel manager
func (tm *TunnelManager) Start() error {
	if tm.running.Load() {
		return fmt.Errorf("tunnel manager already running")
	}

	log.Info("Starting tunnel manager...")

	// Set up connection gater if provided
	if tm.config.ConnGater != nil {
		tm.host.Network().SetConnectionGater(tm.config.ConnGater)
	}

	// Set up relay if enabled
	if tm.config.EnableRelay {
		tm.setupRelay()
	}

	tm.running.Store(true)
	log.Info("Tunnel manager started successfully")

	return nil
}

// Stop stops the tunnel manager
func (tm *TunnelManager) Stop() error {
	if !tm.running.Load() {
		return nil
	}

	log.Info("Stopping tunnel manager...")
	tm.cancel()
	tm.wg.Wait()

	tm.running.Store(false)
	log.Info("Tunnel manager stopped")

	return nil
}

// setupRelay sets up relay support
func (tm *TunnelManager) setupRelay() {
	// TODO: Implement relay setup using libp2p circuit relay
	log.Info("Relay support enabled")
}

// EstablishTunnel establishes a tunnel to a peer
func (tm *TunnelManager) EstablishTunnel(ctx context.Context, peerID peer.ID) (*TunnelInfo, error) {
	if !tm.running.Load() {
		return nil, fmt.Errorf("tunnel manager not running")
	}

	log.Info("Establishing tunnel", "peer", peerID)

	// Try direct connection first
	if tm.config.EnableHolePunching {
		info, err := tm.tryHolePunch(ctx, peerID)
		if err == nil {
			log.Info("Tunnel established via hole punch", "peer", peerID)
			return info, nil
		}
		log.Debug("Hole punch failed, trying direct", "error", err)
	}

	// Try direct connection
	info, err := tm.tryDirectConnection(ctx, peerID)
	if err == nil {
		log.Info("Tunnel established via direct connection", "peer", peerID)
		return info, nil
	}
	log.Debug("Direct connection failed", "error", err)

	// Try relay connection
	if tm.config.EnableRelay {
		info, err := tm.tryRelayConnection(ctx, peerID)
		if err == nil {
			log.Info("Tunnel established via relay", "peer", peerID)
			return info, nil
		}
		log.Debug("Relay connection failed", "error", err)
	}

	return nil, fmt.Errorf("failed to establish tunnel: all methods failed")
}

// tryHolePunch attempts to establish a direct connection via NAT hole punching
func (tm *TunnelManager) tryHolePunch(ctx context.Context, peerID peer.ID) (*TunnelInfo, error) {
	log.Debug("Attempting hole punch", "peer", peerID)

	// TODO: Implement actual hole punching using:
	// 1. STUN to discover public IP:port
	// 2. Signaling exchange with peer
	// 3. Simultaneous UDP send to punch holes
	// 4. Fall back to TCP if UDP fails

	// For now, return an error to fall back to other methods
	return nil, fmt.Errorf("hole punching not implemented")
}

// tryDirectConnection attempts to establish a direct connection
func (tm *TunnelManager) tryDirectConnection(ctx context.Context, peerID peer.ID) (*TunnelInfo, error) {
	log.Debug("Attempting direct connection", "peer", peerID)

	// Check if already connected
	conns := tm.host.Network().ConnsToPeer(peerID)
	if len(conns) > 0 {
		conn := conns[0]

		info := &TunnelInfo{
			PeerID:         peerID,
			ConnectionType: ConnectionTypeDirect,
			LocalAddr:      conn.LocalMultiaddr().String(),
			RemoteAddr:     conn.RemoteMultiaddr().String(),
			EstablishedAt:  time.Now(),
			LastActive:     time.Now(),
		}
		info.Active.Store(true)

		tm.tunnelsMutex.Lock()
		tm.tunnels[peerID] = info
		tm.tunnelsMutex.Unlock()

		return info, nil
	}

	// Try to connect directly
	err := tm.host.Connect(ctx, peer.AddrInfo{ID: peerID})
	if err != nil {
		return nil, fmt.Errorf("failed to connect directly: %w", err)
	}

	// Get connection
	conns = tm.host.Network().ConnsToPeer(peerID)
	if len(conns) == 0 {
		return nil, fmt.Errorf("connection established but cannot get conn")
	}

	conn := conns[0]
	info := &TunnelInfo{
		PeerID:         peerID,
		ConnectionType: ConnectionTypeDirect,
		LocalAddr:      conn.LocalMultiaddr().String(),
		RemoteAddr:     conn.RemoteMultiaddr().String(),
		EstablishedAt:  time.Now(),
		LastActive:     time.Now(),
	}
	info.Active.Store(true)

	tm.tunnelsMutex.Lock()
	tm.tunnels[peerID] = info
	tm.tunnelsMutex.Unlock()

	return info, nil
}

// tryRelayConnection attempts to establish a connection via relay
func (tm *TunnelManager) tryRelayConnection(ctx context.Context, peerID peer.ID) (*TunnelInfo, error) {
	log.Debug("Attempting relay connection", "peer", peerID)

	// TODO: Implement relay connection using circuit relay protocol
	// This would involve:
	// 1. Finding a relay node
	// 2. Requesting a reservation
	// 3. Establishing connection through relay

	return nil, fmt.Errorf("relay connection not implemented")
}

// CloseTunnel closes a tunnel to a peer
func (tm *TunnelManager) CloseTunnel(peerID peer.ID) error {
	tm.tunnelsMutex.Lock()
	defer tm.tunnelsMutex.Unlock()

	info, ok := tm.tunnels[peerID]
	if !ok {
		return fmt.Errorf("tunnel not found: %s", peerID)
	}

	info.Active.Store(false)

	// Close the connection
	tm.host.Network().ClosePeer(peerID)

	delete(tm.tunnels, peerID)

	log.Info("Tunnel closed", "peer", peerID)

	return nil
}

// GetTunnelInfo returns information about a tunnel
func (tm *TunnelManager) GetTunnelInfo(peerID peer.ID) (*TunnelInfo, error) {
	tm.tunnelsMutex.RLock()
	defer tm.tunnelsMutex.RUnlock()

	info, ok := tm.tunnels[peerID]
	if !ok {
		return nil, fmt.Errorf("tunnel not found: %s", peerID)
	}

	return info, nil
}

// GetAllTunnels returns all active tunnels
func (tm *TunnelManager) GetAllTunnels() map[peer.ID]*TunnelInfo {
	tm.tunnelsMutex.RLock()
	defer tm.tunnelsMutex.RUnlock()

	result := make(map[peer.ID]*TunnelInfo)
	for k, v := range tm.tunnels {
		result[k] = v
	}

	return result
}

// IsTunnelActive checks if a tunnel is active
func (tm *TunnelManager) IsTunnelActive(peerID peer.ID) bool {
	tm.tunnelsMutex.RLock()
	defer tm.tunnelsMutex.RUnlock()

	info, ok := tm.tunnels[peerID]
	if !ok {
		return false
	}

	return info.Active.Load()
}

// UpdateLatency updates the latency for a tunnel
func (tm *TunnelManager) UpdateLatency(peerID peer.ID, latency time.Duration) error {
	tm.tunnelsMutex.Lock()
	defer tm.tunnelsMutex.Unlock()

	info, ok := tm.tunnels[peerID]
	if !ok {
		return fmt.Errorf("tunnel not found: %s", peerID)
	}

	info.Latency = latency
	info.LastActive = time.Now()

	return nil
}

// GetBestTunnel returns the best tunnel based on latency
func (tm *TunnelManager) GetBestTunnel(peerIDs []peer.ID) *TunnelInfo {
	tm.tunnelsMutex.RLock()
	defer tm.tunnelsMutex.RUnlock()

	var best *TunnelInfo
	var bestLatency time.Duration

	for _, pid := range peerIDs {
		info, ok := tm.tunnels[pid]
		if !ok || !info.Active.Load() {
			continue
		}

		if best == nil || info.Latency < bestLatency {
			best = info
			bestLatency = info.Latency
		}
	}

	return best
}

// ConnectionManager returns the connection manager
func (tm *TunnelManager) ConnectionManager() *connmgr.ConnectionManager {
	return tm.config.ConnectionManager
}

// Host returns the libp2p host
func (tm *TunnelManager) Host() host.Host {
	return tm.host
}

// NATManager manages NAT traversal operations
type NATManager struct {
	ctx         context.Context
	cancel      context.CancelFunc
	host        host.Host
	stunServers []string
	wg          sync.WaitGroup
}

// NewNATManager creates a new NAT manager
func NewNATManager(host host.Host) *NATManager {
	ctx, cancel := context.WithCancel(context.Background())

	return &NATManager{
		ctx:    ctx,
		cancel: cancel,
		host:   host,
		stunServers: []string{
			"stun.l.google.com:19302",
			"stun1.l.google.com:19302",
		},
	}
}

// Start starts the NAT manager
func (nm *NATManager) Start() error {
	log.Info("Starting NAT manager...")

	// TODO: Implement NAT detection and port mapping
	// 1. Detect if behind NAT
	// 2. Discover public IP via STUN
	// 3. Set up port mapping via UPnP/NAT-PMP

	log.Info("NAT manager started")

	return nil
}

// Stop stops the NAT manager
func (nm *NATManager) Stop() error {
	log.Info("Stopping NAT manager...")
	nm.cancel()
	nm.wg.Wait()
	log.Info("NAT manager stopped")

	return nil
}

// GetExternalIP returns the external IP address
func (nm *NATManager) GetExternalIP() (net.IP, error) {
	// TODO: Use STUN to get external IP
	return nil, fmt.Errorf("not implemented")
}

// DiscoverNATType discovers the NAT type
func (nm *NATManager) DiscoverNATType() (string, error) {
	// TODO: Implement NAT type detection
	return "unknown", nil
}

// RelayService provides relay functionality for peers behind NATs
type RelayService struct {
	ctx          context.Context
	cancel       context.CancelFunc
	host         host.Host
	reservations map[peer.ID]*RelayReservation
	resMutex     sync.RWMutex
	wg           sync.WaitGroup
}

// RelayReservation holds a relay reservation
type RelayReservation struct {
	PeerID     peer.ID
	RemoteAddr string
	ExpiresAt  time.Time
}

// NewRelayService creates a new relay service
func (rs *RelayService) Start() error {
	log.Info("Starting relay service...")

	// TODO: Implement circuit relay protocol
	// 1. Accept relay reservations from peers
	// 2. Forward traffic between peers
	// 3. Manage reservation lifecycle

	log.Info("Relay service started")

	return nil
}

// Stop stops the relay service
func (rs *RelayService) Stop() error {
	log.Info("Stopping relay service...")
	rs.cancel()
	rs.wg.Wait()
	log.Info("Relay service stopped")

	return nil
}

// Reservation returns a relay reservation
func (rs *RelayService) GetReservation(peerID peer.ID) (*RelayReservation, error) {
	rs.resMutex.RLock()
	defer rs.resMutex.RUnlock()

	res, ok := rs.reservations[peerID]
	if !ok {
		return nil, fmt.Errorf("reservation not found")
	}

	return res, nil
}
