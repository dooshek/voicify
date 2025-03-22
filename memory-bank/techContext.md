# Technical Context

## Technologies
- **Language**: Go 1.21+
- **Audio Processing**: FFmpeg
- **Transcription**: OpenAI Whisper API
- **Configuration**: YAML
- **Logging**: Structured logging with color support
- **Input Handling**:
  - X11: x11 package dependencies
  - Wayland: input group permissions

## Development Environment
- Go development environment
- Required system packages:
  - For X11: `libx11-dev`, `libxtst-dev`, `libxkbcommon-dev`
  - For Wayland: Input group membership (`sudo usermod -aG input $USER`)
- FFmpeg installed and in PATH
- OpenAI API key configured in `.env` or environment variables

## Build System
- Standard Go build toolchain
- Simple build command: `go build -o voicify ./cmd/voicify/main.go`
- No complex build scripts or makefiles identified

## Deployment
- Binary executable distributed as `voicify`
- Configuration stored in `$HOME/.config/voicify/voicify.yaml`
- Recordings saved to `$HOME/.config/voicify/recordings/`

## Dependencies
- **External APIs**:
  - OpenAI Whisper for transcription
- **System Dependencies**:
  - FFmpeg
  - X11 or Wayland development libraries
- **Go Libraries**: (Inferred from README and project structure)
  - Keyboard handling libraries
  - YAML parsing libraries
  - Audio processing libraries
  - HTTP clients for API interaction

## Technical Constraints
- Requires proper system permissions for global keyboard shortcuts
- OpenAI API key required for transcription functionality
- FFmpeg must be properly installed for audio processing
- Different system requirements based on display server (X11 vs Wayland)

## Performance Considerations
- Audio file size and transcription time
- Keyboard shortcut response time
- API rate limits for transcription service

## Security Considerations
- API key storage and protection
- Audio file management and retention
- Permission requirements for input monitoring
