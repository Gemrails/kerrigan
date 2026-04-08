package log

import (
	"testing"

	"github.com/sirupsen/logrus"
)

func TestLoggerInit(t *testing.T) {
	// Test initialization with different levels
	tests := []struct {
		name      string
		level     string
		format    string
		wantPanic bool
	}{
		{"debug level", "debug", "text", false},
		{"info level", "info", "text", false},
		{"warn level", "warn", "text", false},
		{"error level", "error", "text", false},
		{"json format", "info", "json", false},
		{"text format", "info", "text", false},
		{"empty level", "", "text", false},
		{"invalid level", "invalid", "text", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Init should not panic
			Init(tt.level, tt.format)
			logger := GetLogger()
			if logger == nil {
				t.Error("GetLogger() returned nil")
			}
		})
	}
}

func TestLoggerLevels(t *testing.T) {
	Init("debug", "text")
	logger := GetLogger()

	if logger == nil {
		t.Fatal("GetLogger() returned nil")
	}

	if logger.Level != logrus.DebugLevel {
		t.Errorf("initial logger level = %d, want %d", logger.Level, logrus.DebugLevel)
	}
}

func TestGetLogger(t *testing.T) {
	// GetLogger without Init should return a valid logger
	logger := GetLogger()
	if logger == nil {
		t.Error("GetLogger() returned nil without Init")
	}
}

func TestWithField(t *testing.T) {
	Init("info", "text")
	entry := WithField("key", "value")
	if entry == nil {
		t.Error("WithField() returned nil")
	}
}

func TestWithFields(t *testing.T) {
	Init("info", "text")
	fields := map[string]interface{}{
		"key1": "value1",
		"key2": 123,
	}
	entry := WithFields(fields)
	if entry == nil {
		t.Error("WithFields() returned nil")
	}
}

func TestLogMethods(t *testing.T) {
	Init("debug", "text")

	// These should not panic
	Debug("debug message")
	Info("info message")
	Warn("warn message")
	Error("error message")
}

func TestLoggerMultipleInits(t *testing.T) {
	// Multiple Init calls should not panic (uses sync.Once)
	Init("debug", "text")
	Init("info", "json")
	Init("error", "text")

	logger := GetLogger()
	if logger == nil {
		t.Error("GetLogger() returned nil after multiple Init")
	}
}
