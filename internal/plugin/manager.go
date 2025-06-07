package plugin

import (
	"fmt"
	"os"
	"path/filepath"
	"plugin"
	"reflect"
	"sync"

	"github.com/dooshek/voicify/internal/logger"
	"github.com/dooshek/voicify/internal/types"
	"github.com/dooshek/voicify/pkg/pluginapi"
)

// Manager handles plugin loading and management
type Manager struct {
	plugins     []types.VoicifyPlugin
	pluginsDir  string
	loadedPaths map[string]bool
	mu          sync.RWMutex
}

// NewManager creates a new plugin managerls
func NewManager(pluginsDir string) *Manager {
	return &Manager{
		plugins:     make([]types.VoicifyPlugin, 0),
		pluginsDir:  pluginsDir,
		loadedPaths: make(map[string]bool),
	}
}

// LoadPlugins loads plugins from the plugins directory
func (m *Manager) LoadPlugins() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Ensure plugins directory exists
	if err := os.MkdirAll(m.pluginsDir, 0o755); err != nil {
		return fmt.Errorf("failed to create plugins directory: %w", err)
	}

	// Load plugins
	entries, err := os.ReadDir(m.pluginsDir)
	if err != nil {
		return fmt.Errorf("failed to read plugins directory: %w", err)
	}

	logger.Debugf("Scanning for plugins in %s", m.pluginsDir)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		pluginPath := filepath.Join(m.pluginsDir, entry.Name())
		mainSoPath := filepath.Join(pluginPath, "main.so")

		// Skip if plugin is already loaded
		if m.loadedPaths[mainSoPath] {
			logger.Debugf("Plugin already loaded: %s", mainSoPath)
			continue
		}

		// Skip if main.so doesn't exist
		if _, err := os.Stat(mainSoPath); os.IsNotExist(err) {
			logger.Debugf("Skipping directory %s (no main.so found)", entry.Name())
			continue
		}

		logger.Debugf("Loading plugin from: %s", mainSoPath)
		plugin, err := m.loadPlugin(mainSoPath)
		if err != nil {
			logger.Warnf("Failed to load plugin %s: %v", entry.Name(), err)
			continue
		}

		// Initialize the plugin
		if err := plugin.Initialize(); err != nil {
			logger.Warnf("Failed to initialize plugin %s: %v", entry.Name(), err)
			continue
		}

		// Add to loaded plugins
		m.plugins = append(m.plugins, plugin)
		m.loadedPaths[mainSoPath] = true

		// Log successful plugin load
		meta := plugin.GetMetadata()
		logger.Debugf("Loaded plugin: %s v%s by %s", meta.Name, meta.Version, meta.Author)
		logger.Infof("Using plugin %s", meta.Name)

		// Debug each action provided by the plugin
		dummyTranscription := "test transcription for checking actions"
		actions := plugin.GetActions(dummyTranscription)
		logger.Debugf("Plugin %s provides %d actions:", meta.Name, len(actions))
		for i, action := range actions {
			actionMeta := action.GetMetadata()
			var commands string
			if actionMeta.LLMCommands != nil {
				commands = fmt.Sprintf("LLM Commands: %v", *actionMeta.LLMCommands)
			} else {
				commands = "No LLM Commands"
			}
			logger.Debugf("  Action[%d]: %s - %s (%s)", i+1, actionMeta.Name, actionMeta.Description, commands)
		}
	}

	logger.Debugf("Loaded %d plugin(s)", len(m.plugins))
	return nil
}

// loadPlugin loads a single plugin from a .so file
func (m *Manager) loadPlugin(path string) (types.VoicifyPlugin, error) {
	// Open the plugin
	plug, err := plugin.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open plugin: %w", err)
	}

	// Look up the CreatePlugin symbol
	sym, err := plug.Lookup("CreatePlugin")
	if err != nil {
		return nil, fmt.Errorf("plugin does not export 'CreatePlugin' symbol: %w", err)
	}

	// Cast the symbol to a function
	switch createFunc := sym.(type) {
	case func() types.VoicifyPlugin:
		// If the plugin returns types.VoicifyPlugin directly, use it
		return createFunc(), nil
	case func() interface{}:
		// If the plugin returns interface{}, try to adapt it
		pluginInstance := createFunc()
		if plugin, ok := pluginInstance.(types.VoicifyPlugin); ok {
			return plugin, nil
		}
		// If it doesn't implement types.VoicifyPlugin, create an adapter
		return &pluginAdapter{plugin: pluginInstance}, nil
	default:
		// Try one more approach for pluginapi.VoicifyPlugin
		// This is needed because Go plugins are very strict about types
		createFuncRaw, ok := sym.(func() pluginapi.VoicifyPlugin)
		if !ok {
			return nil, fmt.Errorf("plugin 'CreatePlugin' has unsupported signature: %T", sym)
		}
		pluginInstance := createFuncRaw()
		return &pluginAdapter{plugin: pluginInstance}, nil
	}
}

// pluginAdapter adapts a pluginapi.VoicifyPlugin to a types.VoicifyPlugin
type pluginAdapter struct {
	plugin interface{}
}

// Initialize calls the Initialize method on the pluginapi plugin
func (a *pluginAdapter) Initialize() error {
	// Use reflection to call Initialize
	init := reflect.ValueOf(a.plugin).MethodByName("Initialize")
	results := init.Call(nil)
	if !results[0].IsNil() {
		return results[0].Interface().(error)
	}
	return nil
}

// GetMetadata adapts the pluginapi.PluginMetadata to types.PluginMetadata
func (a *pluginAdapter) GetMetadata() types.PluginMetadata {
	// Use reflection to call GetMetadata
	getMetadata := reflect.ValueOf(a.plugin).MethodByName("GetMetadata")
	results := getMetadata.Call(nil)
	apiMetadata := results[0].Interface()

	// Extract fields using reflection
	metadataValue := reflect.ValueOf(apiMetadata)

	return types.PluginMetadata{
		Name:        metadataValue.FieldByName("Name").String(),
		Version:     metadataValue.FieldByName("Version").String(),
		Description: metadataValue.FieldByName("Description").String(),
		Author:      metadataValue.FieldByName("Author").String(),
	}
}

// GetActions adapts the pluginapi.PluginAction slices to types.PluginAction
func (a *pluginAdapter) GetActions(transcription string) []types.PluginAction {
	// Use reflection to call GetActions
	getActions := reflect.ValueOf(a.plugin).MethodByName("GetActions")
	args := []reflect.Value{reflect.ValueOf(transcription)}
	results := getActions.Call(args)
	apiActions := results[0].Interface()

	// Convert to a slice of reflection values
	apiActionsValue := reflect.ValueOf(apiActions)
	length := apiActionsValue.Len()

	// Create a slice to hold the adapted actions
	typesActions := make([]types.PluginAction, length)

	// Adapt each action
	for i := 0; i < length; i++ {
		apiAction := apiActionsValue.Index(i).Interface()
		typesActions[i] = &actionAdapter{apiAction: apiAction}
	}

	return typesActions
}

// actionAdapter adapts a pluginapi.PluginAction to a types.PluginAction
type actionAdapter struct {
	apiAction interface{}
}

// Execute calls the Execute method on the pluginapi action
func (a *actionAdapter) Execute(transcription string) error {
	// Use reflection to call Execute
	actionName := a.GetMetadata().Name
	logger.Debugf("Plugin action %s: starting Execute with transcription: %s", actionName, transcription)

	execute := reflect.ValueOf(a.apiAction).MethodByName("Execute")
	args := []reflect.Value{reflect.ValueOf(transcription)}

	logger.Debugf("Plugin action %s: calling Execute method", actionName)
	results := execute.Call(args)

	if !results[0].IsNil() {
		err := results[0].Interface().(error)
		logger.Debugf("Plugin action %s: Execute returned error: %v", actionName, err)
		return err
	}

	logger.Debugf("Plugin action %s: Execute completed successfully", actionName)
	return nil
}

// GetMetadata adapts the pluginapi.ActionMetadata to types.ActionMetadata
func (a *actionAdapter) GetMetadata() types.ActionMetadata {
	// Use reflection to call GetMetadata
	getMetadata := reflect.ValueOf(a.apiAction).MethodByName("GetMetadata")
	results := getMetadata.Call(nil)
	apiMetadata := results[0].Interface()

	// Extract fields using reflection
	metadataValue := reflect.ValueOf(apiMetadata)

	// Handle LLMCommands which is a pointer
	var llmCommands *[]string
	llmField := metadataValue.FieldByName("LLMCommands")
	if !llmField.IsNil() {
		commands := llmField.Elem().Interface().([]string)
		llmCommands = &commands
	}

	return types.ActionMetadata{
		Name:        metadataValue.FieldByName("Name").String(),
		Description: metadataValue.FieldByName("Description").String(),
		LLMCommands: llmCommands,
		Priority:    int(metadataValue.FieldByName("Priority").Int()),
	}
}

// GetPlugins returns all loaded plugins
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
