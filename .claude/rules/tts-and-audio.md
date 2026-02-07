---
paths:
  - "internal/tts/**"
  - "internal/audio/**"
---

# TTS i Audio - Voicify

## Text-to-Speech

### Providery
- OpenAI TTS (`internal/tts/`)
- OpenAI Realtime API
- ElevenLabs (opcjonalny)

### Voice mapping i płeć

| Voice | Płeć | Polskie formy czasowników |
|-------|------|--------------------------|
| alloy | kobieta | "zrobiła", "powiedziała" |
| echo | mężczyzna | "zrobił", "powiedział" |
| fable | mężczyzna | "zrobił", "powiedział" |
| onyx | mężczyzna | "zrobił", "powiedział" |
| nova | kobieta | "zrobiła", "powiedziała" |
| shimmer | kobieta | "zrobiła", "powiedziała" |

### System prompt template

Template z `%s` placeholder na tekst. Instrukcja: "Mów wyraźnie i szybko" dla szybszej mowy.

### Konfiguracja (internal/types/types.go)

```go
type TTSConfig struct {
    Provider string  // "openai", "realtime", "elevenlabs"
    Voice    string  // "alloy", "echo", ...
    Speed    float64
    Prompt   string  // system prompt template
}
```

## Audio Recording

### Dwa tryby nagrywania

1. **Standard** (`internal/audio/recorder.go`)
   - Nagrywa do pliku, wysyła po zakończeniu
   - Używany w Post-AutoPaste i Post-Router

2. **Realtime** (`internal/audio/realtime_recorder.go`)
   - Streaming audio do OpenAI Realtime API
   - Tekst pojawia się na bieżąco
   - Używany w trybie Realtime

### Technologie
- **miniaudio** (via go-malgo) - cross-platform audio capture
- **ffmpeg** (via ffmpeg-go) - konwersja formatów audio
- **WAV** (pkg/wav/) - utiliy do konwersji WAV

### Ważne

- Audio wymaga uprawnień: grupa `audio`, `pipewire`, `realtime`
- PipeWire/PulseAudio jako backend
- Realtime API wymaga WebSocket connection
