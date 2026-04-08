package pricing

import (
	"testing"
	"time"
)

func TestGPUPricing(t *testing.T) {
	config := Config{
		BasePrice:    0.001, // 0.001元/秒
		Currency:     "CNY",
		MinimumPrice: 0.01,
		TierMultiplier: map[string]float64{
			"RTX 4090": 1.0,
			"A100":     3.0,
			"H100":     8.0,
		},
		MemoryMultiplier: map[string]float64{
			"low":    1.0,
			"medium": 1.5,
			"high":   2.0,
		},
	}

	engine := NewEngine(config)

	// Test pricing for RTX 4090 with 24GB for 1 hour
	result, err := engine.CalculatePrice("RTX 4090", 24*1024, time.Hour, 50)
	if err != nil {
		t.Errorf("CalculatePrice() error = %v", err)
		return
	}

	if result.FinalPrice <= 0 {
		t.Error("expected positive final price")
	}

	// Verify pricing breakdown
	t.Logf("RTX 4090 pricing: UnitPrice=%.6f, FinalPrice=%.6f, Duration=%.2f",
		result.UnitPrice, result.FinalPrice, result.Duration)
}

func TestTierMultiplier(t *testing.T) {
	tests := []struct {
		name               string
		gpuName            string
		expectedMultiplier float64
	}{
		{"RTX 4090", "RTX 4090", 1.0},
		{"A100", "A100", 3.0},
		{"H100", "H100", 8.0},
		{"Unknown GPU", "Unknown", 1.0}, // default
	}

	config := Config{
		BasePrice: 0.001,
		TierMultiplier: map[string]float64{
			"default":  1.0,
			"RTX 4090": 1.0,
			"A100":     3.0,
			"H100":     8.0,
		},
		MemoryMultiplier: map[string]float64{
			"default": 1.0,
		},
	}

	engine := NewEngine(config)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := engine.CalculatePrice(tt.gpuName, 16*1024, time.Hour, 50)
			if err != nil {
				t.Errorf("CalculatePrice() error = %v", err)
				return
			}

			// The tier multiplier affects the unit price
			// BasePrice * TierMultiplier * MemoryMultiplier = UnitPrice
			expectedUnitPrice := config.BasePrice * tt.expectedMultiplier
			if result.UnitPrice != expectedUnitPrice {
				t.Errorf("TierMultiplier for %s = %f, want %f",
					tt.gpuName, result.UnitPrice/config.BasePrice, tt.expectedMultiplier)
			}
		})
	}
}

func TestMemoryMultiplier(t *testing.T) {
	tests := []struct {
		name               string
		memoryGB           int
		expectedMultiplier float64
	}{
		{"low memory (8GB)", 8, 1.0},
		{"medium memory (24GB)", 24, 1.5},
		{"high memory (40GB)", 40, 2.0},
	}

	config := Config{
		BasePrice: 0.001,
		TierMultiplier: map[string]float64{
			"default": 1.0,
		},
		MemoryMultiplier: map[string]float64{
			"default": 1.0,
			"low":     1.0,
			"medium":  1.5,
			"high":    2.0,
		},
	}

	engine := NewEngine(config)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := engine.CalculatePrice("RTX 4090", tt.memoryGB*1024, time.Hour, 50)
			if err != nil {
				t.Errorf("CalculatePrice() error = %v", err)
				return
			}

			// The memory multiplier affects the unit price
			expectedUnitPrice := config.BasePrice * tt.expectedMultiplier
			if result.UnitPrice != expectedUnitPrice {
				t.Errorf("MemoryMultiplier for %s = %f, want %f",
					tt.name, result.UnitPrice/config.BasePrice, tt.expectedMultiplier)
			}
		})
	}
}

func TestCalculatePriceWithVolumeDiscount(t *testing.T) {
	config := Config{
		BasePrice: 0.001,
		TierMultiplier: map[string]float64{
			"default": 1.0,
		},
		MemoryMultiplier: map[string]float64{
			"default": 1.0,
		},
		VolumeDiscount: map[int]float64{
			100:  0.95,
			500:  0.90,
			1000: 0.85,
		},
	}

	engine := NewEngine(config)

	// Test with 500 hours of usage
	result, err := engine.CalculatePriceWithVolumeDiscount("RTX 4090", 24*1024, time.Hour, 50, 500)
	if err != nil {
		t.Errorf("CalculatePriceWithVolumeDiscount() error = %v", err)
		return
	}

	// Should have volume discount applied
	if result.Discount == 0 {
		t.Error("expected volume discount to be applied")
	}

	t.Logf("Volume discount: OriginalPrice=%.6f, FinalPrice=%.6f, Discount=%.2f%%",
		result.TotalPrice, result.FinalPrice, result.Discount*100)
}

func TestGetPricePerHour(t *testing.T) {
	config := Config{
		BasePrice: 0.001,
		TierMultiplier: map[string]float64{
			"default": 1.0,
			"H100":    3.0,
		},
		MemoryMultiplier: map[string]float64{
			"default": 1.0,
			"40GB":    2.0,
		},
	}

	engine := NewEngine(config)

	pricePerHour, err := engine.GetPricePerHour("H100", 40*1024)
	if err != nil {
		t.Errorf("GetPricePerHour() error = %v", err)
		return
	}

	if pricePerHour <= 0 {
		t.Error("expected positive price per hour")
	}

	// H100 with 40GB: 0.001 * 3.0 * 2.0 = 0.006元/秒 = 21.6元/小时
	expectedPrice := 0.001 * 3.0 * 2.0 * 3600
	if pricePerHour < expectedPrice*0.9 || pricePerHour > expectedPrice*1.1 {
		t.Errorf("GetPricePerHour() = %f, expected around %f", pricePerHour, expectedPrice)
	}
}

func TestPricingEngine_InvalidDuration(t *testing.T) {
	config := Config{
		BasePrice: 0.001,
	}

	engine := NewEngine(config)

	// Test with zero duration
	_, err := engine.CalculatePrice("RTX 4090", 24*1024, 0, 50)
	if err == nil {
		t.Error("expected error for zero duration")
	}

	// Test with negative duration
	_, err = engine.CalculatePrice("RTX 4090", 24*1024, -time.Hour, 50)
	if err == nil {
		t.Error("expected error for negative duration")
	}
}

func TestCreateBill(t *testing.T) {
	config := Config{
		BasePrice: 0.001,
		Currency:  "CNY",
	}

	engine := NewEngine(config)

	records := []*UsageRecord{
		{
			GPUID:      "gpu-001",
			UserID:     "user-001",
			Duration:   3600, // 1 hour
			GPUName:    "RTX 4090",
			UnitPrice:  0.001,
			TotalPrice: 3.6,
		},
		{
			GPUID:      "gpu-002",
			UserID:     "user-001",
			Duration:   1800, // 30 minutes
			GPUName:    "RTX 4090",
			UnitPrice:  0.001,
			TotalPrice: 1.8,
		},
	}

	bill := engine.CreateBill("user-001", records, "2024-01")

	if bill.UserID != "user-001" {
		t.Errorf("bill.UserID = %s, want user-001", bill.UserID)
	}

	if bill.Currency != "CNY" {
		t.Errorf("bill.Currency = %s, want CNY", bill.Currency)
	}

	expectedTotal := 3.6 + 1.8
	if bill.TotalAmount != expectedTotal {
		t.Errorf("bill.TotalAmount = %f, want %f", bill.TotalAmount, expectedTotal)
	}
}

func TestAddDiscount(t *testing.T) {
	config := Config{
		BasePrice: 0.001,
	}

	engine := NewEngine(config)

	bill := &Bill{
		ID:          "bill-001",
		UserID:      "user-001",
		TotalAmount: 100.0,
		FinalAmount: 100.0,
	}

	// Add 10% discount
	engine.AddDiscount(bill, "percentage", "volume", "10% off", 0, 0.90)

	expectedFinal := 100.0 * 0.90
	if bill.FinalAmount != expectedFinal {
		t.Errorf("bill.FinalAmount = %f, want %f", bill.FinalAmount, expectedFinal)
	}

	if len(bill.Discounts) != 1 {
		t.Errorf("expected 1 discount, got %d", len(bill.Discounts))
	}
}

func TestGetPriceEstimate(t *testing.T) {
	config := Config{
		BasePrice: 0.001,
	}

	engine := NewEngine(config)

	result, err := engine.GetPriceEstimate("H100", 80*1024, time.Hour)
	if err != nil {
		t.Errorf("GetPriceEstimate() error = %v", err)
		return
	}

	if result.GPUName != "H100" {
		t.Errorf("result.GPUName = %s, want H100", result.GPUName)
	}

	if result.MemoryGB != 80 {
		t.Errorf("result.MemoryGB = %d, want 80", result.MemoryGB)
	}
}

func TestPricingResultBreakdown(t *testing.T) {
	config := Config{
		BasePrice: 0.001,
		TierMultiplier: map[string]float64{
			"default":  1.0,
			"RTX 4090": 1.0,
		},
		MemoryMultiplier: map[string]float64{
			"default": 1.0,
		},
	}

	engine := NewEngine(config)

	result, err := engine.CalculatePrice("RTX 4090", 24*1024, time.Hour, 50)
	if err != nil {
		t.Fatalf("CalculatePrice() error = %v", err)
	}

	if len(result.Breakdown) == 0 {
		t.Error("expected non-empty breakdown")
	}

	// Check breakdown items
	for _, item := range result.Breakdown {
		t.Logf("Breakdown: %s = %.4f %s", item.Name, item.Amount, item.Unit)
	}
}
