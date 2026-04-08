package stats

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/kerrigan/kerrigan/pkg/log"
)

// Collector manages traffic statistics collection
type Collector struct {
	config CollectorConfig
	mu     sync.RWMutex
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// Per-region statistics
	regionStats map[string]*RegionStats

	// Per-session statistics
	sessionStats map[string]*SessionStats
	sessionMutex sync.RWMutex

	// Global counters
	totalBytesIn  int64
	totalBytesOut int64
	totalSessions int64

	// Reporting
	lastReport time.Time
	reporter   *Reporter
}

// CollectorConfig holds collector configuration
type CollectorConfig struct {
	StatsDir       string        // Directory to store stats files
	ReportInterval time.Duration // Interval between reports
	FlushInterval  time.Duration // Interval to flush to disk
	MaxSessions    int           // Max number of sessions to track
	RetentionDays  int           // Number of days to retain stats
}

// RegionStats holds statistics for a region
type RegionStats struct {
	Region          string
	TotalBytesIn    int64
	TotalBytesOut   int64
	ActiveSessions  int64
	TotalSessions   int64
	AvgSessionBytes int64
	PeakSessions    int64
	FirstActivity   time.Time
	LastActivity    time.Time
}

// SessionStats holds statistics for a session
type SessionStats struct {
	SessionID string
	Region    string
	ClientIP  string
	BytesIn   int64
	BytesOut  int64
	StartTime time.Time
	EndTime   time.Time
	Duration  time.Duration
	Protocol  string
}

// Reporter handles stats reporting
type Reporter interface {
	Report(stats *StatsReport)
}

// StatsReport represents a complete statistics report
type StatsReport struct {
	Timestamp      time.Time
	PeriodStart    time.Time
	PeriodEnd      time.Time
	TotalBytesIn   int64
	TotalBytesOut  int64
	TotalSessions  int64
	ActiveSessions int64
	RegionStats    map[string]*RegionStats
	TopSessions    []*SessionStats
}

// NewCollector creates a new statistics collector
func NewCollector(statsDir string, reportIntervalSec int) *Collector {
	if statsDir == "" {
		statsDir = "/var/lib/kerrigan/proxy/stats"
	}

	cfg := CollectorConfig{
		StatsDir:       statsDir,
		ReportInterval: time.Duration(reportIntervalSec) * time.Second,
		FlushInterval:  5 * time.Minute,
		MaxSessions:    10000,
		RetentionDays:  30,
	}

	if cfg.ReportInterval == 0 {
		cfg.ReportInterval = 60 * time.Second
	}

	ctx, cancel := context.WithCancel(context.Background())

	collector := &Collector{
		config:       cfg,
		ctx:          ctx,
		cancel:       cancel,
		regionStats:  make(map[string]*RegionStats),
		sessionStats: make(map[string]*SessionStats),
		reporter:     &DefaultReporter{},
	}

	// Ensure stats directory exists
	if err := os.MkdirAll(statsDir, 0755); err != nil {
		log.Warn("Failed to create stats directory", "dir", statsDir, "error", err)
	}

	return collector
}

// Start starts the statistics collector
func (c *Collector) Start() {
	log.Info("Starting stats collector", "interval", c.config.ReportInterval)

	// Start background workers
	c.wg.Add(1)
	go c.reportWorker()

	c.wg.Add(1)
	go c.flushWorker()

	c.wg.Add(1)
	go c.cleanupWorker()
}

// Stop stops the statistics collector
func (c *Collector) Stop() error {
	log.Info("Stopping stats collector")
	c.cancel()
	c.wg.Wait()

	// Final flush
	c.flushToDisk()

	return nil
}

// RecordConnection records a new connection
func (c *Collector) RecordConnection(region string, bytesIn, bytesOut int64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.regionStats[region]; !exists {
		c.regionStats[region] = &RegionStats{
			Region:        region,
			FirstActivity: time.Now(),
		}
	}

	stats := c.regionStats[region]
	stats.TotalSessions++
	stats.ActiveSessions++
	stats.LastActivity = time.Now()

	if stats.ActiveSessions > stats.PeakSessions {
		stats.PeakSessions = stats.ActiveSessions
	}

	// Update global counters
	atomic.AddInt64(&c.totalSessions, 1)
}

// RecordDisconnection records a disconnection
func (c *Collector) RecordDisconnection(region string, bytesIn, bytesOut int64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if stats, exists := c.regionStats[region]; exists {
		stats.ActiveSessions--
		stats.TotalBytesIn += bytesIn
		stats.TotalBytesOut += bytesOut

		if stats.TotalSessions > 0 {
			stats.AvgSessionBytes = (stats.TotalBytesIn + stats.TotalBytesOut) / stats.TotalSessions
		}
	}

	// Update global counters
	atomic.AddInt64(&c.totalBytesIn, bytesIn)
	atomic.AddInt64(&c.totalBytesOut, bytesOut)
}

// RecordSession records session statistics
func (c *Collector) RecordSession(sessionID, region, clientIP, protocol string, bytesIn, bytesOut int64, duration time.Duration) {
	c.sessionMutex.Lock()
	defer c.sessionMutex.Unlock()

	if len(c.sessionStats) >= c.config.MaxSessions {
		// Remove oldest session
		var oldest string
		var oldestTime time.Time
		for id, sess := range c.sessionStats {
			if oldestTime.IsZero() || sess.EndTime.Before(oldestTime) {
				oldest = id
				oldestTime = sess.EndTime
			}
		}
		if oldest != "" {
			delete(c.sessionStats, oldest)
		}
	}

	c.sessionStats[sessionID] = &SessionStats{
		SessionID: sessionID,
		Region:    region,
		ClientIP:  clientIP,
		BytesIn:   bytesIn,
		BytesOut:  bytesOut,
		StartTime: time.Now().Add(-duration),
		EndTime:   time.Now(),
		Duration:  duration,
		Protocol:  protocol,
	}
}

// GetRegionStats returns statistics for a specific region
func (c *Collector) GetRegionStats(region string) (*RegionStats, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	stats, exists := c.regionStats[region]
	return stats, exists
}

// GetAllRegionStats returns all region statistics
func (c *Collector) GetAllRegionStats() map[string]*RegionStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make(map[string]*RegionStats)
	for region, stats := range c.regionStats {
		result[region] = stats
	}

	return result
}

// GetGlobalStats returns global statistics
func (c *Collector) GetGlobalStats() (bytesIn, bytesOut, sessions int64) {
	bytesIn = atomic.LoadInt64(&c.totalBytesIn)
	bytesOut = atomic.LoadInt64(&c.totalBytesOut)
	sessions = atomic.LoadInt64(&c.totalSessions)

	return bytesIn, bytesOut, sessions
}

// GetTopSessions returns top sessions by traffic
func (c *Collector) GetTopSessions(limit int) []*SessionStats {
	c.sessionMutex.RLock()
	defer c.sessionMutex.RUnlock()

	sessions := make([]*SessionStats, 0, len(c.sessionStats))
	for _, sess := range c.sessionStats {
		sessions = append(sessions, sess)
	}

	// Sort by total bytes
	for i := 0; i < len(sessions); i++ {
		for j := i + 1; j < len(sessions); j++ {
			if sessions[i].BytesIn+sessions[i].BytesOut < sessions[j].BytesIn+sessions[j].BytesOut {
				sessions[i], sessions[j] = sessions[j], sessions[i]
			}
		}
	}

	if limit > 0 && len(sessions) > limit {
		sessions = sessions[:limit]
	}

	return sessions
}

// GetReport generates a statistics report
func (c *Collector) GetReport() *StatsReport {
	c.mu.RLock()
	regionStats := make(map[string]*RegionStats)
	for region, stats := range c.regionStats {
		regionStats[region] = stats
	}
	c.mu.RUnlock()

	bytesIn, bytesOut, sessions := c.GetGlobalStats()

	activeSessions := int64(0)
	for _, stats := range regionStats {
		activeSessions += stats.ActiveSessions
	}

	report := &StatsReport{
		Timestamp:      time.Now(),
		PeriodStart:    c.lastReport,
		PeriodEnd:      time.Now(),
		TotalBytesIn:   bytesIn,
		TotalBytesOut:  bytesOut,
		TotalSessions:  sessions,
		ActiveSessions: activeSessions,
		RegionStats:    regionStats,
		TopSessions:    c.GetTopSessions(10),
	}

	c.lastReport = time.Now()

	return report
}

// reportWorker periodically generates and reports statistics
func (c *Collector) reportWorker() {
	defer c.wg.Done()

	ticker := time.NewTicker(c.config.ReportInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			report := c.GetReport()
			c.reporter.Report(report)
		}
	}
}

// flushWorker periodically flushes statistics to disk
func (c *Collector) flushWorker() {
	defer c.wg.Done()

	ticker := time.NewTicker(c.config.FlushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			c.flushToDisk()
		}
	}
}

// flushToDisk flushes statistics to disk
func (c *Collector) flushToDisk() {
	c.mu.RLock()
	regionStats := make(map[string]*RegionStats)
	for region, stats := range c.regionStats {
		regionStats[region] = stats
	}
	c.mu.RUnlock()

	sessionStats := c.GetTopSessions(1000)

	// Write region stats
	regionFile := filepath.Join(c.config.StatsDir, "regions.json")
	if data, err := json.MarshalIndent(regionStats, "", "  "); err == nil {
		if err := os.WriteFile(regionFile, data, 0644); err != nil {
			log.Warn("Failed to write region stats", "error", err)
		}
	}

	// Write summary
	bytesIn, bytesOut, sessions := c.GetGlobalStats()
	summary := struct {
		Timestamp   time.Time `json:"timestamp"`
		BytesIn     int64     `json:"bytes_in"`
		BytesOut    int64     `json:"bytes_out"`
		Sessions    int64     `json:"sessions"`
		RegionCount int       `json:"region_count"`
	}{
		Timestamp:   time.Now(),
		BytesIn:     bytesIn,
		BytesOut:    bytesOut,
		Sessions:    sessions,
		RegionCount: len(regionStats),
	}

	summaryFile := filepath.Join(c.config.StatsDir, "summary.json")
	if data, err := json.MarshalIndent(summary, "", "  "); err == nil {
		if err := os.WriteFile(summaryFile, data, 0644); err != nil {
			log.Warn("Failed to write summary", "error", err)
		}
	}

	log.Debug("Stats flushed to disk", "regions", len(regionStats), "sessions", len(sessionStats))
}

// cleanupWorker periodically cleans up old statistics
func (c *Collector) cleanupWorker() {
	defer c.wg.Done()

	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			c.cleanupOldStats()
		}
	}
}

// cleanupOldStats removes old statistics files
func (c *Collector) cleanupOldStats() {
	entries, err := os.ReadDir(c.config.StatsDir)
	if err != nil {
		return
	}

	cutoff := time.Now().AddDate(0, 0, -c.config.RetentionDays)

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		if info.ModTime().Before(cutoff) {
			path := filepath.Join(c.config.StatsDir, entry.Name())
			if err := os.Remove(path); err != nil {
				log.Warn("Failed to remove old stats file", "path", path, "error", err)
			}
		}
	}
}

// SetReporter sets the stats reporter
func (c *Collector) SetReporter(reporter Reporter) {
	c.reporter = reporter
}

// DefaultReporter is the default implementation of Reporter
type DefaultReporter struct{}

// Report outputs the stats report
func (r *DefaultReporter) Report(stats *StatsReport) {
	log.Info("Stats Report",
		"bytes_in", stats.TotalBytesIn,
		"bytes_out", stats.TotalBytesOut,
		"sessions", stats.TotalSessions,
		"active", stats.ActiveSessions,
		"regions", len(stats.RegionStats),
	)
}

// BillingCycleData holds billing cycle data
type BillingCycleData struct {
	CycleID         string
	StartTime       time.Time
	EndTime         time.Time
	TotalBytesIn    int64
	TotalBytesOut   int64
	TotalCost       int64 // cents
	RegionBreakdown map[string]RegionBilling
}

// RegionBilling holds billing data for a region
type RegionBilling struct {
	Region       string
	BytesIn      int64
	BytesOut     int64
	Cost         int64
	SessionCount int64
}

// CycleManager manages billing cycles
type CycleManager struct {
	mu           sync.RWMutex
	currentCycle *BillingCycleData
	cycleLength  time.Duration
	priceEngine  interface {
		CalculateCost(region string, bytes int64) (cost int64, discount int64)
		GetRegionPrice(region string) string
	}
}

// NewCycleManager creates a new cycle manager
func NewCycleManager(cycleLength time.Duration, priceEngine interface {
	CalculateCost(region string, bytes int64) (cost int64, discount int64)
	GetRegionPrice(region string) string
}) *CycleManager {
	if cycleLength == 0 {
		cycleLength = 30 * 24 * time.Hour // 30 days
	}

	return &CycleManager{
		cycleLength: cycleLength,
		priceEngine: priceEngine,
		currentCycle: &BillingCycleData{
			CycleID:         fmt.Sprintf("cycle-%d", time.Now().Unix()),
			StartTime:       time.Now(),
			RegionBreakdown: make(map[string]RegionBilling),
		},
	}
}

// RecordUsage records usage for a billing cycle
func (cm *CycleManager) RecordUsage(region string, bytesIn, bytesOut int64) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if cm.currentCycle == nil {
		cm.currentCycle = &BillingCycleData{
			CycleID:         fmt.Sprintf("cycle-%d", time.Now().Unix()),
			StartTime:       time.Now(),
			RegionBreakdown: make(map[string]RegionBilling),
		}
	}

	cm.currentCycle.TotalBytesIn += bytesIn
	cm.currentCycle.TotalBytesOut += bytesOut

	// Update region breakdown
	if _, exists := cm.currentCycle.RegionBreakdown[region]; !exists {
		cm.currentCycle.RegionBreakdown[region] = RegionBilling{
			Region: region,
		}
	}

	regionBilling := cm.currentCycle.RegionBreakdown[region]
	regionBilling.BytesIn += bytesIn
	regionBilling.BytesOut += bytesOut
	regionBilling.SessionCount++

	// Calculate cost
	cost, _ := cm.priceEngine.CalculateCost(region, bytesIn+bytesOut)
	regionBilling.Cost += cost
	cm.currentCycle.TotalCost += cost

	cm.currentCycle.RegionBreakdown[region] = regionBilling
}

// GetCurrentCycle returns the current billing cycle data
func (cm *CycleManager) GetCurrentCycle() *BillingCycleData {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	if cm.currentCycle == nil {
		return nil
	}

	cycleCopy := *cm.currentCycle
	return &cycleCopy
}

// GetCycleInfo returns information about the current cycle
func (cm *CycleManager) GetCycleInfo() (cycleID string, start time.Time, daysRemaining int, estimatedCost int64) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	if cm.currentCycle != nil {
		cycleID = cm.currentCycle.CycleID
		start = cm.currentCycle.StartTime
		elapsed := time.Since(start)
		daysRemaining = int((cm.cycleLength - elapsed) / (24 * time.Hour))
		if daysRemaining < 0 {
			daysRemaining = 0
		}
		estimatedCost = cm.currentCycle.TotalCost
	}

	return cycleID, start, daysRemaining, estimatedCost
}

// StartNewCycle starts a new billing cycle
func (cm *CycleManager) StartNewCycle() *BillingCycleData {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Finalize current cycle
	oldCycle := cm.currentCycle

	// Start new cycle
	cm.currentCycle = &BillingCycleData{
		CycleID:         fmt.Sprintf("cycle-%d", time.Now().Unix()),
		StartTime:       time.Now(),
		EndTime:         time.Now().Add(cm.cycleLength),
		RegionBreakdown: make(map[string]RegionBilling),
	}

	return oldCycle
}

// GetPastCycles returns past billing cycles
func (cm *CycleManager) GetPastCycles() []*BillingCycleData {
	// In a full implementation, this would load from persistent storage
	return nil
}

// FormatBytes formats bytes in human-readable format
func FormatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// FormatDuration formats duration in human-readable format
func FormatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	if d < time.Hour {
		return fmt.Sprintf("%.1fm", d.Minutes())
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%.1fh", d.Hours())
	}
	return fmt.Sprintf("%.1fd", d.Hours()/24)
}
