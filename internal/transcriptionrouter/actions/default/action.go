package defaultaction

import (
	"github.com/dooshek/voicify/internal/logger"
	"github.com/dooshek/voicify/internal/types"
	"github.com/dooshek/voicify/pkg/clipboard"
)

// Name changed from Action to PluginAction to match interface
type PluginAction struct {
	transcription string
}

func New(transcription string) *PluginAction {
	return &PluginAction{
		transcription: transcription,
	}
}

func (a *PluginAction) Name() string {
	return "default"
}

func (a *PluginAction) Execute(text string) error {
	logger.Debug("Action[default]: copying processed transcription to clipboard")
	if err := clipboard.CopyToClipboard(text); err != nil {
		logger.Error("Action[default]: failed to copy text to clipboard", err)
		return err
	}

	return nil
}

func (a *PluginAction) GetMetadata() types.ActionMetadata {
	return types.ActionMetadata{
		Name:        "default",
		Description: "domyślna akcja kopiująca tekst do schowka",
		Priority:    1,
	}
}
