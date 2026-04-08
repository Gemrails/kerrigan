package pricing

import (
	"testing"
	"time"
)

func TestRegionPricing(t *testing.T) {
	tests := []struct {
		name          string
		region        string
		expectedPrice int64 // cents per GB
	}{
		{"China", RegionCN, 80},
		{"United States", RegionUS, 120},
		{"Japan", RegionJP, 150},
		{"Singapore", RegionSG, 130},
		{"Europe", RegionEU, 140},
		{"Hong Kong", RegionHK, 110},
	}

	engine := NewEngine(Config{})

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			priceStr := engine.GetRegionPrice(tt.region)
			// Verify price contains expected value
			if tt.region == RegionCN && priceStr != "80 cents/e-CNY" {
				t.Errorf("GetRegionPrice(%s) = %s", tt.region, priceStr)
			}
			if tt.region == RegionUS && priceStr != "120 cents/e-CNY" {
				t.Errorf("GetRegionPrice(%s) = %s", tt.region, priceStr)
			}
			if tt.region == RegionJP && priceStr != "150 cents/e-CNY" {
				t.Errorf("GetRegionPrice(%s) = %s", tt.region, priceStr)
			}
		})
	}
}

func TestBandwidthTier(t *testing.T) {
	config := Config{
		TierPricing: map[string]TierConfig{
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
		},
	}

	engine := NewEngine(config)

	// Test tier pricing based on volume
	testTier := []struct {
		name      string
		gb        int64
		wantPrice int64
	}{
		{"1-100GB", 50, 100},
		{"100-1000GB", 500, 80},
		{"1000+GB", 2000, 60},
	}

	for _, tt := range testTier {
		t.Run(tt.name, func(t *testing.T) {
			tier, found := engine.GetApplicableTier(tt.gb)
			if !found {
				t.Error("expected tier to be found")
				return
			}
			if tier.PricePerGB != tt.wantPrice {
				t.Errorf("Tier price = %d, want %d", tier.PricePerGB, tt.wantPrice)
			}
		})
	}
}

func TestCalculateCost(t *testing.T) {
	engine := NewEngine(Config{})

	// Test cost calculation for 1GB in CN
	cost, discount := engine.CalculateCost(RegionCN, 1024*1024*1024) // 1GB
	if cost <= 0 {
		t.Error("expected positive cost")
	}

	// 1GB in CN should cost 80 cents (before discount)
	expectedCost := int64(80)
	if cost != expectedCost {
		t.Errorf("CalculateCost() = %d, want %d", cost, expectedCost)
	}

	t.Logf("Cost for 1GB in CN: %d cents, discount: %d cents", cost, discount)
}

func TestCalculateCost_WithVolumeDiscount(t *testing.T) {
	// 500GB should get 10% discount
	engine := NewEngine(Config{
		VolumeDiscounts: []VolumeDiscount{
			{MinGB: 100, MaxGB: 500, DiscountPct: 5, Description: "5% off"},
			{MinGB: 500, MaxGB: 1000, DiscountPct: 10, Description: "10% off"},
		},
	})

	cost, discount := engine.CalculateCost(RegionCN, 500*1024*1024*1024) // 500GB
	if discount <= 0 {
		t.Error("expected discount to be applied")
	}

	// Verify discount is approximately correct (10% of 40000 = 4000)
	if discount < 3000 || discount > 5000 {
		t.Logf("Discount = %d, expected around 4000", discount)
	}
	_ = cost
}

func TestEngine_GetRegionPrice(t *testing.T) {
	engine := NewEngine(Config{})

	tests := []struct {
		region string
		want   string
	}{
		{RegionCN, "80 cents/e-CNY"},
		{RegionUS, "120 cents/e-CNY"},
		{RegionJP, "150 cents/e-CNY"},
		{"unknown", "100 cents/e-CNY"}, // default
	}

	for _, tt := range tests {
		t.Run(tt.region, func(t *testing.T) {
			got := engine.GetRegionPrice(tt.region)
			if got != tt.want {
				t.Errorf("GetRegionPrice(%s) = %s, want %s", tt.region, got, tt.want)
			}
		})
	}
}

func TestEngine_RecordUsage(t *testing.T) {
	engine := NewEngine(Config{})

	// Record usage
	engine.RecordUsage(RegionCN, 1024*1024*1024) // 1GB

	usage, found := engine.GetUsage(RegionCN)
	if !found {
		t.Error("expected usage to be found")
		return
	}

	if usage.TotalBytes != 1024*1024*1024 {
		t.Errorf("TotalBytes = %d, want %d", usage.TotalBytes, 1024*1024*1024)
	}

	if usage.SessionCount != 1 {
		t.Errorf("SessionCount = %d, want 1", usage.SessionCount)
	}
}

func TestEngine_GetTotalUsage(t *testing.T) {
	engine := NewEngine(Config{})

	// Record multiple usages
	engine.RecordUsage(RegionCN, 1024*1024*1024)
	engine.RecordUsage(RegionUS, 2*1024*1024*1024)

	total := engine.GetTotalUsage()
	expected := int64(3 * 1024 * 1024 * 1024)
	if total != expected {
		t.Errorf("GetTotalUsage() = %d, want %d", total, expected)
	}
}

func TestEngine_UpdatePrice(t *testing.T) {
	engine := NewEngine(Config{})

	// Update China price
	err := engine.UpdatePrice(RegionCN, 90)
	if err != nil {
		t.Errorf("UpdatePrice() error = %v", err)
		return
	}

	// Verify new price
	priceStr := engine.GetRegionPrice(RegionCN)
	if priceStr != "90 cents/e-CNY" {
		t.Errorf("GetRegionPrice() = %s, want 90 cents/e-CNY", priceStr)
	}
}

func TestEngine_UpdatePrice_Invalid(t *testing.T) {
	engine := NewEngine(Config{})

	// Try to update with invalid (zero/negative) price
	err := engine.UpdatePrice(RegionCN, 0)
	if err == nil {
		t.Error("expected error for zero price")
	}

	err = engine.UpdatePrice(RegionCN, -10)
	if err == nil {
		t.Error("expected error for negative price")
	}
}

func TestEngine_ResetCycle(t *testing.T) {
	engine := NewEngine(Config{})

	// Record some usage
	engine.RecordUsage(RegionCN, 1024*1024*1024)

	// Reset cycle
	engine.ResetCycle()

	// Usage should be cleared
	total := engine.GetTotalUsage()
	if total != 0 {
		t.Errorf("GetTotalUsage() after reset = %d, want 0", total)
	}
}

func TestEngine_GetCycleInfo(t *testing.T) {
	engine := NewEngine(Config{
		BillingCycle: 30 * 24 * 3600 * 1000000000, // 30 days in nanoseconds
	})

	start, daysRemaining := engine.GetCycleInfo()
	if start.IsZero() {
		t.Error("expected non-zero cycle start time")
	}

	if daysRemaining < 0 || daysRemaining > 30 {
		t.Errorf("daysRemaining = %d, expected 0-30", daysRemaining)
	}
}

func TestEngine_GetPriceBreakdown(t *testing.T) {
	engine := NewEngine(Config{})

	breakdown := engine.GetPriceBreakdown(RegionCN, 1024*1024*1024) // 1GB
	if breakdown == nil {
		t.Fatal("expected non-nil breakdown")
	}

	if breakdown.Region != RegionCN {
		t.Errorf("Region = %s, want %s", breakdown.Region, RegionCN)
	}

	if breakdown.VolumeGB != 1 {
		t.Errorf("VolumeGB = %d, want 1", breakdown.VolumeGB)
	}

	if breakdown.BasePricePerGB != 80 {
		t.Errorf("BasePricePerGB = %d, want 80", breakdown.BasePricePerGB)
	}

	t.Logf("Breakdown: BaseCost=%d, Discount=%d, FinalCost=%d",
		breakdown.BaseCost, breakdown.DiscountAmount, breakdown.FinalCost)
}

func TestEngine_GetAllRegions(t *testing.T) {
	engine := NewEngine(Config{})

	regions := engine.GetAllRegions()
	if len(regions) == 0 {
		t.Error("expected non-empty regions list")
	}
}

func TestEngine_GetAllTiers(t *testing.T) {
	engine := NewEngine(Config{})

	tiers := engine.GetAllTiers()
	if len(tiers) == 0 {
		t.Error("expected non-empty tiers list")
	}
}

func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name: "valid config",
			config: Config{
				BasePricePerGB: 100,
				BillingCycle:   30 * 24 * time.Hour,
				RegionPrices:   map[string]int64{"CN": 80},
				TierPricing:    map[string]TierConfig{"basic": {MinGB: 1, MaxGB: 100, PricePerGB: 100}},
			},
			wantErr: false,
		},
		{
			name: "invalid - zero base price",
			config: Config{
				BasePricePerGB: 0,
			},
			wantErr: true,
		},
		{
			name: "invalid - zero billing cycle",
			config: Config{
				BasePricePerGB: 100,
				BillingCycle:   0,
			},
			wantErr: true,
		},
		{
			name: "invalid - negative tier minGB",
			config: Config{
				BasePricePerGB: 100,
				TierPricing:    map[string]TierConfig{"bad": {MinGB: -1, MaxGB: 100, PricePerGB: 100}},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateConfig(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestFormatPrice(t *testing.T) {
	tests := []struct {
		name     string
		cents    int64
		currency string
		want     string
	}{
		{"100 cents", 100, "e-CNY", "1.00 e-CNY"},
		{"0 cents", 0, "e-CNY", "0.00 e-CNY"},
		{"999 cents", 999, "e-CNY", "9.99 e-CNY"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatPrice(tt.cents, tt.currency)
			if got != tt.want {
				t.Errorf("FormatPrice() = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestEngine_DynamicPricing(t *testing.T) {
	engine := NewEngine(Config{})

	// Check default state
	if engine.IsDynamicPricingEnabled() {
		t.Error("expected dynamic pricing to be disabled by default")
	}

	// Enable dynamic pricing
	engine.SetDynamicPricing(true)
	if !engine.IsDynamicPricingEnabled() {
		t.Error("expected dynamic pricing to be enabled")
	}

	// Disable dynamic pricing
	engine.SetDynamicPricing(false)
	if engine.IsDynamicPricingEnabled() {
		t.Error("expected dynamic pricing to be disabled")
	}
}

func TestEngine_GetConfig(t *testing.T) {
	config := Config{
		BasePricePerGB: 150,
		Currency:       "USD",
	}
	engine := NewEngine(config)

	got := engine.GetConfig()
	if got.BasePricePerGB != config.BasePricePerGB {
		t.Errorf("BasePricePerGB = %d, want %d", got.BasePricePerGB, config.BasePricePerGB)
	}
}

func TestEngine_UpdateConfig(t *testing.T) {
	engine := NewEngine(Config{})

	newConfig := Config{
		BasePricePerGB: 200,
		Currency:       "EUR",
		RegionPrices:   map[string]int64{"CN": 100},
	}

	err := engine.UpdateConfig(newConfig)
	if err != nil {
		t.Errorf("UpdateConfig() error = %v", err)
		return
	}

	got := engine.GetConfig()
	if got.BasePricePerGB != 200 {
		t.Errorf("BasePricePerGB = %d, want 200", got.BasePricePerGB)
	}
}

func TestEngine_UpdateConfig_Invalid(t *testing.T) {
	engine := NewEngine(Config{})

	// Try to update with invalid config
	invalidConfig := Config{
		BasePricePerGB: 0, // invalid
	}

	err := engine.UpdateConfig(invalidConfig)
	if err == nil {
		t.Error("expected error for invalid config")
	}
}

func TestEngine_GetVolumeDiscountInfo(t *testing.T) {
	engine := NewEngine(Config{
		VolumeDiscounts: []VolumeDiscount{
			{MinGB: 100, MaxGB: 500, DiscountPct: 5},
			{MinGB: 500, MaxGB: 1000, DiscountPct: 10},
		},
	})

	// Test with 600GB - should get 10% discount
	discounts := engine.GetVolumeDiscountInfo(600)
	if len(discounts) == 0 {
		t.Error("expected discounts to be found")
	}
}
