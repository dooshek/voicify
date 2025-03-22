# Voicify

Voicify is a command-line tool that enables voice recording and transcription with global keyboard shortcuts. Record your voice with a keystroke and have it automatically transcribed using LLMs like OpenAI Whisper. The tool can intelligently call actions (plugins) based on your voice input, allowing for seamless voice-controlled workflows.

## Features

- **Global Keyboard Shortcuts**: Start and stop recording with customizable keyboard shortcuts
- **Automatic Transcription**: Convert voice recordings to text using OpenAI Whisper
- **Clipboard Integration**: Automatically copy transcriptions to clipboard
- **Plugin Architecture**: Route transcriptions to different actions based on content
- **Cross-Platform**: Supports both X11 and Wayland display servers

## Installation

### Prerequisites

- Go 1.21 or higher
- FFmpeg installed and in PATH
- OpenAI API key
- Display server dependencies:
  - X11: `libx11-dev`, `libxtst-dev`, `libxkbcommon-dev`
  - Wayland: Input group membership (`sudo usermod -aG input $USER`)

### Building from source

```bash
# Clone the repository
git clone https://github.com/dooshek/voicify.git
cd voicify

# Build the binary
go build -o voicify ./cmd/voicify/main.go

```

## Configuration

Voicify stores its configuration in `$HOME/.config/voicify/voicify.yaml`. On first run, a configuration wizard will guide you through setup:

```bash
voicify --wizard
```

You'll need to provide:
- Your OpenAI API key
- Preferred keyboard shortcuts
- Plugin configurations

## Usage

```bash
# Start with default configuration
voicify

# Run the configuration wizard
voicify --wizard

# Set the logging level
voicify --log-level debug
```

### Basic Workflow

1. Press the configured keyboard shortcut to start recording
2. Speak clearly into your microphone
3. Press the shortcut again to stop recording
4. Wait for transcription to complete
5. The transcription will be copied to your clipboard and processed by any matching plugins

## Plugin System

Voicify includes a plugin architecture that routes transcriptions to appropriate actions based on content:

- **VSCode Plugin**: Execute commands in Visual Studio Code
- **Email Plugin**: Process email-related transcriptions
- More plugins coming soon!

Developers can create custom plugins by implementing the `PluginAction` interface:

```go
type PluginAction interface {
    Execute(transcription string) error
    GetMetadata() ActionMetadata
}
```

## Roadmap

- Web content plugin for saving articles to Obsidian
- Memory storage functionality with vector database integration
- Enhanced AI capabilities beyond basic transcription
- Additional specialized plugins

## License

[MIT License](LICENSE)

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.
