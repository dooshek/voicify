package notification

import (
	"fmt"
	"os/exec"

	"github.com/dooshek/voicify/internal/logger"
)

type darwinNotifier struct{}

func newDarwinNotifier() platformNotifier {
	return &darwinNotifier{}
}

func (n *darwinNotifier) send(title, message string) error {
	logger.Debugf("Sending macOS notification: %s - %s", title, message)
	script := fmt.Sprintf(`display notification "%s" with title "%s"`, message, title)
	cmd := exec.Command("osascript", "-e", script)
	if err := cmd.Run(); err != nil {
		logger.Errorf("Failed to send macOS notification: %v", err)
		return err
	}
	logger.Debug("Successfully sent macOS notification")
	return nil
}

func (n *darwinNotifier) playStartBeep() error {
	logger.Debug("Playing start beep sound")
	cmd := exec.Command("afplay", "/System/Library/Sounds/Ping.aiff")
	if err := cmd.Run(); err != nil {
		logger.Errorf("Failed to play start beep: %v", err)
		return err
	}
	return nil
}

func (n *darwinNotifier) playStopBeep() error {
	logger.Debug("Playing stop beep sound")
	cmd := exec.Command("afplay", "/System/Library/Sounds/Basso.aiff")
	if err := cmd.Run(); err != nil {
		logger.Errorf("Failed to play stop beep: %v", err)
		return err
	}
	return nil
}

func (n *darwinNotifier) playProgressBeep() error {
	cmd := exec.Command("afplay", "/System/Library/Sounds/Tink.aiff")
	return cmd.Run()
}

func (n *darwinNotifier) playTranscriptionOverBeep() error {
	cmd := exec.Command("afplay", "/System/Library/Sounds/Glass.aiff")
	return cmd.Run()
}
