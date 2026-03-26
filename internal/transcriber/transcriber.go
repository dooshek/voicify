package transcriber

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/dooshek/voicify/internal/fileops"
	"github.com/dooshek/voicify/internal/llm"
	"github.com/dooshek/voicify/internal/logger"
	"github.com/dooshek/voicify/internal/state"
	"github.com/dooshek/voicify/internal/types"
)

type Transcriber struct {
	provider            llm.Provider
	entropycoreProvider llm.Provider
	fileOps             fileops.FileOps
}

func NewTranscriber() (*Transcriber, error) {
	logger.Debug("Initializing transcriber")
	fileOps, err := fileops.NewDefaultFileOps()
	if err != nil {
		logger.Warnf("Failed to initialize file operations: %v", err)
	}

	provider, err := llm.NewProvider(state.Get().GetTranscriptionProvider())
	if err != nil {
		logger.Error("Failed to initialize LLM provider", err)
		return nil, fmt.Errorf("failed to initialize LLM provider: %w", err)
	}

	t := &Transcriber{
		provider: provider,
		fileOps:  fileOps,
	}

	// Lazily initialize Entropycore provider if key is configured
	if key := state.Get().Config.LLM.Keys.EntropycoreKey; key != "" {
		ep, err := llm.NewProvider(types.ProviderEntropycore)
		if err != nil {
			logger.Warnf("Failed to initialize Entropycore provider: %v", err)
		} else {
			t.entropycoreProvider = ep
		}
	}

	logger.Debug("Transcriber initialized successfully")
	return t, nil
}

func (t *Transcriber) getProvider() (llm.Provider, error) {
	model := state.Get().Config.LLM.Transcription.Model
	if strings.HasPrefix(model, "entropycore-") {
		if t.entropycoreProvider == nil {
			return nil, fmt.Errorf("entropycore model selected but no API key configured (entropycore_api_key in voicify.yaml)")
		}
		return t.entropycoreProvider, nil
	}
	return t.provider, nil
}

func (t *Transcriber) TranscribeFile(filename string) (string, error) {
	logger.Debugf("Starting transcription of file: %s", filename)

	provider, err := t.getProvider()
	if err != nil {
		logger.Error("Failed to get transcription provider", err)
		return "", err
	}

	const maxRetries = 4
	delay := 2 * time.Second
	var lastErr error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		audioFile, err := os.Open(filename)
		if err != nil {
			logger.Error("Error opening audio file", err)
			return "", fmt.Errorf("error opening audio file: %w", err)
		}

		text, err := provider.TranscribeAudio(context.Background(), filename, audioFile)
		audioFile.Close()

		if err == nil {
			if attempt > 1 {
				logger.Infof("Transcription succeeded on attempt %d/%d", attempt, maxRetries)
			}
			logger.Debug("Transcription completed successfully")
			return text, nil
		}

		lastErr = err
		if attempt < maxRetries {
			logger.Warnf("Transcription attempt %d/%d failed, retrying in %v: %v", attempt, maxRetries, delay, err)
			time.Sleep(delay)
			delay *= 2
		}
	}

	logger.Error("Transcription failed after all retries", lastErr)
	return "", fmt.Errorf("error transcribing audio: %w", lastErr)
}
