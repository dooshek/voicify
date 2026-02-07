---
paths:
  - "**/*.go"
---

# Konwencje Go - Voicify

## KRYTYCZNE: Sygnatury loggera

Pakiet `internal/logger/` opakowuje zerolog. Sygnatury są **niestandardowe** - błąd jest DRUGIM argumentem w Error/Errorf:

```go
// Bez error:
logger.Debug(msg string)
logger.Debugf(format string, v ...interface{})
logger.Info(msg string)
logger.Infof(format string, v ...interface{})
logger.Warn(msg string)
logger.Warnf(format string, v ...interface{})

// Z error - error jest ZAWSZE 2. argumentem:
logger.Error(msg string, err error)
logger.Errorf(format string, err error, v ...interface{})
```

### Częste błędy - NIE RÓB TEGO:

```go
// BŁĄD: brak error
logger.Error("coś poszło nie tak")

// BŁĄD: error na końcu (jak fmt.Printf)
logger.Errorf("failed to do %s: %v", name, err)

// DOBRZE:
logger.Error("coś poszło nie tak", err)
logger.Errorf("failed to do %s", err, name)
```

## Error handling

```go
// Propaguj błędy z kontekstem
if err != nil {
    return fmt.Errorf("operation X failed: %w", err)
}

// Loguj na granicy (handler, main loop)
if err := doSomething(); err != nil {
    logger.Error("failed to do something", err)
}
```

## Formatowanie i narzędzia

- **Formatter**: `goimports` (~/go/bin/goimports) - auto via hook po edycji
- **Linter**: `staticcheck` (~/go/bin/staticcheck)
- **Vet**: `go vet ./...`

## Nazewnictwo

- Pakiety: lowercase, bez podkreśleń (`pluginapi`, `windowdetect`)
- Interfejsy: rzeczownikowe (`VoicifyPlugin`, `PluginAction`)
- Errors: `fmt.Errorf("context: %w", err)`
- Pliki: snake_case (`mcp_client.go`, `agentic_loop.go`)

## Moduł

- Ścieżka: `github.com/dooshek/voicify`
- Go: 1.23.7
- Wersja: v0.2.2

## Wzorce w projekcie

### Graceful degradation
Zewnętrzne serwisy (MCP, OpenAI) mogą nie być dostępne. Loguj błąd, ale nie crashuj:
```go
if err := initExternalService(); err != nil {
    logger.Error("service unavailable, degrading gracefully", err)
    // kontynuuj bez serwisu
}
```

### Lazy initialization
Ciężkie zasoby inicjalizuj przy pierwszym użyciu, nie przy starcie:
```go
func (p *Plugin) ensureInitialized() error {
    if p.client != nil {
        return nil
    }
    // init...
}
```

### Context i shutdown
Main używa `os.Signal` do graceful shutdown. Pluginy powinny respektować context cancellation.
