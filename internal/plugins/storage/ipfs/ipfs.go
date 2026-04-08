package ipfs

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"

	shell "github.com/ipfs/go-ipfs-api"
	"github.com/kerrigan/kerrigan/pkg/log"
)

// Client wraps the IPFS shell client
type Client struct {
	ctx    context.Context
	shell  *shell.Shell
	mu     sync.RWMutex
	closed bool
}

// NewClient creates a new IPFS client
func NewClient(ctx context.Context, apiAddr string) (*Client, error) {
	if apiAddr == "" {
		apiAddr = "localhost:5001"
	}

	sh := shell.NewShell(apiAddr)

	// Test connection
	if _, err := sh.ID(); err != nil {
		return nil, fmt.Errorf("failed to connect to IPFS API at %s: %w", apiAddr, err)
	}

	return &Client{
		ctx:   ctx,
		shell: sh,
	}, nil
}

// Close closes the IPFS client
func (c *Client) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closed = true
	// Shell doesn't have explicit close, just set flag
}

// Add adds data to IPFS and returns the CID
func (c *Client) Add(ctx context.Context, data []byte) (string, error) {
	c.mu.RLock()
	if c.closed {
		c.mu.RUnlock()
		return "", fmt.Errorf("client closed")
	}
	c.mu.RUnlock()

	reader := strings.NewReader(string(data))
	cid, err := c.shell.Add(reader)
	if err != nil {
		return "", fmt.Errorf("failed to add data to IPFS: %w", err)
	}

	log.Debug("Added data to IPFS", "cid", cid)
	return cid, nil
}

// AddFile adds a file to IPFS and returns the CID and size
func (c *Client) AddFile(ctx context.Context, path string) (string, int64, error) {
	c.mu.RLock()
	if c.closed {
		c.mu.RUnlock()
		return "", 0, fmt.Errorf("client closed")
	}
	c.mu.RUnlock()

	cid, err := c.shell.AddFilepath(path)
	if err != nil {
		return "", 0, fmt.Errorf("failed to add file to IPFS: %w", err)
	}

	// Get file size
	size, err := c.getFileSize(path)
	if err != nil {
		log.Warn("Failed to get file size", "error", err)
		size = 0
	}

	log.Debug("Added file to IPFS", "cid", cid, "size", size)
	return cid, size, nil
}

// AddDir adds a directory to IPFS and returns the root CID
func (c *Client) AddDir(ctx context.Context, dirPath string) (string, error) {
	c.mu.RLock()
	if c.closed {
		c.mu.RUnlock()
		return "", fmt.Errorf("client closed")
	}
	c.mu.RUnlock()

	cid, err := c.shell.AddDir(dirPath)
	if err != nil {
		return "", fmt.Errorf("failed to add directory to IPFS: %w", err)
	}

	log.Debug("Added directory to IPFS", "cid", cid)
	return cid, nil
}

// Cat retrieves data from IPFS by CID
func (c *Client) Cat(ctx context.Context, cid string) ([]byte, error) {
	c.mu.RLock()
	if c.closed {
		c.mu.RUnlock()
		return nil, fmt.Errorf("client closed")
	}
	c.mu.RUnlock()

	reader, err := c.shell.Cat(cid)
	if err != nil {
		return nil, fmt.Errorf("failed to cat from IPFS: %w", err)
	}
	defer reader.Close()

	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read data: %w", err)
	}

	log.Debug("Retrieved data from IPFS", "cid", cid, "size", len(data))
	return data, nil
}

// CatReader returns a reader for the CID
func (c *Client) CatReader(ctx context.Context, cid string) (io.ReadCloser, error) {
	c.mu.RLock()
	if c.closed {
		c.mu.RUnlock()
		return nil, fmt.Errorf("client closed")
	}
	c.mu.RUnlock()

	reader, err := c.shell.Cat(cid)
	if err != nil {
		return nil, fmt.Errorf("failed to cat from IPFS: %w", err)
	}

	return reader, nil
}

// Pin pins a CID to local IPFS node
func (c *Client) Pin(ctx context.Context, cid string) error {
	c.mu.RLock()
	if c.closed {
		c.mu.RUnlock()
		return fmt.Errorf("client closed")
	}
	c.mu.RUnlock()

	err := c.shell.Pin(cid)
	if err != nil {
		return fmt.Errorf("failed to pin CID: %w", err)
	}

	log.Debug("Pinned CID", "cid", cid)
	return nil
}

// Unpin unpins a CID from local IPFS node
func (c *Client) Unpin(ctx context.Context, cid string) error {
	c.mu.RLock()
	if c.closed {
		c.mu.RUnlock()
		return fmt.Errorf("client closed")
	}
	c.mu.RUnlock()

	err := c.shell.Unpin(cid)
	if err != nil {
		return fmt.Errorf("failed to unpin CID: %w", err)
	}

	log.Debug("Unpinned CID", "cid", cid)
	return nil
}

// PinList returns all pinned CIDs
func (c *Client) PinList(ctx context.Context) ([]string, error) {
	c.mu.RLock()
	if c.closed {
		c.mu.RUnlock()
		return nil, fmt.Errorf("client closed")
	}
	c.mu.RUnlock()

	pinned, err := c.shell.Pins()
	if err != nil {
		return nil, fmt.Errorf("failed to get pin list: %w", err)
	}

	var cids []string
	for cid := range pinned {
		cids = append(cids, cid)
	}

	return cids, nil
}

// IsPinned checks if a CID is pinned
func (c *Client) IsPinned(ctx context.Context, cid string) (bool, error) {
	c.mu.RLock()
	if c.closed {
		c.mu.RUnlock()
		return false, fmt.Errorf("client closed")
	}
	c.mu.RUnlock()

	pinned, err := c.shell.Pins()
	if err != nil {
		return false, fmt.Errorf("failed to check pin status: %w", err)
	}

	_, ok := pinned[cid]
	return ok, nil
}

// GetBlockSize returns the size of a block for given CID
func (c *Client) GetBlockSize(ctx context.Context, cid string) (int64, error) {
	c.mu.RLock()
	if c.closed {
		c.mu.RUnlock()
		return 0, fmt.Errorf("client closed")
	}
	c.mu.RUnlock()

	stat, err := c.shell.ObjectStat(cid)
	if err != nil {
		return 0, fmt.Errorf("failed to get object stat: %w", err)
	}

	return int64(stat.CumulativeSize), nil
}

// GetDagNode returns a dag node for given CID
func (c *Client) GetDagNode(ctx context.Context, cid string) (*DagNode, error) {
	c.mu.RLock()
	if c.closed {
		c.mu.RUnlock()
		return nil, fmt.Errorf("client closed")
	}
	c.mu.RUnlock()

	data, err := c.shell.DagGet(cid)
	if err != nil {
		return nil, fmt.Errorf("failed to get DAG: %w", err)
	}

	return &DagNode{
		CID:  cid,
		Data: data,
	}, nil
}

// DagNode represents an IPFS DAG node
type DagNode struct {
	CID  string
	Data string
}

// PinWithMetadata pins a CID with metadata
func (c *Client) PinWithMetadata(ctx context.Context, cid string, metadata map[string]string) error {
	c.mu.RLock()
	if c.closed {
		c.mu.RUnlock()
		return fmt.Errorf("client closed")
	}
	c.mu.RUnlock()

	// Note: Basic shell doesn't support metadata pinning
	// This is a placeholder for advanced implementation
	return c.shell.Pin(cid)
}

// BlockPut puts a block directly to IPFS
func (c *Client) BlockPut(ctx context.Context, data []byte, codec string) (string, error) {
	c.mu.RLock()
	if c.closed {
		c.mu.RUnlock()
		return "", fmt.Errorf("client closed")
	}
	c.mu.RUnlock()

	cid, err := c.shell.BlockPut(data, codec, "")
	if err != nil {
		return "", fmt.Errorf("failed to put block: %w", err)
	}

	log.Debug("Put block to IPFS", "cid", cid)
	return cid, nil
}

// BlockGet gets a block directly from IPFS
func (c *Client) BlockGet(ctx context.Context, cid string) ([]byte, error) {
	c.mu.RLock()
	if c.closed {
		c.mu.RUnlock()
		return nil, fmt.Errorf("client closed")
	}
	c.mu.RUnlock()

	data, err := c.shell.BlockGet(cid)
	if err != nil {
		return nil, fmt.Errorf("failed to get block: %w", err)
	}

	return data, nil
}

// GetRefs returns references for a CID
func (c *Client) GetRefs(ctx context.Context, cid string) ([]string, error) {
	c.mu.RLock()
	if c.closed {
		c.mu.RUnlock()
		return nil, fmt.Errorf("client closed")
	}
	c.mu.RUnlock()

	refs, err := c.shell.Refs(cid, false)
	if err != nil {
		return nil, fmt.Errorf("failed to get refs: %w", err)
	}

	var refList []string
	for ref := range refs {
		refList = append(refList, ref)
	}

	return refList, nil
}

// FindProviders finds providers for a CID
func (c *Client) FindProviders(ctx context.Context, cid string) ([]ProviderInfo, error) {
	c.mu.RLock()
	if c.closed {
		c.mu.RUnlock()
		return nil, fmt.Errorf("client closed")
	}
	c.mu.RUnlock()

	providers, err := c.shell.FindProviders(cid)
	if err != nil {
		return nil, fmt.Errorf("failed to find providers: %w", err)
	}

	var result []ProviderInfo
	for _, p := range providers {
		result = append(result, ProviderInfo{
			ID:        p.ID,
			Addr:      p.Addr,
			Connected: p.Connected,
		})
	}

	return result, nil
}

// ProviderInfo represents an IPFS provider
type ProviderInfo struct {
	ID        string
	Addr      string
	Connected bool
}

// GetIPNSEntry retrieves an IPNS entry
func (c *Client) GetIPNSEntry(ctx context.Context, name string) (string, error) {
	c.mu.RLock()
	if c.closed {
		c.mu.RUnlock()
		return "", fmt.Errorf("client closed")
	}
	c.mu.RUnlock()

	resolved, err := c.shell.Resolve(name)
	if err != nil {
		return "", fmt.Errorf("failed to resolve IPNS: %w", err)
	}

	return resolved, nil
}

// PublishIPNS publishes content to IPNS
func (c *Client) PublishIPNS(ctx context.Context, key string, cid string) (string, error) {
	c.mu.RLock()
	if c.closed {
		c.mu.RUnlock()
		return "", fmt.Errorf("client closed")
	}
	c.mu.RUnlock()

	name, err := c.shell.Publish(key, cid)
	if err != nil {
		return "", fmt.Errorf("failed to publish to IPNS: %w", err)
	}

	log.Debug("Published to IPNS", "key", key, "name", name)
	return name, nil
}

// GetFileSize returns the file size for a CID
func (c *Client) GetFileSize(ctx context.Context, cid string) (int64, error) {
	c.mu.RLock()
	if c.closed {
		c.mu.RUnlock()
		return 0, fmt.Errorf("client closed")
	}
	c.mu.RUnlock()

	stat, err := c.shell.FilesStat(ctx, "/ipfs/"+cid)
	if err != nil {
		return 0, fmt.Errorf("failed to get file stat: %w", err)
	}

	return int64(stat.Size), nil
}

// FilesLs lists files in a directory
func (c *Client) FilesLs(ctx context.Context, path string) ([]FileInfo, error) {
	c.mu.RLock()
	if c.closed {
		c.mu.RUnlock()
		return nil, fmt.Errorf("client closed")
	}
	c.mu.RUnlock()

	entries, err := c.shell.FilesLs(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("failed to list files: %w", err)
	}

	var files []FileInfo
	for _, e := range entries {
		files = append(files, FileInfo{
			Name: e.Name,
			Path: e.Path,
			Size: int64(e.Size),
			Type: e.Type,
		})
	}

	return files, nil
}

// FileInfo represents IPFS file information
type FileInfo struct {
	Name string
	Path string
	Size int64
	Type string
}

func (c *Client) getFileSize(path string) (int64, error) {
	return 0, nil // Placeholder - actual implementation would use os.Stat
}
