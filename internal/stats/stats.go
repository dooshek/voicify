package stats

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/dooshek/voicify/internal/logger"
)

// ModelStats holds statistics for a specific model
type ModelStats struct {
	TotalSeconds   float64 `json:"total_seconds"`
	RecordingCount int     `json:"recording_count"`
}

// Stats holds all recording statistics
type Stats struct {
	Models map[string]*ModelStats `json:"models"`
}

// StatsManager manages recording statistics persistence
type StatsManager struct {
	stats    Stats
	filePath string
	mu       sync.Mutex
}

// NewStatsManager creates a new stats manager and loads existing data
func NewStatsManager() (*StatsManager, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	filePath := filepath.Join(homeDir, ".config", "voicify", "stats.json")

	sm := &StatsManager{
		filePath: filePath,
		stats: Stats{
			Models: make(map[string]*ModelStats),
		},
	}

	// Load existing stats if available
	if err := sm.load(); err != nil {
		logger.Debugf("Could not load stats (will start fresh): %v", err)
	}

	return sm, nil
}

// AddRecording adds a new recording to statistics and persists immediately
func (sm *StatsManager) AddRecording(model string, durationSeconds float64) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.stats.Models == nil {
		sm.stats.Models = make(map[string]*ModelStats)
	}

	if _, exists := sm.stats.Models[model]; !exists {
		sm.stats.Models[model] = &ModelStats{
			TotalSeconds:   0,
			RecordingCount: 0,
		}
	}

	sm.stats.Models[model].TotalSeconds += durationSeconds
	sm.stats.Models[model].RecordingCount++

	// Save immediately
	if err := sm.save(); err != nil {
		logger.Error("Failed to save stats after adding recording", err)
	}
}

// GetStats returns a deep copy of current statistics
func (sm *StatsManager) GetStats() Stats {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Deep copy to prevent external modification
	statsCopy := Stats{
		Models: make(map[string]*ModelStats),
	}

	for model, modelStats := range sm.stats.Models {
		statsCopy.Models[model] = &ModelStats{
			TotalSeconds:   modelStats.TotalSeconds,
			RecordingCount: modelStats.RecordingCount,
		}
	}

	return statsCopy
}

// GetStatsJSON returns statistics as a JSON string (for D-Bus)
func (sm *StatsManager) GetStatsJSON() (string, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	data, err := json.Marshal(sm.stats)
	if err != nil {
		return "", fmt.Errorf("failed to marshal stats to JSON: %w", err)
	}

	return string(data), nil
}

// Reset clears all statistics and persists empty state
func (sm *StatsManager) Reset() error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.stats = Stats{
		Models: make(map[string]*ModelStats),
	}

	if err := sm.save(); err != nil {
		return fmt.Errorf("failed to save reset stats: %w", err)
	}

	return nil
}

// load reads statistics from disk (internal use)
func (sm *StatsManager) load() error {
	data, err := os.ReadFile(sm.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist yet - start with empty stats
			logger.Debugf("Stats file not found, starting fresh: %s", sm.filePath)
			return nil
		}
		return fmt.Errorf("failed to read stats file: %w", err)
	}

	if err := json.Unmarshal(data, &sm.stats); err != nil {
		return fmt.Errorf("failed to unmarshal stats: %w", err)
	}

	// Ensure models map is initialized
	if sm.stats.Models == nil {
		sm.stats.Models = make(map[string]*ModelStats)
	}

	logger.Debugf("Loaded stats from %s", sm.filePath)
	return nil
}

// save writes statistics to disk (internal use)
func (sm *StatsManager) save() error {
	// Ensure directory exists
	dir := filepath.Dir(sm.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create stats directory: %w", err)
	}

	// Marshal with indentation for readability
	data, err := json.MarshalIndent(sm.stats, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal stats: %w", err)
	}

	// Write atomically by writing to temp file and renaming
	tempFile := sm.filePath + ".tmp"
	if err := os.WriteFile(tempFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write temp stats file: %w", err)
	}

	if err := os.Rename(tempFile, sm.filePath); err != nil {
		return fmt.Errorf("failed to rename temp stats file: %w", err)
	}

	logger.Debugf("Saved stats to %s", sm.filePath)
	return nil
}
