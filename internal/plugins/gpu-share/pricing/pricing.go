package pricing

import (
	"fmt"
	"sync"
	"time"
)

// Config holds pricing configuration
type Config struct {
	BasePrice           float64            `json:"base_price"` // 元/秒
	TierMultiplier      map[string]float64 `json:"tier_multiplier"`
	MemoryMultiplier    map[string]float64 `json:"memory_multiplier"`
	UtilizationDiscount map[int]float64    `json:"utilization_discount"` // utilization -> discount
	TimeDiscount        map[string]float64 `json:"time_discount"`        // time range -> discount
	VolumeDiscount      map[int]float64    `json:"volume_discount"`      // hours -> discount
	MinimumPrice        float64            `json:"minimum_price"`        // 最低价格（元）
	Currency            string             `json:"currency"`             // 货币单位
}

// GPUPricing holds GPU pricing configuration
type GPUPricing struct {
	BasePrice        float64            `json:"base_price"`        // 元/秒
	TierMultiplier   map[string]float64 `json:"tier_multiplier"`   // GPU型号系数
	MemoryMultiplier map[string]float64 `json:"memory_multiplier"` // 显存系数
}

// PricingResult represents the pricing calculation result
type PricingResult struct {
	UnitPrice  float64          `json:"unit_price"`  // 单价（元/秒）
	TotalPrice float64          `json:"total_price"` // 总价（元）
	Duration   float64          `json:"duration"`    // 时长（秒）
	GPUName    string           `json:"gpu_name"`    // GPU型号
	MemoryGB   int              `json:"memory_gb"`   // 显存大小
	Discount   float64          `json:"discount"`    // 折扣
	FinalPrice float64          `json:"final_price"` // 最终价格
	Currency   string           `json:"currency"`    // 货币
	Breakdown  []PriceBreakdown `json:"breakdown"`   // 价格明细
}

// PriceBreakdown represents a single price component
type PriceBreakdown struct {
	Name     string  `json:"name"`     // 名称
	Amount   float64 `json:"amount"`   // 金额
	Quantity float64 `json:"quantity"` // 数量
	Unit     string  `json:"unit"`     // 单位
}

// UsageRecord represents a usage record for billing
type UsageRecord struct {
	ID           string    `json:"id"`             // 记录ID
	GPUID        string    `json:"gpu_id"`         // GPU ID
	UserID       string    `json:"user_id"`        // 用户ID
	StartTime    time.Time `json:"start_time"`     // 开始时间
	EndTime      time.Time `json:"end_time"`       // 结束时间
	Duration     float64   `json:"duration"`       // 时长（秒）
	GPUName      string    `json:"gpu_name"`       // GPU型号
	MemoryUsedMB int       `json:"memory_used_mb"` // 使用显存
	UnitPrice    float64   `json:"unit_price"`     // 单价
	TotalPrice   float64   `json:"total_price"`    // 总价
	TaskID       string    `json:"task_id"`        // 任务ID
	ModelID      string    `json:"model_id"`       // 模型ID
}

// Bill represents a user bill
type Bill struct {
	ID          string         `json:"id"`           // 账单ID
	UserID      string         `json:"user_id"`      // 用户ID
	Period      string         `json:"period"`       // 账期
	TotalAmount float64        `json:"total_amount"` // 总金额
	Currency    string         `json:"currency"`     // 货币
	Records     []*UsageRecord `json:"records"`      // 使用记录
	Discounts   []Discount     `json:"discounts"`    // 折扣
	FinalAmount float64        `json:"final_amount"` // 最终金额
	CreatedAt   time.Time      `json:"created_at"`   // 创建时间
	Status      string         `json:"status"`       // 状态
}

// Discount represents a discount
type Discount struct {
	Type        string  `json:"type"`        // 折扣类型
	Name        string  `json:"name"`        // 折扣名称
	Amount      float64 `json:"amount"`      // 折扣金额
	Percentage  float64 `json:"percentage"`  // 折扣百分比
	Description string  `json:"description"` // 描述
}

// Engine is the pricing engine
type Engine struct {
	config Config
	mu     sync.RWMutex
}

// NewEngine creates a new pricing engine
func NewEngine(config Config) *Engine {
	if config.BasePrice == 0 {
		config.BasePrice = 0.001 // 0.001元/秒 = 3.6元/小时
	}
	if config.Currency == "" {
		config.Currency = "CNY"
	}
	if config.MinimumPrice == 0 {
		config.MinimumPrice = 0.01 // 最低1分钱
	}

	// Set default tier multipliers
	if config.TierMultiplier == nil {
		config.TierMultiplier = map[string]float64{
			"default":  1.0,
			"H100":     3.0,
			"A100":     2.5,
			"A10":      1.5,
			"RTX 4090": 1.2,
			"RTX 3090": 1.0,
			"RTX 3080": 0.8,
			"V100":     1.8,
			"T4":       0.7,
			"L40":      2.0,
		}
	}

	// Set default memory multipliers
	if config.MemoryMultiplier == nil {
		config.MemoryMultiplier = map[string]float64{
			"default": 1.0,
			"80GB":    2.0,
			"40GB":    1.5,
			"24GB":    1.2,
			"16GB":    1.0,
			"12GB":    0.8,
			"8GB":     0.6,
			"6GB":     0.4,
		}
	}

	// Set default utilization discounts
	if config.UtilizationDiscount == nil {
		config.UtilizationDiscount = map[int]float64{
			100: 1.0, // 100% 利用率不打折
			90:  0.95,
			80:  0.90,
			70:  0.85,
			60:  0.80,
			50:  0.75,
			0:   0.70, // 低于50% 打7折
		}
	}

	// Set default time discounts
	if config.TimeDiscount == nil {
		config.TimeDiscount = map[string]float64{
			"off_peak": 0.8,  // 闲时8折
			"night":    0.7,  // 夜间7折
			"weekend":  0.9,  // 周末9折
			"holiday":  0.85, // 节假日8.5折
		}
	}

	// Set default volume discounts
	if config.VolumeDiscount == nil {
		config.VolumeDiscount = map[int]float64{
			100:  0.95, // 100小时以上 5% off
			500:  0.90, // 500小时以上 10% off
			1000: 0.85, // 1000小时以上 15% off
			5000: 0.80, // 5000小时以上 20% off
		}
	}

	return &Engine{
		config: config,
	}
}

// CalculatePrice calculates the price for GPU usage
func (e *Engine) CalculatePrice(gpuName string, memoryMB int, duration time.Duration, utilization int) (*PricingResult, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	durationSeconds := duration.Seconds()
	if durationSeconds <= 0 {
		return nil, fmt.Errorf("invalid duration: %v", duration)
	}

	// Get GPU tier multiplier
	tierMultiplier := e.getTierMultiplier(gpuName)

	// Get memory multiplier
	memoryGB := memoryMB / 1024
	memoryMultiplier := e.getMemoryMultiplier(memoryGB)

	// Calculate base price
	basePrice := e.config.BasePrice * tierMultiplier * memoryMultiplier

	// Get utilization discount
	utilizationDiscount := e.getUtilizationDiscount(utilization)

	// Calculate unit price
	unitPrice := basePrice * utilizationDiscount

	// Calculate total price
	totalPrice := unitPrice * durationSeconds

	// Apply time discount based on current time
	timeDiscount := e.getTimeDiscount()
	totalPrice *= timeDiscount

	// Ensure minimum price
	if totalPrice < e.config.MinimumPrice {
		totalPrice = e.config.MinimumPrice
	}

	// Calculate discount
	discount := 1.0 - (totalPrice / (unitPrice * durationSeconds))
	if discount < 0 {
		discount = 0
	}

	result := &PricingResult{
		UnitPrice:  unitPrice,
		TotalPrice: totalPrice,
		Duration:   durationSeconds,
		GPUName:    gpuName,
		MemoryGB:   memoryGB,
		Discount:   discount,
		FinalPrice: totalPrice,
		Currency:   e.config.Currency,
		Breakdown: []PriceBreakdown{
			{
				Name:     "Base Price",
				Amount:   e.config.BasePrice,
				Quantity: 1,
				Unit:     "元/秒",
			},
			{
				Name:     "GPU Tier",
				Amount:   tierMultiplier,
				Quantity: 1,
				Unit:     "x",
			},
			{
				Name:     "Memory",
				Amount:   memoryMultiplier,
				Quantity: float64(memoryGB),
				Unit:     "GB",
			},
			{
				Name:     "Utilization",
				Amount:   utilizationDiscount,
				Quantity: 1,
				Unit:     "x",
			},
			{
				Name:     "Duration",
				Amount:   durationSeconds,
				Quantity: 1,
				Unit:     "秒",
			},
		},
	}

	return result, nil
}

// CalculatePriceWithVolumeDiscount calculates price with volume discount
func (e *Engine) CalculatePriceWithVolumeDiscount(gpuName string, memoryMB int, duration time.Duration, utilization int, totalHours float64) (*PricingResult, error) {
	result, err := e.CalculatePrice(gpuName, memoryMB, duration, utilization)
	if err != nil {
		return nil, err
	}

	// Apply volume discount
	volumeDiscount := e.getVolumeDiscount(totalHours)
	result.Discount = 1.0 - (result.FinalPrice * volumeDiscount / result.TotalPrice)
	result.FinalPrice *= volumeDiscount

	// Add volume discount to breakdown
	result.Breakdown = append(result.Breakdown, PriceBreakdown{
		Name:     "Volume Discount",
		Amount:   volumeDiscount,
		Quantity: totalHours,
		Unit:     "小时",
	})

	return result, nil
}

// getTierMultiplier returns the tier multiplier for a GPU
func (e *Engine) getTierMultiplier(gpuName string) float64 {
	for key, multiplier := range e.config.TierMultiplier {
		if contains(gpuName, key) {
			return multiplier
		}
	}
	return e.config.TierMultiplier["default"]
}

// getMemoryMultiplier returns the memory multiplier
func (e *Engine) getMemoryMultiplier(memoryGB int) float64 {
	// Find the best matching tier
	tiers := []int{80, 40, 24, 16, 12, 8, 6}
	for _, tier := range tiers {
		if memoryGB >= tier {
			key := fmt.Sprintf("%dGB", tier)
			if multiplier, exists := e.config.MemoryMultiplier[key]; exists {
				return multiplier
			}
		}
	}
	return e.config.MemoryMultiplier["default"]
}

// getUtilizationDiscount returns the utilization discount
func (e *Engine) getUtilizationDiscount(utilization int) float64 {
	// Find the best matching tier
	tiers := []int{100, 90, 80, 70, 60, 50, 0}
	for _, tier := range tiers {
		if utilization >= tier {
			if discount, exists := e.config.UtilizationDiscount[tier]; exists {
				return discount
			}
		}
	}
	return 1.0
}

// getTimeDiscount returns the time-based discount
func (e *Engine) getTimeDiscount() float64 {
	now := time.Now()
	hour := now.Hour()

	// Night discount: 22:00 - 6:00
	if hour >= 22 || hour < 6 {
		if discount, exists := e.config.TimeDiscount["night"]; exists {
			return discount
		}
	}

	// Weekend discount
	if now.Weekday() == time.Saturday || now.Weekday() == time.Sunday {
		if discount, exists := e.config.TimeDiscount["weekend"]; exists {
			return discount
		}
	}

	// Off-peak discount: 12:00 - 14:00, 18:00 - 22:00
	if (hour >= 12 && hour < 14) || (hour >= 18 && hour < 22) {
		if discount, exists := e.config.TimeDiscount["off_peak"]; exists {
			return discount
		}
	}

	return 1.0
}

// getVolumeDiscount returns the volume discount
func (e *Engine) getVolumeDiscount(hours float64) float64 {
	// Find the best matching tier
	tiers := []int{5000, 1000, 500, 100, 0}
	for _, tier := range tiers {
		if hours >= float64(tier) {
			if discount, exists := e.config.VolumeDiscount[tier]; exists {
				return discount
			}
		}
	}
	return 1.0
}

// CreateUsageRecord creates a usage record
func (e *Engine) CreateUsageRecord(gpuID, userID, taskID, modelID, gpuName string, startTime time.Time, endTime time.Time, memoryMB int, unitPrice float64) *UsageRecord {
	duration := endTime.Sub(startTime).Seconds()
	totalPrice := unitPrice * duration

	return &UsageRecord{
		ID:           generateRecordID(),
		GPUID:        gpuID,
		UserID:       userID,
		StartTime:    startTime,
		EndTime:      endTime,
		Duration:     duration,
		GPUName:      gpuName,
		MemoryUsedMB: memoryMB,
		UnitPrice:    unitPrice,
		TotalPrice:   totalPrice,
		TaskID:       taskID,
		ModelID:      modelID,
	}
}

// CreateBill creates a bill from usage records
func (e *Engine) CreateBill(userID string, records []*UsageRecord, period string) *Bill {
	var totalAmount float64
	for _, record := range records {
		totalAmount += record.TotalPrice
	}

	return &Bill{
		ID:          generateBillID(),
		UserID:      userID,
		Period:      period,
		TotalAmount: totalAmount,
		Currency:    e.config.Currency,
		Records:     records,
		Discounts:   []Discount{},
		FinalAmount: totalAmount,
		CreatedAt:   time.Now(),
		Status:      "pending",
	}
}

// AddDiscount adds a discount to a bill
func (e *Engine) AddDiscount(bill *Bill, discountType, name, description string, amount, percentage float64) {
	bill.Discounts = append(bill.Discounts, Discount{
		Type:        discountType,
		Name:        name,
		Amount:      amount,
		Percentage:  percentage,
		Description: description,
	})

	// Recalculate final amount
	bill.FinalAmount = bill.TotalAmount
	for _, discount := range bill.Discounts {
		if discount.Percentage > 0 {
			bill.FinalAmount *= discount.Percentage
		} else {
			bill.FinalAmount -= discount.Amount
		}
	}
}

// GetPriceEstimate returns a price estimate without creating a record
func (e *Engine) GetPriceEstimate(gpuName string, memoryMB int, duration time.Duration) (*PricingResult, error) {
	return e.CalculatePrice(gpuName, memoryMB, duration, 50) // Assume 50% utilization for estimates
}

// GetPricePerHour returns the price per hour for a GPU
func (e *Engine) GetPricePerHour(gpuName string, memoryMB int) (float64, error) {
	result, err := e.CalculatePrice(gpuName, memoryMB, time.Hour, 50)
	if err != nil {
		return 0, err
	}
	return result.FinalPrice, nil
}

// UpdateBasePrice updates the base price
func (e *Engine) UpdateBasePrice(price float64) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.config.BasePrice = price
}

// UpdateTierMultiplier updates a tier multiplier
func (e *Engine) UpdateTierMultiplier(gpuName string, multiplier float64) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.config.TierMultiplier[gpuName] = multiplier
}

// GetConfig returns the current configuration
func (e *Engine) GetConfig() Config {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.config
}

// Helper functions
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsAny(s, substr))
}

func containsAny(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func generateRecordID() string {
	return fmt.Sprintf("rec-%d", time.Now().UnixNano())
}

func generateBillID() string {
	return fmt.Sprintf("bill-%d", time.Now().UnixNano())
}
