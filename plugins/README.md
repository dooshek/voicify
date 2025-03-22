# Voicify Plugin Development Guide

This guide explains how to create plugins for Voicify.

## Plugin Structure

Each plugin should be in its own directory under `plugins/`. For example:

```
plugins/
  vscode/
    main.go
  email/
    main.go
  myplugin/
    main.go
```

## Plugin Interface

All plugins must implement the `VoicifyPlugin` interface from `github.com/dooshek/voicify/internal/types`:

```go
type VoicifyPlugin interface {
	// Initialize is called when the plugin is loaded
	Initialize() error

	// GetActions returns a list of actions provided by this plugin
	GetActions(transcription string) []types.PluginAction

	// GetMetadata returns metadata about the plugin
	GetMetadata() types.PluginMetadata
}
```

## Plugin Metadata

Plugins should provide metadata through the `GetMetadata()` method:

```go
func (p *MyPlugin) GetMetadata() types.PluginMetadata {
	return types.PluginMetadata{
		Name:        "myplugin",
		Version:     "1.0.0",
		Description: "My awesome plugin",
		Author:      "Your Name",
	}
}
```

## Actions

Plugins can provide one or more actions. Each action must implement the `PluginAction` interface:

```go
type PluginAction interface {
	Execute(transcription string) error
	GetMetadata() types.ActionMetadata
}
```

## Action Metadata

Actions should provide metadata through the `GetMetadata()` method:

```go
func (a *MyPluginAction) GetMetadata() types.ActionMetadata {
	return types.ActionMetadata{
		Name:        "myaction",
		Description: "My awesome action",
		LLMCommands: &[]string{
			"command1",
			"command2",
			"command3",
		},
		Priority: 1,
	}
}
```

- `Name`: The name of the action
- `Description`: A description of what the action does
- `LLMCommands`: A list of commands that will trigger this action via LLM
- `Priority`: The priority of the action (higher priority = more important)

## Building a Plugin

To build a plugin, you need to:

1. Create a Go package in a subdirectory of `plugins/`
2. Implement the `VoicifyPlugin` interface
3. Export a `CreatePlugin()` function that returns a new instance of your plugin
4. Build the plugin as a shared object (`*.so`) file

### Example Plugin

```go
package main

import (
	"github.com/dooshek/voicify/internal/logger"
	"github.com/dooshek/voicify/internal/types"
)

type MyPlugin struct{}

func (p *MyPlugin) Initialize() error {
	logger.Info("MyPlugin initialized")
	return nil
}

func (p *MyPlugin) GetMetadata() types.PluginMetadata {
	return types.PluginMetadata{
		Name:        "myplugin",
		Version:     "1.0.0",
		Description: "My awesome plugin",
		Author:      "Your Name",
	}
}

func (p *MyPlugin) GetActions(transcription string) []types.PluginAction {
	return []types.PluginAction{
		&MyPluginAction{transcription: transcription},
	}
}

type MyPluginAction struct {
	transcription string
}

func (a *MyPluginAction) Execute(transcription string) error {
	logger.Infof("MyPluginAction executed with: %s", transcription)
	return nil
}

func (a *MyPluginAction) GetMetadata() types.ActionMetadata {
	return types.ActionMetadata{
		Name:        "myaction",
		Description: "My awesome action",
		LLMCommands: &[]string{
			"do my action",
			"execute my action",
		},
		Priority: 1,
	}
}

// CreatePlugin creates a new instance of the plugin
// This function is loaded by the plugin manager
func CreatePlugin() types.VoicifyPlugin {
	return &MyPlugin{}
}

// This is required for Go plugins
var (
	// Export the plugin creation function
	_ = CreatePlugin
)
```

### Building the Plugin

To build your plugin, run:

```bash
cd plugins/myplugin
go build -buildmode=plugin -o main.so main.go
```

## Loading Plugins

Plugins are automatically loaded when Voicify starts. The plugin manager:

1. Scans the `plugins/` directory for plugin directories
2. Loads each plugin's `main.so` file
3. Calls `CreatePlugin()` to get a new instance of the plugin
4. Calls `Initialize()` on the plugin
5. Adds the plugin's actions to the router

## Debugging Plugins

Plugin load errors are logged to the Voicify log file. Check the logs if your plugin isn't loading correctly.
