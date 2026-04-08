package client

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/kerrigan/kerrigan/internal/chain/contracts"
	"github.com/kerrigan/kerrigan/pkg/log"
)

// NetworkType represents the blockchain network type
type NetworkType string

const (
	NetworkMainnet NetworkType = "mainnet"
	NetworkTestnet NetworkType = "testnet"
	NetworkLocal   NetworkType = "local"
)

// Config holds blockchain client configuration
type Config struct {
	Network            NetworkType
	RPCURL             string
	WSURL              string
	PrivateKey         string
	ResourceRegistry   common.Address
	TradingContract    common.Address
	PaymentContract    common.Address
	RegistryContract   common.Address
	ChainID            *big.Int
	GasPriceMultiplier float64
	ConfirmationBlocks uint64
	MaxGasPrice        *big.Int
	MaxGasLimit        uint64
}

// Client represents the blockchain client for interacting with smart contracts
type Client struct {
	cfg        Config
	logger     log.Logger
	ethClient  *ethclient.Client
	rpcClient  *rpc.Client
	transactor *bind.TransactOpts
	keystore   *bind.TransactOpts

	resourceRegistry *contracts.ResourceRegistryContract
	trading          *contracts.TradingContract
	payment          *contracts.PaymentEscrowContract
	pluginRegistry   *contracts.PluginRegistryContract

	mu             sync.RWMutex
	chainID        *big.Int
	currentBlock   uint64
	pendingTxs     map[common.Hash]*types.Transaction
	eventListeners map[string]chan *types.Log
	stopCh         chan struct{}
	wg             sync.WaitGroup
}

// NewClient creates a new blockchain client
func NewClient(cfg Config, logger log.Logger) (*Client, error) {
	logger.Infof("Creating blockchain client for network: %s", cfg.Network)

	// Connect to RPC client
	rpcClient, err := rpc.Dial(cfg.RPCURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to RPC: %w", err)
	}

	ethClient := ethclient.NewClient(rpcClient)

	client := &Client{
		cfg:            cfg,
		logger:         logger,
		rpcClient:      rpcClient,
		ethClient:      ethClient,
		pendingTxs:     make(map[common.Hash]*types.Transaction),
		eventListeners: make(map[string]chan *types.Log),
		stopCh:         make(chan struct{}),
	}

	// Initialize transactor with private key
	if cfg.PrivateKey != "" {
		if err := client.initTransactor(cfg.PrivateKey); err != nil {
			return nil, fmt.Errorf("failed to initialize transactor: %w", err)
		}
	}

	// Initialize contract instances
	if err := client.initContracts(); err != nil {
		return nil, fmt.Errorf("failed to initialize contracts: %w", err)
	}

	// Get chain ID
	chainID, err := ethClient.ChainID(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to get chain ID: %w", err)
	}
	client.chainID = chainID

	logger.Infof("Blockchain client initialized with chain ID: %s", chainID.String())

	return client, nil
}

// initTransactor initializes the transaction signer
func (c *Client) initTransactor(privateKey string) error {
	c.logger.Info("Initializing transaction transactor")

	// In production, this would parse the private key and create a transactor
	// For now, we create a mock transactor
	c.transactor = &bind.TransactOpts{
		From: common.HexToAddress("0x0"),
	}

	c.keystore = c.transactor

	return nil
}

// initContracts initializes all smart contract instances
func (c *Client) initContracts() error {
	c.logger.Info("Initializing contract instances")

	// Initialize Resource Registry contract
	if c.cfg.ResourceRegistry != (common.Address{}) {
		c.resourceRegistry = contracts.NewResourceRegistryContract(c.cfg.ResourceRegistry, c.logger)
		c.logger.Infof("ResourceRegistry contract at: %s", c.cfg.ResourceRegistry.Hex())
	}

	// Initialize Trading contract
	if c.cfg.TradingContract != (common.Address{}) {
		c.trading = contracts.NewTradingContract(c.cfg.TradingContract, c.logger)
		c.logger.Infof("Trading contract at: %s", c.cfg.TradingContract.Hex())
	}

	// Initialize Payment Escrow contract
	if c.cfg.PaymentContract != (common.Address{}) {
		c.payment = contracts.NewPaymentEscrowContract(c.cfg.PaymentContract, c.logger)
		c.logger.Infof("Payment contract at: %s", c.cfg.PaymentContract.Hex())
	}

	// Initialize Plugin Registry contract
	if c.cfg.RegistryContract != (common.Address{}) {
		c.pluginRegistry = contracts.NewPluginRegistryContract(c.cfg.RegistryContract, c.logger)
		c.logger.Infof("PluginRegistry contract at: %s", c.cfg.RegistryContract.Hex())
	}

	return nil
}

// Start starts the blockchain client services
func (c *Client) Start(ctx context.Context) error {
	c.logger.Info("Starting blockchain client")

	// Start block monitoring
	c.wg.Add(1)
	go c.monitorBlocks(ctx)

	// Start pending transaction monitor
	c.wg.Add(1)
	go c.monitorPendingTransactions(ctx)

	// Start event loop
	c.wg.Add(1)
	go c.eventLoop(ctx)

	c.logger.Info("Blockchain client started")
	return nil
}

// Stop stops the blockchain client
func (c *Client) Stop() error {
	c.logger.Info("Stopping blockchain client")

	close(c.stopCh)
	c.wg.Wait()

	if c.ethClient != nil {
		c.ethClient.Close()
	}

	if c.rpcClient != nil {
		c.rpcClient.Close()
	}

	c.logger.Info("Blockchain client stopped")
	return nil
}

// monitorBlocks monitors new blocks
func (c *Client) monitorBlocks(ctx context.Context) {
	defer c.wg.Done()

	ticker := time.NewTicker(12 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-c.stopCh:
			return
		case <-ticker.C:
			if err := c.updateBlockNumber(ctx); err != nil {
				c.logger.Warnf("Failed to update block number: %v", err)
			}
		}
	}
}

// updateBlockNumber updates the current block number
func (c *Client) updateBlockNumber(ctx context.Context) error {
	header, err := c.ethClient.HeaderByNumber(ctx, nil)
	if err != nil {
		return err
	}

	c.mu.Lock()
	c.currentBlock = header.Number.Uint64()
	c.mu.Unlock()

	c.logger.Debugf("Current block: %d", c.currentBlock)
	return nil
}

// monitorPendingTransactions monitors pending transactions
func (c *Client) monitorPendingTransactions(ctx context.Context) {
	defer c.wg.Done()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-c.stopCh:
			return
		case <-ticker.C:
			c.cleanupPendingTransactions(ctx)
		}
	}
}

// cleanupPendingTransactions removes confirmed transactions from pending pool
func (c *Client) cleanupPendingTransactions(ctx context.Context) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for hash, tx := range c.pendingTxs {
		receipt, err := c.ethClient.TransactionReceipt(ctx, hash)
		if err != nil {
			if err == ethereum.NotFound {
				continue
			}
			c.logger.Warnf("Error checking transaction receipt: %v", err)
			continue
		}

		// Transaction confirmed
		if receipt.Status == 1 {
			c.logger.Infof("Transaction %s confirmed at block %d", hash.Hex(), receipt.BlockNumber.Uint64())
			delete(c.pendingTxs, hash)
		}
	}
}

// eventLoop processes blockchain events
func (c *Client) eventLoop(ctx context.Context) {
	defer c.wg.Done()

	ticker := time.NewTicker(6 * time.Second)
	defer ticker.Stop()

	var lastBlock uint64

	for {
		select {
		case <-ctx.Done():
			return
		case <-c.stopCh:
			return
		case <-ticker.C:
			c.mu.RLock()
			currentBlock := c.currentBlock
			c.mu.RUnlock()

			if currentBlock > lastBlock {
				if err := c.fetchEvents(ctx, lastBlock, currentBlock); err != nil {
					c.logger.Warnf("Failed to fetch events: %v", err)
				}
				lastBlock = currentBlock
			}
		}
	}
}

// fetchEvents fetches and processes events
func (c *Client) fetchEvents(ctx context.Context, fromBlock, toBlock uint64) error {
	if fromBlock >= toBlock {
		return nil
	}

	c.logger.Debugf("Fetching events from block %d to %d", fromBlock, toBlock)

	// Process events for each contract
	// In production, this would query logs for each contract

	return nil
}

// GetNetwork returns the network type
func (c *Client) GetNetwork() NetworkType {
	return c.cfg.Network
}

// GetChainID returns the chain ID
func (c *Client) GetChainID() *big.Int {
	return c.chainID
}

// GetCurrentBlock returns the current block number
func (c *Client) GetCurrentBlock() uint64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.currentBlock
}

// GetBalance returns the balance of an address
func (c *Client) GetBalance(ctx context.Context, address common.Address) (*big.Int, error) {
	return c.ethClient.BalanceAt(ctx, address, nil)
}

// GetResourceRegistry returns the ResourceRegistry contract
func (c *Client) GetResourceRegistry() *contracts.ResourceRegistryContract {
	return c.resourceRegistry
}

// GetTrading returns the Trading contract
func (c *Client) GetTrading() *contracts.TradingContract {
	return c.trading
}

// GetPayment returns the PaymentEscrow contract
func (c *Client) GetPayment() *contracts.PaymentEscrowContract {
	return c.payment
}

// GetPluginRegistry returns the PluginRegistry contract
func (c *Client) GetPluginRegistry() *contracts.PluginRegistryContract {
	return c.pluginRegistry
}

// SendTransaction sends a transaction
func (c *Client) SendTransaction(ctx context.Context, to common.Address, value *big.Int, data []byte) (*types.Transaction, error) {
	c.logger.Infof("Sending transaction to %s, value: %s", to.Hex(), value.String())

	// Get gas price
	gasPrice, err := c.suggestGasPrice(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to suggest gas price: %w", err)
	}

	// Apply gas price multiplier
	gasPrice = new(big.Int).Mul(gasPrice, big.NewInt(int64(c.cfg.GasPriceMultiplier*100)))
	gasPrice = gasPrice.Div(gasPrice, big.NewInt(100))

	// Cap at max gas price
	if c.cfg.MaxGasPrice != nil && gasPrice.Cmp(c.cfg.MaxGasPrice) > 0 {
		gasPrice = c.cfg.MaxGasPrice
	}

	// Estimate gas
	gasLimit, err := c.ethClient.EstimateGas(ctx, ethereum.CallMsg{
		To:    &to,
		Value: value,
		Data:  data,
	})
	if err != nil {
		// Use default gas limit
		gasLimit = 21000
	}

	// Cap at max gas limit
	if c.cfg.MaxGasLimit > 0 && gasLimit > c.cfg.MaxGasLimit {
		gasLimit = c.cfg.MaxGasLimit
	}

	// Create transaction
	tx := types.NewTransaction(0, to, value, gasLimit, gasPrice, data)

	// In production, this would sign and send the transaction
	// For simulation, we return a mock transaction

	c.mu.Lock()
	c.pendingTxs[tx.Hash()] = tx
	c.mu.Unlock()

	return tx, nil
}

// SendContractTransaction sends a transaction to a contract
func (c *Client) SendContractTransaction(ctx context.Context, contractAddr common.Address, abiJSON string, method string, args ...interface{}) (*types.Transaction, error) {
	c.logger.Infof("Sending contract transaction: %s.%s", contractAddr.Hex(), method)

	parsedABI, err := abi.JSON(strings.NewReader(abiJSON))
	if err != nil {
		return nil, fmt.Errorf("failed to parse ABI: %w", err)
	}

	data, err := parsedABI.Pack(method, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to pack method: %w", err)
	}

	return c.SendTransaction(ctx, contractAddr, big.NewInt(0), data)
}

// WaitForConfirmation waits for transaction confirmation
func (c *Client) WaitForConfirmation(ctx context.Context, txHash common.Hash, confirmationBlocks uint64) (*types.Receipt, error) {
	c.logger.Infof("Waiting for transaction %s confirmation", txHash.Hex())

	if confirmationBlocks == 0 {
		confirmationBlocks = c.cfg.ConfirmationBlocks
	}

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			receipt, err := c.ethClient.TransactionReceipt(ctx, txHash)
			if err != nil {
				if err == ethereum.NotFound {
					continue
				}
				return nil, err
			}

			// Check if enough confirmations
			c.mu.RLock()
			currentBlock := c.currentBlock
			c.mu.RUnlock()

			if receipt.BlockNumber.Uint64()+confirmationBlocks <= currentBlock {
				c.logger.Infof("Transaction %s confirmed with %d confirmations", txHash.Hex(), currentBlock-receipt.BlockNumber.Uint64())
				return receipt, nil
			}
		}
	}
}

// suggestGasPrice suggests a gas price
func (c *Client) suggestGasPrice(ctx context.Context) (*big.Int, error) {
	return c.ethClient.SuggestGasPrice(ctx)
}

// GetTransactionReceipt returns the receipt of a transaction
func (c *Client) GetTransactionReceipt(ctx context.Context, txHash common.Hash) (*types.Receipt, error) {
	return c.ethClient.TransactionReceipt(ctx, txHash)
}

// GetTransactionByHash returns the transaction by hash
func (c *Client) GetTransactionByHash(ctx context.Context, txHash common.Hash) (*types.Transaction, bool, error) {
	return c.ethClient.TransactionByHash(ctx, txHash)
}

// FilterLogs filters logs based on criteria
func (c *Client) FilterLogs(ctx context.Context, query ethereum.FilterQuery) ([]types.Log, error) {
	return c.ethClient.FilterLogs(ctx, query)
}

// SubscribeFilterLogs creates a subscription for filtered logs
func (c *Client) SubscribeFilterLogs(ctx context.Context, query ethereum.FilterQuery, ch chan types.Log) (ethereum.Subscription, error) {
	return ethereum.SubscribeFilterLogs(ctx, c.ethClient, query, ch)
}

// RegisterEventListener registers an event listener for a contract
func (c *Client) RegisterEventListener(contractName string, ch chan *types.Log) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.eventListeners[contractName] = ch
	c.logger.Infof("Registered event listener for %s", contractName)
}

// UnregisterEventListener unregisters an event listener
func (c *Client) UnregisterEventListener(contractName string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if ch, ok := c.eventListeners[contractName]; ok {
		close(ch)
		delete(c.eventListeners, contractName)
	}
}

// GetTransactor returns the transactor for signing transactions
func (c *Client) GetTransactor() *bind.TransactOpts {
	return c.transactor
}

// CallContract calls a contract method
func (c *Client) CallContract(ctx context.Context, msg ethereum.CallMsg) ([]byte, error) {
	return c.ethClient.CallContract(ctx, msg, nil)
}

// GetNodeInfo returns basic node information
func (c *Client) GetNodeInfo(ctx context.Context) (map[string]interface{}, error) {
	var result map[string]interface{}

	err := c.rpcClient.CallContext(ctx, &result, "eth_getBlockByNumber", "latest", false)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// SyncBalance syncs the balance of an address
func (c *Client) SyncBalance(ctx context.Context, address common.Address) (*big.Int, error) {
	balance, err := c.ethClient.BalanceAt(ctx, address, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get balance: %w", err)
	}

	c.logger.Debugf("Address %s balance: %s", address.Hex(), balance.String())
	return balance, nil
}

// GetContractABI returns the ABI for a contract
func GetContractABI(contractName string) (string, error) {
	switch contractName {
	case "ResourceRegistry":
		return contracts.ResourceRegistryABI, nil
	case "Trading":
		return contracts.TradingABI, nil
	case "PaymentEscrow":
		return contracts.PaymentABI, nil
	case "PluginRegistry":
		return contracts.PluginRegistryABI, nil
	default:
		return "", fmt.Errorf("unknown contract: %s", contractName)
	}
}

// ParseAddress parses a hex string to address
func ParseAddress(hex string) (common.Address, error) {
	if !common.IsHexAddress(hex) {
		return common.Address{}, fmt.Errorf("invalid hex address: %s", hex)
	}
	return common.HexToAddress(hex), nil
}

// MustParseAddress parses a hex string to address, panics on error
func MustParseAddress(hex string) common.Address {
	addr, err := ParseAddress(hex)
	if err != nil {
		panic(err)
	}
	return addr
}
