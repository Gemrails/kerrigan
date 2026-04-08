package data

import (
	"context"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/kerrigan/kerrigan/pkg/log"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/xtaci/kcp-go/v5"
	"golang.org/x/crypto/chacha20"
)

const (
	// DefaultPort is the default KCP data port
	DefaultPort = 38889
	// BufferSize is the default buffer size
	BufferSize = 1024 * 1024 // 1MB
	// MaxConnections is the maximum number of KCP connections
	MaxConnections = 100
	// KeepAliveInterval is the keepalive interval
	KeepAliveInterval = 10 * time.Second
	// ConnectionTimeout is the connection timeout
	ConnectionTimeout = 30 * time.Second
	// PoolSize is the connection pool size
	PoolSize = 10
)

// Config holds the data plane configuration
type Config struct {
	ListenAddr    string
	Port          int
	EncryptionKey []byte
	KCPMode       string // fast, normal, manual
	KCPParams     *KCPParams
}

// KCPParams holds KCP-specific parameters
type KCPParams struct {
	// SMUX window size
	WindowSize int
	// ACK ratio
	AckRatio int
	// MTU size
	MTU int
	// Retransmission timeout in ms
	Timeout int
	// No congestion control
	NoCongestion bool
	// Underlying socket buffer size
	SndBuf int
	// Underlying socket buffer size
	RcvBuf int
	// Keepalive interval
	KeepAlive int
	// Cryptography algorithm
	Crypt string
}

// DefaultKCPParams returns default KCP parameters
func DefaultKCPParams() *KCPParams {
	return &KCPParams{
		WindowSize:   128,
		AckRatio:     2,
		MTU:          1400,
		Timeout:      3000,
		NoCongestion: true,
		SndBuf:       16777216,
		RcvBuf:       16777216,
		KeepAlive:    10,
		Crypt:        "chacha20",
	}
}

// ConnectionInfo holds information about a KCP connection
type ConnectionInfo struct {
	PeerID      peer.ID
	RemoteAddr  string
	LocalAddr   string
	ConnectedAt time.Time
	LastActive  time.Time
	BytesSent   atomic.Int64
	BytesRecv   atomic.Int64
	Active      atomic.Bool
}

// KCPConn wraps a KCP connection with metadata
type KCPConn struct {
	conn   *kcp.UDPSession
	info   *ConnectionInfo
	cipher *chacha20.Cipher
}

// DataPlane is the data plane implementation using KCP
type DataPlane struct {
	ctx        context.Context
	cancel     context.CancelFunc
	config     *Config
	listener   *kcp.Listener
	conns      map[peer.ID]*KCPConn
	connsMutex sync.RWMutex
	pool       *ConnectionPool
	wg         sync.WaitGroup
	running    atomic.Bool
	stats      Stats
}

// Stats holds data plane statistics
type Stats struct {
	TotalConnections  atomic.Int64
	ActiveConnections atomic.Int64
	TotalBytesSent    atomic.Int64
	TotalBytesRecv    atomic.Int64
	TotalPacketsSent  atomic.Int64
	TotalPacketsRecv  atomic.Int64
}

// ConnectionPool manages a pool of connections to a peer
type ConnectionPool struct {
	mu      sync.Mutex
	pools   map[peer.ID][]*kcp.UDPSession
	active  map[peer.ID]*kcp.UDPSession
	maxSize int
}

// New creates a new data plane
func New(cfg *Config) (*DataPlane, error) {
	ctx, cancel := context.WithCancel(context.Background())

	dp := &DataPlane{
		ctx:    ctx,
		cancel: cancel,
		config: cfg,
		conns:  make(map[peer.ID]*KCPConn),
	}

	// Set default KCP params if not provided
	if dp.config.KCPParams == nil {
		dp.config.KCPParams = DefaultKCPParams()
	}

	// Initialize connection pool
	dp.pool = &ConnectionPool{
		pools:   make(map[peer.ID][]*kcp.UDPSession),
		active:  make(map[peer.ID]*kcp.UDPSession),
		maxSize: PoolSize,
	}

	return dp, nil
}

// Start starts the data plane
func (dp *DataPlane) Start() error {
	if dp.running.Load() {
		return fmt.Errorf("data plane already running")
	}

	log.Info("Starting data plane...")

	// Parse listen address
	listenAddr := dp.config.ListenAddr
	if listenAddr == "" {
		listenAddr = fmt.Sprintf(":%d", dp.config.Port)
	}

	// Create KCP listener
	listener, err := kcp.Listen(listenAddr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", listenAddr, err)
	}
	dp.listener = listener

	// Set up the listener with KCP parameters
	err = dp.setupListener()
	if err != nil {
		dp.listener.Close()
		return fmt.Errorf("failed to setup listener: %w", err)
	}

	// Start accept loop
	dp.wg.Add(1)
	go dp.acceptLoop()

	// Start stats collector
	dp.wg.Add(1)
	go dp.statsCollector()

	dp.running.Store(true)
	log.Info("Data plane started successfully", "listen_addr", listenAddr)

	return nil
}

// setupListener configures the KCP listener with parameters
func (dp *DataPlane) setupListener() error {
	if dp.listener == nil {
		return fmt.Errorf("listener not initialized")
	}

	// KCP parameters are set via SetReadDeadline/SetWriteDeadline for tuning
	// The actual parameters are handled when creating sessions
	return nil
}

// acceptLoop accepts incoming KCP connections
func (dp *DataPlane) acceptLoop() {
	defer dp.wg.Done()

	for {
		select {
		case <-dp.ctx.Done():
			return
		default:
		}

		// Set accept deadline
		dp.listener.SetReadDeadline(time.Now().Add(1 * time.Second))

		conn, err := dp.listener.Accept()
		if err != nil {
			if dp.ctx.Err() != nil {
				return
			}
			// Timeout is expected, continue
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}
			log.Error("Failed to accept connection", "error", err)
			continue
		}

		// Handle the new connection
		dp.wg.Add(1)
		go dp.handleConnection(conn)
	}
}

// handleConnection handles a new KCP connection
func (dp *DataPlane) handleConnection(conn net.Conn) {
	defer dp.wg.Done()

	// Get remote address
	remoteAddr := conn.RemoteAddr().String()

	log.Info("Accepted new KCP connection", "remote_addr", remoteAddr)

	// Update stats
	dp.stats.TotalConnections.Add(1)
	dp.stats.ActiveConnections.Add(1)
	defer dp.stats.ActiveConnections.Add(-1)

	// TODO: Handle the connection data
	// This would typically involve:
	// 1. Performing handshake with peer ID exchange
	// 2. Setting up encryption
	// 3. Starting data transfer

	conn.Close()
}

// Stop stops the data plane
func (dp *DataPlane) Stop() error {
	if !dp.running.Load() {
		return nil
	}

	log.Info("Stopping data plane...")
	dp.cancel()
	dp.wg.Wait()

	if dp.listener != nil {
		dp.listener.Close()
	}

	// Close all connections
	dp.connsMutex.Lock()
	for _, kcpConn := range dp.conns {
		if kcpConn.conn != nil {
			kcpConn.conn.Close()
		}
	}
	dp.conns = make(map[peer.ID]*KCPConn)
	dp.connsMutex.Unlock()

	dp.running.Store(false)
	log.Info("Data plane stopped")

	return nil
}

// Connect connects to a peer via KCP
func (dp *DataPlane) Connect(ctx context.Context, peerID peer.ID, addr string) (*ConnectionInfo, error) {
	if !dp.running.Load() {
		return nil, fmt.Errorf("data plane not running")
	}

	// Get connection from pool or create new
	conn, err := dp.getOrCreateConnection(ctx, peerID, addr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}

	info := &ConnectionInfo{
		PeerID:      peerID,
		RemoteAddr:  addr,
		LocalAddr:   conn.LocalAddr().String(),
		ConnectedAt: time.Now(),
		LastActive:  time.Now(),
	}
	info.Active.Store(true)

	// Store connection
	dp.connsMutex.Lock()
	dp.conns[peerID] = &KCPConn{
		conn: conn,
		info: info,
	}
	dp.connsMutex.Unlock()

	log.Info("Connected to peer via KCP", "peer", peerID, "addr", addr)

	return info, nil
}

// getOrCreateConnection gets or creates a KCP connection
func (dp *DataPlane) getOrCreateConnection(ctx context.Context, peerID peer.ID, addr string) (*kcp.UDPSession, error) {
	// Try to get from pool first
	dp.pool.mu.Lock()
	if pool, ok := dp.pool.pools[peerID]; ok && len(pool) > 0 {
		conn := pool[len(pool)-1]
		dp.pool.pools[peerID] = pool[:len(pool)-1]
		dp.pool.active[peerID] = conn
		dp.pool.mu.Unlock()
		return conn, nil
	}
	dp.pool.mu.Unlock()

	// Create new connection
	conn, err := kcp.Dial(addr)
	if err != nil {
		return nil, fmt.Errorf("failed to dial: %w", err)
	}

	// Apply KCP parameters
	err = dp.applyKCPParams(conn)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to apply KCP params: %w", err)
	}

	// Set up encryption if key is provided
	if len(dp.config.EncryptionKey) > 0 {
		cipher, err := chacha20.NewCipher(dp.config.EncryptionKey, make([]byte, 12))
		if err != nil {
			conn.Close()
			return nil, fmt.Errorf("failed to create cipher: %w", err)
		}
		_ = cipher // TODO: Use for encryption/decryption
	}

	return conn, nil
}

// applyKCPParams applies KCP parameters to a session
func (dp *DataPlane) applyKCPParams(conn *kcp.UDPSession) error {
	if conn == nil || dp.config.KCPParams == nil {
		return nil
	}

	params := dp.config.KCPParams

	// Set parameters through KCP control
	kcper := conn.GetKCP()
	if kcper != nil {
		kcper.WndSize(params.WindowSize, params.WindowSize)
		kcper.SetMtu(uint32(params.MTU))
		kcper.SetTimeout(params.Timeout)
		kcper.NoDelay(!params.NoCongestion, 0, params.AckRatio, 0)
	}

	return nil
}

// Disconnect disconnects from a peer
func (dp *DataPlane) Disconnect(peerID peer.ID) error {
	dp.connsMutex.Lock()
	defer dp.connsMutex.Unlock()

	kcpConn, ok := dp.conns[peerID]
	if !ok {
		return fmt.Errorf("connection not found")
	}

	// Return to pool if not full
	dp.pool.mu.Lock()
	if len(dp.pool.pools[peerID]) < dp.pool.maxSize {
		dp.pool.pools[peerID] = append(dp.pool.pools[peerID], kcpConn.conn)
	} else {
		kcpConn.conn.Close()
	}
	delete(dp.pool.active, peerID)
	dp.pool.mu.Unlock()

	delete(dp.conns, peerID)

	log.Info("Disconnected from peer", "peer", peerID)

	return nil
}

// Send sends data to a peer
func (dp *DataPlane) Send(ctx context.Context, peerID peer.ID, data []byte) (int, error) {
	dp.connsMutex.RLock()
	kcpConn, ok := dp.conns[peerID]
	dp.connsMutex.RUnlock()

	if !ok {
		return 0, fmt.Errorf("connection not found: %s", peerID)
	}

	if !kcpConn.conn.IsConnected() {
		return 0, fmt.Errorf("connection not connected")
	}

	n, err := kcpConn.conn.Write(data)
	if err != nil {
		return n, fmt.Errorf("failed to write: %w", err)
	}

	kcpConn.info.BytesSent.Add(int64(n))
	kcpConn.info.LastActive = time.Now()
	dp.stats.TotalBytesSent.Add(int64(n))
	dp.stats.TotalPacketsSent.Add(1)

	return n, nil
}

// Receive receives data from a peer
func (dp *DataPlane) Receive(ctx context.Context, peerID peer.ID, buf []byte) (int, error) {
	dp.connsMutex.RLock()
	kcpConn, ok := dp.conns[peerID]
	dp.connsMutex.RUnlock()

	if !ok {
		return 0, fmt.Errorf("connection not found: %s", peerID)
	}

	if !kcpConn.conn.IsConnected() {
		return 0, fmt.Errorf("connection not connected")
	}

	n, err := kcpConn.conn.Read(buf)
	if err != nil {
		return n, fmt.Errorf("failed to read: %w", err)
	}

	kcpConn.info.BytesRecv.Add(int64(n))
	kcpConn.info.LastActive = time.Now()
	dp.stats.TotalBytesRecv.Add(int64(n))
	dp.stats.TotalPacketsRecv.Add(1)

	return n, nil
}

// GetConnectionInfo returns information about a connection
func (dp *DataPlane) GetConnectionInfo(peerID peer.ID) (*ConnectionInfo, error) {
	dp.connsMutex.RLock()
	defer dp.connsMutex.RUnlock()

	kcpConn, ok := dp.conns[peerID]
	if !ok {
		return nil, fmt.Errorf("connection not found: %s", peerID)
	}

	return kcpConn.info, nil
}

// GetAllConnections returns all active connections
func (dp *DataPlane) GetAllConnections() map[peer.ID]*ConnectionInfo {
	dp.connsMutex.RLock()
	defer dp.connsMutex.RUnlock()

	result := make(map[peer.ID]*ConnectionInfo)
	for peerID, kcpConn := range dp.conns {
		result[peerID] = kcpConn.info
	}

	return result
}

// IsConnected checks if connected to a peer
func (dp *DataPlane) IsConnected(peerID peer.ID) bool {
	dp.connsMutex.RLock()
	defer dp.connsMutex.RUnlock()

	kcpConn, ok := dp.conns[peerID]
	if !ok {
		return false
	}

	return kcpConn.conn.IsConnected()
}

// statsCollector collects and logs statistics
func (dp *DataPlane) statsCollector() {
	defer dp.wg.Done()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-dp.ctx.Done():
			return
		case <-ticker.C:
			dp.logStats()
		}
	}
}

// logLogs logs current statistics
func (dp *DataPlane) logStats() {
	log.Debug("Data plane stats",
		"total_connections", dp.stats.TotalConnections.Load(),
		"active_connections", dp.stats.ActiveConnections.Load(),
		"total_bytes_sent", dp.stats.TotalBytesSent.Load(),
		"total_bytes_recv", dp.stats.TotalBytesRecv.Load(),
	)
}

// GetStats returns current statistics
func (dp *DataPlane) GetStats() Stats {
	return Stats{
		TotalConnections:  dp.stats.TotalConnections,
		ActiveConnections: dp.stats.ActiveConnections,
		TotalBytesSent:    dp.stats.TotalBytesSent,
		TotalBytesRecv:    dp.stats.TotalBytesRecv,
	}
}

// GetPort returns the listening port
func (dp *DataPlane) GetPort() int {
	if dp.listener == nil {
		return 0
	}

	addr := dp.listener.Addr().String()
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		return 0
	}

	if port == "" {
		return dp.config.Port
	}

	var p int
	fmt.Sscanf(port, "%d", &p)
	return p
}
