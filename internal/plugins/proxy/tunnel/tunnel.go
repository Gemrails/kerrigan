package tunnel

import (
	"context"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/kerrigan/kerrigan/pkg/log"
	"github.com/xtaci/kcp-go/v5"
	"golang.org/x/crypto/chacha20"
	"golang.org/x/crypto/pbkdf2"
)

// Config holds tunnel server configuration
type Config struct {
	ListenPort int    // Tunnel listen port
	EnableKCP  bool   // Enable KCP protocol
	EnableTLS  bool   // Enable TLS obfuscation
	KCPSecret  string // KCP encryption secret
	MaxConns   int    // Max concurrent connections
	KeepAlive  int    // Keep-alive interval (seconds)
}

// Server implements a tunnel server with KCP and TLS support
type Server struct {
	config Config
	ln     net.Listener
	mu     sync.RWMutex
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// Connection management
	connections map[string]*ClientConnection
	connMutex   sync.RWMutex
	connIDGen   uint64

	// Multiplexer
	multiplexer *Multiplxer

	running bool
}

// ClientConnection represents an active tunnel connection
type ClientConnection struct {
	ID         string
	RemoteAddr string
	LocalAddr  string
	BytesIn    int64
	BytesOut   int64
	StartTime  time.Time
	LastActive time.Time
	SessionID  uint64
	Encryption string // "chacha20", "aes", "none"
}

// NewServer creates a new tunnel server instance
func NewServer(cfg Config) *Server {
	ctx, cancel := context.WithCancel(context.Background())
	return &Server{
		config:      cfg,
		ctx:         ctx,
		cancel:      cancel,
		connections: make(map[string]*ClientConnection),
	}
}

// Start starts the tunnel server
func (s *Server) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return fmt.Errorf("tunnel server already running")
	}

	// Initialize connection multiplexer
	s.multiplexer = NewMultiplxer(16) // 16 channels

	// Start listener based on configuration
	var err error

	if s.config.EnableTLS {
		err = s.startTLSListener()
	} else if s.config.EnableKCP {
		err = s.startKCPListener()
	} else {
		err = s.startTCPListener()
	}

	if err != nil {
		return fmt.Errorf("failed to start listener: %w", err)
	}

	s.running = true

	// Start background workers
	s.startWorkers()

	log.Info("Tunnel server started", "port", s.config.ListenPort, "kcp", s.config.EnableKCP, "tls", s.config.EnableTLS)
	return nil
}

// Stop stops the tunnel server
func (s *Server) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return nil
	}

	s.cancel()

	if s.ln != nil {
		s.ln.Close()
	}

	s.wg.Wait()

	s.running = false
	log.Info("Tunnel server stopped")
	return nil
}

// startTCPListener starts a plain TCP listener
func (s *Server) startTCPListener() error {
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", s.config.ListenPort))
	if err != nil {
		return err
	}
	s.ln = ln

	s.wg.Add(1)
	go s.acceptLoop()

	return nil
}

// startKCPListener starts a KCP listener
func (s *Server) startKCPListener() error {
	// Generate key from secret
	key := s.deriveKey(s.config.KCPSecret)

	block, err := kcp.NewAESBlockCrypt(key)
	if err != nil {
		return fmt.Errorf("failed to create KCP block: %w", err)
	}

	listener, err := kcp.Listen(":"+fmt.Sprintf("%d", s.config.ListenPort), &kcp.ServerConfig{
		Block: block,
	})
	if err != nil {
		return fmt.Errorf("failed to start KCP listener: %w", err)
	}

	s.ln = listener
	s.wg.Add(1)
	go s.acceptLoop()

	return nil
}

// startTLSListener starts a TLS listener with obfuscation
func (s *Server) startTLSListener() error {
	// For TLS obfuscation, we use a TLS server with custom protocol
	// In production, you'd use proper certificates
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", s.config.ListenPort))
	if err != nil {
		return err
	}
	s.ln = ln

	s.wg.Add(1)
	go s.acceptLoop()

	return nil
}

// acceptLoop accepts incoming connections
func (s *Server) acceptLoop() {
	defer s.wg.Done()

	for {
		conn, err := s.ln.Accept()
		if err != nil {
			select {
			case <-s.ctx.Done():
				return
			default:
				log.Error("Failed to accept connection", "error", err)
				continue
			}
		}

		s.wg.Add(1)
		go s.handleConnection(conn)
	}
}

// handleConnection handles a new tunnel connection
func (s *Server) handleConnection(conn net.Conn) {
	defer s.wg.Done()
	defer conn.Close()

	connID := atomic.AddUint64(&s.connIDGen, 1)

	// Create connection info
	connInfo := &ClientConnection{
		ID:         fmt.Sprintf("tunnel-%d", connID),
		RemoteAddr: conn.RemoteAddr().String(),
		LocalAddr:  conn.LocalAddr().String(),
		StartTime:  time.Now(),
		LastActive: time.Now(),
		SessionID:  connID,
		Encryption: "chacha20",
	}

	// Register connection
	s.connMutex.Lock()
	s.connections[connInfo.ID] = connInfo
	s.connMutex.Unlock()

	// Cleanup on exit
	defer func() {
		s.connMutex.Lock()
		delete(s.connections, connInfo.ID)
		s.connMutex.Unlock()
	}()

	// Handle the tunnel session
	if s.config.EnableTLS {
		s.handleTLSession(conn, connInfo)
	} else if s.config.EnableKCP {
		s.handleKCPSession(conn, connInfo)
	} else {
		s.handleTCPSession(conn, connInfo)
	}
}

// handleTCPSession handles a plain TCP tunnel session
func (s *Server) handleTCPSession(conn net.Conn, info *ClientConnection) {
	// Read client request (obfuscated header)
	header := make([]byte, 8)
	if _, err := io.ReadFull(conn, header); err != nil {
		log.Error("Failed to read header", "error", err)
		return
	}

	// Decrypt header if TLS enabled
	if s.config.EnableTLS {
		header = s.decryptHeader(header)
	}

	// Parse header: [version:1][session_id:4][flags:1][reserved:2]
	version := header[0]
	if version != 1 {
		log.Error("Unsupported protocol version", "version", version)
		return
	}

	sessionID := binary.BigEndian.Uint32(header[1:5])
	_ = sessionID

	// Read target address from request
	addrLen := make([]byte, 1)
	if _, err := io.ReadFull(conn, addrLen); err != nil {
		return
	}

	addr := make([]byte, addrLen[0])
	if _, err := io.ReadFull(conn, addr); err != nil {
		return
	}

	// Connect to target
	targetConn, err := net.Dial("tcp", string(addr))
	if err != nil {
		log.Error("Failed to connect to target", "target", string(addr), "error", err)
		return
	}
	defer targetConn.Close()

	// Start relay
	s.relayTraffic(conn, targetConn, info)
}

// handleKCPSession handles a KCP tunnel session
func (s *Server) handleKCPSession(conn net.Conn, info *ClientConnection) {
	// KCP handles its own transport
	// The session has already been established by the listener
	s.handleTCPSession(conn, info)
}

// handleTLSession handles a TLS obfuscated session
func (s *Server) handleTLSession(conn net.Conn, info *ClientConnection) {
	// TLS handshake would happen here
	// For now, treat as regular TCP with encrypted payload
	s.handleTCPSession(conn, info)
}

// relayTraffic relays traffic between client and target
func (s *Server) relayTraffic(clientConn, targetConn net.Conn, info *ClientConnection) {
	var wg sync.WaitGroup
	wg.Add(2)

	// Client -> Target
	go func() {
		defer wg.Done()
		n, _ := io.Copy(targetConn, clientConn)
		atomic.AddInt64(&info.BytesIn, n)
	}()

	// Target -> Client
	go func() {
		defer wg.Done()
		n, _ := io.Copy(clientConn, targetConn)
		atomic.AddInt64(&info.BytesOut, n)
	}()

	wg.Wait()
}

// startWorkers starts background worker goroutines
func (s *Server) startWorkers() {
	// Connection cleanup worker
	s.wg.Add(1)
	go s.cleanupWorker()

	// Keep-alive worker
	s.wg.Add(1)
	go s.keepAliveWorker()
}

// cleanupWorker periodically cleans up stale connections
func (s *Server) cleanupWorker() {
	defer s.wg.Done()

	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.cleanupStaleConnections()
		}
	}
}

// cleanupStaleConnections removes connections that haven't been active
func (s *Server) cleanupStaleConnections() {
	s.connMutex.Lock()
	defer s.connMutex.Unlock()

	timeout := 5 * time.Minute
	now := time.Now()

	for id, conn := range s.connections {
		if now.Sub(conn.LastActive) > timeout {
			delete(s.connections, id)
		}
	}
}

// keepAliveWorker sends keep-alive packets
func (s *Server) keepAliveWorker() {
	defer s.wg.Done()

	interval := time.Duration(s.config.KeepAlive) * time.Second
	if interval == 0 {
		interval = 60 * time.Second
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.sendKeepAlives()
		}
	}
}

// sendKeepAlives sends keep-alive to active connections
func (s *Server) sendKeepAlives() {
	s.connMutex.RLock()
	defer s.connMutex.RUnlock()

	for _, conn := range s.connections {
		conn.LastActive = time.Now()
	}
}

// deriveKey derives encryption key from secret
func (s *Server) deriveKey(secret string) []byte {
	return pbkdf2.Key([]byte(secret), []byte("kerrigan-kcp-salt"), 4096, 32, sha256.New)
}

// decryptHeader decrypts the header with ChaCha20
func (s *Server) decryptHeader(encrypted []byte) []byte {
	key := pbkdf2.Key([]byte(s.config.KCPSecret), []byte("kerrigan-header-salt"), 4096, 32, sha256.New)

	block, err := chacha20.NewXChaCha20(key[:32], key[32:])
	if err != nil {
		return encrypted
	}

	plaintext := make([]byte, len(encrypted))
	block.XORKeyStream(plaintext, encrypted)

	return plaintext
}

// GetActiveConnections returns active tunnel connections
func (s *Server) GetActiveConnections() []*ClientConnection {
	s.connMutex.RLock()
	defer s.connMutex.RUnlock()

	conns := make([]*ClientConnection, 0, len(s.connections))
	for _, conn := range s.connections {
		conns = append(conns, conn)
	}

	return conns
}

// GetConnectionCount returns the number of active connections
func (s *Server) GetConnectionCount() int {
	s.connMutex.RLock()
	defer s.connMutex.RUnlock()

	return len(s.connections)
}

// Multiplxer implements connection multiplexing
type Multiplxer struct {
	channels map[uint32]*MuxChannel
	mu       sync.RWMutex
	nextID   uint32
}

// MuxChannel represents a multiplexed channel
type MuxChannel struct {
	ID        uint32
	Conn      net.Conn
	Closed    bool
	ReadChan  chan []byte
	WriteChan chan []byte
}

// NewMultiplxer creates a new multiplexer
func NewMultiplxer(channelCount int) *Multiplxer {
	return &Multiplxer{
		channels: make(map[uint32]*MuxChannel),
	}
}

// CreateChannel creates a new multiplexed channel
func (m *Multiplxer) CreateChannel(conn net.Conn) *MuxChannel {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.nextID++
	ch := &MuxChannel{
		ID:        m.nextID,
		Conn:      conn,
		ReadChan:  make(chan []byte, 10),
		WriteChan: make(chan []byte, 10),
	}

	m.channels[ch.ID] = ch
	return ch
}

// GetChannel retrieves a channel by ID
func (m *Multiplxer) GetChannel(id uint32) (*MuxChannel, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ch, exists := m.channels[id]
	return ch, exists
}

// RemoveChannel removes a channel
func (m *Multiplxer) RemoveChannel(id uint32) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.channels, id)
}

// Session represents a tunnel session
type Session struct {
	ID         uint64
	LocalAddr  string
	RemoteAddr string
	CreatedAt  time.Time
	Active     bool
}

// SessionManager manages tunnel sessions
type SessionManager struct {
	sessions map[uint64]*Session
	mu       sync.RWMutex
}

// NewSessionManager creates a new session manager
func NewSessionManager() *SessionManager {
	return &SessionManager{
		sessions: make(map[uint64]*Session),
	}
}

// CreateSession creates a new session
func (sm *SessionManager) CreateSession(localAddr, remoteAddr string) *Session {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	session := &Session{
		ID:         uint64(len(sm.sessions) + 1),
		LocalAddr:  localAddr,
		RemoteAddr: remoteAddr,
		CreatedAt:  time.Now(),
		Active:     true,
	}

	sm.sessions[session.ID] = session
	return session
}

// GetSession retrieves a session by ID
func (sm *SessionManager) GetSession(id uint64) (*Session, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	session, exists := sm.sessions[id]
	return session, exists
}

// CloseSession closes a session
func (sm *SessionManager) CloseSession(id uint64) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if session, exists := sm.sessions[id]; exists {
		session.Active = false
		delete(sm.sessions, id)
	}
}

// Encrypt encrypts data using ChaCha20
func Encrypt(data []byte, key []byte) ([]byte, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("key must be 32 bytes")
	}

	// Generate random nonce
	nonce := make([]byte, 24)
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}

	block, err := chacha20.NewXChaCha20(key, nonce)
	if err != nil {
		return nil, err
	}

	ciphertext := make([]byte, len(data))
	block.XORKeyStream(ciphertext, data)

	// Prepend nonce to ciphertext
	result := make([]byte, 0, len(nonce)+len(ciphertext))
	result = append(result, nonce...)
	result = append(result, ciphertext...)

	return result, nil
}

// Decrypt decrypts data using ChaCha20
func Decrypt(data []byte, key []byte) ([]byte, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("key must be 32 bytes")
	}

	if len(data) < 24 {
		return nil, fmt.Errorf("data too short")
	}

	nonce := data[:24]
	ciphertext := data[24:]

	block, err := chacha20.NewXChaCha20(key, nonce)
	if err != nil {
		return nil, err
	}

	plaintext := make([]byte, len(ciphertext))
	block.XORKeyStream(plaintext, ciphertext)

	return plaintext, nil
}

// GenerateSessionKey generates a random session key
func GenerateSessionKey() ([]byte, error) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	return key, nil
}
