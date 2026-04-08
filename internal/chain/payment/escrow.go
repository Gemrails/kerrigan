package payment

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/kerrigan/kerrigan/internal/chain/client"
	"github.com/kerrigan/kerrigan/internal/chain/contracts"
	"github.com/kerrigan/kerrigan/internal/chain/payment/ecny"
	"github.com/kerrigan/kerrigan/pkg/log"
)

// EscrowState represents the state of an escrow
type EscrowState int

const (
	EscrowStatePending    EscrowState = 1 // Escrow created, awaiting payment
	EscrowStateFunded     EscrowState = 2 // Payment received, active
	EscrowStateActive     EscrowState = 3 // Resource usage in progress
	EscrowStateReleasing  EscrowState = 4 // Funds being released
	EscrowStateCompleted  EscrowState = 5 // All funds released, completed
	EscrowStateDisputed   EscrowState = 6 // Under dispute
	EscrowStateRefunding  EscrowState = 7 // Refund in progress
	EscrowStateRefunded   EscrowState = 8 // Fully refunded
	EscrowStateCancelled  EscrowState = 9 // Cancelled
)

// Escrow represents an escrow instance for resource trading
type Escrow struct {
	EscrowID     string              // Unique escrow identifier
	Seller       common.Address      // Provider receiving payment
	Buyer        common.Address      // Consumer paying for resources
	ResourceID   common.Address      // Resource being rented
	OrderID      string              // Associated order ID
	TotalAmount  *big.Int            // Total locked amount
	ReleasedAmount *big.Int         // Amount released to seller
	RefundedAmount *big.Int          // Amount refunded to buyer
	State        EscrowState         // Current escrow state
	StartTime    time.Time          // Escrow start time
	EndTime      time.Time          // Expected end time
	ActualEndTime time.Time         // Actual end time
	DisputeReason string             // Reason for dispute
	Metadata     string             // Additional metadata
	CreatedAt    time.Time          // Creation timestamp
	UpdatedAt    time.Time          // Last update timestamp
}

// UsageRecord represents a resource usage record
type UsageRecord struct {
	RecordID    string    // Unique record ID
	EscrowID    string    // Associated escrow ID
	CPUUsed     uint64    // CPU compute units used
	GPUUsed     uint64    // GPU compute units used
	MemoryUsedMB uint64  // Memory used in MB
	StorageUsedMB uint64 // Storage used in MB
	BandwidthUsedMB uint64 // Bandwidth used in MB
	StartTime   time.Time // Usage period start
	EndTime     time.Time // Usage period end
	Cost        *big.Int  // Calculated cost
	CreatedAt   time.Time // Record creation time
}

// Dispute represents a dispute case
type Dispute struct {
	DisputeID   string          // Unique dispute identifier
	EscrowID    string          // Associated escrow ID
	Initiator   common.Address  // Who initiated the dispute
	Reason      string          // Dispute reason
	Evidence    string          // Supporting evidence
	Resolution  string          // Resolution details
	SellerAmount *big.Int       // Amount awarded to seller
	BuyerAmount  *big.Int       // Amount awarded to buyer
	IsResolved  bool            // Whether dispute is resolved
	CreatedAt   time.Time       // Creation timestamp
	ResolvedAt  time.Time       // Resolution timestamp
}

// EscrowConfig holds escrow manager configuration
type EscrowConfig struct {
	ReleaseInterval  time.Duration // Interval for automatic fund release
	RefundTimeout    time.Duration // Timeout for refund requests
	DisputeTimeout   time.Duration // Timeout for dispute resolution
	AutoRelease      bool          // Enable automatic fund release
	MinReleaseAmount *big.Int     // Minimum amount for release
	FeePercentage    *big.Int      // Platform fee percentage (basis points)
}

// Manager manages escrow instances
type Manager struct {
	cfg       EscrowConfig
	logger    log.Logger
	bcClient  *client.Client
	ecnyClient *ecny.ECNYClient
	paymentContract *contracts.PaymentEscrowContract

	mu          sync.RWMutex
	escrows     map[string]*Escrow
	usageRecords map[string][]*UsageRecord
	disputes    map[string]*Dispute
	stopCh      chan struct{}
	wg          sync.WaitGroup
}

// NewManager creates a new escrow manager
func NewManager(cfg EscrowConfig, bcClient *client.Client, ecnyClient *ecny.ECNYClient, logger log.Logger) (*Manager, error) {
	logger.Info("Creating escrow manager")

	if cfg.ReleaseInterval == 0 {
		cfg.ReleaseInterval = 1 * time.Hour
	}

	if cfg.RefundTimeout == 0 {
		cfg.RefundTimeout = 24 * time.Hour
	}

	if cfg.DisputeTimeout == 0 {
		cfg.DisputeTimeout = 48 * time.Hour
	}

	if cfg.MinReleaseAmount == nil {
		cfg.MinReleaseAmount = big.NewInt(1e15) // 0.001 ETH
	}

	if cfg.FeePercentage == nil {
		cfg.FeePercentage = big.NewInt(250) // 2.5%
	}

	mgr := &Manager{
		cfg:       cfg,
		logger:    logger,
		bcClient:  bcClient,
		ecnyClient: ecnyClient,
		escrows:   make(map[string]*Escrow),
		usageRecords: make(map[string][]*UsageRecord),
		disputes:  make(map[string]*Dispute),
		stopCh:    make(chan struct{}),
	}

	// Initialize payment contract if available
	if bcClient != nil {
		mgr.paymentContract = bcClient.GetPayment()
	}

	logger.Info("Escrow manager created")
	return mgr, nil
}

// Start starts the escrow manager services
func (m *Manager) Start(ctx context.Context) error {
	m.logger.Info("Starting escrow manager")

	// Start automatic release process
	if m.cfg.AutoRelease {
		m.wg.Add(1)
		go m.processAutoReleases(ctx)
	}

	// Start usage reporting
	m.wg.Add(1)
	go m.processUsageReporting(ctx)

	m.logger.Info("Escrow manager started")
	return nil
}

// Stop stops the escrow manager
func (m *Manager) Stop() error {
	m.logger.Info("Stopping escrow manager")

	close(m.stopCh)
	m.wg.Wait()

	m.logger.Info("Escrow manager stopped")
	return nil
}

// processAutoReleases processes automatic fund releases
func (m *Manager) processAutoReleases(ctx context.Context) {
	defer m.wg.Done()

	ticker := time.NewTicker(m.cfg.ReleaseInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.releaseEligibleFunds(ctx)
		}
	}
}

// releaseEligibleFunds releases funds for eligible escrows
func (m *Manager) releaseEligibleFunds(ctx context.Context) {
	m.mu.RLock()
	var activeEscrows []*Escrow
	for _, escrow := range m.escrows {
		if escrow.State == EscrowStateActive {
			activeEscrows = append(activeEscrows, escrow)
		}
	}
	m.mu.RUnlock()

	for _, escrow := range activeEscrows {
		if err := m.ReleaseFunds(ctx, escrow.EscrowID, nil); err != nil {
			m.logger.Warnf("Failed to release funds for escrow %s: %v", escrow.EscrowID, err)
		}
	}
}

// processUsageReporting processes usage reporting
func (m *Manager) processUsageReporting(ctx context.Context) {
	defer m.wg.Done()

	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.reportUsage(ctx)
		}
	}
}

// reportUsage reports usage for all active escrows
func (m *Manager) reportUsage(ctx context.Context) {
	m.mu.RLock()
	var activeEscrows []*Escrow
	for _, escrow := range m.escrows {
		if escrow.State == EscrowStateActive {
			activeEscrows = append(activeEscrows, escrow)
		}
	}
	m.mu.RUnlock()

	for _, escrow := range activeEscrows {
		// In production, this would collect actual usage metrics
		// and report to the payment contract
		m.logger.Debugf("Reporting usage for escrow %s", escrow.EscrowID)
	}
}

// CreateEscrow creates a new escrow
func (m *Manager) CreateEscrow(ctx context.Context, seller, resourceID common.Address, totalAmount *big.Int, orderID string, duration time.Duration) (*Escrow, error) {
	m.logger.Infof("Creating escrow: seller=%s, resource=%s, amount=%s, duration=%s",
		seller.Hex(), resourceID.Hex(), totalAmount.String(), duration.String())

	// Generate escrow ID
	escrowID := generateEscrowID(seller, resourceID, totalAmount, orderID)

	escrow := &Escrow{
		EscrowID:       escrowID,
		Seller:         seller,
		Buyer:          common.HexToAddress("0x0"),
		ResourceID:     resourceID,
		OrderID:        orderID,
		TotalAmount:    totalAmount,
		ReleasedAmount: big.NewInt(0),
		RefundedAmount: big.NewInt(0),
		State:          EscrowStatePending,
		StartTime:      time.Now(),
		EndTime:        time.Now().Add(duration),
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	m.mu.Lock()
	m.escrows[escrowID] = escrow
	m.mu.Unlock()

	m.logger.Infof("Escrow created: %s", escrowID)
	return escrow, nil
}

// FundEscrow funds an escrow with payment
func (m *Manager) FundEscrow(ctx context.Context, escrowID string, buyer common.Address, paymentTxID string) error {
	m.logger.Infof("Funding escrow %s from buyer %s", escrowID, buyer.Hex())

	m.mu.Lock()
	escrow, ok := m.escrows[escrowID]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("escrow not found: %s", escrowID)
	}

	if escrow.State != EscrowStatePending {
		m.mu.Unlock()
		return fmt.Errorf("escrow in invalid state: %d", escrow.State)
	}

	escrow.Buyer = buyer
	escrow.State = EscrowStateFunded
	escrow.UpdatedAt = time.Now()
	m.mu.Unlock()

	// Create payment transaction
	paymentReq := ecny.PaymentRequest{
		To:       escrow.Seller,
		Amount:   escrow.TotalAmount,
		Method:   ecny.PaymentMethodEscrow,
		OrderID:  escrow.OrderID,
		EscrowID: escrowID,
	}

	if m.ecnyClient != nil {
		tx, err := m.ecnyClient.CreatePayment(ctx, paymentReq)
		if err != nil {
			return fmt.Errorf("failed to create payment: %w", err)
		}

		_, err = m.ecnyClient.ExecutePayment(ctx, tx.TxID)
		if err != nil {
			return fmt.Errorf("failed to execute payment: %w", err)
		}
	}

	// Update blockchain contract if available
	if m.paymentContract != nil {
		_, err := m.paymentContract.CreateEscrow(ctx, m.bcClient.GetTransactor(), escrow.Seller, escrow.ResourceID, escrow.TotalAmount, uint64(time.Until(escrow.EndTime).Seconds()))
		if err != nil {
			m.logger.Warnf("Failed to create blockchain escrow: %v", err)
		}
	}

	m.logger.Infof("Escrow %s funded", escrowID)
	return nil
}

// ActivateEscrow activates an escrow for resource usage
func (m *Manager) ActivateEscrow(ctx context.Context, escrowID string) error {
	m.logger.Infof("Activating escrow: %s", escrowID)

	m.mu.Lock()
	escrow, ok := m.escrows[escrowID]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("escrow not found: %s", escrowID)
	}

	if escrow.State != EscrowStateFunded {
		m.mu.Unlock()
		return fmt.Errorf("escrow in invalid state: %d", escrow.State)
	}

	escrow.State = EscrowStateActive
	escrow.UpdatedAt = time.Now()
	m.mu.Unlock()

	m.logger.Infof("Escrow %s activated", escrowID)
	return nil
}

// ReleaseFunds releases funds to the seller
func (m *Manager) ReleaseFunds(ctx context.Context, escrowID string, amount *big.Int) error {
	m.logger.Infof("Releasing funds for escrow: %s", escrowID)

	m.mu.Lock()
	escrow, ok := m.escrows[escrowID)
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("escrow not found: %s", escrowID)
	}

	if escrow.State != EscrowStateActive && escrow.State != EscrowStateReleasing {
		m.mu.Unlock()
		return fmt.Errorf("escrow in invalid state for release: %d", escrow.State)
	}

	// Calculate release amount
	if amount == nil {
		// Calculate based on usage
		records := m.usageRecords[escrowID]
		totalCost := big.NewInt(0)
		for _, record := range records {
			totalCost = new(big.Int).Add(totalCost, record.Cost)
		}

		// If no usage records, release a portion
		if totalCost.Sign() == 0 {
			releaseAmount := new(big.Int).Div(escrow.TotalAmount, big.NewInt(10)) // 10% of total
			amount = releaseAmount
		} else {
			amount = totalCost
		}
	}

	// Ensure minimum release amount
	if amount.Cmp(m.cfg.MinReleaseAmount) < 0 {
		m.mu.Unlock()
		return fmt.Errorf("amount below minimum: %s < %s", amount.String(), m.cfg.MinReleaseAmount.String())
	}

	// Calculate remaining amount
	remaining := new(big.Int).Sub(escrow.TotalAmount, escrow.ReleasedAmount)
	remaining = new(big.Int).Sub(remaining, escrow.RefundedAmount)

	if amount.Cmp(remaining) > 0 {
		amount = remaining
	}

	escrow.ReleasedAmount = new(big.Int).Add(escrow.ReleasedAmount, amount)
	escrow.State = EscrowStateReleasing
	escrow.UpdatedAt = time.Now()
	m.mu.Unlock()

	// Update blockchain contract
	if m.paymentContract != nil {
		escrowIDBytes := []byte(escrowID)
		_, err := m.paymentContract.ReleaseFunds(ctx, m.bcClient.GetTransactor(), escrowIDBytes, amount)
		if err != nil {
			m.logger.Warnf("Failed to release funds on blockchain: %v", err)
		}
	}

	// Transfer funds to seller via e-CNY
	if m.ecnyClient != nil && escrow.Buyer != (common.Address{}) {
		paymentReq := ecny.PaymentRequest{
			To:       escrow.Seller,
			Amount:   amount,
			Method:   ecny.PaymentMethodEscrow,
			OrderID:  escrow.OrderID,
			EscrowID: escrowID,
		}

		tx, err := m.ecnyClient.CreatePayment(ctx, paymentReq)
		if err != nil {
			m.logger.Warnf("Failed to create payment: %v", err)
		} else {
			_, err = m.ecnyClient.ExecutePayment(ctx, tx.TxID)
			if err != nil {
				m.logger.Warnf("Failed to execute payment: %v", err)
			}
		}
	}

	m.logger.Infof("Released %s wei to seller for escrow %s", amount.String(), escrowID)
	return nil
}

// Refund refunds remaining funds to buyer
func (m *Manager) Refund(ctx context.Context, escrowID string) error {
	m.logger.Infof("Refunding escrow: %s", escrowID)

	m.mu.Lock()
	escrow, ok := m.escrows[escrowID)
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("escrow not found: %s", escrowID)
	}

	if escrow.State != EscrowStateActive && escrow.State != EscrowStateFunded && escrow.State != EscrowStateReleasing {
		m.mu.Unlock()
		return fmt.Errorf("escrow in invalid state for refund: %d", escrow.State)
	}

	// Calculate refundable amount
	refundable := new(big.Int).Sub(escrow.TotalAmount, escrow.ReleasedAmount)
	refundable = new(big.Int).Sub(refundable, escrow.RefundedAmount)

	if refundable.Sign() <= 0 {
		m.mu.Unlock()
		return fmt.Errorf("no funds to refund")
	}

	escrow.RefundedAmount = new(big.Int).Add(escrow.RefundedAmount, refundable)
	escrow.State = EscrowStateRefunding
	escrow.ActualEndTime = time.Now()
	escrow.UpdatedAt = time.Now()
	m.mu.Unlock()

	// Update blockchain contract
	if m.paymentContract != nil {
		escrowIDBytes := []byte(escrowID)
		_, err := m.paymentContract.RefundRemaining(ctx, m.bcClient.GetTransactor(), escrowIDBytes)
		if err != nil {
			m.logger.Warnf("Failed to refund on blockchain: %v", err)
		}
	}

	// Refund to buyer via e-CNY
	if m.ecnyClient != nil && escrow.Buyer != (common.Address{}) {
		_, err := m.ecnyClient.Refund(ctx, escrowID)
		if err != nil {
			m.logger.Warnf("Failed to refund: %v", err)
		}
	}

	m.logger.Infof("Refunded %s wei to buyer for escrow %s", refundable.String(), escrowID)
	return nil
}

// CompleteEscrow completes an escrow
func (m *Manager) CompleteEscrow(ctx context.Context, escrowID string) error {
	m.logger.Infof("Completing escrow: %s", escrowID)

	m.mu.Lock()
	escrow, ok := m.escrows[escrowID)
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("escrow not found: %s", escrowID)
	}

	// Release any remaining funds
	if escrow.ReleasedAmount.Cmp(escrow.TotalAmount) < 0 {
		remaining := new(big.Int).Sub(escrow.TotalAmount, escrow.ReleasedAmount)
		escrow.ReleasedAmount = escrow.TotalAmount

		// Release to seller
		if m.ecnyClient != nil {
			paymentReq := ecny.PaymentRequest{
				To:       escrow.Seller,
				Amount:   remaining,
				Method:   ecny.PaymentMethodEscrow,
				OrderID:  escrow.OrderID,
				EscrowID: escrowID,
			}

			tx, err := m.ecnyClient.CreatePayment(ctx, paymentReq)
			if err != nil {
				m.logger.Warnf("Failed to create final payment: %v", err)
			} else {
				m.ecnyClient.ExecutePayment(ctx, tx.TxID)
			}
		}
	}

	escrow.State = EscrowStateCompleted
	escrow.ActualEndTime = time.Now()
	escrow.UpdatedAt = time.Now()
	m.mu.Unlock()

	m.logger.Infof("Escrow %s completed", escrowID)
	return nil
}

// CancelEscrow cancels an escrow
func (m *Manager) CancelEscrow(ctx context.Context, escrowID string) error {
	m.logger.Infof("Cancelling escrow: %s", escrowID)

	m.mu.Lock()
	escrow, ok := m.escrows[escrowID)
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("escrow not found: %s", escrowID)
	}

	if escrow.State != EscrowStatePending && escrow.State != EscrowStateFunded {
		m.mu.Unlock()
		return fmt.Errorf("cannot cancel escrow in state: %d", escrow.State)
	}

	escrow.State = EscrowStateCancelled
	escrow.UpdatedAt = time.Now()
	m.mu.Unlock()

	m.logger.Infof("Escrow %s cancelled", escrowID)
	return nil
}

// OpenDispute opens a dispute for an escrow
func (m *Manager) OpenDispute(ctx context.Context, escrowID string, initiator common.Address, reason, evidence string) (*Dispute, error) {
	m.logger.Infof("Opening dispute for escrow: %s", escrowID)

	m.mu.Lock()
	escrow, ok := m.escrows[escrowID)
	if !ok {
		m.mu.Unlock()
		return nil, fmt.Errorf("escrow not found: %s", escrowID)
	}

	if escrow.State != EscrowStateActive && escrow.State != EscrowStateReleasing {
		m.mu.Unlock()
		return nil, fmt.Errorf("cannot open dispute in state: %d", escrow.State)
	}

	escrow.State = EscrowStateDisputed
	escrow.DisputeReason = reason
	escrow.UpdatedAt = time.Now()

	disputeID := generateDisputeID(escrowID, initiator, reason)
	dispute := &Dispute{
		DisputeID:  disputeID,
		EscrowID:   escrowID,
		Initiator:  initiator,
		Reason:     reason,
		Evidence:   evidence,
		IsResolved: false,
		CreatedAt:  time.Now(),
	}

	m.disputes[disputeID] = dispute
	m.mu.Unlock()

	// Update blockchain contract
	if m.paymentContract != nil {
		escrowIDBytes := []byte(escrowID)
		_, err := m.paymentContract.OpenDispute(ctx, m.bcClient.GetTransactor(), escrowIDBytes, reason, evidence)
		if err != nil {
			m.logger.Warnf("Failed to open dispute on blockchain: %v", err)
		}
	}

	m.logger.Infof("Dispute %s opened for escrow %s", disputeID, escrowID)
	return dispute, nil
}

// ResolveDispute resolves a dispute
func (m *Manager) ResolveDispute(ctx context.Context, disputeID string, resolution string, sellerAmount, buyerAmount *big.Int) error {
	m.logger.Infof("Resolving dispute: %s", disputeID)

	m.mu.Lock()
	dispute, ok := m.disputes[disputeID)
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("dispute not found: %s", disputeID)
	}

	if dispute.IsResolved {
		m.mu.Unlock()
		return fmt.Errorf("dispute already resolved")
	}

	dispute.Resolution = resolution
	dispute.SellerAmount = sellerAmount
	dispute.BuyerAmount = buyerAmount
	dispute.IsResolved = true
	dispute.ResolvedAt = time.Now()

	// Update escrow state
	escrow, ok := m.escrows[dispute.EscrowID)
	if ok {
		escrow.State = EscrowStateCompleted
		escrow.UpdatedAt = time.Now()
	}

	m.mu.Unlock()

	// Update blockchain contract
	if m.paymentContract != nil {
		disputeIDBytes := []byte(disputeID)
		_, err := m.paymentContract.ResolveDispute(ctx, m.bcClient.GetTransactor(), disputeIDBytes, resolution, sellerAmount, buyerAmount)
		if err != nil {
			m.logger.Warnf("Failed to resolve dispute on blockchain: %v", err)
		}
	}

	m.logger.Infof("Dispute %s resolved", disputeID)
	return nil
}

// RecordUsage records resource usage for an escrow
func (m *Manager) RecordUsage(ctx context.Context, escrowID string, usage *UsageRecord) error {
	m.logger.Debugf("Recording usage for escrow: %s", escrowID)

	m.mu.Lock()
	escrow, ok := m.escrows[escrowID)
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("escrow not found: %s", escrowID)
	}

	if escrow.State != EscrowStateActive {
		m.mu.Unlock()
		return fmt.Errorf("escrow not active")
	}

	usage.EscrowID = escrowID
	usage.CreatedAt = time.Now()

	m.usageRecords[escrowID] = append(m.usageRecords[escrowID], usage)
	m.mu.Unlock()

	// Report to blockchain
	if m.paymentContract != nil {
		usageMeter := contracts.UsageMeter{
			CPUUsed:      usage.CPUUsed,
			GPUUsed:      usage.GPUUsed,
			MemoryUsedMB: usage.MemoryUsedMB,
			StorageUsedMB: usage.StorageUsedMB,
			BandwidthUsedMB: usage.BandwidthUsedMB,
			StartTime:    uint64(usage.StartTime.Unix()),
			EndTime:      uint64(usage.EndTime.Unix()),
		}

		escrowIDBytes := []byte(escrowID)
		_, err := m.paymentContract.ReportUsage(ctx, m.bcClient.GetTransactor(), escrowIDBytes, usageMeter)
		if err != nil {
			m.logger.Warnf("Failed to report usage on blockchain: %v", err)
		}
	}

	return nil
}

// GetEscrow returns an escrow by ID
func (m *Manager) GetEscrow(escrowID string) (*Escrow, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	escrow, ok := m.escrows[escrowID)
	if !ok {
		return nil, fmt.Errorf("escrow not found: %s", escrowID)
	}

	return escrow, nil
}

// GetDispute returns a dispute by ID
func (m *Manager) GetDispute(disputeID string) (*Dispute, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	dispute, ok := m.disputes[disputeID]
	if !ok {
		return nil, fmt.Errorf("dispute not found: %s", disputeID)
	}

	return dispute, nil
}

// GetEscrowsByBuyer returns all escrows for a buyer
func (m *Manager) GetEscrowsByBuyer(buyer common.Address) []*Escrow {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*Escrow
	for _, escrow := range m.escrows {
		if escrow.Buyer == buyer {
			result = append(result, escrow)
		}
	}

	return result
}

// GetEscrowsBySeller returns all escrows for a seller
func (m *Manager) GetEscrowsBySeller(seller common.Address) []*Escrow {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*Escrow
	for _, escrow := range m.escrows {
		if escrow.Seller == seller {
			result = append(result, escrow)
		}
	}

	return result
}

// GetUsageRecords returns usage records for an escrow
func (m *Manager) GetUsageRecords(escrowID string) []*UsageRecord {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.usageRecords[escrowID]
}

// GetActiveEscrows returns all active escrows
func (m *Manager) GetActiveEscrows() []*Escrow {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*Escrow
	for _, escrow := range m.escrows {
		if escrow.State == EscrowStateActive || escrow.State == EscrowStateFunded {
			result = append(result, escrow)
		}
	}

	return result
}

// generateEscrowID generates a unique escrow ID
func generateEscrowID(seller common.Address, resourceID common.Address, amount *big.Int, orderID string) string {
	data := fmt.Sprintf("%s:%s:%s:%s:%d", seller.Hex(), resourceID.Hex(), amount.String(), orderID, time.Now().UnixNano())
	hash := sha256.Sum256([]byte(data))
	return "escrow_" + hex.EncodeToString(hash[:16])
}

// generateDisputeID generates a unique dispute ID
func generateDisputeID(escrowID string, initiator common.Address, reason string) string {
	data := fmt.Sprintf("%s:%s:%s:%d", escrowID, initiator.Hex(), reason, time.Now().UnixNano())
	hash := sha256.Sum256([]byte(data))
	return "dispute_" + hex.EncodeToString(hash[:16])
}

// FormatEscrowSummary returns a human-readable string
func (e *Escrow) FormatEscrowSummary() string {
	return fmt.Sprintf("Escrow{id=%s, seller=%s, buyer=%s, total=%s, released=%s, state=%d}",
		e.EscrowID, e.Seller.Hex()[:6], e.Buyer.Hex()[:6], e.TotalAmount.String(), e.ReleasedAmount.String(), e.State)
}

// FormatUsageSummary returns a human-readable string for usage
func (u *UsageRecord) FormatUsageSummary() string {
	return fmt.Sprintf("Usage{cpu=%d, gpu=%d, memory=%dMB, storage=%dMB, bandwidth=%dMB, cost=%s}",
		u.CPUUsed, u.GPUUsed, u.MemoryUsedMB, u.StorageUsedMB, u.BandwidthUsedMB, u.Cost.String())
}

// CalculateUsageCost calculates cost based on usage
func CalculateUsageCost(usage *UsageRecord, pricePerCPU, pricePerGPU, pricePerMB uint64) *big.Int {
	cost := big.NewInt(0)

	cost = cost.Add(cost, big.NewInt(int64(usage.CPUUsed)*int64(pricePerCPU)))
	cost = cost.Add(cost, big.NewInt(int64(usage.GPUUsed)*int64(pricePerGPU)))
	cost = cost.Add(cost, big.NewInt(int64(usage.MemoryUsedMB)*int64(pricePerMB)))
	cost = cost.Add(cost, big.NewInt(int64(usage.StorageUsedMB)*int64(pricePerMB)))
	cost = cost.Add(cost, big.NewInt(int64(usage.BandwidthUsedMB)*int64(pricePerMB)))

	return cost
}

// ParseMetadata parses metadata JSON
func (e *Escrow) ParseMetadata(v interface{}) error {
	if e.Metadata == "" {
		return nil
	}
	return json.Unmarshal([]byte(e.Metadata), v)
}

// SetMetadata sets metadata from a struct
func (e *Escrow) SetMetadata(v interface{}) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	e.Metadata = string(data)
	return nil
}

// ManagerFromConfig creates a manager from configuration
func ManagerFromConfig(cfg map[string]interface{}, bcClient *client.Client, ecnyClient *ecny.ECNYClient, logger log.Logger) (*Manager, error) {
	escrowConfig := EscrowConfig{}

	if interval, ok := cfg["release_interval"].(float64); ok {
		escrowConfig.ReleaseInterval = time.Duration(interval) * time.Second
	}

	if timeout, ok := cfg["refund_timeout"].(float64); ok {
		escrowConfig.RefundTimeout = time.Duration(timeout) * time.Second
	}

	if autoRelease, ok := cfg["auto_release"].(bool); ok {
		escrowConfig.AutoRelease = autoRelease
	}

	return NewManager(escrowConfig, bcClient, ecnyClient, logger)
}
