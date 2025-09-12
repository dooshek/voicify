package pluginapi

// PluginMetadata contains information about a plugin
type PluginMetadata struct {
	Name        string
	Version     string
	Description string
	Author      string
}

// ActionMetadata contains information about an action
type ActionMetadata struct {
	Name            string
	Description     string
	LLMRouterPrompt *string
	SkipDefaultAction bool
	Priority        int
}

// PluginAction represents an action provided by a plugin
type PluginAction interface {
	Execute(transcription string) error
	GetMetadata() ActionMetadata
}

// VoicifyPlugin is the interface that all plugins must implement
type VoicifyPlugin interface {
	Initialize() error
	GetMetadata() PluginMetadata
	GetActions(transcription string) []PluginAction
}
