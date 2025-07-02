package tts

import (
	"context"
)

// TTSProvider defines the interface for text-to-speech providers
type TTSProvider interface {
	// Speak converts text to speech and plays it immediately
	Speak(ctx context.Context, text string, voice string) error

	// GetAudio converts text to speech and returns audio data
	GetAudio(ctx context.Context, text string, voice string) ([]byte, error)

	// GetAvailableVoices returns list of available voices
	GetAvailableVoices() []string

	// GetProviderName returns the name of the provider
	GetProviderName() string
}

// AudioFormat represents supported audio formats
type AudioFormat string

const (
	FormatOpus AudioFormat = "opus"
	FormatMP3  AudioFormat = "mp3"
	FormatAAC  AudioFormat = "aac"
	FormatFLAC AudioFormat = "flac"
	FormatPCM  AudioFormat = "pcm"
)
