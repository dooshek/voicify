package plugin

import (
	"fmt"
	"sync"

	"github.com/dooshek/voicify/internal/logger"
	"github.com/dooshek/voicify/internal/types"
)

// Manager handles plugin registration and management
type Manager struct {
	plugins []types.VoicifyPlugin
	mu      sync.RWMutex
}

// NewManager creates a new plugin manager
func NewManager() *Manager {
	return &Manager{
		plugins: make([]types.VoicifyPlugin, 0),
	}
}

// RegisterPlugin registers a plugin with the manager
func (m *Manager) RegisterPlugin(plugin types.VoicifyPlugin) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Initialize the plugin
	if err := plugin.Initialize(); err != nil {
		logger.Warnf("Failed to initialize plugin: %v", err)
		return err
	}

	// Add to registered plugins
	m.plugins = append(m.plugins, plugin)

	// Log successful plugin registration
	meta := plugin.GetMetadata()
	logger.Debugf("Registered plugin: %s v%s by %s", meta.Name, meta.Version, meta.Author)
	logger.Infof("Using plugin %s", meta.Name)

	// Debug each action provided by the plugin
	dummyTranscription := "test transcription for checking actions"
	actions := plugin.GetActions(dummyTranscription)
	logger.Debugf("Plugin %s provides %d actions:", meta.Name, len(actions))
	for i, action := range actions {
		actionMeta := action.GetMetadata()
		var prompt string
		if actionMeta.LLMRouterPrompt != nil {
			prompt = fmt.Sprintf("LLM Router Prompt: %s", *actionMeta.LLMRouterPrompt)
		} else {
			prompt = "No LLM Router Prompt"
		}
		logger.Debugf("  Action[%d]: %s - %s (%s)", i+1, actionMeta.Name, actionMeta.Description, prompt)
	}

	return nil
}

// GetPlugins returns all registered plugins
func (m *Manager) GetPlugins() []types.VoicifyPlugin {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]types.VoicifyPlugin, len(m.plugins))
	copy(result, m.plugins)
	return result
}

// GetAllActions returns all actions from all plugins for a given transcription
func (m *Manager) GetAllActions(transcription string) []types.PluginAction {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var allActions []types.PluginAction

	for _, p := range m.plugins {
		actions := p.GetActions(transcription)
		allActions = append(allActions, actions...)
	}

	return allActions
}
