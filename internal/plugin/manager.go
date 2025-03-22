package plugin

import (
	"fmt"
	"os"
	"path/filepath"
	"plugin"
	"sync"

	"github.com/dooshek/voicify/internal/logger"
	"github.com/dooshek/voicify/internal/types"
)

// Manager handles plugin loading and management
type Manager struct {
	plugins     []types.VoicifyPlugin
	pluginsDir  string
	loadedPaths map[string]bool
	mu          sync.RWMutex
}

// NewManager creates a new plugin manager
func NewManager(pluginsDir string) *Manager {
	return &Manager{
		plugins:     make([]types.VoicifyPlugin, 0),
		pluginsDir:  pluginsDir,
		loadedPaths: make(map[string]bool),
	}
}

// LoadPlugins loads all plugins from the plugins directory
func (m *Manager) LoadPlugins() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Create plugins directory if it doesn't exist
	if err := os.MkdirAll(m.pluginsDir, 0o755); err != nil {
		return fmt.Errorf("failed to create plugins directory: %w", err)
	}

	// Get all plugin directories
	entries, err := os.ReadDir(m.pluginsDir)
	if err != nil {
		return fmt.Errorf("failed to read plugins directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		pluginPath := filepath.Join(m.pluginsDir, entry.Name())

		// Check if we've already loaded this plugin
		if m.loadedPaths[pluginPath] {
			continue
		}

		// Look for the main.so file in the plugin directory
		mainPath := filepath.Join(pluginPath, "main.so")
		if _, err := os.Stat(mainPath); os.IsNotExist(err) {
			logger.Errorf("Plugin %s missing main.so file", err, entry.Name())
			continue
		}

		// Load the plugin
		p, err := m.loadPlugin(mainPath)
		if err != nil {
			logger.Errorf("Failed to load plugin %s: %v", err, entry.Name())
			continue
		}

		// Initialize the plugin
		if err := p.Initialize(); err != nil {
			logger.Errorf("Failed to initialize plugin %s", err, entry.Name())
			continue
		}

		// Add the plugin to our list
		m.plugins = append(m.plugins, p)
		m.loadedPaths[pluginPath] = true

		metadata := p.GetMetadata()
		logger.Infof("Loaded plugin: %s v%s by %s", metadata.Name, metadata.Version, metadata.Author)
	}

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

	// Check if the symbol is of the correct type
	createFunc, ok := sym.(func() types.VoicifyPlugin)
	if !ok {
		return nil, fmt.Errorf("plugin 'CreatePlugin' has wrong type: %T", sym)
	}

	// Create the plugin instance
	return createFunc(), nil
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
