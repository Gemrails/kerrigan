package agent

import (
	"sync"
	"time"
)

// AgentType represents the type of agent framework
type AgentType string

const (
	AgentTypeOpenClaw AgentType = "openclaw"
	AgentTypeDeerFlow AgentType = "deerflow"
)

// AgentCapability represents the capabilities of an agent
type AgentCapability string

const (
	CapWebSearch    AgentCapability = "web_search"
	CapCodeExec     AgentCapability = "code_execution"
	CapDeepResearch AgentCapability = "deep_research"
	CapImageGen     AgentCapability = "image_generation"
	CapSubAgent     AgentCapability = "sub_agent_orchestration"
)

// AgentStatus represents the status of an agent node
type AgentStatus string

const (
	AgentStatusOnline  AgentStatus = "online"
	AgentStatusBusy    AgentStatus = "busy"
	AgentStatusOffline AgentStatus = "offline"
)

// AgentNode represents a node with an agent framework
type AgentNode struct {
	NodeID        string                 `json:"node_id"`
	AgentType     AgentType              `json:"agent_type"`
	Endpoint      string                 `json:"endpoint"` // WebSocket or HTTP URL
	Capabilities  []AgentCapability      `json:"capabilities"`
	Status        AgentStatus            `json:"status"`
	LastHeartbeat time.Time              `json:"last_heartbeat"`
	Load          float64                `json:"load"` // 0-1
	Metadata      map[string]interface{} `json:"metadata"`
}

// NewAgentNode creates a new agent node
func NewAgentNode(nodeID string, agentType AgentType, endpoint string, capabilities []AgentCapability) *AgentNode {
	return &AgentNode{
		NodeID:        nodeID,
		AgentType:     agentType,
		Endpoint:      endpoint,
		Capabilities:  capabilities,
		Status:        AgentStatusOnline,
		LastHeartbeat: time.Now(),
		Load:          0.0,
		Metadata:      make(map[string]interface{}),
	}
}

// Registry manages agent nodes
type Registry struct {
	mu    sync.RWMutex
	nodes map[string]*AgentNode // nodeID -> AgentNode
}

// NewRegistry creates a new registry
func NewRegistry() *Registry {
	return &Registry{
		nodes: make(map[string]*AgentNode),
	}
}

// AddNode adds or updates a node in the registry
func (r *Registry) AddNode(node *AgentNode) error {
	if node == nil {
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.nodes[node.NodeID] = node
	node.LastHeartbeat = time.Now()
	return nil
}

// RemoveNode removes a node from the registry
func (r *Registry) RemoveNode(nodeID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.nodes[nodeID]; !exists {
		return nil // Already removed
	}

	delete(r.nodes, nodeID)
	return nil
}

// GetNode returns a node by ID
func (r *Registry) GetNode(nodeID string) *AgentNode {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.nodes[nodeID]
}

// ListNodes returns all nodes
func (r *Registry) ListNodes() []*AgentNode {
	r.mu.RLock()
	defer r.mu.RUnlock()

	nodes := make([]*AgentNode, 0, len(r.nodes))
	for _, node := range r.nodes {
		nodes = append(nodes, node)
	}

	return nodes
}

// FindNode finds a node matching the predicate
func (r *Registry) FindNode(predicate func(*AgentNode) bool) *AgentNode {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, node := range r.nodes {
		if predicate(node) {
			return node
		}
	}

	return nil
}

// FindNodes finds all nodes matching the predicate
func (r *Registry) FindNodes(predicate func(*AgentNode) bool) []*AgentNode {
	r.mu.RLock()
	defer r.mu.RUnlock()

	nodes := make([]*AgentNode, 0)
	for _, node := range r.nodes {
		if predicate(node) {
			nodes = append(nodes, node)
		}
	}

	return nodes
}

// UpdateNodeStatus updates the status of a node
func (r *Registry) UpdateNodeStatus(nodeID string, status AgentStatus) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	node, exists := r.nodes[nodeID]
	if !exists {
		return nil
	}

	node.Status = status
	node.LastHeartbeat = time.Now()
	return nil
}

// UpdateNodeLoad updates the load of a node
func (r *Registry) UpdateNodeLoad(nodeID string, load float64) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	node, exists := r.nodes[nodeID]
	if !exists {
		return nil
	}

	node.Load = load
	node.LastHeartbeat = time.Now()
	return nil
}

// CleanStaleNodes removes nodes that haven't sent heartbeat
func (r *Registry) CleanStaleNodes(timeout time.Duration) int {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	cleaned := 0

	for id, node := range r.nodes {
		if now.Sub(node.LastHeartbeat) > timeout {
			delete(r.nodes, id)
			cleaned++
		}
	}

	return cleaned
}

// GetNodesByType returns all nodes of a specific type
func (r *Registry) GetNodesByType(agentType AgentType) []*AgentNode {
	r.mu.RLock()
	defer r.mu.RUnlock()

	nodes := make([]*AgentNode, 0)
	for _, node := range r.nodes {
		if node.AgentType == agentType {
			nodes = append(nodes, node)
		}
	}

	return nodes
}

// GetNodesByCapability returns all nodes with a specific capability
func (r *Registry) GetNodesByCapability(cap AgentCapability) []*AgentNode {
	r.mu.RLock()
	defer r.mu.RUnlock()

	nodes := make([]*AgentNode, 0)
	for _, node := range r.nodes {
		for _, c := range node.Capabilities {
			if c == cap {
				nodes = append(nodes, node)
				break
			}
		}
	}

	return nodes
}

// Count returns the number of nodes in the registry
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return len(r.nodes)
}

// CountByStatus returns the number of nodes by status
func (r *Registry) CountByStatus(status AgentStatus) int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	count := 0
	for _, node := range r.nodes {
		if node.Status == status {
			count++
		}
	}

	return count
}
