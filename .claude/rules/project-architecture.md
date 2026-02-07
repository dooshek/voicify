---
description: Architektura projektu Voicify - komponenty, przepływ danych, struktura katalogów
---

# Architektura Voicify

## Komponenty

```
┌─────────────────────┐     D-Bus      ┌──────────────────────┐
│  GNOME Extension    │ ◄────────────► │    Go Backend        │
│  (JavaScript/GJS)   │                │    (cmd/voicify)     │
│                     │                │                      │
│  - UI (St widgets)  │                │  ┌─ audio/recorder   │
│  - Keyboard shorts  │                │  ├─ transcriber/     │
│  - State machine    │                │  ├─ llm/ (OpenAI)    │
│  - Visualization    │                │  ├─ tts/             │
│  - Clipboard paste  │                │  ├─ plugin/          │
└─────────────────────┘                │  ├─ clipboard/       │
                                       │  └─ notification/    │
                                       └──────────────────────┘
```

## Przepływ danych

1. Użytkownik naciska skrót klawiszowy (Ctrl+Super+V/C/D)
2. GNOME Extension przechwytuje shortcut via `grab_accelerator()`
3. Extension wywołuje metodę D-Bus na Go backend (`StartRecording`/`StopRecording`)
4. Go backend nagrywa audio (miniaudio via malgo)
5. Audio -> OpenAI Whisper (transkrypcja)
6. Tekst -> router/plugin lub bezpośrednio clipboard
7. Extension otrzymuje sygnał D-Bus z wynikiem
8. Tekst wklejony do aktywnego okna (clipboard + virtual keyboard na X11)

## Tryby pracy

| Tryb | Flag | Opis |
|------|------|------|
| Keyboard mode | (domyślny) | Standalone, monitoruje klawiaturę bezpośrednio |
| Daemon mode | `--daemon` | Serwer D-Bus, współpracuje z GNOME extension |

## Tryby nagrywania

| Tryb | Skrót | Przepływ |
|------|-------|----------|
| Realtime | Ctrl+Super+V | Stream audio -> OpenAI Realtime API -> tekst na bieżąco |
| Post-AutoPaste | Ctrl+Super+C | Nagraj -> Whisper -> clipboard + auto-paste |
| Post-Router | Ctrl+Super+D | Nagraj -> Whisper -> LLM router -> plugin -> akcja |

## Struktura katalogów

```
cmd/
  voicify/main.go          - Entry point, CLI, daemon/keyboard mode
internal/
  audio/                   - Nagrywanie audio (realtime + standard)
  clipboard/               - Operacje clipboard
  config/                  - Konfiguracja + wizard setup
  dbus/                    - Serwer D-Bus (com.dooshek.voicify.Recorder)
  fileops/                 - Operacje plikowe
  keyboard/                - Monitoring klawiszy (X11/Wayland)
  llm/                     - Providery LLM (OpenAI, Groq)
  logger/                  - Structured logging (zerolog)
  notification/            - Notyfikacje systemowe
  plugin/                  - System pluginów + konkretne pluginy
    linear/                - Plugin Linear (MCP client + agentic loop)
  state/                   - Globalny state management
  textprocessor/           - Przetwarzanie tekstu
  transcriber/             - Speech-to-text
  transcriptionrouter/     - Routing transkrypcji do pluginów
  tts/                     - Text-to-speech (OpenAI, Realtime)
  types/                   - Definicje typów, struktury konfiguracji
  windowdetect/            - Wykrywanie okien (X11/Wayland/Darwin)
pkg/
  pluginapi/               - Publiczny interfejs pluginów
  wav/                     - Konwersja WAV
gnome-extension/           - GNOME Shell extension
  extension.js             - Główny kod rozszerzenia
  metadata.json            - Metadane (UUID: voicify@dooshek.com)
  prefs.js                 - Okno preferencji
  schemas/                 - GSettings schemas
  stylesheet.css           - Style
```

## Konfiguracja

Plik: `~/.config/voicify/voicify.yaml`

Struktury w `internal/types/types.go`:
- `Config` - główna konfiguracja
- `KeyBinding` - skróty klawiszowe
- `LLMConfig` - API keys, modele, parametry
- `TTSConfig` - głos, prędkość, provider

## Zależności zewnętrzne

- OpenAI API (Whisper, GPT, TTS, Realtime)
- Groq API (alternatywny LLM)
- Linear MCP (via npx mcp-remote)
- D-Bus session bus
- PipeWire/PulseAudio (audio)
- miniaudio (via malgo)
- ffmpeg (konwersja audio)
