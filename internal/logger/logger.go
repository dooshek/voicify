package logger

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/rs/zerolog"
)

// Level represents logging levels
type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
)

var (
	currentLevel Level     = LevelInfo
	output       io.Writer = os.Stdout
	logFile      *os.File
	logger       zerolog.Logger
)

func (l Level) String() string {
	switch l {
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

// SetOutputFile sets the logger output to a file
func SetOutputFile(filename string) error {
	// Create directory if it doesn't exist
	dir := filepath.Dir(filename)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}

	// Open file in append mode
	f, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}

	logFile = f
	output = f

	// Reinitialize logger with new output
	initLogger()
	return nil
}

// CloseLogFile closes the log file if it's open
func CloseLogFile() {
	if logFile != nil {
		logFile.Close()
		logFile = nil
		output = os.Stdout
		initLogger()
	}
}

func initLogger() {
	consoleWriter := zerolog.ConsoleWriter{
		Out:        output,
		TimeFormat: "15:04:05",
		NoColor:    logFile != nil, // Disable colors when writing to file
	}

	logger = zerolog.New(consoleWriter).With().Timestamp().Logger()
}

func init() {
	initLogger()
}

// SetLevel sets the global log level
func SetLevel(level string) {
	switch level {
	case "debug":
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	case "info":
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	case "warn":
		zerolog.SetGlobalLevel(zerolog.WarnLevel)
	case "error":
		zerolog.SetGlobalLevel(zerolog.ErrorLevel)
	default:
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}
}

// Debug logs a debug message
func Debug(msg string) {
	logger.Debug().Msg(msg)
}

// Debugf logs a debug message with formatting
func Debugf(format string, v ...interface{}) {
	logger.Debug().Msgf(format, v...)
}

// Info logs an info message
func Info(msg string) {
	logger.Info().Msg(msg)
}

// Infof logs an info message with formatting
func Infof(format string, v ...interface{}) {
	logger.Info().Msgf(format, v...)
}

// Warn logs a warning message
func Warn(msg string) {
	logger.Warn().Msg(msg)
}

// Warnf logs a warning message with formatting
func Warnf(format string, v ...interface{}) {
	logger.Warn().Msgf(format, v...)
}

// Error logs an error message with the error object
func Error(msg string, err error) {
	logger.Error().Err(err).Msg(msg)
}

// Errorf logs an error message with formatting and the error object
func Errorf(format string, err error, v ...interface{}) {
	logger.Error().Err(err).Msgf(format, v...)
}

// GetCurrentLevel returns the current logging level
func GetCurrentLevel() Level {
	return currentLevel
}
