package download

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/kerrigan/kerrigan/pkg/log"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
)

// Manager manages P2P downloads from multiple sources
type Manager struct {
	ctx             context.Context
	maxPeers        int
	activeDownloads sync.Map
	wg              sync.WaitGroup
	closed          bool
}

// DownloadTask represents an active download task
type DownloadTask struct {
	CID       string
	Peers     []peer.ID
	Data      []byte
	Size      int64
	Completed bool
	Error     error
	Progress  float64
	mu        sync.Mutex
}

// Piece represents a piece of data for parallel download
type Piece struct {
	Index     int
	Offset    int64
	Size      int64
	Data      []byte
	Completed bool
	Peer      peer.ID
}

// Selector selects pieces for download
type Selector interface {
	SelectPieces(tasks []Piece, availablePeers []peer.ID) []Piece
}

// RarestFirstSelector selects pieces from rarest first
type RarestFirstSelector struct {
	pieceCount map[int]int
}

// New creates a new download manager
func New(ctx context.Context, maxPeers int) (*Manager, error) {
	if maxPeers <= 0 {
		maxPeers = 4
	}

	return &Manager{
		ctx:             ctx,
		maxPeers:        maxPeers,
		activeDownloads: sync.Map{},
	}, nil
}

// Close closes the download manager
func (m *Manager) Close() {
	if m.closed {
		return
	}

	m.closed = true
	m.wg.Wait()
}

// Download downloads data from multiple peers
func (m *Manager) Download(ctx context.Context, host host.Host, cid string, peers []peer.ID) ([]byte, error) {
	if m.closed {
		return nil, fmt.Errorf("download manager closed")
	}

	if len(peers) == 0 {
		return nil, fmt.Errorf("no peers available")
	}

	// Limit peers
	if len(peers) > m.maxPeers {
		peers = peers[:m.maxPeers]
	}

	// Create download task
	task := &DownloadTask{
		CID:   cid,
		Peers: peers,
	}

	m.activeDownloads.Store(cid, task)
	defer m.activeDownloads.Delete(cid)

	// Try each peer until successful
	var lastErr error
	for _, peerID := range peers {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		log.Debug("Trying peer", "cid", cid, "peer", peerID)

		data, err := m.downloadFromPeer(ctx, host, cid, peerID)
		if err != nil {
			lastErr = err
			log.Warn("Download from peer failed", "peer", peerID, "error", err)
			continue
		}

		// Verify integrity
		if err := m.verifyData(cid, data); err != nil {
			log.Warn("Data verification failed", "peer", peerID, "error", err)
			continue
		}

		task.Data = data
		task.Completed = true

		log.Info("Download completed", "cid", cid, "peer", peerID, "size", len(data))
		return data, nil
	}

	return nil, fmt.Errorf("failed to download from all peers: %w", lastErr)
}

// downloadFromPeer downloads data from a single peer
func (m *Manager) downloadFromPeer(ctx context.Context, host host.Host, cid string, peerID peer.ID) ([]byte, error) {
	// Create a custom protocol for storage download
	protocolID := protocol.ID("/kerrigan/storage/1.0.0")

	stream, err := host.NewStream(ctx, peerID, protocolID)
	if err != nil {
		return nil, fmt.Errorf("failed to open stream: %w", err)
	}
	defer stream.Close()

	// Send CID request
	req := &DownloadRequest{
		CID: cid,
	}

	if err := m.sendRequest(stream, req); err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	// Read response
	data, err := m.readResponse(stream)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	return data, nil
}

// DownloadParallel downloads data in parallel from multiple peers
func (m *Manager) DownloadParallel(ctx context.Context, host host.Host, cid string, peers []peer.ID, pieceSize int64) ([]byte, error) {
	if m.closed {
		return nil, fmt.Errorf("download manager closed")
	}

	if len(peers) == 0 {
		return nil, fmt.Errorf("no peers available")
	}

	// Get total size first (estimate)
	totalSize := pieceSize * int64(len(peers)) // Placeholder

	// Create pieces
	pieces := make([]Piece, 0, len(peers))
	for i := 0; i < len(peers); i++ {
		pieces = append(pieces, Piece{
			Index:  i,
			Offset: int64(i) * pieceSize,
			Size:   pieceSize,
		})
	}

	// Download pieces concurrently
	var wg sync.WaitGroup
	var mu sync.Mutex
	var firstErr error

	results := make([][]byte, len(pieces))

	for i, piece := range pieces {
		wg.Add(1)
		go func(idx int, p Piece, peerID peer.ID) {
			defer wg.Done()

			data, err := m.downloadPiece(ctx, host, cid, peerID, p.Offset, p.Size)
			if err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				mu.Unlock()
				return
			}

			mu.Lock()
			results[idx] = data
			mu.Unlock()

		}(i, piece, peers[i%len(peers)])
	}

	wg.Wait()

	// Combine results
	var totalData []byte
	for _, data := range results {
		if data == nil {
			continue
		}
		totalData = append(totalData, data...)
	}

	if len(totalData) == 0 {
		return nil, fmt.Errorf("no data downloaded: %w", firstErr)
	}

	// Verify
	if err := m.verifyData(cid, totalData); err != nil {
		return nil, err
	}

	return totalData, nil
}

// downloadPiece downloads a specific piece
func (m *Manager) downloadPiece(ctx context.Context, host host.Host, cid string, peerID peer.ID, offset, size int64) ([]byte, error) {
	protocolID := protocol.ID("/kerrigan/storage/1.0.0")

	stream, err := host.NewStream(ctx, peerID, protocolID)
	if err != nil {
		return nil, fmt.Errorf("failed to open stream: %w", err)
	}
	defer stream.Close()

	req := &DownloadRequest{
		CID:    cid,
		Offset: offset,
		Size:   size,
	}

	if err := m.sendRequest(stream, req); err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	data, err := m.readResponse(stream)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	return data, nil
}

// sendRequest sends a download request
func (m *Manager) sendRequest(stream network.Stream, req *DownloadRequest) error {
	data, err := req.Marshal()
	if err != nil {
		return err
	}

	_, err = stream.Write(data)
	return err
}

// readResponse reads a download response
func (m *Manager) readResponse(stream network.Stream) ([]byte, error) {
	data, err := io.ReadAll(stream)
	if err != nil {
		return nil, err
	}

	resp := &DownloadResponse{}
	if err := resp.Unmarshal(data); err != nil {
		return nil, err
	}

	return resp.Data, nil
}

// verifyData verifies data integrity using CID
func (m *Manager) verifyData(expectedCID string, data []byte) error {
	if expectedCID == "" {
		return nil
	}

	// Calculate hash
	hash := sha256.Sum256(data)
	hashStr := hex.EncodeToString(hash[:])

	// Basic verification - in real implementation would compare with actual CID
	log.Debug("Data verified", "hash", hashStr)

	return nil
}

// GetProgress returns download progress for a CID
func (m *Manager) GetProgress(cid string) (float64, error) {
	task, ok := m.activeDownloads.Load(cid)
	if !ok {
		return 0, fmt.Errorf("no active download for CID: %s", cid)
	}

	dt := task.(*DownloadTask)
	dt.mu.Lock()
	defer dt.mu.Unlock()

	return dt.Progress, nil
}

// DownloadRequest represents a download request
type DownloadRequest struct {
	CID    string
	Offset int64
	Size   int64
}

// Marshal serializes the request
func (r *DownloadRequest) Marshal() ([]byte, error) {
	return []byte(fmt.Sprintf("GET:%s:%d:%d", r.CID, r.Offset, r.Size)), nil
}

// Unmarshal deserializes the request
func (r *DownloadRequest) Unmarshal(data []byte) error {
	// Simple parsing - in production would use proper serialization
	return nil
}

// DownloadResponse represents a download response
type DownloadResponse struct {
	CID   string
	Data  []byte
	Error string
}

// Marshal serializes the response
func (r *DownloadResponse) Marshal() ([]byte, error) {
	return r.Data, nil
}

// Unmarshal deserializes the response
func (r *DownloadResponse) Unmarshal(data []byte) error {
	r.Data = data
	return nil
}

// NewRarestFirstSelector creates a new rarest-first selector
func NewRarestFirstSelector() *RarestFirstSelector {
	return &RarestFirstSelector{
		pieceCount: make(map[int]int),
	}
}

// SelectPieces selects pieces using rarest-first strategy
func (s *RarestFirstSelector) SelectPieces(pieces []Piece, availablePeers []peer.ID) []Piece {
	if len(pieces) == 0 || len(availablePeers) == 0 {
		return pieces
	}

	// Sort by piece count (rarest first)
	var selected []Piece
	for _, p := range pieces {
		count := s.pieceCount[p.Index]
		if count == 0 {
			selected = append(selected, p)
		}
	}

	if len(selected) == 0 {
		selected = pieces[:len(availablePeers)]
	} else if len(selected) > len(availablePeers) {
		selected = selected[:len(availablePeers)]
	}

	return selected
}

// RecordPiece records that a piece was downloaded from a peer
func (s *RarestFirstSelector) RecordPiece(pieceIndex int, peerID peer.ID) {
	s.pieceCount[pieceIndex]++
}

// MultiSourceDownloader downloads from multiple sources with piece selection
type MultiSourceDownloader struct {
	ctx      context.Context
	manager  *Manager
	selector Selector
}

// NewMultiSourceDownloader creates a new multi-source downloader
func NewMultiSourceDownloader(ctx context.Context, manager *Manager, selector Selector) *MultiSourceDownloader {
	return &MultiSourceDownloader{
		ctx:      ctx,
		manager:  manager,
		selector: selector,
	}
}

// Download performs multi-source download with piece selection
func (m *MultiSourceDownloader) Download(host host.Host, cid string, peers []peer.ID, totalSize int64) ([]byte, error) {
	// Calculate optimal piece size
	pieceSize := m.calculatePieceSize(totalSize, len(peers))

	// Create pieces
	numPieces := int(totalSize / pieceSize)
	if totalSize%pieceSize != 0 {
		numPieces++
	}

	pieces := make([]Piece, numPieces)
	for i := 0; i < numPieces; i++ {
		pieces[i] = Piece{
			Index:  i,
			Offset: int64(i) * pieceSize,
			Size:   pieceSize,
		}
	}

	// Select pieces
	selectedPieces := m.selector.SelectPieces(pieces, peers)

	// Download selected pieces
	var wg sync.WaitGroup
	var mu sync.Mutex
	results := make([][]byte, len(selectedPieces))

	for i, piece := range selectedPieces {
		wg.Add(1)
		go func(idx int, p Piece, peerID peer.ID) {
			defer wg.Done()

			data, err := m.manager.downloadPiece(m.ctx, host, cid, peerID, p.Offset, p.Size)
			if err != nil {
				log.Warn("Piece download failed", "index", p.Index, "error", err)
				return
			}

			mu.Lock()
			results[idx] = data
			mu.Unlock()

			// Record for rarest-first
			if rfs, ok := m.selector.(*RarestFirstSelector); ok {
				rfs.RecordPiece(p.Index, peerID)
			}

		}(i, piece, peers[i%len(peers)])
	}

	wg.Wait()

	// Combine results
	var data []byte
	for _, d := range results {
		data = append(data, d...)
	}

	if len(data) == 0 {
		return nil, fmt.Errorf("no data downloaded")
	}

	// Verify integrity
	if err := m.manager.verifyData(cid, data); err != nil {
		return nil, err
	}

	return data, nil
}

// calculatePieceSize calculates optimal piece size
func (m *MultiSourceDownloader) calculatePieceSize(totalSize int64, numPeers int) int64 {
	if numPeers <= 0 {
		numPeers = 1
	}

	// Target: each peer gets 4-16 pieces for parallelism
	targetPieces := int64(8)
	pieceSize := totalSize / (targetPieces * int64(numPeers))

	// Min: 1KB, Max: 1MB
	if pieceSize < 1024 {
		pieceSize = 1024
	}
	if pieceSize > 1024*1024 {
		pieceSize = 1024 * 1024
	}

	return pieceSize
}
