package transcriber

import (
	"context"
	"fmt"
	"os"

	"github.com/dooshek/voicify/internal/fileops"
	"github.com/dooshek/voicify/internal/llm"
	"github.com/dooshek/voicify/internal/logger"
	"github.com/dooshek/voicify/internal/state"
)

type Transcriber struct {
	provider llm.Provider
	fileOps  fileops.FileOps
}

func NewTranscriber() (*Transcriber, error) {
	logger.Debug("Initializing transcriber")
	fileOps, err := fileops.NewDefaultFileOps()
	if err != nil {
		logger.Warnf("Failed to initialize file operations: %v", err)
	}

	provider, err := llm.NewProvider(state.Get().GetTranscriptionProvider())
	if err != nil {
		logger.Errorf("Failed to initialize LLM provider: %v", err)
		return nil, fmt.Errorf("failed to initialize LLM provider: %w", err)
	}

	logger.Debug("Transcriber initialized successfully")
	return &Transcriber{
		provider: provider,
		fileOps:  fileOps,
	}, nil
}

func (t *Transcriber) TranscribeFile(filename string) (string, error) {
	logger.Debugf("Starting transcription of file: %s", filename)
	audioFile, err := os.Open(filename)
	if err != nil {
		logger.Errorf("Error opening audio file: %v", err)
		return "", fmt.Errorf("error opening audio file: %w", err)
	}
	defer audioFile.Close()

	text, err := t.provider.TranscribeAudio(context.Background(), filename, audioFile)
	if err != nil {
		logger.Errorf("Error during transcription: %v", err)
		return "", fmt.Errorf("error transcribing audio: %w", err)
	}

	logger.Debug("Transcription completed successfully")
	return text, nil
}
