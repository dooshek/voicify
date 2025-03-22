# System Patterns

## Architecture Overview
Voicify uses a modular architecture with the following components:

```
voicify
├── cmd/            // Command-line entrypoints
├── internal/       // Internal packages not meant for external use
└── pkg/            // Public packages that could be imported by other applications
```

## Core Components
1. **Configuration (`config`)**: Manages user settings and setup wizard
2. **Keyboard (`keyboard`)**: Handles global keyboard shortcut registration and events
3. **Audio (`audio`)**: Manages recording, saving, and processing audio files
4. **Transcriber (`transcriber`)**: Integrates with OpenAI Whisper for transcription
5. **File Operations (`fileops`)**: Manages file-related operations
6. **Notification (`notification`)**: Provides system notifications
7. **Transcription Router (`transcriptionrouter`)**: Routes transcriptions to appropriate plugins

## Design Patterns

### Plugin Architecture
The application implements a plugin system through the `transcriptionrouter` package:

```
Transcription → Router → Action Plugins
                      ├→ VSCode Plugin
                      ├→ Email Plugin
                      └→ Other Plugins
```

Each plugin implements the `PluginAction` interface:
```go
type PluginAction interface {
    Execute(transcription string) error
    GetMetadata() ActionMetadata
}
```

### Configuration Management
- Uses YAML for configuration storage
- Configuration wizard for initial setup
- Environment variables for sensitive data (API keys)

### Command Pattern
- Uses flags package for command-line argument parsing
- Implements command pattern for different operations

### Observer Pattern
- Keyboard events trigger audio recording
- Transcription completion triggers notification and clipboard actions

## Data Flow
1. User triggers keyboard shortcut
2. Audio recording starts
3. User triggers shortcut again to stop recording
4. Audio is processed and sent to OpenAI Whisper
5. Transcription is received and processed by the router
6. Appropriate plugins execute actions based on transcription content
7. Result is copied to clipboard and optionally pasted

## Technical Decisions
1. Go language for cross-platform compatibility and performance
2. FFmpeg for audio processing
3. OpenAI Whisper for high-quality transcription
4. YAML for configuration for human readability
5. Modular design for extensibility
