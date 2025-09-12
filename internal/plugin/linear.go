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

	logger.Debug("Linear plugin: Linear is focused, executing action")
	return PasteWithReturn(transcription)
}

// GetMetadata returns metadata about the action
func (a *LinearAction) GetMetadata() pluginapi.ActionMetadata {
	return pluginapi.ActionMetadata{
		Name:        "linear",
		Description: "wykonanie akcji w Linear",
		Priority:    2,
	}
}

// NewLinearPlugin creates a new instance of the Linear plugin
func NewLinearPlugin() pluginapi.VoicifyPlugin {
	return &LinearPlugin{}
}
