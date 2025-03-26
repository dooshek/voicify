# Technical Context

## Technology Stack
- **Language**: Go 1.21+
- **Audio Processing**: FFmpeg
- **Machine Learning**: OpenAI Whisper API
- **Input Handling**: X11 and Wayland libraries
- **Configuration**: YAML

## Development Environment
- **Build System**: Go modules
- **Testing Framework**: Go testing package
- **CI/CD**: Not specified yet
- **Version Control**: Git

## Dependencies
- **External Services**:
  - OpenAI API for transcription
- **System Dependencies**:
  - FFmpeg for audio processing
  - X11: libx11-dev, libxtst-dev, libxkbcommon-dev
  - Wayland: Input group membership

## Technical Constraints
- Must work on both X11 and Wayland display servers
- Requires appropriate permissions for global keyboard shortcuts
- Needs internet connectivity for transcription service
- Depends on external API availability and rate limits

## Code Organization
- **cmd/**: Application entry points
- **internal/**: Internal packages
  - **audio/**: Audio recording and processing
  - **config/**: Configuration management
  - **input/**: Input handling for different display servers
  - **transcription/**: Transcription service integration
  - **plugin/**: Plugin system and implementations
- **pkg/**: Reusable packages
- **plugins/**: Plugin implementations

## Build and Deployment
- **Build Command**: `go build -o voicify ./cmd/voicify/main.go`
- **Installation**: Manual build from source
- **Configuration**: `$HOME/.config/voicify/voicify.yaml`

## Security Considerations
- API key storage in configuration
- System permissions for input handling
- Audio data transmission security

## Extensibility Points
- Plugin interface for custom actions
- Configuration options for user preferences
- Potential for additional transcription services
