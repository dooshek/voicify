package transcriber

import (
	"context"
	"fmt"
	"os"
	"strings"

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
	audioFile, err := os.Open(filename)
	if err != nil {
		logger.Error("Error opening audio file", err)
		return "", fmt.Errorf("error opening audio file: %w", err)
	}
	defer audioFile.Close()

	provider, err := t.getProvider()
	if err != nil {
		logger.Error("Failed to get transcription provider", err)
		return "", err
	}

	text, err := provider.TranscribeAudio(context.Background(), filename, audioFile)
	if err != nil {
		logger.Error("Error during transcription", err)
		return "", fmt.Errorf("error transcribing audio: %w", err)
	}

	logger.Debug("Transcription completed successfully")
	return text, nil
}
