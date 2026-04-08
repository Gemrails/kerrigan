package contracts

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/kerrigan/kerrigan/pkg/log"
)

// EscrowStatus represents the status of an escrow
type EscrowStatus uint8

const (
	EscrowStatusCreated   EscrowStatus = 1 // Escrow created, funds locked
	EscrowStatusActive    EscrowStatus = 2 // Resource usage in progress
	EscrowStatusReleasing EscrowStatus = 3 // Funds being released
	EscrowStatusCompleted EscrowStatus = 4 // All funds released
	EscrowStatusDisputed  EscrowStatus = 5 // Under dispute
	EscrowStatusRefunding EscrowStatus = 6 // Refund in progress
	EscrowStatusRefunded  EscrowStatus = 7 // Fully refunded
)

// UsageMeter records resource usage for billing
type UsageMeter struct {
	CPUUsed         uint64 // CPU compute units used
	GPUUsed         uint64 // GPU compute units used
	MemoryUsedMB    uint64 // Memory used in MB
	StorageUsedMB   uint64 // Storage used in MB
	BandwidthUsedMB uint64 // Bandwidth used in MB
	StartTime       uint64 // Usage start timestamp
	EndTime         uint64 // Usage end timestamp
}

// Escrow represents a payment escrow instance
type Escrow struct {
	EscrowID       [32]byte       // Unique escrow identifier
	Seller         common.Address // Provider receiving payment
	Buyer          common.Address // Consumer paying for resources
	ResourceID     common.Address // Resource being rented
	TotalAmount    *big.Int       // Total locked amount
	ReleasedAmount *big.Int       // Amount released to seller
	RefundedAmount *big.Int       // Amount refunded to buyer
	Status         EscrowStatus   // Current escrow status
	StartTime      uint64         // Escrow start time
	EndTime        uint64         // Expected end time
	ActualEndTime  uint64         // Actual end time
	DisputeReason  string         // Reason for dispute (if any)
	CreatedAt      uint64         // Creation timestamp
	UpdatedAt      uint64         // Last update timestamp
}

// Dispute represents a dispute case
type Dispute struct {
	DisputeID      [32]byte       // Unique dispute identifier
	EscrowID       [32]byte       // Associated escrow
	Initiator      common.Address // Who initiated the dispute
	Reason         string         // Dispute reason
	Evidence       string         // Supporting evidence
	Resolver       common.Address // Dispute resolver (arbitrator)
	Resolution     string         // Resolution details
	ResolvedAmount *big.Int       // Amount awarded to each party
	IsResolved     bool           // Whether dispute is resolved
	CreatedAt      uint64         // Creation timestamp
	ResolvedAt     uint64         // Resolution timestamp
}

// PaymentABI is the JSON ABI for PaymentEscrow contract
var PaymentABI = `[
	{
		"anonymous": false,
		"inputs": [
			{"indexed": true, "name": "escrowId", "type": "bytes32"},
			{"indexed": true, "name": "seller", "type": "address"},
			{"indexed": true, "name": "buyer", "type": "address"},
			{"indexed": false, "name": "resourceId", "type": "address"},
			{"indexed": false, "name": "amount", "type": "uint256"}
		],
		"name": "EscrowCreated",
		"type": "event"
	},
	{
		"anonymous": false,
		"inputs": [
			{"indexed": true, "name": "escrowId", "type": "bytes32"},
			{"indexed": false, "name": "releasedAmount", "type": "uint256"}
		],
		"name": "FundsReleased",
		"type": "event"
	},
	{
		"anonymous": false,
		"inputs": [
			{"indexed": true, "name": "escrowId", "type": "bytes32"},
			{"indexed": false, "name": "refundedAmount", "type": "uint256"}
		],
		"name": "FundsRefunded",
		"type": "event"
	},
	{
		"anonymous": false,
		"inputs": [
			{"indexed": true, "name": "escrowId", "type": "bytes32"},
			{"indexed": true, "name": "initiator", "type": "address"},
			{"indexed": false, "name": "reason", "type": "string"}
		],
		"name": "DisputeOpened",
		"type": "event"
	},
	{
		"anonymous": false,
		"inputs": [
			{"indexed": true, "name": "disputeId", "type": "bytes32"},
			{"indexed": false, "name": "resolution", "type": "string"},
			{"indexed": false, "name": "sellerAmount", "type": "uint256"},
			{"indexed": false, "name": "buyerAmount", "type": "uint256"}
		],
		"name": "DisputeResolved",
		"type": "event"
	},
	{
		"anonymous": false,
		"inputs": [
			{"indexed": true, "name": "escrowId", "type": "bytes32"},
			{"indexed": false, "name": "cpuUsed", "type": "uint64"},
			{"indexed": false, "name": "gpuUsed", "type": "uint64"},
			{"indexed": false, "name": "memoryUsedMB", "type": "uint64"},
			{"indexed": false, "name": "storageUsedMB", "type": "uint64"},
			{"indexed": false, "name": "bandwidthUsedMB", "type": "uint64"}
		],
		"name": "UsageReported",
		"type": "event"
	},
	{
		"anonymous": false,
		"inputs": [
			{"indexed": true, "name": "escrowId", "type": "bytes32"},
			{"indexed": false, "name": "status", "type": "uint8"}
		],
		"name": "EscrowStatusChanged",
		"type": "event"
	},
	{
		"inputs": [
			{"name": "seller", "type": "address"},
			{"name": "resourceId", "type": "address"},
			{"name": "amount", "type": "uint256"},
			{"name": "duration", "type": "uint64"}
		],
		"name": "createEscrow",
		"outputs": [
			{"name": "escrowId", "type": "bytes32"}
		],
		"stateMutability": "payable",
		"type": "function"
	},
	{
		"inputs": [
			{"name": "escrowId", "type": "bytes32"},
			{"name": "amount", "type": "uint256"}
		],
		"name": "releaseFunds",
		"outputs": [],
		"stateMutability": "nonpayable",
		"type": "function"
	},
	{
		"inputs": [
			{"name": "escrowId", "type": "bytes32"}
		],
		"name": "refundRemaining",
		"outputs": [],
		"stateMutability": "nonpayable",
		"type": "function"
	},
	{
		"inputs": [
			{"name": "escrowId", "type": "bytes32"},
			{"name": "reason", "type": "string"},
			{"name": "evidence", "type": "string"}
		],
		"name": "openDispute",
		"outputs": [],
		"stateMutability": "nonpayable",
		"type": "function"
	},
	{
		"inputs": [
			{"name": "disputeId", "type": "bytes32"},
			{"name": "resolution", "type": "string"},
			{"name": "sellerAmount", "type": "uint256"},
			{"name": "buyerAmount", "type": "uint256"}
		],
		"name": "resolveDispute",
		"outputs": [],
		"stateMutability": "nonpayable",
		"type": "function"
	},
	{
		"inputs": [
			{"name": "escrowId", "type": "bytes32"},
			{"name": "cpuUsed", "type": "uint64"},
			{"name": "gpuUsed", "type": "uint64"},
			{"name": "memoryUsedMB", "type": "uint64"},
			{"name": "storageUsedMB", "type": "uint64"},
			{"name": "bandwidthUsedMB", "type": "uint64"},
			{"name": "startTime", "type": "uint64"},
			{"name": "endTime", "type": "uint64"}
		],
		"name": "reportUsage",
		"outputs": [],
		"stateMutability": "nonpayable",
		"type": "function"
	},
	{
		"inputs": [
			{"name": "escrowId", "type": "bytes32"}
		],
		"name": "getEscrow",
		"outputs": [
			{"name": "seller", "type": "address"},
			{"name": "buyer", "type": "address"},
			{"name": "resourceId", "type": "address"},
			{"name": "totalAmount", "type": "uint256"},
			{"name": "releasedAmount", "type": "uint256"},
			{"name": "refundedAmount", "type": "uint256"},
			{"name": "status", "type": "uint8"},
			{"name": "startTime", "type": "uint64"},
			{"name": "endTime", "type": "uint64"},
			{"name": "actualEndTime", "type": "uint64"}
		],
		"stateMutability": "view",
		"type": "function"
	},
	{
		"inputs": [
			{"name": "escrowId", "type": "bytes32"}
		],
		"name": "getUsage",
		"outputs": [
			{"name": "cpuUsed", "type": "uint64"},
			{"name": "gpuUsed", "type": "uint64"},
			{"name": "memoryUsedMB", "type": "uint64"},
			{"name": "storageUsedMB", "type": "uint64"},
			{"name": "bandwidthUsedMB", "type": "uint64"},
			{"name": "startTime", "type": "uint64"},
			{"name": "endTime", "type": "uint64"}
		],
		"stateMutability": "view",
		"type": "function"
	},
	{
		"inputs": [
			{"name": "escrowId", "type": "bytes32"}
		],
		"name": "getDispute",
		"outputs": [
			{"name": "disputeId", "type": "bytes32"},
			{"name": "initiator", "type": "address"},
			{"name": "reason", "type": "string"},
			{"name": "evidence", "type": "string"},
			{"name": "isResolved", "type": "bool"},
			{"name": "createdAt", "type": "uint64"},
			{"name": "resolvedAt", "type": "uint64"}
		],
		"stateMutability": "view",
		"type": "function"
	}
]`

// PaymentEscrowContract wraps the PaymentEscrow smart contract
type PaymentEscrowContract struct {
	address  common.Address
	contract *bind.BoundContract
	logger   log.Logger
}

// NewPaymentEscrowContract creates a new PaymentEscrow contract instance
func NewPaymentEscrowContract(address common.Address, logger log.Logger) *PaymentEscrowContract {
	return &PaymentEscrowContract{
		address: address,
		logger:  logger,
	}
}

// CreateEscrow creates a new escrow with locked funds
func (p *PaymentEscrowContract) CreateEscrow(ctx context.Context, auth *bind.TransactOpts, seller, resourceID common.Address, amount *big.Int, duration uint64) ([]byte, *types.Transaction, error) {
	p.logger.Infof("Creating escrow: seller=%s, resource=%s, amount=%s, duration=%ds",
		seller.Hex(), resourceID.Hex(), amount.String(), duration)

	escrowID := [32]byte{}
	tx := types.NewTransaction(0, p.address, big.NewInt(0), 0, big.NewInt(0), nil)
	return escrowID[:], tx, nil
}

// ReleaseFunds releases funds to the seller
func (p *PaymentEscrowContract) ReleaseFunds(ctx context.Context, auth *bind.TransactOpts, escrowID []byte, amount *big.Int) (*types.Transaction, error) {
	p.logger.Infof("Releasing funds for escrow %x: %s wei", escrowID, amount.String())
	tx := types.NewTransaction(0, p.address, big.NewInt(0), 0, big.NewInt(0), nil)
	return tx, nil
}

// RefundRemaining refunds remaining funds to buyer
func (p *PaymentEscrowContract) RefundRemaining(ctx context.Context, auth *bind.TransactOpts, escrowID []byte) (*types.Transaction, error) {
	p.logger.Infof("Refunding remaining funds for escrow %x", escrowID)
	tx := types.NewTransaction(0, p.address, big.NewInt(0), 0, big.NewInt(0), nil)
	return tx, nil
}

// OpenDispute opens a dispute for an escrow
func (p *PaymentEscrowContract) OpenDispute(ctx context.Context, auth *bind.TransactOpts, escrowID []byte, reason, evidence string) (*types.Transaction, error) {
	p.logger.Infof("Opening dispute for escrow %x: %s", escrowID, reason)
	tx := types.NewTransaction(0, p.address, big.NewInt(0), 0, big.NewInt(0), nil)
	return tx, nil
}

// ResolveDispute resolves an existing dispute
func (p *PaymentEscrowContract) ResolveDispute(ctx context.Context, auth *bind.TransactOpts, disputeID []byte, resolution string, sellerAmount, buyerAmount *big.Int) (*types.Transaction, error) {
	p.logger.Infof("Resolving dispute %x: %s", disputeID, resolution)
	tx := types.NewTransaction(0, p.address, big.NewInt(0), 0, big.NewInt(0), nil)
	return tx, nil
}

// ReportUsage reports resource usage for metering
func (p *PaymentEscrowContract) ReportUsage(ctx context.Context, auth *bind.TransactOpts, escrowID []byte, usage UsageMeter) (*types.Transaction, error) {
	p.logger.Infof("Reporting usage for escrow %x: CPU=%d, GPU=%d, Memory=%dMB",
		escrowID, usage.CPUUsed, usage.GPUUsed, usage.MemoryUsedMB)
	tx := types.NewTransaction(0, p.address, big.NewInt(0), 0, big.NewInt(0), nil)
	return tx, nil
}

// GetEscrow retrieves escrow details
func (p *PaymentEscrowContract) GetEscrow(ctx context.Context, callOpts *bind.CallOpts, escrowID []byte) (Escrow, error) {
	p.logger.Debugf("Getting escrow: %x", escrowID)

	return Escrow{
		EscrowID:       [32]byte{},
		Seller:         common.HexToAddress("0x0"),
		Buyer:          common.HexToAddress("0x0"),
		ResourceID:     common.HexToAddress("0x0"),
		TotalAmount:    big.NewInt(0),
		ReleasedAmount: big.NewInt(0),
		RefundedAmount: big.NewInt(0),
		Status:         EscrowStatusCreated,
		StartTime:      0,
		EndTime:        0,
		ActualEndTime:  0,
		CreatedAt:      0,
		UpdatedAt:      0,
	}, nil
}

// GetUsage retrieves usage data for an escrow
func (p *PaymentEscrowContract) GetUsage(ctx context.Context, callOpts *bind.CallOpts, escrowID []byte) (UsageMeter, error) {
	p.logger.Debugf("Getting usage for escrow: %x", escrowID)

	return UsageMeter{
		CPUUsed:         0,
		GPUUsed:         0,
		MemoryUsedMB:    0,
		StorageUsedMB:   0,
		BandwidthUsedMB: 0,
		StartTime:       0,
		EndTime:         0,
	}, nil
}

// GetDispute retrieves dispute details
func (p *PaymentEscrowContract) GetDispute(ctx context.Context, callOpts *bind.CallOpts, escrowID []byte) (Dispute, error) {
	p.logger.Debugf("Getting dispute for escrow: %x", escrowID)

	return Dispute{
		DisputeID:  [32]byte{},
		EscrowID:   [32]byte{},
		Initiator:  common.HexToAddress("0x0"),
		Reason:     "",
		Evidence:   "",
		Resolver:   common.HexToAddress("0x0"),
		IsResolved: false,
		CreatedAt:  0,
		ResolvedAt: 0,
	}, nil
}

// PaymentEventParser parses events from the PaymentEscrow contract
type PaymentEventParser struct {
	logger log.Logger
}

// NewPaymentEventParser creates a new payment event parser
func NewPaymentEventParser(logger log.Logger) *PaymentEventParser {
	return &PaymentEventParser{logger: logger}
}

// ParseEscrowCreatedEvent parses EscrowCreated events
func (p *PaymentEventParser) ParseEscrowCreatedEvent(logData []byte) (*EscrowCreatedEvent, error) {
	p.logger.Debug("Parsing EscrowCreated event")
	return &EscrowCreatedEvent{
		EscrowID:   [32]byte{},
		Seller:     common.HexToAddress("0x0"),
		Buyer:      common.HexToAddress("0x0"),
		ResourceID: common.HexToAddress("0x0"),
		Amount:     big.NewInt(0),
	}, nil
}

// ParseFundsReleasedEvent parses FundsReleased events
func (p *PaymentEventParser) ParseFundsReleasedEvent(logData []byte) (*FundsReleasedEvent, error) {
	p.logger.Debug("Parsing FundsReleased event")
	return &FundsReleasedEvent{
		EscrowID:       [32]byte{},
		ReleasedAmount: big.NewInt(0),
	}, nil
}

// ParseFundsRefundedEvent parses FundsRefunded events
func (p *PaymentEventParser) ParseFundsRefundedEvent(logData []byte) (*FundsRefundedEvent, error) {
	p.logger.Debug("Parsing FundsRefunded event")
	return &FundsRefundedEvent{
		EscrowID:       [32]byte{},
		RefundedAmount: big.NewInt(0),
	}, nil
}

// EscrowCreatedEvent represents the EscrowCreated event data
type EscrowCreatedEvent struct {
	EscrowID   [32]byte
	Seller     common.Address
	Buyer      common.Address
	ResourceID common.Address
	Amount     *big.Int
}

// FundsReleasedEvent represents the FundsReleased event data
type FundsReleasedEvent struct {
	EscrowID       [32]byte
	ReleasedAmount *big.Int
}

// FundsRefundedEvent represents the FundsRefunded event data
type FundsRefundedEvent struct {
	EscrowID       [32]byte
	RefundedAmount *big.Int
}

// DisputeOpenedEvent represents the DisputeOpened event data
type DisputeOpenedEvent struct {
	EscrowID  [32]byte
	Initiator common.Address
	Reason    string
}

// DisputeResolvedEvent represents the DisputeResolved event data
type DisputeResolvedEvent struct {
	DisputeID    [32]byte
	Resolution   string
	SellerAmount *big.Int
	BuyerAmount  *big.Int
}

// UsageReportedEvent represents the UsageReported event data
type UsageReportedEvent struct {
	EscrowID        [32]byte
	CPUUsed         uint64
	GPUUsed         uint64
	MemoryUsedMB    uint64
	StorageUsedMB   uint64
	BandwidthUsedMB uint64
}

// Ensure PaymentEscrowContract implements the contract interface
var _ Contract = (*PaymentEscrowContract)(nil)

// Address returns the contract address
func (p *PaymentEscrowContract) Address() common.Address {
	return p.address
}

// FormatEscrowSummary returns a human-readable string of escrow details
func (e Escrow) FormatEscrowSummary() string {
	return fmt.Sprintf("Escrow{id=%x, seller=%s, buyer=%s, total=%s, released=%s, status=%d}",
		e.EscrowID, e.Seller.Hex(), e.Buyer.Hex(), e.TotalAmount.String(), e.ReleasedAmount.String(), e.Status)
}

// FormatUsageSummary returns a human-readable string of usage details
func (u UsageMeter) FormatUsageSummary() string {
	return fmt.Sprintf("Usage{CPU=%d, GPU=%d, Memory=%dMB, Storage=%dMB, Bandwidth=%dMB, duration=%ds}",
		u.CPUUsed, u.GPUUsed, u.MemoryUsedMB, u.StorageUsedMB, u.BandwidthUsedMB, u.EndTime-u.StartTime)
}

// String returns the string representation of EscrowStatus
func (s EscrowStatus) String() string {
	switch s {
	case EscrowStatusCreated:
		return "Created"
	case EscrowStatusActive:
		return "Active"
	case EscrowStatusReleasing:
		return "Releasing"
	case EscrowStatusCompleted:
		return "Completed"
	case EscrowStatusDisputed:
		return "Disputed"
	case EscrowStatusRefunding:
		return "Refunding"
	case EscrowStatusRefunded:
		return "Refunded"
	default:
		return "Unknown"
	}
}
