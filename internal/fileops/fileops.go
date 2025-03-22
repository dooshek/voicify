package fileops

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"syscall"

	"github.com/dooshek/voicify/internal/logger"
)

// ErrConfigNotFound is returned when a configuration file does not exist
var ErrConfigNotFound = errors.New("configuration file not found")

// ErrProcessAlreadyRunning is returned when the voicify process is already running
var ErrProcessAlreadyRunning = errors.New("voicify process is already running")

// FileOps interface defines operations for managing files in the voicify config directory
type FileOps interface {
	// GetConfigDir returns the full path to the voicify config directory
	GetConfigDir() string

	// GetRecordingsDir returns the full path to the recordings directory
	GetRecordingsDir() string

	// SaveConfig saves data to a file in the config directory
	SaveConfig(filename string, data []byte) error

	// LoadConfig loads data from a file in the config directory
	LoadConfig(filename string) ([]byte, error)

	// SaveRecording saves recording data to the recordings directory
	SaveRecording(filename string, data []byte) error

	// ListRecordings returns a list of recordings in the recordings directory
	ListRecordings() ([]string, error)

	// DeleteRecording deletes a recording from the recordings directory
	DeleteRecording(filename string) error

	// EnsureDirectories creates necessary directories if they don't exist
	EnsureDirectories() error

	// SavePID saves the current process ID to a file
	SavePID() error

	// CheckPID checks if another instance is running
	// Returns ErrProcessAlreadyRunning if another instance is running
	CheckPID() error

	// CleanupPID removes the PID file
	CleanupPID() error

	// HandleExit ensures proper cleanup of PID file on application exit
	HandleExit()

	// GetResourcesDir returns the full path to the resources directory
	GetResourcesDir() string

	// GetAudioDir returns the full path to the audio resources directory
	GetAudioDir() string

	// GetPromptsDir returns the full path to the prompts resources directory
	GetPromptsDir() string

	// GetBaseDir returns the parent directory for voicify
	GetBaseDir() string

	// GetPluginsDir returns the full path to the plugins directory
	GetPluginsDir() string
}

// DefaultFileOps implements FileOps interface
type DefaultFileOps struct {
	configDir string
}

// NewDefaultFileOps creates a new DefaultFileOps instance
func NewDefaultFileOps() (*DefaultFileOps, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	return &DefaultFileOps{
		configDir: filepath.Join(homeDir, ".config", "voicify"),
	}, nil
}

func (f *DefaultFileOps) GetConfigDir() string {
	return f.configDir
}

func (f *DefaultFileOps) GetRecordingsDir() string {
	return filepath.Join(f.configDir, "recordings")
}

func (f *DefaultFileOps) SaveConfig(filename string, data []byte) error {
	path := filepath.Join(f.configDir, filename)
	return os.WriteFile(path, data, 0o644)
}

func (f *DefaultFileOps) LoadConfig(filename string) ([]byte, error) {
	path := filepath.Join(f.configDir, filename)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, ErrConfigNotFound
	}
	return os.ReadFile(path)
}

func (f *DefaultFileOps) SaveRecording(filename string, data []byte) error {
	path := filepath.Join(f.GetRecordingsDir(), filename)
	return os.WriteFile(path, data, 0o644)
}

func (f *DefaultFileOps) ListRecordings() ([]string, error) {
	files, err := os.ReadDir(f.GetRecordingsDir())
	if err != nil {
		return nil, err
	}

	var recordings []string
	for _, file := range files {
		if !file.IsDir() {
			recordings = append(recordings, file.Name())
		}
	}
	return recordings, nil
}

func (f *DefaultFileOps) DeleteRecording(filename string) error {
	path := filepath.Join(f.GetRecordingsDir(), filename)
	return os.Remove(path)
}

func (f *DefaultFileOps) EnsureDirectories() error {
	// Create config directory
	if err := os.MkdirAll(f.configDir, 0o755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Create recordings directory
	if err := os.MkdirAll(f.GetRecordingsDir(), 0o755); err != nil {
		return fmt.Errorf("failed to create recordings directory: %w", err)
	}

	// Create resources directories
	dirs := []string{
		f.GetResourcesDir(),
		f.GetAudioDir(),
		f.GetPromptsDir(),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	return nil
}

func (f *DefaultFileOps) getPIDFilePath() string {
	return filepath.Join(f.configDir, "voicify.pid")
}

func (f *DefaultFileOps) SavePID() error {
	pidFile := f.getPIDFilePath()
	pid := os.Getpid()
	return os.WriteFile(pidFile, []byte(strconv.Itoa(pid)), 0o644)
}

func (f *DefaultFileOps) CheckPID() error {
	pidFile := f.getPIDFilePath()

	data, err := os.ReadFile(pidFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // PID file doesn't exist, application is not running
		}
		return fmt.Errorf("error reading PID file: %w", err)
	}

	pid, err := strconv.Atoi(string(data))
	if err != nil {
		return fmt.Errorf("invalid PID in file: %w", err)
	}

	// Check if process exists by sending signal 0
	process, err := os.FindProcess(pid)
	if err != nil {
		return nil // Process doesn't exist
	}

	err = process.Signal(syscall.Signal(0))
	if err == nil {
		return ErrProcessAlreadyRunning
	}

	// If we get here, the process doesn't exist but the PID file does
	logger.Debug("Found stale PID file, will be overwritten")
	return nil
}

func (f *DefaultFileOps) CleanupPID() error {
	return os.Remove(f.getPIDFilePath())
}

func (f *DefaultFileOps) HandleExit() {
	if err := f.CleanupPID(); err != nil {
		logger.Error("Failed to cleanup PID file on exit", err)
	}
}

func (f *DefaultFileOps) GetResourcesDir() string {
	return filepath.Join(f.configDir, "resources")
}

func (f *DefaultFileOps) GetAudioDir() string {
	return filepath.Join(f.GetResourcesDir(), "audio")
}

func (f *DefaultFileOps) GetPromptsDir() string {
	return filepath.Join(f.GetResourcesDir(), "prompts")
}

func (f *DefaultFileOps) GetBaseDir() string {
	return filepath.Dir(f.configDir)
}

func (f *DefaultFileOps) GetPluginsDir() string {
	return filepath.Join(f.GetBaseDir(), "plugins")
}
