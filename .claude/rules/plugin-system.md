---
paths:
  - "internal/plugin/**"
  - "pkg/pluginapi/**"
---

# System pluginów - Voicify

## Interfejs (pkg/pluginapi/types.go)

```go
type VoicifyPlugin interface {
    Initialize() error
    GetMetadata() PluginMetadata
    GetActions(transcription string) []PluginAction
}

type PluginAction interface {
    Execute(transcription string) error
    GetMetadata() ActionMetadata
}

type PluginMetadata struct {
    Name, Version, Description, Author string
}

type ActionMetadata struct {
    Name            string
    Description     string
    LLMRouterPrompt *string   // prompt dla routera LLM
    SkipDefaultAction bool
    Priority        int
}
```

## Dodatkowe interfejsy pluginapi

- `pluginapi/clipboard.go` - operacje clipboard
- `pluginapi/paste.go` - RequestPaste()
- `pluginapi/window.go` - wykrywanie okien
- `pluginapi/logger.go` - logger dla pluginów

## Struktura plików pluginu

**Prosty plugin:**
```
internal/plugin/myplugin.go
```

**Złożony plugin (np. Linear):**
```
internal/plugin/linear.go          - główny plik, rejestracja
internal/plugin/linear/
    mcp_client.go                  - MCP client
    agentic_loop.go                - pętla agentyczna LLM
```

## Rejestracja pluginu

Plugin musi być zarejestrowany w registry (`internal/plugin/`). Pattern:

```go
func NewMyPlugin(deps Dependencies) *MyPlugin {
    return &MyPlugin{deps: deps}
}

func (p *MyPlugin) Initialize() error {
    // setup, może być lazy
    return nil
}

func (p *MyPlugin) GetMetadata() pluginapi.PluginMetadata {
    return pluginapi.PluginMetadata{
        Name:    "my-plugin",
        Version: "1.0.0",
    }
}

func (p *MyPlugin) GetActions(transcription string) []pluginapi.PluginAction {
    return []pluginapi.PluginAction{&MyAction{}}
}
```

## Istniejące pluginy

- **Default** - domyślna akcja (clipboard paste)
- **Linear** - integracja z Linear (MCP + agentic loop)
- **VSCode** - integracja z VS Code

## Wzorce

- **Lazy init**: ciężkie zasoby inicjalizuj przy pierwszym użyciu
- **Graceful degradation**: jeśli zewnętrzny serwis niedostępny, loguj i kontynuuj
- **RequestPaste()**: plugin może poprosić o wklejenie tekstu do aktywnego okna
