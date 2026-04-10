package log

import (
	"os"
	"sync"

	"github.com/sirupsen/logrus"
)

var (
	logger *logrus.Logger
	once   sync.Once
)

// Init initializes the global logger
func Init(level string, format string) {
	once.Do(func() {
		logger = logrus.New()

		// Set log level
		switch level {
		case "debug":
			logger.SetLevel(logrus.DebugLevel)
		case "info":
			logger.SetLevel(logrus.InfoLevel)
		case "warn":
			logger.SetLevel(logrus.WarnLevel)
		case "error":
			logger.SetLevel(logrus.ErrorLevel)
		default:
			logger.SetLevel(logrus.InfoLevel)
		}

		// Set output
		logger.SetOutput(os.Stdout)

		// Set format
		if format == "json" {
			logger.SetFormatter(&logrus.JSONFormatter{
				TimestampFormat: "2006-01-02 15:04:05",
			})
		} else {
			logger.SetFormatter(&logrus.TextFormatter{
				TimestampFormat: "2006-01-02 15:04:05",
				FullTimestamp:   true,
				DisableColors:   false,
			})
		}
	})
}

// GetLogger returns the global logger
func GetLogger() *logrus.Logger {
	if logger == nil {
		Init("info", "")
	}
	return logger
}

// WithField returns a logger entry with a field
func WithField(key string, value interface{}) *logrus.Entry {
	return GetLogger().WithField(key, value)
}

// WithFields returns a logger entry with multiple fields
func WithFields(fields logrus.Fields) *logrus.Entry {
	return GetLogger().WithFields(fields)
}

// Debug logs a debug message
func Debug(args ...interface{}) {
	GetLogger().Debug(args...)
}

// Debugf logs a formatted debug message
func Debugf(format string, args ...interface{}) {
	GetLogger().Debugf(format, args...)
}

// Info logs an info message
func Info(args ...interface{}) {
	GetLogger().Info(args...)
}

// Infof logs a formatted info message
func Infof(format string, args ...interface{}) {
	GetLogger().Infof(format, args...)
}

// Warn logs a warning message
func Warn(args ...interface{}) {
	GetLogger().Warn(args...)
}

// Warnf logs a formatted warning message
func Warnf(format string, args ...interface{}) {
	GetLogger().Warnf(format, args...)
}

// Error logs an error message
func Error(args ...interface{}) {
	GetLogger().Error(args...)
}

// Errorf logs a formatted error message
func Errorf(format string, args ...interface{}) {
	GetLogger().Errorf(format, args...)
}

// Fatal logs a fatal message and exits
func Fatal(args ...interface{}) {
	GetLogger().Fatal(args...)
}

// Fatalf logs a formatted fatal message and exits
func Fatalf(format string, args ...interface{}) {
	GetLogger().Fatalf(format, args...)
}
