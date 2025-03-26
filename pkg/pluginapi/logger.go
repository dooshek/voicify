package pluginapi

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

// LogLevel represents logging levels
type LogLevel int

const (
	// Debug level for detailed information
	LevelDebug LogLevel = iota
	// Info level for general information
	LevelInfo
	// Warn level for warnings
	LevelWarn
	// Error level for errors
	LevelError
)

var (
	currentLevel LogLevel  = LevelInfo
	output       io.Writer = os.Stdout
	mu           sync.Mutex
)

// Logger provides logging functionality for plugins
type Logger struct{}

// NewLogger creates a new logger instance
func NewLogger() *Logger {
	return &Logger{}
}

// Debug logs a debug message
func (l *Logger) Debug(message string) {
	if currentLevel <= LevelDebug {
		writeLog("DEBUG", message)
	}
}

// Debugf logs a formatted debug message
func (l *Logger) Debugf(format string, args ...interface{}) {
	if currentLevel <= LevelDebug {
		writeLog("DEBUG", fmt.Sprintf(format, args...))
	}
}

// Info logs an info message
func (l *Logger) Info(message string) {
	if currentLevel <= LevelInfo {
		writeLog("INFO", message)
	}
}

// Infof logs a formatted info message
func (l *Logger) Infof(format string, args ...interface{}) {
	if currentLevel <= LevelInfo {
		writeLog("INFO", fmt.Sprintf(format, args...))
	}
}

// Warn logs a warning message
func (l *Logger) Warn(message string) {
	if currentLevel <= LevelWarn {
		writeLog("WARN", message)
	}
}

// Warnf logs a formatted warning message
func (l *Logger) Warnf(format string, args ...interface{}) {
	if currentLevel <= LevelWarn {
		writeLog("WARN", fmt.Sprintf(format, args...))
	}
}

// Error logs an error message with an optional error
func (l *Logger) Error(message string, err error) {
	if currentLevel <= LevelError {
		errMsg := message
		if err != nil {
			errMsg = fmt.Sprintf("%s: %v", message, err)
		}
		writeLog("ERROR", errMsg)
	}
}

// Errorf logs a formatted error message
func (l *Logger) Errorf(format string, args ...interface{}) {
	if currentLevel <= LevelError {
		writeLog("ERROR", fmt.Sprintf(format, args...))
	}
}

// writeLog writes a log message with the specified level
func writeLog(level, message string) {
	mu.Lock()
	defer mu.Unlock()
	timestamp := time.Now().Format("15:04:05")
	fmt.Fprintf(output, "%s [%s] %s\n", timestamp, level, message)
}

// SetLogLevel sets the global log level for this logger
func SetLogLevel(level LogLevel) {
	mu.Lock()
	defer mu.Unlock()
	currentLevel = level
}

// SetOutput sets the output writer for this logger
func SetOutput(w io.Writer) {
	mu.Lock()
	defer mu.Unlock()
	output = w
}
