package plugin

import (
	"github.com/dooshek/voicify/internal/logger"
	"github.com/dooshek/voicify/pkg/pluginapi"
)

// LinearPlugin is a plugin for Linear
type LinearPlugin struct{}

// LinearAction is the Linear action
type LinearAction struct {
	transcription string
}

// Initialize initializes the Linear plugin
func (p *LinearPlugin) Initialize() error {
	logger.Debug("Linear plugin initialized")
	return nil
}

// GetMetadata returns metadata about the plugin
func (p *LinearPlugin) GetMetadata() pluginapi.PluginMetadata {
	return pluginapi.PluginMetadata{
		Name:        "linear",
		Version:     "1.0.0",
		Description: "Plugin for Linear",
		Author:      "Voicify Team",
	}
}

// GetActions returns a list of actions provided by this plugin
func (p *LinearPlugin) GetActions(transcription string) []pluginapi.PluginAction {
	return []pluginapi.PluginAction{
		&LinearAction{transcription: transcription},
	}
}

// Execute executes the Linear action
func (a *LinearAction) Execute(transcription string) error {
	logger.Debugf("Linear plugin: Checking if Linear should execute action for transcription: %s", transcription)

	if !IsAppFocused("Linear") {
		logger.Debug("Linear plugin: Linear is not open, skipping action")
		return nil
	}

	logger.Info("🔧 Linear plugin: Akcja została uruchomiona - plugin Linear jest aktywny")
	logger.Debug("Linear plugin: Linear is focused, executing action")

	// Na razie tylko komunikat - implementacja będzie dodana później
	logger.Debugf("Linear plugin: Transkrypcja do przetworzenia: %s", transcription)

	return nil
}

// GetMetadata returns metadata about the action
func (a *LinearAction) GetMetadata() pluginapi.ActionMetadata {
	prompt := `linear: Zarządzanie ticketami w Linear. Rozpoznaje komendy związane z tworzeniem, edycją i zarządzaniem ticketami/issue'ami w aplikacji Linear. Przykłady: "stwórz tiket", "edytuj ticket", "dodaj nowy issue w Linear", "zmień status tiketu", "linear zadanie", "w linearze stwórz", "nowy ticket do projektu"`

	return pluginapi.ActionMetadata{
		Name:            "linear",
		Description:     "wykonanie akcji w Linear - tworzenie i edycja ticketów",
		Priority:        2,
		LLMRouterPrompt: &prompt,
	}
}

// NewLinearPlugin creates a new instance of the Linear plugin
func NewLinearPlugin() pluginapi.VoicifyPlugin {
	return &LinearPlugin{}
}
