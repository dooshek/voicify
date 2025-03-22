# Project Brief: Voicify

## Overview
Voicify is a command-line tool that enables voice recording and transcription with global keyboard shortcuts. It allows users to quickly record audio and have it automatically transcribed using OpenAI Whisper.

## Core Goals
1. Provide effortless voice-to-text capabilities through global keyboard shortcuts
2. Transcribe voice recordings accurately using OpenAI Whisper
3. Support both X11 and Wayland display servers
4. Offer configurable key bindings for user customization
5. Automatically copy transcriptions to clipboard for seamless workflow integration
6. Enable plugin-based architecture for extending functionality based on transcription content

## Target Users
- Developers and power users who want to quickly capture thoughts or commands
- Users who prefer voice input over typing for certain tasks
- Anyone looking to integrate voice commands into their workflow

## Success Criteria
- Global keyboard shortcuts work reliably across X11 and Wayland
- Transcription is accurate and fast
- Configuration is straightforward and persistent
- Plugin system effectively routes transcriptions to appropriate actions

## Constraints
- Requires FFmpeg for audio processing
- Requires OpenAI API key for transcription
- Has specific system dependencies for keyboard shortcut functionality
