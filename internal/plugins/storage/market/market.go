package market

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/kerrigan/kerrigan/pkg/log"
)

// Market manages storage provider registration and marketplace
type Market struct {
	ctx       context.Context
	dataDir   string
	providers map[string]*Provider
	orders    map[string]*Order
	mu        sync.RWMutex
	wg        sync.WaitGroup
	running   bool
}

// Provider represents a storage provider
type Provider struct {
	ID           string
	PeerID       string
	PricePerGB   int64
	MinDuration  time.Duration
	TotalSpace   int64
	UsedSpace    int64
	Reputation   float64
	ActiveOrders int
	RegisteredAt time.Time
	LastUpdate   time.Duration
}

// Order represents a storage order
type Order struct {
	ID         string
	ProviderID string
	CID        string
	Size       int64
	Duration   time.Duration
	Price      int64
	Status     OrderStatus
	CreatedAt  time.Time
	ExpiresAt  time.Time
}

// OrderStatus represents the status of an order
type OrderStatus string

const (
	OrderStatusPending   OrderStatus = "pending"
	OrderStatusActive    OrderStatus = "active"
	OrderStatusCompleted OrderStatus = "completed"
	OrderStatusFailed    OrderStatus = "failed"
	OrderStatusCancelled OrderStatus = "cancelled"
)

// StorageProof represents a proof of storage
type StorageProof struct {
	CID        string
	ProviderID string
	Root       []byte
	Challenge  []byte
	Response   []byte
	Timestamp  time.Time
	Duration   time.Duration
}

// New creates a new market
func New(ctx context.Context, dataDir string) (*Market, error) {
	m := &Market{
		ctx:       ctx,
		dataDir:   dataDir,
		providers: make(map[string]*Provider),
		orders:    make(map[string]*Order),
	}

	if err := os.MkdirAll(filepath.Join(dataDir, "providers"), 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(dataDir, "orders"), 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	// Load existing data
	if err := m.loadProviders(); err != nil {
		log.Warn("Failed to load providers", "error", err)
	}

	if err := m.loadOrders(); err != nil {
		log.Warn("Failed to load orders", "error", err)
	}

	return m, nil
}

// Close closes the market
func (m *Market) Close() {
	m.running = false
	m.wg.Wait()
}

// RegisterProvider registers this node as a storage provider
func (m *Market) RegisterProvider(ctx context.Context, pricePerGB int64, minDuration time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Generate provider ID
	providerID := generateID()

	provider := &Provider{
		ID:           providerID,
		PricePerGB:   pricePerGB,
		MinDuration:  minDuration,
		TotalSpace:   1e12,
		RegisteredAt: time.Now(),
	}

	m.providers[providerID] = provider

	if err := m.saveProvider(provider); err != nil {
		return fmt.Errorf("failed to save provider: %w", err)
	}

	log.Info("Registered storage provider", "id", providerID, "price", pricePerGB)
	return nil
}

// UpdatePrice updates the storage price
func (m *Market) UpdatePrice(ctx context.Context, pricePerGB int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Find local provider (first one for now)
	var localProvider *Provider
	for _, p := range m.providers {
		localProvider = p
		break
	}

	if localProvider == nil {
		return fmt.Errorf("no provider registered")
	}

	localProvider.PricePerGB = pricePerGB

	if err := m.saveProvider(localProvider); err != nil {
		return fmt.Errorf("failed to save provider: %w", err)
	}

	log.Info("Updated storage price", "price", pricePerGB)
	return nil
}

// FindProviders finds available storage providers
func (m *Market) FindProviders(ctx context.Context, size int64) ([]*Provider, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*Provider
	for _, p := range m.providers {
		available := p.TotalSpace - p.UsedSpace
		if available >= size {
			result = append(result, p)
		}
	}

	// Sort by price (ascending)
	for i := 0; i < len(result)-1; i++ {
		for j := i + 1; j < len(result); j++ {
			if result[j].PricePerGB < result[i].PricePerGB {
				result[i], result[j] = result[j], result[i]
			}
		}
	}

	return result, nil
}

// AddProvider adds an external provider to the market
func (m *Market) AddProvider(provider *Provider) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.providers[provider.ID] = provider
	return m.saveProvider(provider)
}

// GetProvider returns a provider by ID
func (m *Market) GetProvider(id string) (*Provider, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	p, ok := m.providers[id]
	if !ok {
		return nil, fmt.Errorf("provider not found: %s", id)
	}

	return p, nil
}

// RequestStorage requests storage from a provider
func (m *Market) RequestStorage(ctx context.Context, providerID string, cid string, duration time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	provider, ok := m.providers[providerID]
	if !ok {
		return fmt.Errorf("provider not found: %s", providerID)
	}

	// Calculate price
	size, err := m.estimateSize(cid)
	if err != nil {
		return fmt.Errorf("failed to estimate size: %w", err)
	}

	price := provider.PricePerGB * (size / (1024 * 1024 * 1024))
	if price < 1 {
		price = 1 // Minimum price
	}

	order := &Order{
		ID:         generateID(),
		ProviderID: providerID,
		CID:        cid,
		Size:       size,
		Duration:   duration,
		Price:      price,
		Status:     OrderStatusPending,
		CreatedAt:  time.Now(),
		ExpiresAt:  time.Now().Add(duration),
	}

	m.orders[order.ID] = order
	provider.ActiveOrders++

	if err := m.saveOrder(order); err != nil {
		return fmt.Errorf("failed to save order: %w", err)
	}

	log.Info("Created storage order", "order_id", order.ID, "provider", providerID, "cid", cid)
	return nil
}

// GetOrder returns an order by ID
func (m *Market) GetOrder(orderID string) (*Order, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	order, ok := m.orders[orderID]
	if !ok {
		return nil, fmt.Errorf("order not found: %s", orderID)
	}

	return order, nil
}

// CompleteOrder marks an order as completed
func (m *Market) CompleteOrder(orderID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	order, ok := m.orders[orderID]
	if !ok {
		return fmt.Errorf("order not found: %s", orderID)
	}

	order.Status = OrderStatusCompleted

	provider, ok := m.providers[order.ProviderID]
	if ok {
		provider.ActiveOrders--
		provider.UsedSpace += order.Size
		m.saveProvider(provider)
	}

	m.saveOrder(order)

	log.Info("Completed order", "order_id", orderID)
	return nil
}

// CancelOrder cancels an order
func (m *Market) CancelOrder(orderID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	order, ok := m.orders[orderID]
	if !ok {
		return fmt.Errorf("order not found: %s", orderID)
	}

	order.Status = OrderStatusCancelled

	provider, ok := m.providers[order.ProviderID]
	if ok {
		provider.ActiveOrders--
	}

	m.saveOrder(order)

	log.Info("Cancelled order", "order_id", orderID)
	return nil
}

// GenerateProof generates a storage proof for a CID
func (m *Market) GenerateProof(ctx context.Context, cid string) (*StorageProof, error) {
	m.mu.RLock()
	// Find active order for this CID
	var order *Order
	for _, o := range m.orders {
		if o.CID == cid && o.Status == OrderStatusActive {
			order = o
			break
		}
	}
	m.mu.RUnlock()

	if order == nil {
		return nil, fmt.Errorf("no active order for CID: %s", cid)
	}

	// Generate random challenge
	challenge := make([]byte, 32)
	if _, err := rand.Read(challenge); err != nil {
		return nil, fmt.Errorf("failed to generate challenge: %w", err)
	}

	// In a real implementation, this would perform actual PoSP
	// For now, generate a placeholder response
	response := make([]byte, 32)
	if _, err := rand.Read(response); err != nil {
		return nil, fmt.Errorf("failed to generate response: %w", err)
	}

	proof := &StorageProof{
		CID:        cid,
		ProviderID: order.ProviderID,
		Root:       []byte(cid), // Placeholder
		Challenge:  challenge,
		Response:   response,
		Timestamp:  time.Now(),
		Duration:   order.Duration,
	}

	log.Debug("Generated storage proof", "cid", cid)
	return proof, nil
}

// VerifyProof verifies a storage proof
func (m *Market) VerifyProof(ctx context.Context, proof *StorageProof) (bool, error) {
	// In a real implementation, this would verify the proof
	// For now, just check basic fields

	if proof == nil {
		return false, fmt.Errorf("nil proof")
	}

	if proof.CID == "" {
		return false, fmt.Errorf("empty CID")
	}

	if len(proof.Challenge) == 0 || len(proof.Response) == 0 {
		return false, fmt.Errorf("invalid challenge/response")
	}

	// Verify timestamp is recent
	if time.Since(proof.Timestamp) > proof.Duration {
		return false, fmt.Errorf("proof expired")
	}

	log.Debug("Verified storage proof", "cid", proof.CID, "valid", true)
	return true, nil
}

// GetActiveOrders returns all active orders
func (m *Market) GetActiveOrders() []*Order {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*Order
	for _, o := range m.orders {
		if o.Status == OrderStatusActive {
			result = append(result, o)
		}
	}

	return result
}

// UpdateProviderSpace updates the provider's total space
func (m *Market) UpdateProviderSpace(providerID string, totalSpace, usedSpace int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	provider, ok := m.providers[providerID]
	if !ok {
		return fmt.Errorf("provider not found: %s", providerID)
	}

	provider.TotalSpace = totalSpace
	provider.UsedSpace = usedSpace

	return m.saveProvider(provider)
}

// loadProviders loads providers from disk
func (m *Market) loadProviders() error {
	// Placeholder - in real implementation would load from JSON files
	return nil
}

// saveProvider saves a provider to disk
func (m *Market) saveProvider(p *Provider) error {
	data := fmt.Sprintf("%s,%d,%d\n", p.ID, p.PricePerGB, p.TotalSpace)
	path := filepath.Join(m.dataDir, "providers", p.ID+".txt")
	return os.WriteFile(path, []byte(data), 0644)
}

// loadOrders loads orders from disk
func (m *Market) loadOrders() error {
	// Placeholder - in real implementation would load from JSON files
	return nil
}

// saveOrder saves an order to disk
func (m *Market) saveOrder(o *Order) error {
	data := fmt.Sprintf("%s,%s,%s,%d\n", o.ID, o.ProviderID, o.CID, o.Price)
	path := filepath.Join(m.dataDir, "orders", o.ID+".txt")
	return os.WriteFile(path, []byte(data), 0644)
}

// estimateSize estimates the size of data for a CID
func (m *Market) estimateSize(cid string) (int64, error) {
	// Placeholder - would query IPFS for actual size
	return 1024 * 1024, nil // Default 1MB
}

// generateID generates a random ID
func generateID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}
