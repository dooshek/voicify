package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"time"

	"github.com/dooshek/voicify/internal/logger"
	"github.com/dooshek/voicify/internal/state"
)

// EntropycoreProvider implements Provider interface using Entropycore Whisper API
type EntropycoreProvider struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
}

// NewEntropycoreProvider creates new Entropycore provider instance
func NewEntropycoreProvider(apiKey string) *EntropycoreProvider {
	logger.Debugf("Creating Entropycore provider")
	return &EntropycoreProvider{
		apiKey:     apiKey,
		baseURL:    "https://whisper.entropycore.io/v1",
		httpClient: &http.Client{Timeout: 120 * time.Second},
	}
}

// TranscribeAudio sends a transcription request to Entropycore Whisper API
func (p *EntropycoreProvider) TranscribeAudio(ctx context.Context, filename string, reader AudioReader) (string, error) {
	logger.Debugf("Sending transcription request to Entropycore for file: %s", filename)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return "", fmt.Errorf("error creating form file: %w", err)
	}

	if _, err := io.Copy(part, reader); err != nil {
		return "", fmt.Errorf("error copying file data: %w", err)
	}

	// Add model field - Entropycore uses standard Whisper model name
	if err := writer.WriteField("model", "whisper-large-v3"); err != nil {
		return "", fmt.Errorf("error writing model field: %w", err)
	}

	if lang := state.Get().Config.LLM.Transcription.Language; lang != "" {
		if err := writer.WriteField("language", lang); err != nil {
			return "", fmt.Errorf("error writing language field: %w", err)
		}
	}

	if err := writer.Close(); err != nil {
		return "", fmt.Errorf("error closing multipart writer: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/audio/transcriptions", body)
	if err != nil {
		return "", fmt.Errorf("error creating request: %w", err)
	}

	httpReq.Header.Set("Content-Type", writer.FormDataContentType())
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("error sending request to Entropycore: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("error reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("entropycore API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var transcription struct {
		Text  string `json:"text"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error,omitempty"`
	}

	if err := json.Unmarshal(respBody, &transcription); err != nil {
		return "", fmt.Errorf("error unmarshaling response: %w", err)
	}

	if transcription.Error != nil {
		return "", fmt.Errorf("entropycore API error: %s", transcription.Error.Message)
	}

	return transcription.Text, nil
}

// Completion is not supported by Entropycore (transcription-only provider)
func (p *EntropycoreProvider) Completion(ctx context.Context, req CompletionRequest) (string, error) {
	return "", fmt.Errorf("completion not supported by Entropycore provider")
}
