package plugin

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/dooshek/voicify/pkg/pluginapi"
	"github.com/go-vgo/robotgo"
)

// Logger implementation for Linear plugin
type LinearLogger struct{}

var (
	currentLevel LogLevel  = LevelInfo
	output       io.Writer = os.Stdout
	mu           sync.Mutex
)

type LogLevel int

const (
	LevelDebug LogLevel = iota
	LevelInfo
	LevelWarn
	LevelError
)

// NewLinearLogger creates a new logger instance
func NewLinearLogger() *LinearLogger {
	return &LinearLogger{}
}

// Debug logs a debug message
func (l *LinearLogger) Debug(message string) {
	if currentLevel <= LevelDebug {
		writeLog("DEBUG", message)
	}
}

// Debugf logs a formatted debug message
func (l *LinearLogger) Debugf(format string, args ...interface{}) {
	if currentLevel <= LevelDebug {
		writeLog("DEBUG", fmt.Sprintf(format, args...))
	}
}

// Info logs an info message
func (l *LinearLogger) Info(message string) {
	if currentLevel <= LevelInfo {
		writeLog("INFO", message)
	}
}

// Infof logs a formatted info message
func (l *LinearLogger) Infof(format string, args ...interface{}) {
	if currentLevel <= LevelInfo {
		writeLog("INFO", fmt.Sprintf(format, args...))
	}
}

// Warn logs a warning message
func (l *LinearLogger) Warn(message string) {
	if currentLevel <= LevelWarn {
		writeLog("WARN", message)
	}
}

// Warnf logs a formatted warning message
func (l *LinearLogger) Warnf(format string, args ...interface{}) {
	if currentLevel <= LevelWarn {
		writeLog("WARN", fmt.Sprintf(format, args...))
	}
}

// Error logs an error message with an optional error
func (l *LinearLogger) Error(message string, err error) {
	if currentLevel <= LevelError {
		errMsg := message
		if err != nil {
			errMsg = fmt.Sprintf("%s: %v", message, err)
		}
		writeLog("ERROR", errMsg)
	}
}

// Errorf logs a formatted error message
func (l *LinearLogger) Errorf(format string, args ...interface{}) {
	if currentLevel <= LevelError {
		writeLog("ERROR", fmt.Sprintf(format, args...))
	}
}

// writeLog writes a log message with the specified level
func writeLog(level, message string) {
	mu.Lock()
	defer mu.Unlock()
	timestamp := time.Now().Format("15:04:05")
	fmt.Fprintf(output, "%s [%s] %s\n", timestamp, level, message)
}

// Window implementation for Linear plugin
type LinearWindow struct {
	Title string
}

// NewLinearWindow creates a new window instance
func NewLinearWindow() *LinearWindow {
	return &LinearWindow{}
}

// GetFocusedWindow gets the currently focused window
func (w *LinearWindow) GetFocusedWindow() (*LinearWindow, error) {
	// Get window ID
	windowID, err := exec.Command("xdotool", "getactivewindow").Output()
	if err != nil {
		return nil, err
	}

	// Get window name
	windowName, err := exec.Command("xdotool", "getwindowname", strings.TrimSpace(string(windowID))).Output()
	if err != nil {
		return nil, err
	}

	return &LinearWindow{
		Title: strings.TrimSpace(string(windowName)),
	}, nil
}

// Clipboard implementation for Linear plugin
type LinearClipboard struct{}

// NewLinearClipboard creates a new clipboard instance
func NewLinearClipboard() *LinearClipboard {
	return &LinearClipboard{}
}

// CopyToClipboard copies text to the clipboard
func (c *LinearClipboard) CopyToClipboard(text string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		// macOS
		cmd = exec.Command("pbcopy")
	case "linux":
		// Linux
		if _, err := exec.LookPath("xclip"); err != nil {
			return fmt.Errorf("xclip is not installed")
		}
		cmd = exec.Command("xclip", "-selection", "clipboard")
		pipeReader, pipeWriter := io.Pipe()
		cmd.Stdin = pipeReader

		go func() {
			defer pipeWriter.Close()
			pipeWriter.Write([]byte(text))
		}()
	}

	return cmd.Run()
}

// PasteWithReturn pastes text and adds a newline
func (c *LinearClipboard) PasteWithReturn(text string) error {
	// Copy text to clipboard
	if err := c.CopyToClipboard(text); err != nil {
		return err
	}

	if isX11() {
		// Use robotgo for X11
		robotgo.KeyTap("v", "ctrl")
		robotgo.KeyTap("enter")
	} else {
		// Wayland: Use XWayland compatibility layer (most reliable on Fedora)
		cmd := exec.Command("xdotool", "key", "ctrl+v")
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to paste: %v", err)
		}

		// Press Enter
		time.Sleep(50 * time.Millisecond)
		cmd = exec.Command("xdotool", "key", "Return")
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to press enter: %v", err)
		}
	}

	return nil
}

// isX11 checks if the current session is running X11
func isX11() bool {
	session := os.Getenv("XDG_SESSION_TYPE")
	return strings.ToLower(session) == "x11"
}

// LinearPlugin is a plugin for Linear
type LinearPlugin struct{}

// LinearAction is the Linear action
type LinearAction struct {
	transcription string
}

// Initialize initializes the Linear plugin
func (p *LinearPlugin) Initialize() error {
	linearLogger.Debug("Linear plugin initialized")
	return nil
}

// GetMetadata returns metadata about the plugin
func (p *LinearPlugin) GetMetadata() pluginapi.PluginMetadata {
	return pluginapi.PluginMetadata{
		Name:        "linear",
		Version:     "1.0.0",
		Description: "Plugin for Linear",
		Author:      "Voicify Team",
	}
}

// GetActions returns a list of actions provided by this plugin
func (p *LinearPlugin) GetActions(transcription string) []pluginapi.PluginAction {
	return []pluginapi.PluginAction{
		&LinearAction{transcription: transcription},
	}
}

// Execute executes the Linear action
func (a *LinearAction) Execute(transcription string) error {
	linearLogger.Debugf("Checking if Linear should execute action for transcription: %s", transcription)
	window := NewLinearWindow()
	focusedWindow, err := window.GetFocusedWindow()
	if err != nil {
		linearLogger.Error("Error getting focused window", err)
		return err
	}

	linearLogger.Debugf("Checking window title: %s", focusedWindow.Title)
	if !strings.Contains(focusedWindow.Title, "Linear") {
		linearLogger.Debug("Linear is not open, skipping action")
		return nil
	}

	clipboard := NewLinearClipboard()
	clipboard.PasteWithReturn(transcription)

	return nil
}

// GetMetadata returns metadata about the action
func (a *LinearAction) GetMetadata() pluginapi.ActionMetadata {
	return pluginapi.ActionMetadata{
		Name:        "linear",
		Description: "wykonanie akcji w Linear",
		Priority:    2,
	}
}

// NewLinearPlugin creates a new instance of the Linear plugin
func NewLinearPlugin() pluginapi.VoicifyPlugin {
	return &LinearPlugin{}
}

var linearLogger = NewLinearLogger()
