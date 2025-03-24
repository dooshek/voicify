package pluginapi

import (
	"github.com/dooshek/voicify/internal/logger"
)

// Debug logs a debug message
func Debug(message string) {
	logger.Debug(message)
}

// Debugf logs a formatted debug message
func Debugf(format string, args ...interface{}) {
	logger.Debugf(format, args...)
}

// Info logs an info message
func Info(message string) {
	logger.Info(message)
}

// Infof logs a formatted info message
func Infof(format string, args ...interface{}) {
	logger.Infof(format, args...)
}

// Warn logs a warning message
func Warn(message string) {
	logger.Warn(message)
}

// Warnf logs a formatted warning message
func Warnf(format string, args ...interface{}) {
	logger.Warnf(format, args...)
}

// Error logs an error message with an optional error
func Error(message string, err error) {
	logger.Error(message, err)
}

// Errorf logs a formatted error message
func Errorf(format string, args ...interface{}) {
	logger.Errorf(format, args...)
}
