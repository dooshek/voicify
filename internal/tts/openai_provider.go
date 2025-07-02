package tts

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/dooshek/voicify/internal/logger"
	"github.com/sashabaranov/go-openai"
)

// OpenAITTSProvider implements TTSProvider for OpenAI TTS API
type OpenAITTSProvider struct {
	client *openai.Client
	config OpenAIConfig
}

// OpenAIConfig holds OpenAI TTS configuration
type OpenAIConfig struct {
	Model  string  `yaml:"model"`  // "tts-1" or "tts-1-hd"
	Speed  float64 `yaml:"speed"`  // 0.25-4.0, default 1.0
	Format string  `yaml:"format"` // "opus", "mp3", "aac", "flac"
}

// NewOpenAITTSProvider creates a new OpenAI TTS provider
func NewOpenAITTSProvider(apiKey string, config OpenAIConfig) *OpenAITTSProvider {
	client := openai.NewClient(apiKey)

	// Set defaults
	if config.Model == "" {
		config.Model = "tts-1-hd" // Better quality
	}
	if config.Speed == 0 {
		config.Speed = 1.0
	}
	if config.Format == "" {
		config.Format = "opus" // Smaller size, good quality
	}

	return &OpenAITTSProvider{
		client: client,
		config: config,
	}
}

// Speak converts text to speech and plays it immediately
func (p *OpenAITTSProvider) Speak(ctx context.Context, text string, voice string) error {
	// Get audio data
	audioData, err := p.GetAudio(ctx, text, voice)
	if err != nil {
		return err
	}

	// Play the audio
	return p.playAudio(audioData)
}

// GetAudio converts text to speech and returns audio data
func (p *OpenAITTSProvider) GetAudio(ctx context.Context, text string, voice string) ([]byte, error) {
	// Use default voice if not specified
	if voice == "" {
		voice = "nova" // Good default voice
	}

	logger.Infof("Generating TTS for text (length: %d chars) with voice: %s", len(text), voice)

	// Create TTS request
	response, err := p.client.CreateSpeech(ctx, openai.CreateSpeechRequest{
		Model:          openai.SpeechModel(p.config.Model),
		Input:          text,
		Voice:          openai.SpeechVoice(voice),
		Speed:          p.config.Speed,
		ResponseFormat: openai.SpeechResponseFormat(p.config.Format),
	})
	if err != nil {
		logger.Error("OpenAI TTS API error", err)
		return nil, fmt.Errorf("TTS request failed: %w", err)
	}
	defer response.Close()

	// Read audio data
	audioData, err := io.ReadAll(response)
	if err != nil {
		return nil, fmt.Errorf("failed to read audio data: %w", err)
	}

	logger.Infof("Generated %d bytes of %s audio (estimated %s)",
		len(audioData), p.config.Format, p.estimateFileSize(len(audioData)))

	return audioData, nil
}

// playAudio plays audio data using system audio player
func (p *OpenAITTSProvider) playAudio(audioData []byte) error {
	// Create temporary file
	tmpFile, err := os.CreateTemp("", fmt.Sprintf("voicify_tts_*.%s", p.config.Format))
	if err != nil {
		return fmt.Errorf("failed to create temporary file: %w", err)
	}
	defer os.Remove(tmpFile.Name()) // Clean up

	// Write audio data to file
	if _, err := tmpFile.Write(audioData); err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to write audio data: %w", err)
	}
	tmpFile.Close()

	logger.Infof("🔊 Playing TTS audio...")

	// Play audio using paplay (Linux/PulseAudio)
	cmd := exec.Command("paplay", tmpFile.Name())
	if err := cmd.Run(); err != nil {
		// Fallback to other players if paplay is not available
		logger.Debugf("paplay failed, trying fallback players: %v", err)
		return p.playAudioFallback(tmpFile.Name())
	}

	logger.Debugf("✅ TTS audio playback completed")
	return nil
}

// playAudioFallback tries alternative audio players
func (p *OpenAITTSProvider) playAudioFallback(filename string) error {
	// Try different audio players in order of preference
	players := [][]string{
		{"mpv", "--no-video", "--really-quiet", filename},
		{"ffplay", "-nodisp", "-autoexit", "-loglevel", "quiet", filename},
		{"aplay", filename}, // for basic PCM/WAV
		{"vlc", "--intf", "dummy", "--play-and-exit", filename},
	}

	for _, player := range players {
		cmd := exec.Command(player[0], player[1:]...)
		if err := cmd.Run(); err == nil {
			logger.Debugf("✅ TTS audio played using %s", player[0])
			return nil
		}
	}

	return fmt.Errorf("no suitable audio player found (tried: paplay, mpv, ffplay, aplay, vlc)")
}

// GetAvailableVoices returns OpenAI TTS voices
func (p *OpenAITTSProvider) GetAvailableVoices() []string {
	return []string{
		"alloy",   // Neutral, balanced
		"echo",    // Male, clear
		"fable",   // British accent
		"onyx",    // Deep male
		"nova",    // Young female (recommended)
		"shimmer", // Warm female
	}
}

// GetProviderName returns provider name
func (p *OpenAITTSProvider) GetProviderName() string {
	return "OpenAI TTS"
}

// estimateFileSize provides human-readable size estimate
func (p *OpenAITTSProvider) estimateFileSize(bytes int) string {
	if bytes < 1024 {
		return fmt.Sprintf("%d B", bytes)
	} else if bytes < 1024*1024 {
		return fmt.Sprintf("%.1f KB", float64(bytes)/1024)
	} else {
		return fmt.Sprintf("%.1f MB", float64(bytes)/(1024*1024))
	}
}
