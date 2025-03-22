package llm

import (
	"context"
	"fmt"
	"io"

	"github.com/dooshek/voicify/internal/state"
	"github.com/dooshek/voicify/internal/types"
)

// AudioReader represents an interface for reading audio data
type AudioReader interface {
	io.Reader
}

// ChatCompletionMessage represents a message in a chat completion request
type ChatCompletionMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// CompletionRequest represents the parameters for a completion request
type CompletionRequest struct {
	Model       string                  `json:"model"`
	Messages    []ChatCompletionMessage `json:"messages"`
	MaxTokens   int                     `json:"max_tokens,omitempty"`
	Temperature float32                 `json:"temperature,omitempty"`
}

// Provider defines the interface for LLM providers
type Provider interface {
	TranscribeAudio(ctx context.Context, filename string, reader AudioReader) (string, error)
	Completion(ctx context.Context, req CompletionRequest) (string, error)
}

// NewProvider creates a new LLM provider based on the provider type
func NewProvider(providerType types.LLMProvider) (Provider, error) {
	llmKeys := state.Get().Config.LLM.Keys

	switch providerType {
	case types.ProviderOpenAI:
		if llmKeys.OpenAIKey != "" {
			return NewOpenAIProvider(llmKeys.OpenAIKey), nil
		}
	case types.ProviderGroq:
		if llmKeys.GroqKey != "" {
			return NewGroqProvider(llmKeys.GroqKey), nil
		}
	default:
		return nil, fmt.Errorf("unsupported provider type: %s", providerType)
	}

	return nil, fmt.Errorf("no API key provided for provider type: %s", providerType)
}
