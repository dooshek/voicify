package types

import (
	"github.com/dooshek/voicify/pkg/pluginapi"
)

// PluginMetadata contains information about a plugin
type PluginMetadata = pluginapi.PluginMetadata

// ActionMetadata contains information about an action
type ActionMetadata = pluginapi.ActionMetadata

// PluginAction represents an action provided by a plugin
type PluginAction = pluginapi.PluginAction

// VoicifyPlugin is the interface that all plugins must implement
type VoicifyPlugin = pluginapi.VoicifyPlugin
