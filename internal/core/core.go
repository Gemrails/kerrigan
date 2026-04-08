package core

import (
	"context"
	"time"
)

// NodeRole represents the role of a node in the network
type NodeRole string

const (
	RoleProvider  NodeRole = "provider"
	RoleConsumer  NodeRole = "consumer"
	RoleRelay     NodeRole = "relay"
	RoleValidator NodeRole = "validator"
)

// NodeConfig holds the configuration for a node
type NodeConfig struct {
	NodeID         string
	Roles          []NodeRole
	Genesis        bool
	DataDir        string
	ControlPort    int
	DataPort       int
	SeedNodes      []string
	EnabledPlugins []string
}

// Node represents a Kerrigan network node
type Node struct {
	config    *NodeConfig
	startedAt time.Time
}

// NewNode creates a new node instance
func NewNode(ctx context.Context, config *NodeConfig) (*Node, error) {
	return &Node{
		config:    config,
		startedAt: time.Now(),
	}, nil
}

// Start starts the node
func (n *Node) Start() error {
	return nil
}

// Stop stops the node
func (n *Node) Stop(ctx context.Context) error {
	return nil
}
