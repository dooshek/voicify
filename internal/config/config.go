package config

import (
	"fmt"

	"github.com/dooshek/voicify/internal/fileops"
	"github.com/dooshek/voicify/internal/logger"
	"github.com/dooshek/voicify/internal/types"
	"gopkg.in/yaml.v3"
)

const (
	configFilename = "voicify.yaml"
)

func LoadConfig() (*types.Config, error) {
	fileOps, err := fileops.NewDefaultFileOps()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize file operations: %w", err)
	}

	if err := fileOps.EnsureDirectories(); err != nil {
		return nil, fmt.Errorf("failed to create directories: %w", err)
	}

	data, err := fileOps.LoadConfig(configFilename)
	if err != nil {
		if err == fileops.ErrConfigNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config types.Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return &config, nil
}

func SaveConfig(config *types.Config) error {
	fileOps, err := fileops.NewDefaultFileOps()
	if err != nil {
		return fmt.Errorf("failed to initialize file operations: %w", err)
	}

	// Try to load existing config first
	existingConfig, err := LoadConfig()
	if err != nil {
		// Just log the error but continue with new config
		logger.Warnf("Failed to load existing config: %v", err)
	} else if existingConfig != nil {
		// We have an existing config, merge the new settings into it
		mergeConfigs(existingConfig, config)
		config = existingConfig
	}

	// Marshal the config to YAML
	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Save the config using fileOps
	if err := fileOps.SaveConfig(configFilename, data); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	return nil
}

// mergeConfigs merges the sourceConfig into targetConfig, preserving existing values in targetConfig
// that are not explicitly set in sourceConfig
func mergeConfigs(targetConfig, sourceConfig *types.Config) {
	// Only update the key binding if it's explicitly set in the source config
	if sourceConfig.RecordKey.Key != "" {
		targetConfig.RecordKey = sourceConfig.RecordKey
	}

	// Update LLM config if set
	if sourceConfig.LLM.Keys.OpenAIKey != "" {
		targetConfig.LLM.Keys.OpenAIKey = sourceConfig.LLM.Keys.OpenAIKey
	}
	if sourceConfig.LLM.Keys.GroqKey != "" {
		targetConfig.LLM.Keys.GroqKey = sourceConfig.LLM.Keys.GroqKey
	}

	// Update LLM Transcription settings if set
	if sourceConfig.LLM.Transcription.Provider != "" {
		targetConfig.LLM.Transcription.Provider = sourceConfig.LLM.Transcription.Provider
	}
	if sourceConfig.LLM.Transcription.Model != "" {
		targetConfig.LLM.Transcription.Model = sourceConfig.LLM.Transcription.Model
	}
	if sourceConfig.LLM.Transcription.Language != "" {
		targetConfig.LLM.Transcription.Language = sourceConfig.LLM.Transcription.Language
	}

	// Update LLM Router settings if set
	if sourceConfig.LLM.Router.Provider != "" {
		targetConfig.LLM.Router.Provider = sourceConfig.LLM.Router.Provider
	}
	if sourceConfig.LLM.Router.Model != "" {
		targetConfig.LLM.Router.Model = sourceConfig.LLM.Router.Model
	}
	if sourceConfig.LLM.Router.Temperature != 0 {
		targetConfig.LLM.Router.Temperature = sourceConfig.LLM.Router.Temperature
	}

	// Update TTS settings if set
	if sourceConfig.TTS.Provider != "" {
		targetConfig.TTS.Provider = sourceConfig.TTS.Provider
	}
	if sourceConfig.TTS.Voice != "" {
		targetConfig.TTS.Voice = sourceConfig.TTS.Voice
	}
	if sourceConfig.TTS.OpenAI.Model != "" {
		targetConfig.TTS.OpenAI.Model = sourceConfig.TTS.OpenAI.Model
	}
	if sourceConfig.TTS.OpenAI.Speed != 0 {
		targetConfig.TTS.OpenAI.Speed = sourceConfig.TTS.OpenAI.Speed
	}
	if sourceConfig.TTS.OpenAI.Format != "" {
		targetConfig.TTS.OpenAI.Format = sourceConfig.TTS.OpenAI.Format
	}

	// Preserve any additional fields that might be added in the future
	// This is a basic implementation - for more complex nested structures,
	// you might need a more sophisticated merging strategy
}
