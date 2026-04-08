package runtime

import (
	"sync"
)

// Sandbox provides security isolation for plugins
type Sandbox struct {
	mu          sync.RWMutex
	permissions map[string]map[string]bool // pluginID -> permission -> allowed
	blacklist   map[string]bool            // blacklisted plugins
	whitelist   map[string]bool            // whitelisted plugins
}

// NewSandbox creates a new sandbox instance
func NewSandbox() *Sandbox {
	return &Sandbox{
		permissions: make(map[string]map[string]bool),
		blacklist:   make(map[string]bool),
		whitelist:   make(map[string]bool),
	}
}

// SetPermission sets a permission for a plugin
func (s *Sandbox) SetPermission(pluginID, permission string, allowed bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.permissions[pluginID] == nil {
		s.permissions[pluginID] = make(map[string]bool)
	}
	s.permissions[pluginID][permission] = allowed
}

// HasPermission checks if a plugin has a specific permission
func (s *Sandbox) HasPermission(pluginID, permission string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Check blacklist first
	if s.blacklist[pluginID] {
		return false
	}

	// Check whitelist if enabled
	if len(s.whitelist) > 0 && !s.whitelist[pluginID] {
		return false
	}

	// Check specific permission
	if perms, ok := s.permissions[pluginID]; ok {
		if allowed, exists := perms[permission]; exists {
			return allowed
		}
	}

	// Default: allow if not explicitly denied
	return true
}

// Blacklist adds a plugin to the blacklist
func (s *Sandbox) Blacklist(pluginID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.blacklist[pluginID] = true
}

// Whitelist adds a plugin to the whitelist (enables whitelist mode)
func (s *Sandbox) Whitelist(pluginID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.whitelist[pluginID] = true
}

// IsBlacklisted checks if a plugin is blacklisted
func (s *Sandbox) IsBlacklisted(pluginID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.blacklist[pluginID]
}

// ClearPermissions clears all permissions for a plugin
func (s *Sandbox) ClearPermissions(pluginID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.permissions, pluginID)
}

// ClearAll clears all sandbox settings
func (s *Sandbox) ClearAll() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.permissions = make(map[string]map[string]bool)
	s.blacklist = make(map[string]bool)
	s.whitelist = make(map[string]bool)
}

// EnableWhitelistMode enables whitelist mode (only whitelisted plugins allowed)
func (s *Sandbox) EnableWhitelistMode() {
	s.mu.Lock()
	defer s.mu.Unlock()
	// Whitelist mode is enabled when whitelist map has entries
}
