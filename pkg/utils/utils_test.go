package utils

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestGetNodeID(t *testing.T) {
	id := GetNodeID()
	if id == "" {
		t.Error("expected non-empty node ID")
	}
	// UUID format should be 36 characters
	if len(id) != 36 {
		t.Errorf("expected UUID format, got length %d", len(id))
	}
}

func TestGenerateRandomHex(t *testing.T) {
	tests := []struct {
		name     string
		length   int
		wantErr  bool
		checkLen int
	}{
		{"valid 16 bytes", 16, false, 32},
		{"valid 32 bytes", 32, false, 64},
		{"valid 4 bytes", 4, false, 8},
		{"valid 0 bytes", 0, false, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := GenerateRandomHex(tt.length)
			if (err != nil) != tt.wantErr {
				t.Errorf("GenerateRandomHex() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && len(result) != tt.checkLen {
				t.Errorf("GenerateRandomHex() result length = %d, want %d", len(result), tt.checkLen)
			}
		})
	}
}

func TestGetLocalIP(t *testing.T) {
	ip, err := GetLocalIP()
	if err != nil {
		t.Errorf("GetLocalIP() error = %v", err)
		return
	}
	if ip == "" {
		t.Error("expected non-empty IP address")
	}
	// Validate IP format - should contain at least one dot
	hasDot := false
	for i := 0; i < len(ip); i++ {
		if ip[i] == '.' {
			hasDot = true
			break
		}
	}
	if !hasDot {
		t.Errorf("invalid IP format: %s", ip)
	}
}

func TestGetDataDir(t *testing.T) {
	// Test default path
	dir := GetDataDir("")
	if dir == "" {
		t.Error("expected non-empty data directory")
	}

	// Test custom path
	customPath := "/custom/path"
	dir = GetDataDir(customPath)
	if dir != customPath {
		t.Errorf("expected %s, got %s", customPath, dir)
	}
}

func TestEnsureDir(t *testing.T) {
	// Create a temporary directory
	tmpDir := filepath.Join(os.TempDir(), "kerrigan-test-"+time.Now().Format("20060102150405"))
	defer os.RemoveAll(tmpDir)

	err := EnsureDir(tmpDir)
	if err != nil {
		t.Errorf("EnsureDir() error = %v", err)
		return
	}

	// Check if directory exists
	info, err := os.Stat(tmpDir)
	if err != nil {
		t.Errorf("directory does not exist: %v", err)
		return
	}
	if !info.IsDir() {
		t.Error("expected directory, got file")
	}
}

func TestEnsureDir_Nested(t *testing.T) {
	// Test creating nested directories
	tmpDir := filepath.Join(os.TempDir(), "kerrigan-test-nested", "subdir", "deeper")
	defer os.RemoveAll(filepath.Dir(filepath.Dir(tmpDir)))

	err := EnsureDir(tmpDir)
	if err != nil {
		t.Errorf("EnsureDir() nested error = %v", err)
		return
	}

	info, err := os.Stat(tmpDir)
	if err != nil {
		t.Errorf("nested directory does not exist: %v", err)
		return
	}
	if !info.IsDir() {
		t.Error("expected directory, got file")
	}
}

func TestFileExists(t *testing.T) {
	// Create a temporary file
	tmpFile := filepath.Join(os.TempDir(), "kerrigan-test-"+time.Now().Format("20060102150405")+".txt")
	defer os.Remove(tmpFile)

	// Write a test file
	err := os.WriteFile(tmpFile, []byte("test"), 0644)
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Test existing file
	if !FileExists(tmpFile) {
		t.Error("expected FileExists() to return true for existing file")
	}

	// Test non-existing file
	if FileExists("/nonexistent/path/to/file.txt") {
		t.Error("expected FileExists() to return false for non-existing file")
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		name     string
		bytes    int64
		expected string
	}{
		{"0 bytes", 0, "0 B"},
		{"512 bytes", 512, "512 B"},
		{"1024 bytes (1KB)", 1024, "1.0 KB"},
		{"1536 bytes (1.5KB)", 1536, "1.5 KB"},
		{"1024*1024 (1MB)", 1048576, "1.0 MB"},
		{"1024*1024*1024 (1GB)", 1073741824, "1.0 GB"},
		{"1024*1024*1024*1024 (1TB)", 1099511627776, "1.0 TB"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatBytes(tt.bytes)
			if result != tt.expected {
				t.Errorf("FormatBytes(%d) = %s, want %s", tt.bytes, result, tt.expected)
			}
		})
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		expected string
	}{
		{"seconds", 30 * time.Second, "30s"},
		{"minutes", 5 * time.Minute, "5m"},
		{"hours", 2 * time.Hour, "2.0h"},
		{"days", 24 * time.Hour, "1.0d"},
		{"multiple days", 48 * time.Hour, "2.0d"},
		{"sub-minute", 45 * time.Second, "45s"},
		{"sub-hour", 30 * time.Minute, "30m"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatDuration(tt.duration)
			if result != tt.expected {
				t.Errorf("FormatDuration(%v) = %s, want %s", tt.duration, result, tt.expected)
			}
		})
	}
}
