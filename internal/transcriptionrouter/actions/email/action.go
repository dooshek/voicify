package email

import (
	"context"
	"fmt"

	"github.com/dooshek/voicify/internal/llm"
	"github.com/dooshek/voicify/internal/logger"
	"github.com/dooshek/voicify/internal/notification"
	"github.com/dooshek/voicify/internal/types"
	"github.com/dooshek/voicify/pkg/clipboard"
)

type PluginAction struct {
	transcription string
}

func New(transcription string) *PluginAction {
	return &PluginAction{transcription: transcription}
}

const emailPrompt = `
<objective>
You are a professional business email writer. Your task is to transform the given text into a well-structured, professional business email.
</objective>
<rules>
- Use appropriate business language and tone, do not use slang or colloquial language
- Include proper email structure (greeting, body, closing)
- Fix any grammar or spelling mistakes
- Make it concise and clear
- Maintain the original message intent
- Use appropriate level of formality
- Maintain original language
- Don't add "subject" nor "footer" elements, nor any placeholders, create just an email body
- Always return only email body, without any additional text or comments
</rules>

<input>%s</input>
`

func (a *PluginAction) Execute(transcription string) error {
	logger.Debugf("Action[email]: creating email, with transcription: %s", transcription)

	notifier := notification.New()
	prompt := fmt.Sprintf(emailPrompt, transcription)

	provider, err := llm.NewProvider(types.ProviderOpenAI)
	if err != nil {
		logger.Errorf("Action[email]: failed to create LLM provider: %v", err)
		notifier.Notify("Failed to create email", "Error processing with AI")
		return err
	}

	formattedEmail, err := provider.Completion(context.Background(), llm.CompletionRequest{
		Model: "gpt-4o-mini",
		Messages: []llm.ChatCompletionMessage{
			{
				Role:    "system",
				Content: prompt,
			},
		},
		MaxTokens:   1000,
		Temperature: 0.5,
	})
	if err != nil {
		logger.Errorf("Action[email]: failed to process email with LLM: %v", err)
		notifier.Notify("Failed to create email", "Error processing with AI")
		return err
	}

	if err := clipboard.CopyToClipboard(formattedEmail); err != nil {
		logger.Errorf("Action[email]: failed to copy to clipboard: %v", err)
		notifier.Notify("Failed to copy email", "Error copying to clipboard")
		return err
	}

	notifier.Notify("Email created", "Professional email has been copied to clipboard")
	logger.Infof("Action[email]: email created and copied to clipboard")

	return nil
}

func (a *PluginAction) GetMetadata() types.ActionMetadata {
	return types.ActionMetadata{
		Name:        "email",
		Description: "redagowanie maila",
		LLMCommands: &[]string{
			"piszemy maila",
			"napiszmy maila",
			"chciałbym zredagować maila",
		},
		Priority: 3,
	}
}
