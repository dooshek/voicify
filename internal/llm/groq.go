package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"

	"github.com/dooshek/voicify/internal/logger"
	"github.com/dooshek/voicify/internal/state"
)

// GroqProvider implements Provider interface using Groq
type GroqProvider struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
}

type groqRequest struct {
	Model          string          `json:"model"`
	Messages       []groqMessage   `json:"messages"`
	MaxTokens      int             `json:"max_completion_tokens,omitempty"`
	Temperature    float32         `json:"temperature,omitempty"`
	ResponseFormat *responseFormat `json:"response_format,omitempty"`
}

type groqMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type groqResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

type responseFormat struct {
	Type string `json:"type"`
}

// NewGroqProvider creates new Groq provider instance
func NewGroqProvider(apiKey string) *GroqProvider {
	logger.Debugf("Creating Groq provider")
	return &GroqProvider{
		apiKey:     apiKey,
		baseURL:    "https://api.groq.com/openai/v1",
		httpClient: &http.Client{},
	}
}

// TranscribeAudio sends a transcription request to Groq API
func (p *GroqProvider) TranscribeAudio(ctx context.Context, filename string, reader AudioReader) (string, error) {
	logger.Debugf("Sending transcription request for file: %s", filename)

	// Create multipart form body
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Add the file
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return "", fmt.Errorf("error creating form file: %w", err)
	}

	if _, err := io.Copy(part, reader); err != nil {
		return "", fmt.Errorf("error copying file data: %w", err)
	}

	// Add the model field
	if err := writer.WriteField("model", state.Get().Config.LLM.Transcription.Model); err != nil {
		return "", fmt.Errorf("error writing model field: %w", err)
	}

	// Add language to multipart form if specified
	if lang := state.Get().Config.LLM.Transcription.Language; lang != "" {
		if err := writer.WriteField("language", lang); err != nil {
			return "", fmt.Errorf("error writing language field: %w", err)
		}
	}

	// Close the multipart writer
	if err := writer.Close(); err != nil {
		return "", fmt.Errorf("error closing multipart writer: %w", err)
	}

	// Create request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/audio/transcriptions", body)
	if err != nil {
		return "", fmt.Errorf("error creating request: %w", err)
	}

	httpReq.Header.Set("Content-Type", writer.FormDataContentType())
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)

	// Send request
	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("error sending request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("error reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(respBody))
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
		return "", fmt.Errorf("groq API error: %s", transcription.Error.Message)
	}

	return transcription.Text, nil
}

// Completion sends a completion request to Groq API
func (p *GroqProvider) Completion(ctx context.Context, req CompletionRequest) (string, error) {
	logger.Debugf("Sending completion request with model: %s", req.Model)

	if req.MaxTokens == 0 {
		req.MaxTokens = 2000
	}

	if req.Temperature == 0 {
		req.Temperature = 0.5
	}

	messages := make([]groqMessage, len(req.Messages))
	for i, msg := range req.Messages {
		messages[i] = groqMessage(msg)
	}

	groqReq := groqRequest{
		Model:       req.Model,
		Messages:    messages,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
		ResponseFormat: &responseFormat{
			Type: "json_object",
		},
	}

	jsonData, err := json.Marshal(groqReq)
	if err != nil {
		return "", fmt.Errorf("error marshaling request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("error creating request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("error sending request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("error reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	var groqResp groqResponse
	if err := json.Unmarshal(body, &groqResp); err != nil {
		return "", fmt.Errorf("error unmarshaling response: %w", err)
	}

	if groqResp.Error != nil {
		return "", fmt.Errorf("groq API error: %s", groqResp.Error.Message)
	}

	if len(groqResp.Choices) == 0 {
		return "", fmt.Errorf("no completion choices returned from Groq")
	}

	return groqResp.Choices[0].Message.Content, nil
}
