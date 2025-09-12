package plugin

import (
	"github.com/dooshek/voicify/internal/logger"
	"github.com/dooshek/voicify/pkg/pluginapi"
)

// DefaultPlugin is the default plugin that copies text to clipboard and pastes it
type DefaultPlugin struct{}

// DefaultAction is the default action
type DefaultAction struct {
	transcription string
}

// Initialize initializes the default plugin
func (p *DefaultPlugin) Initialize() error {
	logger.Debug("Default plugin initialized")
	return nil
}

// GetMetadata returns metadata about the plugin
func (p *DefaultPlugin) GetMetadata() pluginapi.PluginMetadata {
	return pluginapi.PluginMetadata{
		Name:        "default",
		Version:     "1.0.0",
		Description: "Default plugin for copying text to clipboard and pasting",
		Author:      "Voicify Team",
	}
}

// GetActions returns a list of actions provided by this plugin
func (p *DefaultPlugin) GetActions(transcription string) []pluginapi.PluginAction {
	return []pluginapi.PluginAction{
		&DefaultAction{transcription: transcription},
	}
}

// Execute executes the default action
func (a *DefaultAction) Execute(transcription string) error {
	logger.Debug("Default plugin: Executing default action - copy to clipboard and paste")

	// Copy to clipboard and paste
	return PasteWithReturn(transcription)
}

// GetMetadata returns metadata about the action
func (a *DefaultAction) GetMetadata() pluginapi.ActionMetadata {
	return pluginapi.ActionMetadata{
		Name:              "default",
		Description:       "domyślna akcja - kopiowanie do schowka i wklejanie",
		SkipDefaultAction: false, // To jest domyślna akcja
		Priority:          999,   // Najniższy priorytet - wykonuje się na końcu
	}
}

// NewDefaultPlugin creates a new instance of the default plugin
func NewDefaultPlugin() pluginapi.VoicifyPlugin {
	return &DefaultPlugin{}
}
