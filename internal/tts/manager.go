package tts

import (
	"context"
	"fmt"

	"github.com/dooshek/voicify/internal/logger"
	"github.com/dooshek/voicify/internal/types"
)

// Manager manages TTS providers and handles text-to-speech operations
type Manager struct {
	provider TTSProvider
	config   types.TTSConfig
}

// NewManager creates a new TTS Manager with the specified configuration and API key
func NewManager(config types.TTSConfig, apiKey string) (*Manager, error) {
	provider, err := createProvider(config, apiKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create TTS provider: %w", err)
	}

	logger.Infof("Initialized TTS Manager with provider: %s", provider.GetProviderName())

	return &Manager{
		provider: provider,
		config:   config,
	}, nil
}

// Speak converts text to speech and plays it immediately
func (m *Manager) Speak(ctx context.Context, text string) error {
	if text == "" {
		return fmt.Errorf("text cannot be empty")
	}

	// Use configured default voice
	voice := m.config.Voice
	if voice == "" {
		voice = "nova" // fallback default
	}

	return m.provider.Speak(ctx, text, voice)
}

// SpeakWithVoice converts text to speech and plays it using a specific voice
func (m *Manager) SpeakWithVoice(ctx context.Context, text string, voice string) error {
	if text == "" {
		return fmt.Errorf("text cannot be empty")
	}

	if voice == "" {
		return m.Speak(ctx, text)
	}

	return m.provider.Speak(ctx, text, voice)
}

// GetAudio converts text to speech and returns raw audio data
func (m *Manager) GetAudio(ctx context.Context, text string) ([]byte, error) {
	if text == "" {
		return nil, fmt.Errorf("text cannot be empty")
	}

	// Use configured default voice
	voice := m.config.Voice
	if voice == "" {
		voice = "nova" // fallback default
	}

	return m.provider.GetAudio(ctx, text, voice)
}

// GetAudioWithVoice converts text to speech and returns raw audio data using a specific voice
func (m *Manager) GetAudioWithVoice(ctx context.Context, text string, voice string) ([]byte, error) {
	if text == "" {
		return nil, fmt.Errorf("text cannot be empty")
	}

	if voice == "" {
		return m.GetAudio(ctx, text)
	}

	return m.provider.GetAudio(ctx, text, voice)
}

// GetAvailableVoices returns list of available voices
func (m *Manager) GetAvailableVoices() []string {
	return m.provider.GetAvailableVoices()
}

// GetProviderName returns the name of the current provider
func (m *Manager) GetProviderName() string {
	return m.provider.GetProviderName()
}

// createProvider creates appropriate TTS provider based on configuration and API key
func createProvider(config types.TTSConfig, apiKey string) (TTSProvider, error) {
	switch config.Provider {
	case "openai":
		// Traditional OpenAI TTS API
		if apiKey == "" {
			return nil, fmt.Errorf("OpenAI API key is required for OpenAI TTS provider - configure it using the wizard")
		}

		openaiConfig := OpenAIConfig{
			Model:  config.OpenAI.Model,
			Speed:  config.OpenAI.Speed,
			Format: config.OpenAI.Format,
		}

		return NewOpenAITTSProvider(apiKey, openaiConfig), nil

	case "realtime":
		// OpenAI Realtime API - better quality, lower latency
		if apiKey == "" {
			return nil, fmt.Errorf("OpenAI API key is required for Realtime TTS provider - configure it using the wizard")
		}

		realtimeConfig := RealtimeConfig{
			Model: config.Realtime.Model,
			Speed: config.Realtime.Speed,
			Voice: config.Voice, // Use main voice setting
		}

		return NewRealtimeTTSProvider(apiKey, realtimeConfig), nil

	case "elevenlabs":
		// Future implementation for ElevenLabs
		return nil, fmt.Errorf("ElevenLabs TTS provider not yet implemented")

	default:
		return nil, fmt.Errorf("unsupported TTS provider: %s (supported: openai, realtime, elevenlabs)", config.Provider)
	}
}
