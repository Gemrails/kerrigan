package pricing

import (
	"fmt"
	"sync"
	"time"
)

// Region codes
const (
	RegionCN      = "CN" // China
	RegionUS      = "US" // United States
	RegionJP      = "JP" // Japan
	RegionSG      = "SG" // Singapore
	RegionEU      = "EU" // Europe
	RegionHK      = "HK" // Hong Kong
	RegionDefault = "default"
)

// Config holds pricing configuration
type Config struct {
	// Base price per GB in cents (e-CNY)
	BasePricePerGB int64

	// Region-specific pricing (cents per GB)
	RegionPrices map[string]int64

	// Bandwidth tier pricing
	TierPricing map[string]TierConfig

	// Volume discount thresholds
	VolumeDiscounts []VolumeDiscount

	// Currency (default: e-CNY cents)
	Currency string

	// Billing cycle
	BillingCycle time.Duration

	// Minimum volume (GB)
	MinVolumeGB int64

	// Dynamic pricing enabled
	DynamicPricing bool

	// Price update interval
	PriceUpdateInterval time.Duration
}

// TierConfig represents bandwidth tier configuration
type TierConfig struct {
	Name       string
	MinGB      int64
	MaxGB      int64 // 0 = unlimited
	PricePerGB int64 // cents per GB
	Priority   int   // 0 = highest priority
}

// VolumeDiscount represents volume-based discount
type VolumeDiscount struct {
	MinGB       int64
	MaxGB       int64 // 0 = unlimited
	DiscountPct int   // percentage discount (0-100)
	Description string
}

// Engine implements the pricing engine
type Engine struct {
	config Config
	mu     sync.RWMutex

	// Dynamic pricing state
	currentPrices   map[string]int64
	lastPriceUpdate time.Time

	// Usage tracking
	usage map[string]*RegionUsage // per region usage

	// Billing
	currentCycleStart time.Time
}

// RegionUsage tracks usage per region
type RegionUsage struct {
	Region       string
	TotalBytes   int64
	SessionCount int64
	LastActivity time.Time
}

// NewEngine creates a new pricing engine
func NewEngine(cfg Config) *Engine {
	if cfg.BasePricePerGB == 0 {
		cfg.BasePricePerGB = 100 // Default: 1.00 e-CNY per GB
	}

	if cfg.RegionPrices == nil {
		cfg.RegionPrices = map[string]int64{
			RegionCN: 80,  // China: 0.80 e-CNY/GB (cheapest)
			RegionUS: 120, // US: 1.20 e-CNY/GB
			RegionJP: 150, // Japan: 1.50 e-CNY/GB
			RegionSG: 130, // Singapore: 1.30 e-CNY/GB
			RegionEU: 140, // Europe: 1.40 e-CNY/GB
			RegionHK: 110, // Hong Kong: 1.10 e-CNY/GB
		}
	}

	if cfg.TierPricing == nil {
		cfg.TierPricing = map[string]TierConfig{
			"free": {
				Name:       "Free Tier",
				MinGB:      0,
				MaxGB:      1,
				PricePerGB: 200,
				Priority:   3,
			},
			"basic": {
				Name:       "Basic Tier",
				MinGB:      1,
				MaxGB:      100,
				PricePerGB: 100,
				Priority:   2,
			},
			"pro": {
				Name:       "Pro Tier",
				MinGB:      100,
				MaxGB:      1000,
				PricePerGB: 80,
				Priority:   1,
			},
			"enterprise": {
				Name:       "Enterprise Tier",
				MinGB:      1000,
				MaxGB:      0, // unlimited
				PricePerGB: 60,
				Priority:   0,
			},
		}
	}

	if cfg.VolumeDiscounts == nil {
		cfg.VolumeDiscounts = []VolumeDiscount{
			{MinGB: 100, MaxGB: 500, DiscountPct: 5, Description: "5% off for 100-500 GB"},
			{MinGB: 500, MaxGB: 1000, DiscountPct: 10, Description: "10% off for 500-1000 GB"},
			{MinGB: 1000, MaxGB: 5000, DiscountPct: 15, Description: "15% off for 1-5 TB"},
			{MinGB: 5000, MaxGB: 0, DiscountPct: 25, Description: "25% off for 5+ TB"},
		}
	}

	if cfg.Currency == "" {
		cfg.Currency = "e-CNY"
	}

	if cfg.BillingCycle == 0 {
		cfg.BillingCycle = 30 * 24 * time.Hour // 30 days
	}

	if cfg.MinVolumeGB == 0 {
		cfg.MinVolumeGB = 1
	}

	engine := &Engine{
		config:            cfg,
		currentPrices:     make(map[string]int64),
		usage:             make(map[string]*RegionUsage),
		currentCycleStart: time.Now(),
	}

	// Initialize prices
	for region, price := range cfg.RegionPrices {
		engine.currentPrices[region] = price
	}

	return engine
}

// GetRegionPrice returns the price for a specific region (cents per GB)
func (e *Engine) GetRegionPrice(region string) string {
	e.mu.RLock()
	defer e.mu.RUnlock()

	price := e.currentPrices[region]
	if price == 0 {
		price = e.config.BasePricePerGB
	}

	return fmt.Sprintf("%d cents/%s", price, e.config.Currency)
}

// GetDefaultPrice returns the default price
func (e *Engine) GetDefaultPrice() string {
	e.mu.RLock()
	defer e.mu.RUnlock()

	return fmt.Sprintf("%d cents/%s", e.config.BasePricePerGB, e.config.Currency)
}

// CalculateCost calculates the cost for given region and volume
func (e *Engine) CalculateCost(region string, bytes int64) (cost int64, discount int64) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	gb := float64(bytes) / (1024 * 1024 * 1024)
	gbInt := int64(gb)

	// Get base price for region
	pricePerGB := e.currentPrices[region]
	if pricePerGB == 0 {
		pricePerGB = e.config.BasePricePerGB
	}

	// Calculate base cost
	baseCost := gbInt * pricePerGB

	// Apply volume discount
	discountPct := e.calculateDiscount(gbInt)
	discount = (baseCost * int64(discountPct)) / 100
	cost = baseCost - discount

	return cost, discount
}

// calculateDiscount calculates the applicable discount percentage
func (e *Engine) calculateDiscount(gb int64) int {
	discountPct := 0
	for _, discount := range e.config.VolumeDiscounts {
		if gb >= discount.MinGB {
			if discount.MaxGB == 0 || gb < discount.MaxGB {
				discountPct = discount.DiscountPct
			}
		}
	}
	return discountPct
}

// GetTierPrice returns the price for a specific tier
func (e *Engine) GetTierPrice(tierName string) (TierConfig, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	tier, exists := e.config.TierPricing[tierName]
	return tier, exists
}

// GetApplicableTier returns the applicable tier for a given volume
func (e *Engine) GetApplicableTier(gb int64) (TierConfig, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var bestTier TierConfig
	var found bool

	for _, tier := range e.config.TierPricing {
		if gb >= tier.MinGB {
			if tier.MaxGB == 0 || gb < tier.MaxGB {
				if !found || tier.Priority < bestTier.Priority {
					bestTier = tier
					found = true
				}
			}
		}
	}

	return bestTier, found
}

// RecordUsage records usage for a region
func (e *Engine) RecordUsage(region string, bytes int64) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.usage[region]; !exists {
		e.usage[region] = &RegionUsage{
			Region:       region,
			TotalBytes:   0,
			SessionCount: 0,
			LastActivity: time.Now(),
		}
	}

	e.usage[region].TotalBytes += bytes
	e.usage[region].SessionCount++
	e.usage[region].LastActivity = time.Now()
}

// GetUsage returns usage statistics for a region
func (e *Engine) GetUsage(region string) (*RegionUsage, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	usage, exists := e.usage[region]
	return usage, exists
}

// GetAllUsage returns all usage statistics
func (e *Engine) GetAllUsage() map[string]*RegionUsage {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := make(map[string]*RegionUsage)
	for region, usage := range e.usage {
		result[region] = usage
	}

	return result
}

// GetTotalUsage returns total bytes used across all regions
func (e *Engine) GetTotalUsage() int64 {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var total int64
	for _, usage := range e.usage {
		total += usage.TotalBytes
	}

	return total
}

// UpdatePrice updates the price for a region
func (e *Engine) UpdatePrice(region string, pricePerGB int64) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if pricePerGB <= 0 {
		return fmt.Errorf("price must be positive")
	}

	e.currentPrices[region] = pricePerGB
	e.lastPriceUpdate = time.Now()

	return nil
}

// ResetCycle resets the billing cycle
func (e *Engine) ResetCycle() {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.currentCycleStart = time.Now()
	e.usage = make(map[string]*RegionUsage)
}

// GetCycleInfo returns current billing cycle information
func (e *Engine) GetCycleInfo() (start time.Time, daysRemaining int) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	start = e.currentCycleStart
	elapsed := time.Since(start)
	daysRemaining = int((e.config.BillingCycle - elapsed) / (24 * time.Hour))
	if daysRemaining < 0 {
		daysRemaining = 0
	}

	return start, daysRemaining
}

// GetPriceBreakdown returns detailed price breakdown
func (e *Engine) GetPriceBreakdown(region string, bytes int64) *PriceBreakdown {
	e.mu.RLock()
	defer e.mu.RUnlock()

	gb := float64(bytes) / (1024 * 1024 * 1024)
	gbInt := int64(gb)

	// Get base price
	pricePerGB := e.currentPrices[region]
	if pricePerGB == 0 {
		pricePerGB = e.config.BasePricePerGB
	}

	baseCost := gbInt * pricePerGB

	// Get applicable discount
	discountPct := e.calculateDiscount(gbInt)
	discountAmount := (baseCost * int64(discountPct)) / 100

	// Get applicable tier
	tier, tierFound := e.GetApplicableTier(gbInt)

	return &PriceBreakdown{
		Region:          region,
		VolumeGB:        gbInt,
		BasePricePerGB:  pricePerGB,
		BaseCost:        baseCost,
		DiscountPct:     discountPct,
		DiscountAmount:  discountAmount,
		FinalCost:       baseCost - discountAmount,
		Currency:        e.config.Currency,
		Tier:            tier.Name,
		TierFound:       tierFound,
		VolumeDiscounts: e.config.VolumeDiscounts,
	}
}

// PriceBreakdown represents detailed price breakdown
type PriceBreakdown struct {
	Region          string
	VolumeGB        int64
	BasePricePerGB  int64
	BaseCost        int64
	DiscountPct     int
	DiscountAmount  int64
	FinalCost       int64
	Currency        string
	Tier            string
	TierFound       bool
	VolumeDiscounts []VolumeDiscount
}

// GetAllRegions returns all supported regions
func (e *Engine) GetAllRegions() []string {
	e.mu.RLock()
	defer e.mu.RUnlock()

	regions := make([]string, 0, len(e.currentPrices))
	for region := range e.currentPrices {
		regions = append(regions, region)
	}

	return regions
}

// GetAllTiers returns all available tiers
func (e *Engine) GetAllTiers() []TierConfig {
	e.mu.RLock()
	defer e.mu.RUnlock()

	tiers := make([]TierConfig, 0, len(e.config.TierPricing))
	for _, tier := range e.config.TierPricing {
		tiers = append(tiers, tier)
	}

	return tiers
}

// ValidateConfig validates the pricing configuration
func ValidateConfig(cfg Config) error {
	if cfg.BasePricePerGB <= 0 {
		return fmt.Errorf("base price must be positive")
	}

	if cfg.BillingCycle <= 0 {
		return fmt.Errorf("billing cycle must be positive")
	}

	for region, price := range cfg.RegionPrices {
		if price <= 0 {
			return fmt.Errorf("price for region %s must be positive", region)
		}
	}

	for name, tier := range cfg.TierPricing {
		if tier.MinGB < 0 {
			return fmt.Errorf("tier %s minGB cannot be negative", name)
		}
		if tier.MaxGB > 0 && tier.MinGB >= tier.MaxGB {
			return fmt.Errorf("tier %s minGB must be less than maxGB", name)
		}
		if tier.PricePerGB <= 0 {
			return fmt.Errorf("tier %s price must be positive", name)
		}
	}

	for _, discount := range cfg.VolumeDiscounts {
		if discount.MinGB < 0 {
			return fmt.Errorf("discount minGB cannot be negative")
		}
		if discount.MaxGB > 0 && discount.MinGB >= discount.MaxGB {
			return fmt.Errorf("discount minGB must be less than maxGB")
		}
		if discount.DiscountPct < 0 || discount.DiscountPct > 100 {
			return fmt.Errorf("discount percentage must be between 0 and 100")
		}
	}

	return nil
}

// FormatPrice formats price in human-readable format
func FormatPrice(cents int64, currency string) string {
	dollars := float64(cents) / 100.0
	return fmt.Sprintf("%.2f %s", dollars, currency)
}

// GetVolumeDiscountInfo returns information about applicable volume discounts
func (e *Engine) GetVolumeDiscountInfo(gb int64) []VolumeDiscount {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var applicable []VolumeDiscount
	for _, discount := range e.config.VolumeDiscounts {
		if gb >= discount.MinGB {
			if discount.MaxGB == 0 || gb < discount.MaxGB {
				applicable = append(applicable, discount)
			}
		}
	}

	return applicable
}

// SetDynamicPricing enables or disables dynamic pricing
func (e *Engine) SetDynamicPricing(enabled bool) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.config.DynamicPricing = enabled
}

// IsDynamicPricingEnabled returns whether dynamic pricing is enabled
func (e *Engine) IsDynamicPricingEnabled() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()

	return e.config.DynamicPricing
}

// GetConfig returns the current configuration
func (e *Engine) GetConfig() Config {
	e.mu.RLock()
	defer e.mu.RUnlock()

	return e.config
}

// UpdateConfig updates the pricing configuration
func (e *Engine) UpdateConfig(newConfig Config) error {
	if err := ValidateConfig(newConfig); err != nil {
		return err
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	e.config = newConfig

	// Update prices
	for region, price := range newConfig.RegionPrices {
		e.currentPrices[region] = price
	}

	return nil
}
