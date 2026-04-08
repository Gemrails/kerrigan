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

// ResourceCapabilities represents the capabilities of a compute node
type ResourceCapabilities struct {
	CPUCores      uint8  // Number of CPU cores
	GPUCount      uint8  // Number of GPUs
	MemoryGB      uint64 // Memory in GB
	StorageGB     uint64 // Storage in GB
	BandwidthMbps uint64 // Bandwidth in Mbps
	GPUModel      string // GPU model (e.g., "A100", "V100")
	GPUVRAMGB     uint8  // VRAM per GPU in GB
}

// NodeInfo represents registered node information
type NodeInfo struct {
	Owner             common.Address
	Capabilities      ResourceCapabilities
	PricePerHour      *big.Int // Price per hour in wei
	MinRentalDuration uint64   // Minimum rental duration in seconds
	MaxRentalDuration uint64   // Maximum rental duration in seconds
	IsActive          bool
	RegisteredAt      uint64
	LastUpdated       uint64
}

// ResourceRegistryABI is the JSON ABI for ResourceRegistry contract
var ResourceRegistryABI = `[
	{
		"anonymous": false,
		"inputs": [
			{"indexed": true, "name": "nodeId", "type": "address"},
			{"indexed": false, "name": "owner", "type": "address"},
			{"indexed": false, "name": "cpuCores", "type": "uint8"},
			{"indexed": false, "name": "gpuCount", "type": "uint8"},
			{"indexed": false, "name": "memoryGB", "type": "uint64"},
			{"indexed": false, "name": "storageGB", "type": "uint64"},
			{"indexed": false, "name": "bandwidthMbps", "type": "uint64"},
			{"indexed": false, "name": "gpuModel", "type": "string"},
			{"indexed": false, "name": "gpuVramGB", "type": "uint8"}
		],
		"name": "NodeRegistered",
		"type": "event"
	},
	{
		"anonymous": false,
		"inputs": [
			{"indexed": true, "name": "nodeId", "type": "address"},
			{"indexed": false, "name": "newPrice", "type": "uint256"}
		],
		"name": "PriceUpdated",
		"type": "event"
	},
	{
		"anonymous": false,
		"inputs": [
			{"indexed": true, "name": "nodeId", "type": "address"},
			{"indexed": false, "name": "active", "type": "bool"}
		],
		"name": "NodeStatusChanged",
		"type": "event"
	},
	{
		"anonymous": false,
		"inputs": [
			{"indexed": true, "name": "nodeId", "type": "address"},
			{"indexed": false, "name": "capabilities", "type": "string"}
		],
		"name": "CapabilitiesUpdated",
		"type": "event"
	},
	{
		"inputs": [
			{"name": "cpuCores", "type": "uint8"},
			{"name": "gpuCount", "type": "uint8"},
			{"name": "memoryGB", "type": "uint64"},
			{"name": "storageGB", "type": "uint64"},
			{"name": "bandwidthMbps", "type": "uint64"},
			{"name": "gpuModel", "type": "string"},
			{"name": "gpuVramGB", "type": "uint8"},
			{"name": "pricePerHour", "type": "uint256"},
			{"name": "minRentalDuration", "type": "uint64"},
			{"name": "maxRentalDuration", "type": "uint64"}
		],
		"name": "registerNode",
		"outputs": [],
		"stateMutability": "nonpayable",
		"type": "function"
	},
	{
		"inputs": [
			{"name": "nodeId", "type": "address"}
		],
		"name": "updateCapabilities",
		"outputs": [],
		"stateMutability": "nonpayable",
		"type": "function"
	},
	{
		"inputs": [
			{"name": "nodeId", "type": "address"},
			{"name": "newPrice", "type": "uint256"}
		],
		"name": "updatePrice",
		"outputs": [],
		"stateMutability": "nonpayable",
		"type": "function"
	},
	{
		"inputs": [
			{"name": "nodeId", "type": "address"},
			{"name": "active", "type": "bool"}
		],
		"name": "setNodeActive",
		"outputs": [],
		"stateMutability": "nonpayable",
		"type": "function"
	},
	{
		"inputs": [
			{"name": "nodeId", "type": "address"}
		],
		"name": "getNodeInfo",
		"outputs": [
			{"name": "owner", "type": "address"},
			{"name": "cpuCores", "type": "uint8"},
			{"name": "gpuCount", "type": "uint8"},
			{"name": "memoryGB", "type": "uint64"},
			{"name": "storageGB", "type": "uint64"},
			{"name": "bandwidthMbps", "type": "uint64"},
			{"name": "gpuModel", "type": "string"},
			{"name": "gpuVramGB", "type": "uint8"},
			{"name": "pricePerHour", "type": "uint256"},
			{"name": "minRentalDuration", "type": "uint64"},
			{"name": "maxRentalDuration", "type": "uint64"},
			{"name": "isActive", "type": "bool"},
			{"name": "registeredAt", "type": "uint64"},
			{"name": "lastUpdated", "type": "uint64"}
		],
		"stateMutability": "view",
		"type": "function"
	},
	{
		"inputs": [],
		"name": "getAllNodes",
		"outputs": [
			{"name": "nodes", "type": "address[]"}
		],
		"stateMutability": "view",
		"type": "function"
	},
	{
		"inputs": [
			{"name": "minGPUCount", "type": "uint8"},
			{"name": "minMemoryGB", "type": "uint64"},
			{"name": "minStorageGB", "type": "uint64"}
		],
		"name": "searchNodes",
		"outputs": [
			{"name": "nodes", "type": "address[]"}
		],
		"stateMutability": "view",
		"type": "function"
	},
	{
		"inputs": [
			{"name": "nodeId", "type": "address"}
		],
		"name": "deregisterNode",
		"outputs": [],
		"stateMutability": "nonpayable",
		"type": "function"
	}
]`

// ResourceRegistryContract wraps the ResourceRegistry smart contract
type ResourceRegistryContract struct {
	address  common.Address
	contract *bind.BoundContract
	logger   log.Logger
}

// NewResourceRegistryContract creates a new ResourceRegistry contract instance
func NewResourceRegistryContract(address common.Address, logger log.Logger) *ResourceRegistryContract {
	return &ResourceRegistryContract{
		address: address,
		logger:  logger,
	}
}

// RegisterNode registers a new compute node with the blockchain
func (r *ResourceRegistryContract) RegisterNode(ctx context.Context, auth *bind.TransactOpts, caps ResourceCapabilities, pricePerHour *big.Int, minDuration, maxDuration uint64) (*types.Transaction, error) {
	r.logger.Infof("Registering node with capabilities: CPU=%d, GPU=%d, Memory=%dGB",
		caps.CPUCores, caps.GPUCount, caps.MemoryGB)

	// In production, this would call the actual contract
	// For now, we simulate the transaction
	tx := types.NewTransaction(0, r.address, big.NewInt(0), 0, big.NewInt(0), nil)
	return tx, nil
}

// UpdatePrice updates the price per hour for a node
func (r *ResourceRegistryContract) UpdatePrice(ctx context.Context, auth *bind.TransactOpts, nodeID common.Address, newPrice *big.Int) (*types.Transaction, error) {
	r.logger.Infof("Updating price for node %s to %s wei", nodeID.Hex(), newPrice.String())
	tx := types.NewTransaction(0, r.address, big.NewInt(0), 0, big.NewInt(0), nil)
	return tx, nil
}

// SetNodeActive sets the active status of a node
func (r *ResourceRegistryContract) SetNodeActive(ctx context.Context, auth *bind.TransactOpts, nodeID common.Address, active bool) (*types.Transaction, error) {
	r.logger.Infof("Setting node %s active status to %v", nodeID.Hex(), active)
	tx := types.NewTransaction(0, r.address, big.NewInt(0), 0, big.NewInt(0), nil)
	return tx, nil
}

// UpdateCapabilities updates the capabilities of a node
func (r *ResourceRegistryContract) UpdateCapabilities(ctx context.Context, auth *bind.TransactOpts, nodeID common.Address, caps ResourceCapabilities) (*types.Transaction, error) {
	r.logger.Infof("Updating capabilities for node %s", nodeID.Hex())
	tx := types.NewTransaction(0, r.address, big.NewInt(0), 0, big.NewInt(0), nil)
	return tx, nil
}

// GetNodeInfo retrieves node information from the blockchain
func (r *ResourceRegistryContract) GetNodeInfo(ctx context.Context, callOpts *bind.CallOpts, nodeID common.Address) (NodeInfo, error) {
	r.logger.Debugf("Getting node info for %s", nodeID.Hex())

	// Return mock data for demonstration
	return NodeInfo{
		Owner:             nodeID,
		Capabilities:      ResourceCapabilities{CPUCores: 8, GPUCount: 1, MemoryGB: 32, StorageGB: 512, BandwidthMbps: 1000},
		PricePerHour:      big.NewInt(1e18), // 1 ETH per hour
		MinRentalDuration: 3600,
		MaxRentalDuration: 86400,
		IsActive:          true,
		RegisteredAt:      0,
		LastUpdated:       0,
	}, nil
}

// GetAllNodes retrieves all registered nodes
func (r *ResourceRegistryContract) GetAllNodes(ctx context.Context, callOpts *bind.CallOpts) ([]common.Address, error) {
	r.logger.Debug("Getting all registered nodes")
	return []common.Address{}, nil
}

// SearchNodes searches for nodes matching criteria
func (r *ResourceRegistryContract) SearchNodes(ctx context.Context, callOpts *bind.CallOpts, minGPU uint8, minMemoryGB, minStorageGB uint64) ([]common.Address, error) {
	r.logger.Debugf("Searching nodes: minGPU=%d, minMemory=%dGB, minStorage=%dGB", minGPU, minMemoryGB, minStorageGB)
	return []common.Address{}, nil
}

// DeregisterNode removes a node from the registry
func (r *ResourceRegistryContract) DeregisterNode(ctx context.Context, auth *bind.TransactOpts, nodeID common.Address) (*types.Transaction, error) {
	r.logger.Infof("Deregistering node %s", nodeID.Hex())
	tx := types.NewTransaction(0, r.address, big.NewInt(0), 0, big.NewInt(0), nil)
	return tx, nil
}

// ResourceRegistryEventParser parses events from the ResourceRegistry contract
type ResourceRegistryEventParser struct {
	logger log.Logger
}

// NewResourceRegistryEventParser creates a new event parser
func NewResourceRegistryEventParser(logger log.Logger) *ResourceRegistryEventParser {
	return &ResourceRegistryEventParser{logger: logger}
}

// ParseNodeRegisteredEvent parses NodeRegistered events
func (p *ResourceRegistryEventParser) ParseNodeRegisteredEvent(logData []byte) (*NodeRegisteredEvent, error) {
	p.logger.Debug("Parsing NodeRegistered event")
	return &NodeRegisteredEvent{
		NodeID:    common.HexToAddress("0x0"),
		Owner:     common.HexToAddress("0x0"),
		CPUCores:  0,
		GPUCount:  0,
		MemoryGB:  0,
		StorageGB: 0,
		Bandwidth: 0,
		GPUModel:  "",
		GPUVRAMGB: 0,
	}, nil
}

// NodeRegisteredEvent represents the NodeRegistered event data
type NodeRegisteredEvent struct {
	NodeID    common.Address
	Owner     common.Address
	CPUCores  uint8
	GPUCount  uint8
	MemoryGB  uint64
	StorageGB uint64
	Bandwidth uint64
	GPUModel  string
	GPUVRAMGB uint8
}

// PriceUpdatedEvent represents the PriceUpdated event data
type PriceUpdatedEvent struct {
	NodeID   common.Address
	NewPrice *big.Int
}

// NodeStatusChangedEvent represents the NodeStatusChanged event data
type NodeStatusChangedEvent struct {
	NodeID common.Address
	Active bool
}

// Ensure ResourceRegistryContract implements the contract interface
var _ Contract = (*ResourceRegistryContract)(nil)

// Contract interface defines the common methods for all smart contracts
type Contract interface {
	Address() common.Address
}

// Address returns the contract address
func (r *ResourceRegistryContract) Address() common.Address {
	return r.address
}

// FormatCapabilities returns a human-readable string of capabilities
func (c ResourceCapabilities) FormatCapabilities() string {
	return fmt.Sprintf("CPU:%d, GPU:%d(%s %dGB), Memory:%dGB, Storage:%dGB, Bandwidth:%dMbps",
		c.CPUCores, c.GPUCount, c.GPUModel, c.GPUVRAMGB, c.MemoryGB, c.StorageGB, c.BandwidthMbps)
}
