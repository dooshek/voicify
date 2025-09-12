package plugin

import (
	"github.com/dooshek/voicify/internal/types"
	"github.com/dooshek/voicify/pkg/pluginapi"
)

// pluginAdapter adapts a pluginapi.VoicifyPlugin to a types.VoicifyPlugin
type pluginAdapter struct {
	plugin pluginapi.VoicifyPlugin
}

// NewPluginAdapter creates a new adapter for pluginapi.VoicifyPlugin
func NewPluginAdapter(plugin pluginapi.VoicifyPlugin) types.VoicifyPlugin {
	return &pluginAdapter{plugin: plugin}
}

// Initialize calls the Initialize method on the pluginapi plugin
func (a *pluginAdapter) Initialize() error {
	return a.plugin.Initialize()
}

// GetMetadata adapts the pluginapi.PluginMetadata to types.PluginMetadata
func (a *pluginAdapter) GetMetadata() types.PluginMetadata {
	apiMetadata := a.plugin.GetMetadata()
	return types.PluginMetadata{
		Name:        apiMetadata.Name,
		Version:     apiMetadata.Version,
		Description: apiMetadata.Description,
		Author:      apiMetadata.Author,
	}
}

// GetActions adapts the pluginapi.PluginAction slices to types.PluginAction
func (a *pluginAdapter) GetActions(transcription string) []types.PluginAction {
	apiActions := a.plugin.GetActions(transcription)
	typesActions := make([]types.PluginAction, len(apiActions))

	for i, apiAction := range apiActions {
		typesActions[i] = &actionAdapter{apiAction: apiAction}
	}

	return typesActions
}

// actionAdapter adapts a pluginapi.PluginAction to a types.PluginAction
type actionAdapter struct {
	apiAction pluginapi.PluginAction
}

// Execute calls the Execute method on the pluginapi action
func (a *actionAdapter) Execute(transcription string) error {
	return a.apiAction.Execute(transcription)
}

// GetMetadata adapts the pluginapi.ActionMetadata to types.ActionMetadata
func (a *actionAdapter) GetMetadata() types.ActionMetadata {
	apiMetadata := a.apiAction.GetMetadata()
	return types.ActionMetadata{
		Name:              apiMetadata.Name,
		Description:       apiMetadata.Description,
		LLMRouterPrompt:   apiMetadata.LLMRouterPrompt,
		SkipDefaultAction: apiMetadata.SkipDefaultAction,
		Priority:          apiMetadata.Priority,
	}
}
