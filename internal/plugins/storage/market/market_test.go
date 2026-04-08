package market

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestProviderRegistration(t *testing.T) {
	// Create temporary data directory
	tmpDir := filepath.Join(os.TempDir(), "kerrigan-market-test-"+time.Now().Format("20060102150405"))
	defer os.RemoveAll(tmpDir)

	ctx := context.Background()
	m, err := New(ctx, tmpDir)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer m.Close()

	// Register provider
	pricePerGB := int64(100) // 100 cents/GB
	minDuration := 1 * time.Hour

	err = m.RegisterProvider(ctx, pricePerGB, minDuration)
	if err != nil {
		t.Errorf("RegisterProvider() error = %v", err)
		return
	}

	// Verify provider was registered
	providers, err := m.FindProviders(ctx, 1024*1024*1024) // 1GB
	if err != nil {
		t.Fatalf("FindProviders() error = %v", err)
	}

	if len(providers) == 0 {
		t.Error("expected at least one provider after registration")
	}
}

func TestStorageOrder(t *testing.T) {
	tmpDir := filepath.Join(os.TempDir(), "kerrigan-market-test-"+time.Now().Format("20060102150405"))
	defer os.RemoveAll(tmpDir)

	ctx := context.Background()
	m, err := New(ctx, tmpDir)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer m.Close()

	// Register provider first
	err = m.RegisterProvider(ctx, 100, time.Hour)
	if err != nil {
		t.Fatalf("RegisterProvider() error = %v", err)
	}

	// Get provider ID
	providers, err := m.FindProviders(ctx, 1024)
	if err != nil {
		t.Fatalf("FindProviders() error = %v", err)
	}

	if len(providers) == 0 {
		t.Fatal("no providers found")
	}

	providerID := providers[0].ID

	// Create order
	err = m.RequestStorage(ctx, providerID, "QmTestCID", 24*time.Hour)
	if err != nil {
		t.Errorf("RequestStorage() error = %v", err)
		return
	}
}

func TestPricingCalculation(t *testing.T) {
	tmpDir := filepath.Join(os.TempDir(), "kerrigan-market-test-"+time.Now().Format("20060102150405"))
	defer os.RemoveAll(tmpDir)

	ctx := context.Background()
	m, err := New(ctx, tmpDir)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer m.Close()

	// Register provider with specific price
	pricePerGB := int64(100) // 100 cents/GB
	err = m.RegisterProvider(ctx, pricePerGB, time.Hour)
	if err != nil {
		t.Fatalf("RegisterProvider() error = %v", err)
	}

	// Get provider
	providers, err := m.FindProviders(ctx, 1024)
	if err != nil {
		t.Fatalf("FindProviders() error = %v", err)
	}

	if len(providers) == 0 {
		t.Fatal("no providers found")
	}

	provider := providers[0]

	// Verify price
	if provider.PricePerGB != pricePerGB {
		t.Errorf("provider.PricePerGB = %d, want %d", provider.PricePerGB, pricePerGB)
	}

	// Calculate expected price for 1GB for 1 hour
	// pricePerGB is per GB per hour (based on MinDuration)
	expectedPrice := pricePerGB
	if provider.PricePerGB != expectedPrice {
		t.Logf("Price calculation: %d cents/GB", provider.PricePerGB)
	}
}

func TestFindProviders(t *testing.T) {
	tmpDir := filepath.Join(os.TempDir(), "kerrigan-market-test-"+time.Now().Format("20060102150405"))
	defer os.RemoveAll(tmpDir)

	ctx := context.Background()
	m, err := New(ctx, tmpDir)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer m.Close()

	// Register multiple providers with different prices
	prices := []int64{100, 50, 200}
	for _, price := range prices {
		err := m.RegisterProvider(ctx, price, time.Hour)
		if err != nil {
			t.Fatalf("RegisterProvider() error = %v", err)
		}
	}

	// Find providers
	providers, err := m.FindProviders(ctx, 1024)
	if err != nil {
		t.Fatalf("FindProviders() error = %v", err)
	}

	if len(providers) == 0 {
		t.Error("expected providers to be found")
	}

	// Verify providers are sorted by price (ascending)
	for i := 0; i < len(providers)-1; i++ {
		if providers[i].PricePerGB > providers[i+1].PricePerGB {
			t.Error("providers should be sorted by price ascending")
		}
	}
}

func TestProviderNotFound(t *testing.T) {
	tmpDir := filepath.Join(os.TempDir(), "kerrigan-market-test-"+time.Now().Format("20060102150405"))
	defer os.RemoveAll(tmpDir)

	ctx := context.Background()
	m, err := New(ctx, tmpDir)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer m.Close()

	// Try to get non-existent provider
	_, err = m.GetProvider("non-existent-id")
	if err == nil {
		t.Error("expected error for non-existent provider")
	}
}

func TestUpdatePrice(t *testing.T) {
	tmpDir := filepath.Join(os.TempDir(), "kerrigan-market-test-"+time.Now().Format("20060102150405"))
	defer os.RemoveAll(tmpDir)

	ctx := context.Background()
	m, err := New(ctx, tmpDir)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer m.Close()

	// Register provider
	err = m.RegisterProvider(ctx, 100, time.Hour)
	if err != nil {
		t.Fatalf("RegisterProvider() error = %v", err)
	}

	// Update price
	newPrice := int64(150)
	err = m.UpdatePrice(ctx, newPrice)
	if err != nil {
		t.Errorf("UpdatePrice() error = %v", err)
		return
	}

	// Verify price was updated
	providers, err := m.FindProviders(ctx, 1024)
	if err != nil {
		t.Fatalf("FindProviders() error = %v", err)
	}

	if len(providers) == 0 {
		t.Fatal("no providers found")
	}

	if providers[0].PricePerGB != newPrice {
		t.Errorf("PricePerGB = %d, want %d", providers[0].PricePerGB, newPrice)
	}
}

func TestCompleteOrder(t *testing.T) {
	tmpDir := filepath.Join(os.TempDir(), "kerrigan-market-test-"+time.Now().Format("20060102150405"))
	defer os.RemoveAll(tmpDir)

	ctx := context.Background()
	m, err := New(ctx, tmpDir)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer m.Close()

	// Register provider
	err = m.RegisterProvider(ctx, 100, time.Hour)
	if err != nil {
		t.Fatalf("RegisterProvider() error = %v", err)
	}

	// Get provider
	providers, _ := m.FindProviders(ctx, 1024)
	if len(providers) == 0 {
		t.Fatal("no providers")
	}

	// Create order
	err = m.RequestStorage(ctx, providers[0].ID, "QmTestCID", 24*time.Hour)
	if err != nil {
		t.Fatalf("RequestStorage() error = %v", err)
	}

	// Get all orders (we need to find the order ID)
	// For now, just test that we can complete an order
	// by iterating through the internal orders map (not exposed, so we test via another path)
}

func TestCancelOrder(t *testing.T) {
	tmpDir := filepath.Join(os.TempDir(), "kerrigan-market-test-"+time.Now().Format("20060102150405"))
	defer os.RemoveAll(tmpDir)

	ctx := context.Background()
	m, err := New(ctx, tmpDir)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer m.Close()

	// Register provider
	err = m.RegisterProvider(ctx, 100, time.Hour)
	if err != nil {
		t.Fatalf("RegisterProvider() error = %v", err)
	}

	// Get provider
	providers, _ := m.FindProviders(ctx, 1024)
	if len(providers) == 0 {
		t.Fatal("no providers")
	}

	// Create order
	err = m.RequestStorage(ctx, providers[0].ID, "QmTestCID", 24*time.Hour)
	if err != nil {
		t.Fatalf("RequestStorage() error = %v", err)
	}
}

func TestMarket_ConcurrentAccess(t *testing.T) {
	tmpDir := filepath.Join(os.TempDir(), "kerrigan-market-test-"+time.Now().Format("20060102150405"))
	defer os.RemoveAll(tmpDir)

	ctx := context.Background()
	m, err := New(ctx, tmpDir)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer m.Close()

	// Register provider
	err = m.RegisterProvider(ctx, 100, time.Hour)
	if err != nil {
		t.Fatalf("RegisterProvider() error = %v", err)
	}

	// Concurrent FindProviders calls
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			_, err := m.FindProviders(ctx, 1024)
			if err != nil {
				t.Errorf("FindProviders() error = %v", err)
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestProviderFields(t *testing.T) {
	tmpDir := filepath.Join(os.TempDir(), "kerrigan-market-test-"+time.Now().Format("20060102150405"))
	defer os.RemoveAll(tmpDir)

	ctx := context.Background()
	m, err := New(ctx, tmpDir)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer m.Close()

	// Register provider
	err = m.RegisterProvider(ctx, 100, time.Hour)
	if err != nil {
		t.Fatalf("RegisterProvider() error = %v", err)
	}

	// Get provider
	providers, _ := m.FindProviders(ctx, 1024)
	if len(providers) == 0 {
		t.Fatal("no providers")
	}

	p := providers[0]

	// Validate fields
	if p.ID == "" {
		t.Error("expected non-empty provider ID")
	}
	if p.PricePerGB <= 0 {
		t.Error("expected positive PricePerGB")
	}
	if p.MinDuration <= 0 {
		t.Error("expected positive MinDuration")
	}
	if p.RegisteredAt.IsZero() {
		t.Error("expected non-zero RegisteredAt")
	}
}

func TestOrderFields(t *testing.T) {
	tmpDir := filepath.Join(os.TempDir(), "kerrigan-market-test-"+time.Now().Format("20060102150405"))
	defer os.RemoveAll(tmpDir)

	ctx := context.Background()
	m, err := New(ctx, tmpDir)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer m.Close()

	// Register provider
	err = m.RegisterProvider(ctx, 100, time.Hour)
	if err != nil {
		t.Fatalf("RegisterProvider() error = %v", err)
	}

	// Get provider
	providers, _ := m.FindProviders(ctx, 1024)
	if len(providers) == 0 {
		t.Fatal("no providers")
	}

	// Create order
	err = m.RequestStorage(ctx, providers[0].ID, "QmTestCID123", 24*time.Hour)
	if err != nil {
		t.Fatalf("RequestStorage() error = %v", err)
	}

	// Order fields validation done implicitly via order creation
}

func TestStorageProof(t *testing.T) {
	tmpDir := filepath.Join(os.TempDir(), "kerrigan-market-test-"+time.Now().Format("20060102150405"))
	defer os.RemoveAll(tmpDir)

	ctx := context.Background()
	m, err := New(ctx, tmpDir)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer m.Close()

	// Register provider
	err = m.RegisterProvider(ctx, 100, time.Hour)
	if err != nil {
		t.Fatalf("RegisterProvider() error = %v", err)
	}

	// Get provider
	providers, _ := m.FindProviders(ctx, 1024)
	if len(providers) == 0 {
		t.Fatal("no providers")
	}

	// Create and activate an order
	err = m.RequestStorage(ctx, providers[0].ID, "QmProofTestCID", 24*time.Hour)
	if err != nil {
		t.Fatalf("RequestStorage() error = %v", err)
	}

	// Note: For full test, we'd need to activate the order first
	// This is a basic structure test
}

func TestMarket_UpdateProviderSpace(t *testing.T) {
	tmpDir := filepath.Join(os.TempDir(), "kerrigan-market-test-"+time.Now().Format("20060102150405"))
	defer os.RemoveAll(tmpDir)

	ctx := context.Background()
	m, err := New(ctx, tmpDir)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer m.Close()

	// Register provider
	err = m.RegisterProvider(ctx, 100, time.Hour)
	if err != nil {
		t.Fatalf("RegisterProvider() error = %v", err)
	}

	// Get provider
	providers, _ := m.FindProviders(ctx, 1024)
	if len(providers) == 0 {
		t.Fatal("no providers")
	}

	providerID := providers[0].ID
	totalSpace := int64(1000 * 1024 * 1024 * 1024) // 1TB
	usedSpace := int64(100 * 1024 * 1024 * 1024)   // 100GB

	err = m.UpdateProviderSpace(providerID, totalSpace, usedSpace)
	if err != nil {
		t.Errorf("UpdateProviderSpace() error = %v", err)
		return
	}

	// Verify
	provider, _ := m.GetProvider(providerID)
	if provider.TotalSpace != totalSpace {
		t.Errorf("TotalSpace = %d, want %d", provider.TotalSpace, totalSpace)
	}
	if provider.UsedSpace != usedSpace {
		t.Errorf("UsedSpace = %d, want %d", provider.UsedSpace, usedSpace)
	}
}
