package notification

import (
	"fmt"
	"runtime"

	"github.com/dooshek/voicify/internal/logger"
)

const defaultIcon = ""

// Notifier defines the interface for system notifications
type Notifier interface {
	NotifyRecordingStarted() error
	NotifyTranscribing() error
	NotifyTranscriptionComplete() error
	Notify(title, message string) error
	PlayStartBeep() error
	PlayStopBeep() error
	PlayProgressBeep() error
	PlayTranscriptionOverBeep() error
}

type baseNotifier struct {
	platform platformNotifier
}

type platformNotifier interface {
	send(title, message string) error
	playStartBeep() error
	playStopBeep() error
	playProgressBeep() error
	playTranscriptionOverBeep() error
}

// New creates a new platform-specific notification service
func New() Notifier {
	logger.Debug("Initializing notification system")
	var platform platformNotifier
	switch runtime.GOOS {
	case "darwin":
		logger.Debug("Using Darwin (macOS) notifier")
		platform = newDarwinNotifier()
	default:
		logger.Debug("Using Linux notifier")
		platform = newLinuxNotifier()
	}
	return &baseNotifier{platform: platform}
}

// Common implementation for all platforms
func (n *baseNotifier) NotifyRecordingStarted() error {
	logger.Debug("Sending recording started notification")
	return n.Notify("Audio Recorder", "Recording in progress...")
}

func (n *baseNotifier) NotifyTranscribing() error {
	logger.Debug("Sending transcribing notification")
	return n.Notify("Audio Recorder", "Transcribing...")
}

func (n *baseNotifier) NotifyTranscriptionComplete() error {
	return n.Notify("Audio Recorder", "Complete!")
}

func (n *baseNotifier) Notify(title, message string) error {
	return n.platform.send(title, message)
}

func (n *baseNotifier) PlayStartBeep() error {
	return n.platform.playStartBeep()
}

func (n *baseNotifier) PlayStopBeep() error {
	return n.platform.playStopBeep()
}

func (n *baseNotifier) PlayProgressBeep() error {
	return n.platform.playProgressBeep()
}

func (n *baseNotifier) PlayTranscriptionOverBeep() error {
	return n.platform.playTranscriptionOverBeep()
}

// formatProgressMessage formats the recording progress message
func formatProgressMessage(minutes, seconds int) string {
	return fmt.Sprintf("Recording in progress... %02d:%02d", minutes, seconds)
}
