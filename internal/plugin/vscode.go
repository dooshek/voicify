package plugin

import (
	"github.com/dooshek/voicify/internal/logger"
	"github.com/dooshek/voicify/pkg/pluginapi"
)

// VSCodePlugin is a plugin for VSCode
type VSCodePlugin struct{}

// VSCodeAction is the VSCode action
type VSCodeAction struct {
	transcription string
}

// Initialize initializes the VSCode plugin
func (p *VSCodePlugin) Initialize() error {
	logger.Debug("VSCode plugin initialized")
	return nil
}

// GetMetadata returns metadata about the plugin
func (p *VSCodePlugin) GetMetadata() pluginapi.PluginMetadata {
	return pluginapi.PluginMetadata{
		Name:        "vscode",
		Version:     "1.0.0",
		Description: "Plugin for Visual Studio Code",
		Author:      "Voicify Team",
	}
}

// GetActions returns a list of actions provided by this plugin
func (p *VSCodePlugin) GetActions(transcription string) []pluginapi.PluginAction {
	return []pluginapi.PluginAction{
		&VSCodeAction{transcription: transcription},
	}
}

// Execute executes the VSCode action
func (a *VSCodeAction) Execute(transcription string) error {
	logger.Debugf("VSCode plugin: Checking if VSCode should execute action for transcription: %s", transcription)

	if !IsAppFocused("VSC") {
		logger.Debug("VSCode plugin: VSCode is not open, skipping action")
		return nil
	}

	logger.Debug("VSCode plugin: VSCode is focused, executing action")
	return PasteWithReturn(transcription)
}

// GetMetadata returns metadata about the action
func (a *VSCodeAction) GetMetadata() pluginapi.ActionMetadata {
	return pluginapi.ActionMetadata{
		Name:              "vscode",
		Description:       "wykonanie akcji w edytorze VSCode",
		SkipDefaultAction: true, // VSCode plugin już kopiuje i wkleja tekst
		Priority:          2,
	}
}

// NewVSCodePlugin creates a new instance of the VSCode plugin
func NewVSCodePlugin() pluginapi.VoicifyPlugin {
	return &VSCodePlugin{}
}
