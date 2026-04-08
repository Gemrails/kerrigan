package ecny

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/kerrigan/kerrigan/internal/chain/client"
	"github.com/kerrigan/kerrigan/pkg/log"
)

// PaymentMethod represents the payment method
type PaymentMethod string

const (
	PaymentMethodWallet PaymentMethod = "wallet" // Direct wallet payment
	PaymentMethodEscrow PaymentMethod = "escrow" // Escrow-based payment
	PaymentMethodCredit PaymentMethod = "credit" // Credit-based payment
)

// PaymentStatus represents the status of a payment
type PaymentStatus string

const (
	PaymentStatusPending    PaymentStatus = "pending"    // Payment initiated
	PaymentStatusProcessing PaymentStatus = "processing" // Payment being processed
	PaymentStatusCompleted  PaymentStatus = "completed"  // Payment completed
	PaymentStatusFailed     PaymentStatus = "failed"     // Payment failed
	PaymentStatusRefunded   PaymentStatus = "refunded"   // Payment refunded
)

// Transaction represents an e-CNY payment transaction
type Transaction struct {
	TxID        string         // Transaction ID
	From        common.Address // Payer address
	To          common.Address // Payee address
	Amount      *big.Int       // Amount in wei (1 CNY = 10^18 wei)
	Fee         *big.Int       // Transaction fee
	Method      PaymentMethod  // Payment method
	Status      PaymentStatus  // Payment status
	BlockHash   common.Hash    // Blockchain block hash
	BlockNumber uint64         // Block number
	TxHash      common.Hash    // Transaction hash
	Timestamp   uint64         // Transaction timestamp
	Metadata    string         // Additional metadata
	CreatedAt   time.Time      // Creation time
	UpdatedAt   time.Time      // Last update time
}

// Wallet represents an e-CNY wallet
type Wallet struct {
	Address          common.Address // Wallet address
	Balance          *big.Int       // Current balance in wei
	FrozenBalance    *big.Int       // Frozen balance (in escrow)
	AvailableBalance *big.Int       // Available balance (balance - frozen)
	LastUpdated      time.Time      // Last update timestamp
}

// PaymentRequest represents a payment request
type PaymentRequest struct {
	To          common.Address // Payee address
	Amount      *big.Int       // Amount in wei
	Method      PaymentMethod  // Payment method
	OrderID     string         // Associated order ID
	EscrowID    string         // Associated escrow ID
	Metadata    string         // Additional metadata
	CallbackURL string         // Callback URL for notification
}

// PaymentResult represents the result of a payment
type PaymentResult struct {
	Transaction *Transaction
	Success     bool
	Error       error
}

// GatewayConfig holds e-CNY payment gateway configuration
type GatewayConfig struct {
	GatewayURL     string // e-CNY payment gateway URL
	APIKey         string // API key for authentication
	MerchantID     string // Merchant ID
	Timeout        time.Duration
	RetryCount     int
	CallbackSecret string // Secret for webhook signature verification
}

// ECNYClient represents the e-CNY payment client
type ECNYClient struct {
	cfg      GatewayConfig
	logger   log.Logger
	bcClient *client.Client

	mu           sync.RWMutex
	wallets      map[common.Address]*Wallet
	transactions map[string]*Transaction
	pendingTxs   map[string]*Transaction
	webhookCh    chan *Transaction
	stopCh       chan struct{}
	wg           sync.WaitGroup
}

// NewECNYClient creates a new e-CNY payment client
func NewECNYClient(cfg GatewayConfig, bcClient *client.Client, logger log.Logger) (*ECNYClient, error) {
	logger.Infof("Creating e-CNY client for gateway: %s", cfg.GatewayURL)

	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}

	if cfg.RetryCount == 0 {
		cfg.RetryCount = 3
	}

	ecnyClient := &ECNYClient{
		cfg:          cfg,
		logger:       logger,
		bcClient:     bcClient,
		wallets:      make(map[common.Address]*Wallet),
		transactions: make(map[string]*Transaction),
		pendingTxs:   make(map[string]*Transaction),
		webhookCh:    make(chan *Transaction, 100),
		stopCh:       make(chan struct{}),
	}

	logger.Info("e-CNY client created")
	return ecnyClient, nil
}

// Start starts the e-CNY client services
func (e *ECNYClient) Start(ctx context.Context) error {
	e.logger.Info("Starting e-CNY client")

	// Start transaction monitor
	e.wg.Add(1)
	go e.monitorTransactions(ctx)

	// Start webhook processor
	e.wg.Add(1)
	go e.processWebhooks(ctx)

	e.logger.Info("e-CNY client started")
	return nil
}

// Stop stops the e-CNY client
func (e *ECNYClient) Stop() error {
	e.logger.Info("Stopping e-CNY client")

	close(e.stopCh)
	e.wg.Wait()

	e.logger.Info("e-CNY client stopped")
	return nil
}

// monitorTransactions monitors blockchain transactions
func (e *ECNYClient) monitorTransactions(ctx context.Context) {
	defer e.wg.Done()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-e.stopCh:
			return
		case <-ticker.C:
			e.checkPendingTransactions(ctx)
		}
	}
}

// checkPendingTransactions checks the status of pending transactions
func (e *ECNYClient) checkPendingTransactions(ctx context.Context) {
	e.mu.RLock()
	pending := make(map[string]*Transaction)
	for k, v := range e.pendingTxs {
		pending[k] = v
	}
	e.mu.RUnlock()

	for txID, tx := range pending {
		if tx.TxHash == (common.Hash{}) {
			continue
		}

		receipt, err := e.bcClient.GetTransactionReceipt(ctx, tx.TxHash)
		if err != nil {
			e.logger.Warnf("Failed to get receipt for transaction %s: %v", txID, err)
			continue
		}

		e.mu.Lock()
		if receipt != nil {
			tx.BlockHash = receipt.BlockHash
			tx.BlockNumber = receipt.BlockNumber.Uint64()

			if receipt.Status == 1 {
				tx.Status = PaymentStatusCompleted
				delete(e.pendingTxs, txID)
				e.transactions[txID] = tx
				e.logger.Infof("Transaction %s completed", txID)
			} else {
				tx.Status = PaymentStatusFailed
				delete(e.pendingTxs, txID)
				e.transactions[txID] = tx
				e.logger.Warnf("Transaction %s failed", txID)
			}
		}
		e.mu.Unlock()
	}
}

// processWebhooks processes incoming webhooks
func (e *ECNYClient) processWebhooks(ctx context.Context) {
	defer e.wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case <-e.stopCh:
			return
		case tx := <-e.webhookCh:
			e.handleWebhook(ctx, tx)
		}
	}
}

// handleWebhook processes a webhook notification
func (e *ECNYClient) handleWebhook(ctx context.Context, tx *Transaction) {
	e.logger.Infof("Processing webhook for transaction %s", tx.TxID)

	e.mu.Lock()
	if existing, ok := e.transactions[tx.TxID]; ok {
		existing.Status = tx.Status
		existing.BlockHash = tx.BlockHash
		existing.BlockNumber = tx.BlockNumber
		existing.TxHash = tx.TxHash
		existing.UpdatedAt = time.Now()
	} else {
		e.transactions[tx.TxID] = tx
	}
	e.mu.Unlock()
}

// CreateWallet creates a new wallet for an address
func (e *ECNYClient) CreateWallet(ctx context.Context, address common.Address) (*Wallet, error) {
	e.logger.Infof("Creating wallet for address: %s", address.Hex())

	wallet := &Wallet{
		Address:          address,
		Balance:          big.NewInt(0),
		FrozenBalance:    big.NewInt(0),
		AvailableBalance: big.NewInt(0),
		LastUpdated:      time.Now(),
	}

	e.mu.Lock()
	e.wallets[address] = wallet
	e.mu.Unlock()

	return wallet, nil
}

// GetWallet returns the wallet for an address
func (e *ECNYClient) GetWallet(address common.Address) (*Wallet, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	wallet, ok := e.wallets[address]
	if !ok {
		return nil, fmt.Errorf("wallet not found for address: %s", address.Hex())
	}

	return wallet, nil
}

// SyncWalletBalance syncs the wallet balance from blockchain
func (e *ECNYClient) SyncWalletBalance(ctx context.Context, address common.Address) (*Wallet, error) {
	e.logger.Debugf("Syncing wallet balance for: %s", address.Hex())

	balance, err := e.bcClient.GetBalance(ctx, address)
	if err != nil {
		return nil, fmt.Errorf("failed to get balance: %w", err)
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	wallet, ok := e.wallets[address]
	if !ok {
		wallet = &Wallet{Address: address}
		e.wallets[address] = wallet
	}

	wallet.Balance = balance
	wallet.LastUpdated = time.Now()

	// Calculate available balance
	wallet.AvailableBalance = new(big.Int).Sub(wallet.Balance, wallet.FrozenBalance)
	if wallet.AvailableBalance.Sign() < 0 {
		wallet.AvailableBalance = big.NewInt(0)
	}

	return wallet, nil
}

// CreatePayment creates a payment transaction
func (e *ECNYClient) CreatePayment(ctx context.Context, req PaymentRequest) (*Transaction, error) {
	e.logger.Infof("Creating payment: from=%s, to=%s, amount=%s",
		"", req.To.Hex(), req.Amount.String())

	// Validate wallet balance for direct wallet payments
	if req.Method == PaymentMethodWallet {
		wallet, err := e.GetWallet(req.To)
		if err != nil {
			return nil, err
		}

		if wallet.AvailableBalance.Cmp(req.Amount) < 0 {
			return nil, fmt.Errorf("insufficient balance: have %s, need %s",
				wallet.AvailableBalance.String(), req.Amount.String())
		}
	}

	// Generate transaction ID
	txID := generateTxID(req.To, req.Amount, req.OrderID)

	tx := &Transaction{
		TxID:      txID,
		From:      common.HexToAddress("0x0"),
		To:        req.To,
		Amount:    req.Amount,
		Fee:       big.NewInt(0),
		Method:    req.Method,
		Status:    PaymentStatusPending,
		OrderID:   req.OrderID,
		EscrowID:  req.EscrowID,
		Metadata:  req.Metadata,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	e.mu.Lock()
	e.transactions[txID] = tx
	e.pendingTxs[txID] = tx
	e.mu.Unlock()

	return tx, nil
}

// ExecutePayment executes a payment transaction
func (e *ECNYClient) ExecutePayment(ctx context.Context, txID string) (*PaymentResult, error) {
	e.logger.Infof("Executing payment: %s", txID)

	e.mu.RLock()
	tx, ok := e.transactions[txID]
	e.mu.RUnlock()

	if !ok {
		return &PaymentResult{Success: false, Error: fmt.Errorf("transaction not found: %s", txID)}, nil
	}

	if tx.Status != PaymentStatusPending {
		return &PaymentResult{Transaction: tx, Success: false, Error: fmt.Errorf("invalid transaction status: %s", tx.Status)}, nil
	}

	// Update status to processing
	e.mu.Lock()
	tx.Status = PaymentStatusProcessing
	e.mu.Unlock()

	// Send transaction to blockchain
	bcTx, err := e.bcClient.SendTransaction(ctx, tx.To, tx.Amount, nil)
	if err != nil {
		e.mu.Lock()
		tx.Status = PaymentStatusFailed
		e.mu.Unlock()
		return &PaymentResult{Transaction: tx, Success: false, Error: err}, nil
	}

	// Update transaction with blockchain hash
	e.mu.Lock()
	tx.TxHash = bcTx.Hash()
	e.mu.Unlock()

	e.logger.Infof("Payment %s submitted: tx=%s", txID, bcTx.Hash().Hex())

	return &PaymentResult{Transaction: tx, Success: true}, nil
}

// GetTransaction returns a transaction by ID
func (e *ECNYClient) GetTransaction(txID string) (*Transaction, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	tx, ok := e.transactions[txID]
	if !ok {
		return nil, fmt.Errorf("transaction not found: %s", txID)
	}

	return tx, nil
}

// GetTransactionsByAddress returns all transactions for an address
func (e *ECNYClient) GetTransactionsByAddress(address common.Address) ([]*Transaction, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var result []*Transaction
	for _, tx := range e.transactions {
		if tx.From == address || tx.To == address {
			result = append(result, tx)
		}
	}

	return result, nil
}

// GetTransactionsByOrder returns all transactions for an order
func (e *ECNYClient) GetTransactionsByOrder(orderID string) ([]*Transaction, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var result []*Transaction
	for _, tx := range e.transactions {
		if tx.OrderID == orderID {
			result = append(result, tx)
		}
	}

	return result, nil
}

// Refund refunds a payment
func (e *ECNYClient) Refund(ctx context.Context, txID string) (*Transaction, error) {
	e.logger.Infof("Refunding payment: %s", txID)

	e.mu.RLock()
	tx, ok := e.transactions[txID]
	e.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("transaction not found: %s", txID)
	}

	if tx.Status != PaymentStatusCompleted {
		return nil, fmt.Errorf("cannot refund transaction with status: %s", tx.Status)
	}

	// Create refund transaction
	refundTx := &Transaction{
		TxID:      generateTxID(tx.To, tx.Amount, tx.OrderID+"_refund"),
		From:      tx.To,
		To:        tx.From,
		Amount:    tx.Amount,
		Fee:       big.NewInt(0),
		Method:    tx.Method,
		Status:    PaymentStatusPending,
		OrderID:   tx.OrderID + "_refund",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	e.mu.Lock()
	e.transactions[refundTx.TxID] = refundTx
	e.pendingTxs[refundTx.TxID] = refundTx
	e.mu.Unlock()

	return refundTx, nil
}

// FreezeBalance freezes balance for escrow
func (e *ECNYClient) FreezeBalance(address common.Address, amount *big.Int) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	wallet, ok := e.wallets[address]
	if !ok {
		return fmt.Errorf("wallet not found: %s", address.Hex())
	}

	if wallet.AvailableBalance.Cmp(amount) < 0 {
		return fmt.Errorf("insufficient available balance")
	}

	wallet.FrozenBalance = new(big.Int).Add(wallet.FrozenBalance, amount)
	wallet.AvailableBalance = new(big.Int).Sub(wallet.AvailableBalance, amount)

	return nil
}

// UnfreezeBalance unfreezes balance
func (e *ECNYClient) UnfreezeBalance(address common.Address, amount *big.Int) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	wallet, ok := e.wallets[address]
	if !ok {
		return fmt.Errorf("wallet not found: %s", address.Hex())
	}

	if wallet.FrozenBalance.Cmp(amount) < 0 {
		return fmt.Errorf("insufficient frozen balance")
	}

	wallet.FrozenBalance = new(big.Int).Sub(wallet.FrozenBalance, amount)
	wallet.AvailableBalance = new(big.Int).Add(wallet.AvailableBalance, amount)

	return nil
}

// VerifyWebhookSignature verifies the signature of a webhook
func (e *ECNYClient) VerifyWebhookSignature(payload string, signature string) bool {
	if e.cfg.CallbackSecret == "" {
		return true
	}

	expectedSig := generateSignature(payload, e.cfg.CallbackSecret)
	return signature == expectedSig
}

// generateTxID generates a unique transaction ID
func generateTxID(to common.Address, amount *big.Int, orderID string) string {
	data := fmt.Sprintf("%s:%s:%s:%d", to.Hex(), amount.String(), orderID, time.Now().UnixNano())
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}

// generateSignature generates a HMAC signature
func generateSignature(payload string, secret string) string {
	data := []byte(payload)
	key := []byte(secret)

	hash := sha256.Sum256(append(data, key...))
	return hex.EncodeToString(hash[:])
}

// GetTransactionHistory returns transaction history
func (e *ECNYClient) GetTransactionHistory(address common.Address, limit int) ([]*Transaction, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var result []*Transaction
	for _, tx := range e.transactions {
		if tx.From == address || tx.To == address {
			result = append(result, tx)
		}
	}

	// Sort by timestamp descending
	for i := 0; i < len(result)-1; i++ {
		for j := i + 1; j < len(result); j++ {
			if result[j].CreatedAt.After(result[i].CreatedAt) {
				result[i], result[j] = result[j], result[i]
			}
		}
	}

	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}

	return result, nil
}

// FormatTransactionSummary returns a human-readable string
func (t *Transaction) FormatTransactionSummary() string {
	return fmt.Sprintf("Transaction{id=%s, from=%s, to=%s, amount=%s, status=%s}",
		t.TxID[:8], t.From.Hex()[:6], t.To.Hex()[:6], t.Amount.String(), t.Status)
}

// FormatWalletSummary returns a human-readable string for wallet
func (w *Wallet) FormatWalletSummary() string {
	return fmt.Sprintf("Wallet{address=%s, balance=%s, frozen=%s, available=%s}",
		w.Address.Hex()[:6], w.Balance.String(), w.FrozenBalance.String(), w.AvailableBalance.String())
}

// ConvertCNYToWei converts CNY to wei
func ConvertCNYToWei(cny float64) *big.Int {
	wei := big.NewInt(int64(cny * 1e18))
	return wei
}

// ConvertWeiToCNY converts wei to CNY
func ConvertWeiToCNY(wei *big.Int) float64 {
	cny := new(big.Float).SetInt(wei)
	cny = cny.Quo(cny, big.NewFloat(1e18))
	f, _ := cny.Float64()
	return f
}

// ParseMetadata parses metadata JSON
func (t *Transaction) ParseMetadata(v interface{}) error {
	if t.Metadata == "" {
		return nil
	}
	return json.Unmarshal([]byte(t.Metadata), v)
}

// SetMetadata sets metadata from a struct
func (t *Transaction) SetMetadata(v interface{}) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	t.Metadata = string(data)
	return nil
}

// ECNYClientFromConfig creates an e-CNY client from config
func ECNYClientFromConfig(cfg map[string]interface{}, bcClient *client.Client, logger log.Logger) (*ECNYClient, error) {
	gatewayConfig := GatewayConfig{}

	if url, ok := cfg["gateway_url"].(string); ok {
		gatewayConfig.GatewayURL = url
	}

	if apiKey, ok := cfg["api_key"].(string); ok {
		gatewayConfig.APIKey = apiKey
	}

	if merchantID, ok := cfg["merchant_id"].(string); ok {
		gatewayConfig.MerchantID = merchantID
	}

	if timeout, ok := cfg["timeout"].(float64); ok {
		gatewayConfig.Timeout = time.Duration(timeout) * time.Second
	}

	if secret, ok := cfg["callback_secret"].(string); ok {
		gatewayConfig.CallbackSecret = secret
	}

	return NewECNYClient(gatewayConfig, bcClient, logger)
}

// ValidateAddress validates an e-CNY address
func ValidateAddress(address string) bool {
	return strings.HasPrefix(address, "0x") && len(address) == 42
}

// ParseAmount parses a CNY amount string to wei
func ParseAmount(amount string) (*big.Int, error) {
	// Remove any whitespace and currency symbols
	amount = strings.TrimSpace(amount)
	amount = strings.ReplaceAll(amount, ",", "")
	amount = strings.ReplaceAll(amount, "¥", "")
	amount = strings.ReplaceAll(amount, "CNY", "")
	amount = strings.TrimSpace(amount)

	var value float64
	_, err := fmt.Sscanf(amount, "%f", &value)
	if err != nil {
		return nil, fmt.Errorf("invalid amount: %w", err)
	}

	return ConvertCNYToWei(value), nil
}
