package plugin

import (
	"os"
	"strings"

	"github.com/dooshek/voicify/internal/clipboard"
	"github.com/dooshek/voicify/internal/logger"
	"github.com/dooshek/voicify/internal/windowdetect"
	"github.com/dooshek/voicify/pkg/pluginapi"
)

// GetFocusedWindow gets the currently focused window
func GetFocusedWindow() (*windowdetect.WindowInfo, error) {
	detector, err := windowdetect.New()
	if err != nil {
		return nil, err
	}
	return detector.GetFocusedWindow()
}

// CopyToClipboard copies text to the clipboard
func CopyToClipboard(text string) error {
	logger.Debugf("plugin: CopyToClipboard: %s", text)
	return clipboard.CopyToClipboard(text)
}

// PasteWithReturn pastes text and adds a newline
func PasteWithReturn(text string) error {
	logger.Debugf("plugin: PasteWithReturn: %s", text)
	return clipboard.PasteWithReturn(text)
}

// RequestPaste requests the extension to paste text via DBus (daemon mode only)
// This is the preferred method for pasting in post-transcription mode
// Falls back to clipboard.PasteWithReturn if DBus is not available
func RequestPaste(text string) error {
	logger.Debugf("plugin: RequestPaste: %s", text)

	// Try to use DBus RequestPaste if available
	err := pluginapi.RequestPaste(text)
	if err != nil {
		logger.Debugf("plugin: RequestPaste via DBus failed (%v), falling back to clipboard", err)
		// Fallback to clipboard method
		return clipboard.PasteWithReturn(text)
	}

	logger.Debugf("plugin: RequestPaste via DBus successful")
	return nil
}

// IsX11 checks if the current session is running X11
func IsX11() bool {
	session := os.Getenv("XDG_SESSION_TYPE")
	return strings.ToLower(session) == "x11"
}

// IsAppFocused checks if the specified app is currently focused
func IsAppFocused(appName string) bool {
	window, err := GetFocusedWindow()
	if err != nil {
		logger.Errorf("plugin: Error getting focused window: %v", err)
		return false
	}

	logger.Debugf("plugin: Checking if %s is focused, current window: %s", appName, window.Title)
	return strings.Contains(window.Title, appName)
}
