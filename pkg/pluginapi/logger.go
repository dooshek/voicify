package pluginapi

import (
	"errors"
	"fmt"

	"github.com/dooshek/voicify/internal/logger"
)

// Logger is a wrapper around the logger package
type Logger struct{}

// NewLogger creates a new logger instance
func NewLogger() *Logger {
	return &Logger{}
}

// Debug logs a debug message
func (l *Logger) Debug(message string) {
	logger.Debug(message)
}

// Debugf logs a formatted debug message
func (l *Logger) Debugf(format string, args ...interface{}) {
	logger.Debugf(format, args...)
}

// Info logs an info message
func (l *Logger) Info(message string) {
	logger.Info(message)
}

// Infof logs a formatted info message
func (l *Logger) Infof(format string, args ...interface{}) {
	logger.Infof(format, args...)
}

// Warn logs a warning message
func (l *Logger) Warn(message string) {
	logger.Warn(message)
}

// Warnf logs a formatted warning message
func (l *Logger) Warnf(format string, args ...interface{}) {
	logger.Warnf(format, args...)
}

// Error logs an error message with an optional error
func (l *Logger) Error(message string, err error) {
	logger.Error(message, err)
}

// Errorf logs a formatted error message
func (l *Logger) Errorf(format string, args ...interface{}) {
	// Create a placeholder error since logger.Errorf requires an error
	err := errors.New(fmt.Sprintf(format, args...))
	logger.Error(fmt.Sprintf(format, args...), err)
}
