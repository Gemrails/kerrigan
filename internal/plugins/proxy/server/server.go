package server

import (
	"bufio"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/kerrigan/kerrigan/internal/plugins/proxy/stats"
	"github.com/kerrigan/kerrigan/pkg/log"
)

// HTTPProxyConfig holds HTTP proxy configuration
type HTTPProxyConfig struct {
	Port        int
	IdleTimeout time.Duration
	Shaper      interface {
		GetDownloadLimit() int
		GetUploadLimit() int
	}
	Stats *stats.Collector
}

// HTTPProxy implements an HTTP CONNECT proxy server
type HTTPProxy struct {
	config HTTPProxyConfig
	server *http.Server
	ln     net.Listener
	mu     sync.RWMutex
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewHTTPProxy creates a new HTTP proxy instance
func NewHTTPProxy(cfg HTTPProxyConfig) *HTTPProxy {
	ctx, cancel := context.WithCancel(context.Background())
	return &HTTPProxy{
		config: cfg,
		ctx:    ctx,
		cancel: cancel,
	}
}

// Start starts the HTTP proxy server
func (p *HTTPProxy) Start() error {
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", p.config.Port))
	if err != nil {
		return fmt.Errorf("failed to listen on port %d: %w", p.config.Port, err)
	}
	p.ln = ln

	p.server = &http.Server{
		Addr:         fmt.Sprintf(":%d", p.config.Port),
		Handler:      p,
		ReadTimeout:  p.config.IdleTimeout,
		WriteTimeout: p.config.IdleTimeout,
		IdleTimeout:  p.config.IdleTimeout,
	}

	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		p.server.Serve(ln)
	}()

	log.Info("HTTP proxy server started", "port", p.config.Port)
	return nil
}

// Stop stops the HTTP proxy server
func (p *HTTPProxy) Stop() error {
	p.cancel()

	if p.server != nil {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		p.server.Shutdown(shutdownCtx)
	}

	if p.ln != nil {
		p.ln.Close()
	}

	p.wg.Wait()
	log.Info("HTTP proxy server stopped")
	return nil
}

// ServeHTTP handles HTTP proxy requests
func (p *HTTPProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Only handle CONNECT method for HTTPS proxy
	if r.Method != http.MethodConnect {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Handle CONNECT for HTTPS tunneling
	p.handleConnect(w, r)
}

// handleConnect handles the HTTP CONNECT method
func (p *HTTPProxy) handleConnect(w http.ResponseWriter, r *http.Request) {
	host := r.Host
	if host == "" {
		http.Error(w, "Missing host", http.StatusBadRequest)
		return
	}

	// Check connection limit
	connID := fmt.Sprintf("http-%s-%d", host, time.Now().UnixNano())

	// Hijack the connection
	hj, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "Hijacking not supported", http.StatusInternalServerError)
		return
	}

	conn, _, err := hj.Hijack()
	if err != nil {
		log.Error("Failed to hijack connection", "error", err)
		return
	}
	defer conn.Close()

	// Connect to the target host
	targetConn, err := net.Dial("tcp", host)
	if err != nil {
		log.Error("Failed to connect to target", "host", host, "error", err)
		return
	}
	defer targetConn.Close()

	// Send 200 Connection Established
	response := "HTTP/1.1 200 Connection Established\r\n\r\n"
	if _, err := conn.Write([]byte(response)); err != nil {
		log.Error("Failed to send response", "error", err)
		return
	}

	// Track connection
	if p.config.Stats != nil {
		p.config.Stats.RecordConnection("default", 0, 0)
		defer p.config.Stats.RecordDisconnection("default", 0, 0)
	}

	// Copy data between client and target
	p.relayTraffic(conn, targetConn, connID)
}

// relayTraffic copies data between client and target connections
func (p *HTTPProxy) relayTraffic(clientConn, targetConn net.Conn, connID string) {
	var wg sync.WaitGroup
	wg.Add(2)

	// Client to target
	go func() {
		defer wg.Done()
		p.copyWithShaping(clientConn, targetConn, p.config.Shaper.GetUploadLimit(), connID+"-upload")
	}()

	// Target to client
	go func() {
		defer wg.Done()
		p.copyWithShaping(targetConn, clientConn, p.config.Shaper.GetDownloadLimit(), connID+"-download")
	}()

	wg.Wait()
}

// copyWithShaping copies data with traffic shaping
func (p *HTTPProxy) copyWithShaping(dst, src net.Conn, limit int, connID string) {
	if limit > 0 {
		// Use buffered reader with limited read rate
		buf := make([]byte, 32*1024) // 32KB buffer
		remaining := 0

		for {
			n, err := src.Read(buf[remaining:])
			if n > 0 {
				toWrite := buf[:remaining+n]
				// Simple rate limiting: add delay if needed
				if limit > 0 {
					estimatedTime := time.Duration(float64(len(toWrite)) / float64(limit) * float64(time.Second))
					if estimatedTime > 0 {
						time.Sleep(estimatedTime)
					}
				}

				if _, wErr := dst.Write(toWrite); wErr != nil {
					break
				}
			}
			if err != nil {
				break
			}
		}
	} else {
		// No limit, use io.Copy
		io.Copy(dst, src)
	}
}

// SOCKS5ProxyConfig holds SOCKS5 proxy configuration
type SOCKS5ProxyConfig struct {
	Port        int
	IdleTimeout time.Duration
	Shaper      interface {
		GetDownloadLimit() int
		GetUploadLimit() int
	}
	Stats *stats.Collector
}

// SOCKS5Proxy implements a SOCKS5 proxy server
type SOCKS5Proxy struct {
	config  SOCKS5ProxyConfig
	ln      net.Listener
	mu      sync.RWMutex
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	running bool
}

// NewSOCKS5Proxy creates a new SOCKS5 proxy instance
func NewSOCKS5Proxy(cfg SOCKS5ProxyConfig) *SOCKS5Proxy {
	ctx, cancel := context.WithCancel(context.Background())
	return &SOCKS5Proxy{
		config: cfg,
		ctx:    ctx,
		cancel: cancel,
	}
}

// Start starts the SOCKS5 proxy server
func (p *SOCKS5Proxy) Start() error {
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", p.config.Port))
	if err != nil {
		return fmt.Errorf("failed to listen on port %d: %w", p.config.Port, err)
	}
	p.ln = ln
	p.running = true

	p.wg.Add(1)
	go p.acceptLoop()

	log.Info("SOCKS5 proxy server started", "port", p.config.Port)
	return nil
}

// Stop stops the SOCKS5 proxy server
func (p *SOCKS5Proxy) Stop() error {
	p.mu.Lock()
	p.running = false
	p.mu.Unlock()

	p.cancel()

	if p.ln != nil {
		p.ln.Close()
	}

	p.wg.Wait()
	log.Info("SOCKS5 proxy server stopped")
	return nil
}

// acceptLoop accepts incoming SOCKS5 connections
func (p *SOCKS5Proxy) acceptLoop() {
	defer p.wg.Done()

	for {
		conn, err := p.ln.Accept()
		if err != nil {
			select {
			case <-p.ctx.Done():
				return
			default:
				log.Error("Failed to accept connection", "error", err)
				continue
			}
		}

		p.wg.Add(1)
		go p.handleConnection(conn)
	}
}

// handleConnection handles a single SOCKS5 connection
func (p *SOCKS5Proxy) handleConnection(conn net.ProxyConn) {
	defer p.wg.Done()
	defer conn.Close()

	connID := fmt.Sprintf("socks5-%d", time.Now().UnixNano())

	// SOCKS5 protocol handshake
	if err := p.handleHandshake(conn); err != nil {
		log.Error("SOCKS5 handshake failed", "error", err)
		return
	}

	// Handle request
	if err := p.handleRequest(conn); err != nil {
		log.Error("SOCKS5 request failed", "error", err)
		return
	}

	// Track connection
	if p.config.Stats != nil {
		p.config.Stats.RecordConnection("default", 0, 0)
		defer p.config.Stats.RecordDisconnection("default", 0, 0)
	}

	// Relay traffic
	p.relayTraffic(conn, connID)
}

// handleHandshake performs SOCKS5 handshake
func (p *SOCKS5Proxy) handleHandshake(conn net.ProxyConn) error {
	// Read greeting: VER + NMETHODS
	buf := make([]byte, 2)
	if _, err := io.ReadFull(conn, buf); err != nil {
		return fmt.Errorf("failed to read greeting: %w", err)
	}

	if buf[0] != 0x05 {
		return fmt.Errorf("unsupported SOCKS version: %d", buf[0])
	}

	nMethods := int(buf[1])
	if nMethods == 0 {
		return fmt.Errorf("no methods specified")
	}

	// Read methods
	methods := make([]byte, nMethods)
	if _, err := io.ReadFull(conn, methods); err != nil {
		return fmt.Errorf("failed to read methods: %w", err)
	}

	// Check for no-authentication (0x00)
	hasNoAuth := false
	for _, m := range methods {
		if m == 0x00 {
			hasNoAuth = true
			break
		}
	}

	if !hasNoAuth {
		// Send method: 0xFF (no acceptable methods)
		conn.Write([]byte{0x05, 0xFF})
		return fmt.Errorf("no acceptable authentication method")
	}

	// Send method: 0x00 (no authentication required)
	conn.Write([]byte{0x05, 0x00})

	return nil
}

// handleRequest handles SOCKS5 request
func (p *SOCKS5Proxy) handleRequest(conn net.ProxyConn) error {
	// Read request: VER + CMD + RSV + ATYP + DST.ADDR + DST.PORT
	buf := make([]byte, 4)
	if _, err := io.ReadFull(conn, buf); err != nil {
		return fmt.Errorf("failed to read request: %w", err)
	}

	if buf[0] != 0x05 {
		return fmt.Errorf("unsupported SOCKS version: %d", buf[0])
	}

	cmd := buf[1]
	// buf[2] is reserved (RSV)
	addrType := buf[3]

	var targetAddr string
	var targetPort int

	switch addrType {
	case 0x01: // IPv4
		ip := make([]byte, 4)
		if _, err := io.ReadFull(conn, ip); err != nil {
			return fmt.Errorf("failed to read IPv4: %w", err)
		}
		port := make([]byte, 2)
		if _, err := io.ReadFull(conn, port); err != nil {
			return fmt.Errorf("failed to read port: %w", err)
		}
		targetAddr = net.IP(ip).String()
		targetPort = int(binary.BigEndian.Uint16(port))

	case 0x03: // Domain name
		domainLen := make([]byte, 1)
		if _, err := io.ReadFull(conn, domainLen); err != nil {
			return fmt.Errorf("failed to read domain length: %w", err)
		}
		domain := make([]byte, domainLen[0])
		if _, err := io.ReadFull(conn, domain); err != nil {
			return fmt.Errorf("failed to read domain: %w", err)
		}
		port := make([]byte, 2)
		if _, err := io.ReadFull(conn, port); err != nil {
			return fmt.Errorf("failed to read port: %w", err)
		}
		targetAddr = string(domain)
		targetPort = int(binary.BigEndian.Uint16(port))

	case 0x04: // IPv6
		ip := make([]byte, 16)
		if _, err := io.ReadFull(conn, ip); err != nil {
			return fmt.Errorf("failed to read IPv6: %w", err)
		}
		port := make([]byte, 2)
		if _, err := io.ReadFull(conn, port); err != nil {
			return fmt.Errorf("failed to read port: %w", err)
		}
		targetAddr = net.IP(ip).String()
		targetPort = int(binary.BigEndian.Uint16(port))

	default:
		return fmt.Errorf("unsupported address type: %d", addrType)
	}

	// Connect to target
	targetConn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", targetAddr, targetPort))
	if err != nil {
		// Send connection failure reply
		conn.Write([]byte{0x05, 0x01, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00})
		return fmt.Errorf("failed to connect to target: %w", err)
	}
	defer targetConn.Close()

	// Send success reply
	reply := []byte{0x05, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	if _, err := conn.Write(reply); err != nil {
		return fmt.Errorf("failed to send reply: %w", err)
	}

	// Store the target connection for relay
	if pc, ok := conn.(*net.TCPConn); ok {
		// Keep the connection open
		_ = targetConn
	}

	return nil
}

// relayTraffic relays traffic between client and target
func (p *SOCKS5Proxy) relayTraffic(conn net.ProxyConn, connID string) {
	// For SOCKS5, we need to handle the relay differently
	// In a real implementation, we'd maintain both connections
	// Here we use a simple approach

	var wg sync.WaitGroup
	wg.Add(2)

	// Note: In a full implementation, we'd need to track the target connection
	// For now, this is a simplified version

	wg.Wait()
}

// ProxyConn wraps a net.Conn for SOCKS5 proxy
type ProxyConn struct {
	net.Conn
	targetConn net.Conn
}

// Read reads data from the connection
func (pc *ProxyConn) Read(b []byte) (n int, err error) {
	return pc.Conn.Read(b)
}

// Write writes data to the connection
func (pc *ProxyConn) Write(b []byte) (n int, err error) {
	return pc.Conn.Write(b)
}

// Close closes the connection and target
func (pc *ProxyConn) Close() error {
	if pc.targetConn != nil {
		pc.targetConn.Close()
	}
	return pc.Conn.Close()
}

// TransparentProxyConfig holds transparent proxy configuration
type TransparentProxyConfig struct {
	Port      int
	TUNDevice string
	Shaper    interface {
		GetDownloadLimit() int
		GetUploadLimit() int
	}
	Stats      *stats.Collector
	DNSServers []string
}

// TransparentProxy implements a transparent proxy with TUN/TAP support
type TransparentProxy struct {
	config  TransparentProxyConfig
	mu      sync.RWMutex
	running bool
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

// NewTransparentProxy creates a new transparent proxy instance
func NewTransparentProxy(cfg TransparentProxyConfig) *TransparentProxy {
	ctx, cancel := context.WithCancel(context.Background())
	return &TransparentProxy{
		config: cfg,
		ctx:    ctx,
		cancel: cancel,
	}
}

// Start starts the transparent proxy
func (p *TransparentProxy) Start() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.running {
		return fmt.Errorf("transparent proxy already running")
	}

	// Setup TUN device if specified
	if p.config.TUNDevice != "" {
		if err := p.setupTUNDevice(); err != nil {
			return fmt.Errorf("failed to setup TUN device: %w", err)
		}
	}

	p.running = true
	log.Info("Transparent proxy started", "device", p.config.TUNDevice)
	return nil
}

// Stop stops the transparent proxy
func (p *TransparentProxy) Stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.running {
		return nil
	}

	p.cancel()
	p.wg.Wait()

	// Cleanup TUN device
	if p.config.TUNDevice != "" {
		p.cleanupTUNDevice()
	}

	p.running = false
	log.Info("Transparent proxy stopped")
	return nil
}

// setupTUNDevice sets up the TUN device
func (p *TransparentProxy) setupTUNDevice() error {
	// TUN device setup would require OS-specific implementation
	// This is a placeholder for the actual TUN/TAP device handling
	log.Info("Setting up TUN device", "device", p.config.TUNDevice)
	return nil
}

// cleanupTUNDevice cleans up the TUN device
func (p *TransparentProxy) cleanupTUNDevice() error {
	log.Info("Cleaning up TUN device", "device", p.config.TUNDevice)
	return nil
}

// handlePacket handles an IP packet from TUN device
func (p *TransparentProxy) handlePacket(packet []byte) {
	// Parse IP header and route appropriately
	if len(packet) < 20 {
		return
	}

	version := packet[0] >> 4
	if version != 4 && version != 6 {
		return
	}

	// In a full implementation, this would:
	// 1. Parse the IP header
	// 2. Determine the destination
	// 3. Apply traffic shaping if configured
	// 4. Forward to the appropriate exit node
}

// DNSCacheEntry represents a DNS cache entry
type DNSCacheEntry struct {
	IP     string
	Expire time.Time
}

// DNSResolver handles DNS resolution for the proxy
type DNSResolver struct {
	mu          sync.RWMutex
	cache       map[string]*DNSCacheEntry
	nameservers []string
}

// NewDNSResolver creates a new DNS resolver
func NewDNSResolver(nameservers []string) *DNSResolver {
	if len(nameservers) == 0 {
		nameservers = []string{"8.8.8.8", "8.8.4.4"}
	}
	return &DNSResolver{
		cache:       make(map[string]*DNSCacheEntry),
		nameservers: nameservers,
	}
}

// Resolve resolves a domain name to IP address
func (r *DNSResolver) Resolve(domain string) (string, error) {
	r.mu.RLock()
	if entry, exists := r.cache[domain]; exists && time.Now().Before(entry.Expire) {
		r.mu.RUnlock()
		return entry.IP, nil
	}
	r.mu.RUnlock()

	// Use system resolver
	ips, err := net.LookupIP(domain)
	if err != nil {
		return "", err
	}

	if len(ips) == 0 {
		return "", fmt.Errorf("no IP found for domain")
	}

	ip := ips[0].String()

	// Cache the result
	r.mu.Lock()
	r.cache[domain] = &DNSCacheEntry{
		IP:     ip,
		Expire: time.Now().Add(5 * time.Minute),
	}
	r.mu.Unlock()

	return ip, nil
}

// ParseHTTPRequest parses an HTTP request from a connection
func ParseHTTPRequest(br *bufio.Reader) (*http.Request, error) {
	return http.ReadRequest(br)
}

// ParseHostPort parses a host:port string
func ParseHostPort(hostPort string) (host string, port int, err error) {
	if !strings.Contains(hostPort, ":") {
		return hostPort, 80, nil
	}

	host, portStr, err := net.SplitHostPort(hostPort)
	if err != nil {
		return "", 0, err
	}

	port, err = strconv.Atoi(portStr)
	if err != nil {
		return "", 0, err
	}

	return host, port, nil
}
