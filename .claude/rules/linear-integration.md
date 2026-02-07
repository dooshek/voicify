---
paths:
  - "internal/plugin/linear/**"
  - "internal/plugin/linear.go"
---

# Integracja Linear - Voicify

## Architektura

```
linear.go (plugin entry)
  └── linear/
      ├── mcp_client.go     - MCP protocol client (npx mcp-remote)
      └── agentic_loop.go   - LLM-driven conversation loop
```

## MCP Client (`mcp_client.go`)

### Asynchroniczna inicjalizacja
- Startuje w **background goroutine** (nie blokuje main)
- 3 próby z exponential backoff: 2s, 4s
- Timeout: 10s per próba
- Komunikuje się via `npx mcp-remote` (SSE transport)

### Graceful degradation
- Jeśli MCP nie zainicjalizuje się -> plugin niedostępny
- Jasne logowanie stanu
- Aplikacja kontynuuje bez Linear

```go
// Pattern:
go func() {
    for attempt := 0; attempt < 3; attempt++ {
        if err := client.Connect(); err != nil {
            logger.Errorf("MCP attempt %d failed", err, attempt+1)
            time.Sleep(backoff)
            backoff *= 2
            continue
        }
        // success
        return
    }
    logger.Error("MCP client failed after all attempts", lastErr)
}()
```

## Agentic Loop (`agentic_loop.go`)

### Lazy initialization
- Tworzona przy **pierwszym użyciu**, nie przy starcie pluginu
- Używa shared MCP client z global state

### Przepływ
1. Transkrypcja przychodzi z routera
2. LLM analizuje intencję (np. "stwórz issue w Linear")
3. Loop iteruje: LLM -> tool call -> MCP -> response -> LLM
4. Kończy gdy LLM ma finalną odpowiedź
5. Wynik -> clipboard/paste

### Konfiguracja
- Linear PAT (Personal Access Token) w `~/.config/voicify/voicify.yaml`
- Setup via wizard: `./bin/voicify --wizard`

## Ważne wzorce

- **NIE** inicjalizuj MCP synchronicznie w `Initialize()`
- **ZAWSZE** sprawdź czy MCP client jest gotowy przed użyciem
- **ZAWSZE** obsłuż timeout i retry
- Agentic loop może wykonać wiele iteracji - ustaw limit
