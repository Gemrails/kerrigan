package control

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/kerrigan/kerrigan/pkg/log"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/libp2p/go-libp2p/p2p/security/noise"
	tcp "github.com/libp2p/go-tcp-transport"
	ws "github.com/libp2p/go-ws-transport"
	"github.com/multiformats/go-multiaddr"
)

const (
	// ProtocolID is the libp2p protocol ID for Kerrigan
	ProtocolID = "/kerrigan/1.0.0"
	// HeartbeatInterval is the interval for heartbeat/keepalive
	HeartbeatInterval = 30 * time.Second
	// ConnectionTimeout is the timeout for establishing connections
	ConnectionTimeout = 10 * time.Second
	// MaxPeers is the maximum number of peers to connect
	MaxPeers = 50
)

var (
	// globalHost is the global libp2p host instance
	globalHost host.Host
	hostOnce   sync.Once
	hostMutex  sync.Mutex
)

// Config holds the control plane configuration
type Config struct {
	ListenAddr     string
	BootstrapPeers []string
	Port           int
	KeyPath        string
}

// PeerInfo holds information about a connected peer
type PeerInfo struct {
	ID            peer.ID
	Addr          multiaddr.Multiaddr
	ConnectedAt   time.Time
	LastHeartbeat time.Time
	Protocols     []protocol.ID
}

// ControlPlane is the control plane implementation using libp2p
type ControlPlane struct {
	ctx        context.Context
	cancel     context.CancelFunc
	config     *Config
	host       host.Host
	peers      map[peer.ID]*PeerInfo
	peersMutex sync.RWMutex
	wg         sync.WaitGroup
	running    bool
}

// New creates a new control plane
func New(cfg *Config) (*ControlPlane, error) {
	ctx, cancel := context.WithCancel(context.Background())

	cp := &ControlPlane{
		ctx:    ctx,
		cancel: cancel,
		config: cfg,
		peers:  make(map[peer.ID]*PeerInfo),
	}

	return cp, nil
}

// Start starts the control plane
func (cp *ControlPlane) Start() error {
	if cp.running {
		return fmt.Errorf("control plane already running")
	}

	log.Info("Starting control plane...")

	// Create libp2p host
	host, err := cp.createHost()
	if err != nil {
		return fmt.Errorf("failed to create host: %w", err)
	}
	cp.host = host

	// Set up stream handlers
	cp.setupStreamHandlers()

	// Set up connection handlers
	cp.setupConnectionHandlers()

	// Start heartbeat worker
	cp.wg.Add(1)
	go cp.heartbeatWorker()

	cp.running = true
	log.Info("Control plane started successfully", "listen_addr", cp.config.ListenAddr)

	return nil
}

// Stop stops the control plane
func (cp *ControlPlane) Stop() error {
	if !cp.running {
		return nil
	}

	log.Info("Stopping control plane...")
	cp.cancel()
	cp.wg.Wait()

	if cp.host != nil {
		cp.host.Close()
	}

	cp.running = false
	log.Info("Control plane stopped")

	return nil
}

// createHost creates a new libp2p host
func (cp *ControlPlane) createHost() (host.Host, error) {
	listenAddr := cp.config.ListenAddr
	if listenAddr == "" {
		listenAddr = fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", cp.config.Port)
	}

	// Parse the listen address
	listenMultiaddr, err := multiaddr.NewMultiaddr(listenAddr)
	if err != nil {
		return nil, fmt.Errorf("invalid listen address: %w", err)
	}

	// Build libp2p options
	opts := []libp2p.Option{
		libp2p.ListenAddrs(listenMultiaddr),
		libp2p.Transport(tcp.NewTCPTransport),
		libp2p.Transport(ws.New),
		libp2p.Security(noise.ID, noise.New),
		libp2p.Ping(true),
		libp2p.ConnectionManager(nil), // Will be configured later
	}

	// Create the host
	host, err := libp2p.New(cp.ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create libp2p host: %w", err)
	}

	log.Info("Libp2p host created", "peer_id", host.ID(), "addrs", host.Addrs())

	return host, nil
}

// setupStreamHandlers sets up handlers for incoming streams
func (cp *ControlPlane) setupStreamHandlers() {
	cp.host.SetStreamHandler(protocol.ID(ProtocolID), cp.handleStream)
	cp.host.SetStreamHandler(network.PingProtocol, cp.handlePing)
}

// setupConnectionHandlers sets up handlers for connection events
func (cp *ControlPlane) setupConnectionHandlers() {
	cp.host.Network().Notify(&network.NotifyBundle{
		ConnectedF:    cp.onPeerConnected,
		DisconnectedF: cp.onPeerDisconnected,
	})
}

// handleStream handles incoming streams
func (cp *ControlPlane) handleStream(s network.Stream) {
	log.Debug("Received new stream", "peer", s.Conn().RemotePeer())

	// TODO: Implement protocol negotiation
	// For now, just close the stream after logging
	s.Close()
}

// handlePing handles ping protocol
func (cp *ControlPlane) handlePing(s network.Stream) {
	log.Debug("Received ping", "peer", s.Conn().RemotePeer())
}

// onPeerConnected handles new peer connections
func (cp *ControlPlane) onPeerConnected(net network.Network, conn network.Conn) {
	peerID := conn.RemotePeer()
	addr := conn.RemoteMultiaddr()

	log.Info("Peer connected", "peer", peerID, "addr", addr)

	cp.peersMutex.Lock()
	defer cp.peersMutex.Unlock()

	cp.peers[peerID] = &PeerInfo{
		ID:            peerID,
		Addr:          addr,
		ConnectedAt:   time.Now(),
		LastHeartbeat: time.Now(),
	}
}

// onPeerDisconnected handles peer disconnections
func (cp *ControlPlane) onPeerDisconnected(net network.Network, conn network.Conn) {
	peerID := conn.RemotePeer()

	log.Info("Peer disconnected", "peer", peerID)

	cp.peersMutex.Lock()
	defer cp.peersMutex.Unlock()

	delete(cp.peers, peerID)
}

// heartbeatWorker runs heartbeat/keepalive checks
func (cp *ControlPlane) heartbeatWorker() {
	defer cp.wg.Done()

	ticker := time.NewTicker(HeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-cp.ctx.Done():
			return
		case <-ticker.C:
			cp.checkHeartbeats()
		}
	}
}

// checkHeartbeats checks peer heartbeats and reconnects if needed
func (cp *ControlPlane) checkHeartbeats() {
	cp.peersMutex.RLock()
	defer cp.peersMutex.RUnlock()

	now := time.Now()
	for peerID, info := range cp.peers {
		if now.Sub(info.LastHeartbeat) > HeartbeatInterval*2 {
			log.Warn("Peer heartbeat timeout", "peer", peerID)
			// TODO: Implement reconnection logic
		}
	}
}

// Connect connects to a peer
func (cp *ControlPlane) Connect(ctx context.Context, addr multiaddr.Multiaddr) (peer.ID, error) {
	// Extract peer info from multiaddr
	_, err := peer.AddrInfoFromP2pAddr(addr)
	if err != nil {
		return "", fmt.Errorf("invalid peer address: %w", err)
	}

	// Get the peer ID from the multiaddr
	parts := addr.String()
	var pid peer.ID
	for _, p := range parts {
		if p2p, ok := p.(multiaddr.Component); ok {
			if p2p.Protocol().Code == multiaddr.P_P2P {
				pidStr := p2p.RawValue()
				pid, err = peer.Decode(pidStr)
				if err != nil {
					return "", fmt.Errorf("invalid peer ID: %w", err)
				}
				break
			}
		}
	}

	if pid == "" {
		return "", fmt.Errorf("peer ID not found in address")
	}

	// Connect to the peer
	err = cp.host.Connect(ctx, peer.AddrInfo{
		ID:    pid,
		Addrs: []multiaddr.Multiaddr{addr},
	})
	if err != nil {
		return "", fmt.Errorf("failed to connect: %w", err)
	}

	log.Info("Connected to peer", "peer", pid)

	return pid, nil
}

// ConnectWithAddrInfo connects to a peer using AddrInfo
func (cp *ControlPlane) ConnectWithAddrInfo(ctx context.Context, info peer.AddrInfo) error {
	err := cp.host.Connect(ctx, info)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}

	log.Info("Connected to peer", "peer", info.ID)
	return nil
}

// Disconnect disconnects from a peer
func (cp *ControlPlane) Disconnect(peerID peer.ID) error {
	err := cp.host.Network().ClosePeer(peerID)
	if err != nil {
		return fmt.Errorf("failed to disconnect: %w", err)
	}

	log.Info("Disconnected from peer", "peer", peerID)
	return nil
}

// GetPeerInfo returns information about a connected peer
func (cp *ControlPlane) GetPeerInfo(peerID peer.ID) (*PeerInfo, error) {
	cp.peersMutex.RLock()
	defer cp.peersMutex.RUnlock()

	info, ok := cp.peers[peerID]
	if !ok {
		return nil, fmt.Errorf("peer not found: %s", peerID)
	}

	return info, nil
}

// GetAllPeers returns all connected peers
func (cp *ControlPlane) GetAllPeers() map[peer.ID]*PeerInfo {
	cp.peersMutex.RLock()
	defer cp.peersMutex.RUnlock()

	result := make(map[peer.ID]*PeerInfo)
	for k, v := range cp.peers {
		result[k] = v
	}

	return result
}

// GetHost returns the libp2p host
func (cp *ControlPlane) GetHost() host.Host {
	return cp.host
}

// GetPeerID returns the local peer ID
func (cp *ControlPlane) GetPeerID() peer.ID {
	if cp.host == nil {
		return ""
	}
	return cp.host.ID()
}

// GetListenAddrs returns the listen addresses
func (cp *ControlPlane) GetListenAddrs() []multiaddr.Multiaddr {
	if cp.host == nil {
		return nil
	}
	return cp.host.Addrs()
}

// OpenStream opens a new stream to a peer
func (cp *ControlPlane) OpenStream(ctx context.Context, peerID peer.ID) (network.Stream, error) {
	stream, err := cp.host.NewStream(ctx, peerID, protocol.ID(ProtocolID))
	if err != nil {
		return nil, fmt.Errorf("failed to open stream: %w", err)
	}

	log.Debug("Opened stream", "peer", peerID)
	return stream, nil
}

// SendMessage sends a message to a peer
func (cp *ControlPlane) SendMessage(ctx context.Context, peerID peer.ID, data []byte) error {
	stream, err := cp.OpenStream(ctx, peerID)
	if err != nil {
		return err
	}
	defer stream.Close()

	_, err = stream.Write(data)
	if err != nil {
		return fmt.Errorf("failed to write message: %w", err)
	}

	return nil
}

// IsConnected checks if connected to a peer
func (cp *ControlPlane) IsConnected(peerID peer.ID) bool {
	cp.peersMutex.RLock()
	defer cp.peersMutex.RUnlock()

	_, ok := cp.peers[peerID]
	return ok
}

// GetPort returns the listening port
func (cp *ControlPlane) GetPort() int {
	if cp.host == nil {
		return 0
	}

	for _, addr := range cp.host.Addrs() {
		_, port, err := net.SplitHostPort(addr.String())
		if err == nil {
			if port != "" {
				// Return the control port from config
				return cp.config.Port
			}
		}
	}

	return cp.config.Port
}
