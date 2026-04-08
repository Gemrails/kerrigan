package loader

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/sirupsen/logrus"
)

// Loader handles plugin discovery, loading, and verification
type Loader struct {
	logger        *logrus.Logger
	pluginDir     string
	verifyEnabled bool
	trustKeys     map[string]*ecdsa.PublicKey
	mu            sync.RWMutex
}

// PluginManifest contains plugin metadata from manifest file
type PluginManifest struct {
	ID           string           `json:"id"`
	Name         string           `json:"name"`
	Version      string           `json:"version"`
	Description  string           `json:"description"`
	Author       string           `json:"author"`
	License      string           `json:"license"`
	Homepage     string           `json:"homepage"`
	Capabilities []string         `json:"capabilities"`
	Dependencies []DependencySpec `json:"dependencies"`
	ResourceType string           `json:"resource_type"`
	Signature    string           `json:"signature"`
	Checksum     string           `json:"checksum"`
	EntryPoint   string           `json:"entry_point"`
}

// DependencySpec specifies a plugin dependency
type DependencySpec struct {
	ID       string `json:"id"`
	Version  string `json:"version"`
	Optional bool   `json:"optional"`
}

// NewLoader creates a new plugin loader
func NewLoader(pluginDir string, logger *logrus.Logger) (*Loader, error) {
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		return nil, fmt.Errorf("create plugin directory: %w", err)
	}

	return &Loader{
		logger:        logger,
		pluginDir:     pluginDir,
		verifyEnabled: true,
		trustKeys:     make(map[string]*ecdsa.PublicKey),
	}, nil
}

// Discover discovers available plugins in the plugin directory
func (l *Loader) Discover() ([]string, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	var plugins []string

	entries, err := os.ReadDir(l.pluginDir)
	if err != nil {
		return nil, fmt.Errorf("read plugin directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		// Check for manifest file
		manifestPath := filepath.Join(l.pluginDir, entry.Name(), "manifest.json")
		if _, err := os.Stat(manifestPath); err != nil {
			continue // Skip directories without manifest
		}

		plugins = append(plugins, entry.Name())
	}

	l.logger.Infof("Discovered %d plugins in %s", len(plugins), l.pluginDir)
	return plugins, nil
}

// LoadManifest loads and parses a plugin manifest
func (l *Loader) LoadManifest(pluginID string) (*PluginManifest, error) {
	manifestPath := filepath.Join(l.pluginDir, pluginID, "manifest.json")

	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("read manifest: %w", err)
	}

	var manifest PluginManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}

	// Validate required fields
	if manifest.ID == "" || manifest.Name == "" || manifest.Version == "" {
		return nil, fmt.Errorf("invalid manifest: missing required fields")
	}

	return &manifest, nil
}

// VerifySignature verifies the plugin signature
func (l *Loader) VerifySignature(pluginID string, manifest *PluginManifest) error {
	if !l.verifyEnabled {
		l.logger.Debugf("Signature verification disabled, skipping %s", pluginID)
		return nil
	}

	if manifest.Signature == "" {
		return fmt.Errorf("plugin %s has no signature", pluginID)
	}

	// Get plugin files for checksum verification
	files, err := l.getPluginFiles(pluginID)
	if err != nil {
		return fmt.Errorf("get plugin files: %w", err)
	}

	// Calculate actual checksum
	checksum, err := l.calculateChecksum(pluginID, files)
	if err != nil {
		return fmt.Errorf("calculate checksum: %w", err)
	}

	// Verify checksum matches
	if manifest.Checksum != checksum {
		return fmt.Errorf("checksum mismatch for %s", pluginID)
	}

	// If we have trust keys, verify signature
	if len(l.trustKeys) > 0 {
		signatureBytes, err := hex.DecodeString(manifest.Signature)
		if err != nil {
			return fmt.Errorf("decode signature: %w", err)
		}

		// Try each trust key
		for keyID, pubKey := range l.trustKeys {
			err := ecdsa.Verify(pubKey, []byte(checksum), signatureBytes)
			if err == nil {
				l.logger.Infof("Plugin %s verified with key %s", pluginID, keyID)
				return nil
			}
		}

		return fmt.Errorf("signature verification failed for %s", pluginID)
	}

	l.logger.Warnf("No trust keys configured, skipping signature verification for %s", pluginID)
	return nil
}

// CalculateChecksum calculates the checksum of plugin files
func (l *Loader) CalculateChecksum(pluginID string) (string, error) {
	files, err := l.getPluginFiles(pluginID)
	if err != nil {
		return "", fmt.Errorf("get plugin files: %w", err)
	}

	return l.calculateChecksum(pluginID, files)
}

// getPluginFiles returns all files in a plugin directory
func (l *Loader) getPluginFiles(pluginID string) ([]string, error) {
	var files []string

	pluginPath := filepath.Join(l.pluginDir, pluginID)

	err := filepath.Walk(pluginPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip the root directory and manifest (included separately)
		if path == pluginPath {
			return nil
		}

		if info.IsDir() {
			return nil
		}

		// Use relative path
		relPath, err := filepath.Rel(pluginPath, path)
		if err != nil {
			return err
		}

		// Skip manifest in checksum calculation (it's verified separately)
		if relPath == "manifest.json" {
			return nil
		}

		files = append(files, relPath)
		return nil
	})

	if err != nil {
		return nil, err
	}

	sort.Strings(files)
	return files, nil
}

// calculateChecksum calculates SHA256 checksum of plugin files
func (l *Loader) calculateChecksum(pluginID string, files []string) (string, error) {
	hash := sha256.New()
	pluginPath := filepath.Join(l.pluginDir, pluginID)

	for _, file := range files {
		filePath := filepath.Join(pluginPath, file)

		// Add file path to hash
		hash.Write([]byte(file))

		// Add file content to hash
		data, err := os.ReadFile(filePath)
		if err != nil {
			return "", fmt.Errorf("read file %s: %w", file, err)
		}
		hash.Write(data)
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

// ResolveDependencies resolves plugin dependencies
func (l *Loader) ResolveDependencies(manifest *PluginManifest, availablePlugins map[string]*PluginManifest) ([]string, error) {
	var resolved []string
	visited := make(map[string]bool)

	var resolve func(dep DependencySpec) error
	resolve = func(dep DependencySpec) error {
		if visited[dep.ID] {
			return nil
		}

		// Check if dependency is available
		depManifest, exists := availablePlugins[dep.ID]
		if !exists {
			if dep.Optional {
				l.logger.Debugf("Optional dependency %s not found, skipping", dep.ID)
				return nil
			}
			return fmt.Errorf("required dependency %s not found", dep.ID)
		}

		// Check version compatibility
		if !l.checkVersionCompatibility(dep.Version, depManifest.Version) {
			return fmt.Errorf("dependency %s version mismatch: required %s, available %s",
				dep.ID, dep.Version, depManifest.Version)
		}

		visited[dep.ID] = true
		resolved = append(resolved, dep.ID)

		// Resolve transitive dependencies
		for _, transDep := range depManifest.Dependencies {
			if err := resolve(transDep); err != nil {
				return err
			}
		}

		return nil
	}

	for _, dep := range manifest.Dependencies {
		if err := resolve(dep); err != nil {
			return nil, err
		}
	}

	return resolved, nil
}

// checkVersionCompatibility checks if the available version satisfies the requirement
func (l *Loader) checkVersionCompatibility(required, available string) bool {
	// Simple version check - supports semantic versioning
	// Returns true if available >= required (major version must match)

	reqParts := strings.Split(required, ".")
	availParts := strings.Split(available, ".")

	if len(reqParts) < 2 || len(availParts) < 2 {
		return false
	}

	reqMajor := parseVersionPart(reqParts[0])
	availMajor := parseVersionPart(availParts[0])

	// Major version must match
	if reqMajor != availMajor {
		return false
	}

	// If only major matches, any minor version is compatible
	if len(reqParts) == 1 {
		return true
	}

	reqMinor := parseVersionPart(reqParts[1])
	availMinor := parseVersionPart(availParts[1])

	return availMinor >= reqMinor
}

func parseVersionPart(part string) int {
	// Remove any prefix like "v" or ">"
	var num string
	for _, c := range part {
		if c >= '0' && c <= '9' {
			num += string(c)
		} else {
			break
		}
	}

	if num == "" {
		return 0
	}

	var result int
	fmt.Sscanf(num, "%d", &result)
	return result
}

// AddTrustKey adds a trusted public key for signature verification
func (l *Loader) AddTrustKey(keyID string, pubKey *ecdsa.PublicKey) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.trustKeys[keyID] = pubKey
}

// RemoveTrustKey removes a trusted public key
func (l *Loader) RemoveTrustKey(keyID string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.trustKeys, keyID)
}

// LoadTrustKeyFromFile loads a trust key from a PEM file
func (l *Loader) LoadTrustKeyFromFile(keyID, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read key file: %w", err)
	}

	block, _ := pem.Decode(data)
	if block == nil {
		return fmt.Errorf("invalid PEM data")
	}

	// Parse as PKIX public key
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return fmt.Errorf("parse public key: %w", err)
	}

	ecdsaPub, ok := pub.(*ecdsa.PublicKey)
	if !ok {
		return fmt.Errorf("not an ECDSA public key")
	}

	l.mu.Lock()
	l.trustKeys[keyID] = ecdsaPub
	l.mu.Unlock()

	return nil
}

// EnableVerification enables signature verification
func (l *Loader) EnableVerification() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.verifyEnabled = true
}

// DisableVerification disables signature verification
func (l *Loader) DisableVerification() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.verifyEnabled = false
}

// IsVerificationEnabled returns whether verification is enabled
func (l *Loader) IsVerificationEnabled() bool {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.verifyEnabled
}

// GetPluginPath returns the path to a plugin directory
func (l *Loader) GetPluginPath(pluginID string) string {
	return filepath.Join(l.pluginDir, pluginID)
}

// GetEntryPointPath returns the path to a plugin's entry point
func (l *Loader) GetEntryPointPath(pluginID, entryPoint string) string {
	return filepath.Join(l.pluginDir, pluginID, entryPoint)
}

// ReadPluginFile reads a file from a plugin directory
func (l *Loader) ReadPluginFile(pluginID, filename string) ([]byte, error) {
	path := filepath.Join(l.pluginDir, pluginID, filename)
	return os.ReadFile(path)
}

// ListPluginFiles lists all files in a plugin directory
func (l *Loader) ListPluginFiles(pluginID string) ([]string, error) {
	var files []string
	pluginPath := filepath.Join(l.pluginDir, pluginID)

	err := filepath.Walk(pluginPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if path == pluginPath || info.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(pluginPath, path)
		if err != nil {
			return err
		}

		files = append(files, relPath)
		return nil
	})

	if err != nil {
		return nil, err
	}

	sort.Strings(files)
	return files, nil
}

// CreateSignature creates a signature for plugin content using a private key
func (l *Loader) CreateSignature(content string, privKey *ecdsa.PrivateKey) (string, error) {
	if privKey == nil {
		return "", fmt.Errorf("private key is nil")
	}

	r, s, err := ecdsa.Sign(rand.Reader, privKey, []byte(content))
	if err != nil {
		return "", fmt.Errorf("sign: %w", err)
	}

	signature := make([]byte, 0, 64)
	signature = append(signature, r.Bytes()...)
	signature = append(signature, s...)

	return hex.EncodeToString(signature), nil
}

// VerifyChecksum verifies that the plugin files match the expected checksum
func (l *Loader) VerifyChecksum(pluginID, expectedChecksum string) (bool, error) {
	actualChecksum, err := l.CalculateChecksum(pluginID)
	if err != nil {
		return false, err
	}

	return actualChecksum == expectedChecksum, nil
}

// CopyPlugin copies a plugin from source to destination
func (l *Loader) CopyPlugin(srcPluginID, dstPluginID string) error {
	srcPath := filepath.Join(l.pluginDir, srcPluginID)
	dstPath := filepath.Join(l.pluginDir, dstPluginID)

	if _, err := os.Stat(srcPath); err != nil {
		return fmt.Errorf("source plugin not found: %w", err)
	}

	if _, err := os.Stat(dstPath); err == nil {
		return fmt.Errorf("destination plugin already exists")
	}

	return l.copyDir(srcPath, dstPath)
}

// copyDir recursively copies a directory
func (l *Loader) copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		dstPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		}

		return copyFile(path, dstPath)
	})
}

// copyFile copies a single file
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}

// ExtractPlugin extracts a plugin from an archive (tar.gz)
// This is a placeholder - actual implementation would use archive/tar and compress/gzip
func (l *Loader) ExtractPlugin(archivePath, pluginID string) error {
	// Placeholder - actual implementation would extract tar.gz
	return fmt.Errorf("ExtractPlugin not implemented: use external tool for archive extraction")
}

// PackagePlugin packages a plugin into an archive
// This is a placeholder - actual implementation would use archive/tar and compress/gzip
func (l *Loader) PackagePlugin(pluginID, outputPath string) error {
	// Placeholder - actual implementation would create tar.gz
	return fmt.Errorf("PackagePlugin not implemented: use external tool for archive creation")
}

// SignPlugin signs a plugin using a private key
func (l *Loader) SignPlugin(pluginID string, privKey *ecdsa.PrivateKey) error {
	manifest, err := l.LoadManifest(pluginID)
	if err != nil {
		return fmt.Errorf("load manifest: %w", err)
	}

	// Calculate checksum
	checksum, err := l.CalculateChecksum(pluginID)
	if err != nil {
		return fmt.Errorf("calculate checksum: %w", err)
	}

	// Update manifest with checksum
	manifest.Checksum = checksum

	// Create signature
	signature, err := l.CreateSignature(checksum, privKey)
	if err != nil {
		return fmt.Errorf("create signature: %w", err)
	}

	manifest.Signature = signature

	// Save updated manifest
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}

	manifestPath := filepath.Join(l.pluginDir, pluginID, "manifest.json")
	if err := os.WriteFile(manifestPath, data, 0644); err != nil {
		return fmt.Errorf("write manifest: %w", err)
	}

	l.logger.Infof("Signed plugin %s", pluginID)
	return nil
}

// Hash returns the hash function used by the loader
func (l *Loader) Hash() crypto.Hash {
	return crypto.SHA256
}

// VerifyManifest verifies the manifest structure and required fields
func (l *Loader) VerifyManifest(manifest *PluginManifest) error {
	if manifest.ID == "" {
		return fmt.Errorf("manifest missing ID")
	}
	if manifest.Name == "" {
		return fmt.Errorf("manifest missing name")
	}
	if manifest.Version == "" {
		return fmt.Errorf("manifest missing version")
	}
	if manifest.EntryPoint == "" {
		return fmt.Errorf("manifest missing entry_point")
	}

	// Validate resource type
	validTypes := map[string]bool{
		"gpu":       true,
		"storage":   true,
		"bandwidth": true,
		"cpu":       true,
	}

	if !validTypes[manifest.ResourceType] {
		return fmt.Errorf("invalid resource_type: %s", manifest.ResourceType)
	}

	return nil
}
