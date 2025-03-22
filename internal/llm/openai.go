package llm

import (
	"context"
	"fmt"

	"github.com/dooshek/voicify/internal/logger"
	"github.com/dooshek/voicify/internal/state"
	"github.com/sashabaranov/go-openai"
)

// OpenAIProvider implements Provider interface using OpenAI
type OpenAIProvider struct {
	client *openai.Client
}

// NewOpenAIProvider creates new OpenAI provider instance
func NewOpenAIProvider(apiKey string) *OpenAIProvider {
	logger.Debugf("Creating OpenAI provider")

	return &OpenAIProvider{
		client: openai.NewClient(apiKey),
	}
}

// TranscribeAudio implements audio transcription using OpenAI's Whisper model
func (p *OpenAIProvider) TranscribeAudio(ctx context.Context, filename string, reader AudioReader) (string, error) {
	logger.Debugf("Transcription model: %s", state.Get().Config.LLM.Transcription.Model)
	req := openai.AudioRequest{
		Reader:   reader,
		FilePath: filename,
		Format:   openai.AudioResponseFormatText,
		Model:    state.Get().Config.LLM.Transcription.Model,
		Language: state.Get().Config.LLM.Transcription.Language,
	}
	resp, err := p.client.CreateTranscription(ctx, req)
	if err != nil {
		return "", fmt.Errorf("error transcribing audio with OpenAI: %w", err)
	}

	return resp.Text, nil
}

// Completion sends a completion request to OpenAI API
func (p *OpenAIProvider) Completion(ctx context.Context, req CompletionRequest) (string, error) {
	logger.Debugf("Sending completion request with model: %s", req.Model)

	if req.MaxTokens == 0 {
		req.MaxTokens = 2000
	}

	if req.Temperature == 0 {
		req.Temperature = 0.5
	}

	messages := make([]openai.ChatCompletionMessage, len(req.Messages))
	for i, msg := range req.Messages {
		messages[i] = openai.ChatCompletionMessage{
			Role:    msg.Role,
			Content: msg.Content,
		}
	}

	resp, err := p.client.CreateChatCompletion(
		ctx,
		openai.ChatCompletionRequest{
			Model:       req.Model,
			Messages:    messages,
			MaxTokens:   req.MaxTokens,
			Temperature: req.Temperature,
		},
	)
	if err != nil {
		return "", fmt.Errorf("error creating completion with OpenAI: %w", err)
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no completion choices returned from OpenAI")
	}

	return resp.Choices[0].Message.Content, nil
}
